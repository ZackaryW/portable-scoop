package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"portscoop/internal/testutil"
)

func TestPowerShellTypedArgumentsNamedParametersInputExitAndCleanup(t *testing.T) {
	local := envFixture(t, map[string]string{})
	target := filepath.Join(local.Paths.Root, "fixture.ps1")
	source := `param([string]$Name, [switch]$Flag)
$result = @{ name = $Name; flag = [bool]$Flag; arguments = @($args); input = [Console]::In.ReadToEnd(); cwd = (Get-Location).ProviderPath; root = $PSScriptRoot }
[IO.File]::WriteAllText((Join-Path $env:PORTSCOOP_ROOT 'ps-result.json'), ($result | ConvertTo-Json -Compress -Depth 5), [Text.UTF8Encoding]::new($false))
[Console]::Out.WriteLine('stdout')
[Console]::Error.WriteLine('stderr')
exit 37
`
	if err := os.WriteFile(target, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	child, err := BuildEnvironment(local, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"-Name", "雪 π", "-Flag", "", `quoted " value`, `$(throw 'data'); & | < >`, "--env=app"}
	var out, diag bytes.Buffer
	dir := testutil.TempDir(t)
	p, err := startPowerShell(context.Background(), Native{}, local, target, Command{Args: args, Environment: child, Directory: dir, Mode: Wait, Streams: Streams{In: strings.NewReader("raw input"), Out: &out, Err: &diag}})
	if err != nil {
		t.Fatalf("%v stderr=%s", err, &diag)
	}
	code, err := p.Wait(context.Background())
	if err != nil || code != 37 {
		t.Fatalf("status=%d err=%v stderr=%s", code, err, &diag)
	}
	data, err := os.ReadFile(filepath.Join(local.Paths.Root, "ps-result.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Name             string
		Flag             bool
		Arguments        []string
		Input, Cwd, Root string
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Name != "雪 π" || !got.Flag || !reflect.DeepEqual(got.Arguments, args[3:]) || got.Input != "raw input" || got.Cwd != dir || got.Root != local.Paths.Root || !strings.Contains(out.String(), "stdout") || !strings.Contains(diag.String(), "stderr") {
		t.Fatalf("got=%+v stdout=%s stderr=%s", got, &out, &diag)
	}
	entries, _ := os.ReadDir(local.Paths.Temp)
	if len(entries) != 0 {
		t.Fatalf("transport files retained: %v", entries)
	}
}

func TestPowerShellDetachedConsumesPayloadAndSurvives(t *testing.T) {
	local := envFixture(t, map[string]string{})
	target := filepath.Join(local.Paths.Root, "detached.ps1")
	source := `[IO.File]::WriteAllText((Join-Path $env:PORTSCOOP_ROOT 'ps-detached.json'), (@{arguments=@($args);pid=$PID;control=$env:PORTSCOOP_PS_CONTROL} | ConvertTo-Json -Compress), [Text.UTF8Encoding]::new($false))
Start-Sleep -Seconds 30
`
	if err := os.WriteFile(target, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	child, err := BuildEnvironment(local, os.Environ())
	if err != nil {
		t.Fatal(err)
	}
	args := []string{"", "雪", `a"b`, "--option=value"}
	p, err := startPowerShell(context.Background(), Native{}, local, target, Command{Args: args, Environment: child, Directory: local.Paths.Root, Mode: Detach})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Abort()
	if err := p.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	var data []byte
	for time.Now().Before(deadline) {
		data, err = os.ReadFile(filepath.Join(local.Paths.Root, "ps-detached.json"))
		if err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	var got struct {
		Arguments []string
		PID       int
		Control   string
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("%s %v", data, err)
	}
	cleanupApp(t, got.PID)
	if !reflect.DeepEqual(got.Arguments, args) || got.Control != "" || !appAlive(got.PID) {
		t.Fatalf("%+v", got)
	}
	entries, _ := os.ReadDir(local.Paths.Temp)
	if len(entries) != 0 {
		t.Fatalf("invocation retained: %v", entries)
	}
}

func TestPowerShellStartupFailureAndCanceledHandoffLeaveNoAppOrFiles(t *testing.T) {
	for _, cancelHandoff := range []bool{false, true} {
		t.Run(map[bool]string{false: "invalid named parameter", true: "cancel before handoff"}[cancelHandoff], func(t *testing.T) {
			local := envFixture(t, map[string]string{})
			target := filepath.Join(local.Paths.Root, "pending.ps1")
			source := `param([string]$Name)
[IO.File]::WriteAllText((Join-Path $env:PORTSCOOP_ROOT 'must-not-run'), 'ran')
Start-Sleep -Seconds 30
`
			if err := os.WriteFile(target, []byte(source), 0600); err != nil {
				t.Fatal(err)
			}
			child, err := BuildEnvironment(local, os.Environ())
			if err != nil {
				t.Fatal(err)
			}
			args := []string{"-Name"}
			if cancelHandoff {
				args = []string{}
			}
			p, err := startPowerShell(context.Background(), Native{}, local, target, Command{Args: args, Environment: child, Directory: local.Paths.Root, Mode: Detach})
			if cancelHandoff {
				if err != nil {
					t.Fatal(err)
				}
				defer p.Abort()
				pid := p.(*powerShellProcess).Process.(*nativeProcess).command.Process.Pid
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				if err := p.Commit(ctx); err == nil {
					t.Fatal("canceled handoff succeeded")
				}
				assertAppDead(t, pid)
			} else if err == nil {
				_ = p.Abort()
				t.Fatal("invalid named argument accepted")
			}
			if _, err := os.Stat(filepath.Join(local.Paths.Root, "must-not-run")); !os.IsNotExist(err) {
				t.Fatal("uncommitted application executed", err)
			}
			entries, _ := os.ReadDir(local.Paths.Temp)
			if len(entries) != 0 {
				t.Fatal("startup transport retained", entries)
			}
		})
	}
}
