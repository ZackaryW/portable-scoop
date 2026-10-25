package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/backend/scoop"
	"portscoop/internal/delegation"
	"portscoop/internal/registry"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func TestEnvironmentCommandsUseExplicitRegistry(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	options := Options{RegistryPath: filepath.Join(t.TempDir(), "registry.json")}
	for _, args := range [][]string{{"env", "register", root.Path, "--name", "WORK"}, {"env", "default", "work"}, {"env", "unregister", "Work"}} {
		cmd := NewWithOptions("test", options)
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		if err := cmd.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	d, err := (registry.Store{Path: options.RegistryPath}).Read()
	if err != nil || len(d.Entries) != 0 || d.DefaultID != "" {
		t.Fatalf("command lifecycle failed: %+v %v", d, err)
	}
}

type recordingLauncher struct {
	identity string
	calls    []runner.Command
}

func (r *recordingLauncher) Run(ctx context.Context, command runner.Command) (int, error) {
	r.calls = append(r.calls, command)
	if command.Arguments[0] == delegation.HandshakeCommand {
		data, _ := json.Marshal(delegation.Hello{ID: r.identity, ProtocolMajor: 1, LauncherVersion: "test"})
		_, err := command.Streams.Out.Write(data)
		return 0, err
	}
	return 0, nil
}

func TestCentralSelectorIsRemovedBeforeDelegation(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	if err := os.WriteFile(filepath.Join(root.Path, "portscoop.exe"), []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	store := registry.Store{Path: filepath.Join(t.TempDir(), "registry.json")}
	if err := (registry.Manager{Store: store}).Register(context.Background(), root.Path, "work"); err != nil {
		t.Fatal(err)
	}
	process := &recordingLauncher{identity: root.Identity.ID}
	cmd := NewWithOptions("test", Options{RegistryPath: store.Path, ExecutablePath: filepath.Join(t.TempDir(), "portscoop.exe"), Volumes: testutil.Volumes{}, Runner: process})
	cmd.SetArgs([]string{"--env", "work", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(process.calls) != 2 {
		t.Fatalf("expected handshake and one delegation: %d", len(process.calls))
	}
	args := process.calls[1].Arguments
	if fmt.Sprint(args[4:]) != "[list]" {
		t.Fatalf("selector leaked into Scoop arguments: %v", args)
	}
}

func TestInternalEndpointExecutesLocallyAndCannotRediscover(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	backend := &testutil.Backend{}
	options := Options{ExecutablePath: filepath.Join(root.Path, "portscoop.exe"), Backend: backend}
	cmd := NewWithOptions("test", options)
	cmd.SetArgs([]string{delegation.ExecuteCommand, root.Identity.ID, "1", "--", "list"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls := backend.Calls(); len(calls) != 1 || calls[0].Environment.Identity.ID != root.Identity.ID {
		t.Fatal("delegated endpoint did not execute locally")
	}
	options.ExecutablePath = filepath.Join(t.TempDir(), "portscoop.exe")
	cmd = NewWithOptions("test", options)
	cmd.SetArgs([]string{delegation.ExecuteCommand, root.Identity.ID, "1", "--", "list"})
	if err := cmd.Execute(); err == nil || len(backend.Calls()) != 1 {
		t.Fatal("central endpoint recursively resolved a target")
	}
}

func TestDocumentedManagementHelp(t *testing.T) {
	for _, name := range []string{"register", "default", "unregister", "list", "bootstrap"} {
		cmd := NewWithOptions("test", Options{RegistryPath: filepath.Join(t.TempDir(), "registry.json")})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{"env", name, "--help"})
		if err := cmd.Execute(); err != nil || output.Len() == 0 {
			t.Fatalf("missing documented help for %s: %v", name, err)
		}
	}
}

func TestBootstrapRejectsIncompletePortableRuntimeBeforeRegistryWrites(t *testing.T) {
	root := testutil.NewRoot(t, "portable")
	registryPath := filepath.Join(testutil.TempDir(t), "registry.json")
	cmd := NewWithOptions("test", Options{RegistryPath: registryPath, ExecutablePath: filepath.Join(root.Path, "psp.exe"), Initializer: &scoop.Initializer{ReleaseRoot: root.Path}})
	cmd.SetArgs([]string{"env", "bootstrap"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "backend") {
		t.Fatalf("portable launcher accepted incomplete runtime: %v", err)
	}
	if _, err := os.Stat(registryPath); !os.IsNotExist(err) {
		t.Fatal("rejected bootstrap wrote host registration")
	}
}
