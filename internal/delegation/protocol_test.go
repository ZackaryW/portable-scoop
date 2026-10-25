package delegation_test

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/delegation"
	"portscoop/internal/environment"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

type fakeLauncher struct {
	hello delegation.Hello
	calls []runner.Command
}

func (f *fakeLauncher) Run(ctx context.Context, command runner.Command) (int, error) {
	f.calls = append(f.calls, command)
	if command.Arguments[0] == delegation.HandshakeCommand {
		data, _ := json.Marshal(f.hello)
		_, err := command.Streams.Out.Write(data)
		return 0, err
	}
	return 17, nil
}

func targetFixture(t *testing.T) resolver.Target {
	root := testutil.NewRoot(t, "work")
	id, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	return resolver.Target{Identity: id, Paths: paths}
}

func TestForwardPreservesIdentityProtocolArgumentsAndDirectory(t *testing.T) {
	target := targetFixture(t)
	fake := &fakeLauncher{hello: delegation.Hello{ID: target.Identity.ID, ProtocolMajor: 1, LauncherVersion: "test"}}
	code, err := (delegation.Client{Runner: fake}).Forward(context.Background(), target, []string{"search", "quotes \" and spaces"}, "working-directory", runner.Streams{})
	if err != nil || code != 17 || len(fake.calls) != 2 {
		t.Fatalf("forward failed: %d %v", code, err)
	}
	call := fake.calls[1]
	if call.Arguments[0] != delegation.ExecuteCommand || call.Arguments[1] != target.Identity.ID || call.Arguments[2] != "1" || call.Arguments[3] != "--" || call.Arguments[5] != "quotes \" and spaces" || call.Directory != "working-directory" {
		t.Fatalf("invocation changed: %+v", call)
	}
}

func TestHandshakeRejectsProtocolOrIdentityBeforeForwarding(t *testing.T) {
	target := targetFixture(t)
	for _, hello := range []delegation.Hello{{ID: target.Identity.ID, ProtocolMajor: 2}, {ID: "wrong", ProtocolMajor: 1}} {
		fake := &fakeLauncher{hello: hello}
		if _, err := (delegation.Client{Runner: fake}).Forward(context.Background(), target, []string{"list"}, "", runner.Streams{}); err == nil || len(fake.calls) != 1 {
			t.Fatal("incompatible launcher received backend invocation")
		}
	}
}

func TestLocalEndpointRechecksAdjacentIdentity(t *testing.T) {
	target := targetFixture(t)
	executable := filepath.Join(target.Paths.Root, "portscoop.exe")
	other, _ := environment.NewIdentity("replacement")
	testutil.WriteIdentity(t, target.Paths.Root, other)
	if _, err := delegation.Local(nil, executable, target.Identity.ID, 1); err == nil || !strings.Contains(err.Error(), "identity changed") {
		t.Fatal("replaced identity was accepted")
	}
	central := filepath.Join(t.TempDir(), "portscoop.exe")
	if _, err := delegation.Local(nil, central, target.Identity.ID, 1); err == nil {
		t.Fatal("internal execution entered central discovery")
	}
	if _, err := delegation.Local(nil, executable, other.ID, 2); err == nil {
		t.Fatal("unsupported execute protocol was accepted")
	}
}

func TestHandshakeOutputIsBounded(t *testing.T) {
	target := targetFixture(t)
	fake := &fakeLauncher{hello: delegation.Hello{ID: target.Identity.ID, ProtocolMajor: 1, LauncherVersion: strings.Repeat("x", 70*1024)}}
	if _, err := (delegation.Client{Runner: fake}).Forward(context.Background(), target, []string{"list"}, "", runner.Streams{}); err == nil || len(fake.calls) != 1 {
		t.Fatal("oversized handshake was accepted")
	}
}
