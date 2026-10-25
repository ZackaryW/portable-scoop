package runner_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portscoop/internal/delegation"
	"portscoop/internal/environment"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

const recordingEntry = `
$ErrorActionPreference = 'Stop'
. "$PSScriptRoot\request-reader.ps1"
$request = Read-PortscoopRequest
$data = @{ arguments = @($request.arguments); cwd = (Get-Location).ProviderPath; scoop = $env:SCOOP } | ConvertTo-Json -Depth 6
[IO.File]::WriteAllText((Join-Path $env:PORTSCOOP_ROOT 'received.json'), $data, (New-Object Text.UTF8Encoding($false)))
`

func transportFixture(t *testing.T) environment.Context {
	t.Helper()
	root := filepath.Join(testutil.TempDir(t), "portable (work)")
	id, err := environment.NewIdentity("work")
	if err != nil {
		t.Fatal(err)
	}
	testutil.WriteIdentity(t, root, id)
	paths, err := environment.PathsAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(paths.Adapter), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(paths.Adapter, []byte(recordingEntry), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(paths.Adapter), "request-reader.ps1"), []byte(runner.RequestReader()), 0600); err != nil {
		t.Fatal(err)
	}
	cwd := filepath.Join(testutil.TempDir(t), "caller (directory)")
	if err := os.MkdirAll(cwd, 0700); err != nil {
		t.Fatal(err)
	}
	return environment.Context{Identity: id, Paths: paths, WorkingDirectory: cwd}
}

type delegatedTransport struct{ local environment.Context }

func (d delegatedTransport) Run(ctx context.Context, command runner.Command) (int, error) {
	if command.Arguments[0] == delegation.HandshakeCommand {
		data, err := delegation.EncodeHello(d.local, "fixture")
		if err != nil {
			return 0, err
		}
		_, err = command.Streams.Out.Write(data)
		return 0, err
	}
	local, err := delegation.Local(nil, filepath.Join(d.local.Paths.Root, "portscoop.exe"), command.Arguments[1], 1)
	if err != nil {
		return 0, err
	}
	local.WorkingDirectory = command.Directory
	prepared, cleanup, err := runner.PreparePowerShell(local, command.Arguments[4:], os.Environ(), command.Streams)
	if err != nil {
		return 0, err
	}
	defer cleanup()
	process := exec.CommandContext(ctx, prepared.Executable, prepared.Arguments...)
	process.Env = prepared.Environment
	process.Dir = prepared.Directory
	if output, err := process.CombinedOutput(); err != nil {
		return 0, fmt.Errorf("delegated transport: %w: %s", err, output)
	}
	return 0, nil
}

func TestDelegationToPowerShellPreservesArgumentVector(t *testing.T) {
	local := transportFixture(t)
	args := []string{"search", "", `quoted " value`, "π 你好", `$(); & |`}
	_, err := (delegation.Client{Runner: delegatedTransport{local}}).Forward(context.Background(), resolver.Target{Identity: local.Identity, Paths: local.Paths}, args, local.WorkingDirectory, runner.Streams{})
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(local.Paths.Root, "received.json"))
	if err != nil {
		t.Fatal(err)
	}
	var received struct {
		Arguments []string `json:"arguments"`
	}
	if err := json.Unmarshal(data, &received); err != nil || !reflect.DeepEqual(received.Arguments, args) {
		t.Fatalf("delegated values changed: %+v %v", received, err)
	}
}

func TestPowerShell51JSONArgumentRoundTrip(t *testing.T) {
	local := transportFixture(t)
	args := []string{"search", "", `quotes " and spaces`, "Unicode: π 你好", `$(Write-Output NOT-EVALUATED); & | < >`}
	command, cleanup, err := runner.PreparePowerShell(local, args, os.Environ(), runner.Streams{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	for _, arg := range args[1:] {
		if arg != "" && strings.Contains(strings.Join(command.Arguments, " "), arg) {
			t.Fatal("user data was put on PowerShell command line")
		}
	}
	process := exec.Command(command.Executable, command.Arguments...)
	process.Env = command.Environment
	process.Dir = command.Directory
	if output, err := process.CombinedOutput(); err != nil {
		t.Fatalf("PowerShell transport failed: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(local.Paths.Root, "received.json"))
	if err != nil {
		t.Fatal(err)
	}
	var received struct {
		Arguments []string `json:"arguments"`
		Cwd       string   `json:"cwd"`
		Scoop     string   `json:"scoop"`
	}
	if err := json.Unmarshal(data, &received); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(received.Arguments, args) || received.Cwd != local.WorkingDirectory || received.Scoop != local.Paths.Scoop {
		t.Fatalf("argument/context mismatch: %+v", received)
	}
	cleanup()
	files, _ := filepath.Glob(filepath.Join(local.Paths.Temp, "portscoop-request-*.json"))
	if len(files) != 0 {
		t.Fatal("request transport was not cleaned")
	}
}

func TestTransportRechecksIdentityBeforeReadingArguments(t *testing.T) {
	local := transportFixture(t)
	command, cleanup, err := runner.PreparePowerShell(local, []string{"list"}, os.Environ(), runner.Streams{})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	replacement, _ := environment.NewIdentity("replacement")
	testutil.WriteIdentity(t, local.Paths.Root, replacement)
	process := exec.Command(command.Executable, command.Arguments...)
	process.Env = command.Environment
	process.Dir = command.Directory
	if err := process.Run(); err == nil {
		t.Fatal("backend accepted changed identity")
	}
	if _, err := os.Stat(filepath.Join(local.Paths.Root, "received.json")); !os.IsNotExist(err) {
		t.Fatal("recording backend executed after identity mismatch")
	}
}
