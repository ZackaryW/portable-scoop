package appcli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/application"
	"portscoop/internal/delegation"
	"portscoop/internal/environment"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

type legacyManagement struct{ local environment.Context }

func (l legacyManagement) Run(ctx context.Context, command runner.Command) (int, error) {
	data, err := delegation.EncodeHello(l.local, "legacy")
	if err != nil {
		return 0, err
	}
	_, err = command.Streams.Out.Write(data)
	return 0, err
}

func TestExecutionDeploymentAndRollbackPreserveLegacyManagement(t *testing.T) {
	root := testutil.NewRoot(t, "legacy")
	id, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(root.Path, environment.MarkerRelative)
	before, err := os.ReadFile(marker)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range []string{"portscoop.exe", "psp.exe", "package-data.txt"} {
		if err := os.WriteFile(filepath.Join(root.Path, file), []byte("preserve"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	local := environment.Context{Identity: id, Paths: paths}
	target := resolver.Target{Identity: id, Paths: paths}
	assertManagement := func() {
		t.Helper()
		if err := (delegation.Client{Runner: legacyManagement{local}}).Verify(context.Background(), target, root.Path); err != nil {
			t.Fatal(err)
		}
	}
	assertManagement()
	in := application.Invocation{Shim: "app", Mode: application.Detach, Directory: root.Path}
	if _, err := (application.Client{}).Forward(context.Background(), target, in, application.Streams{}); err == nil || !strings.Contains(err.Error(), "compatible execution support") {
		t.Fatal(err)
	}
	// Compatible deployment and rollback change the added execution entry only.
	if err := os.WriteFile(filepath.Join(root.Path, "pspe.exe"), []byte("verified fixture deployment"), 0600); err != nil {
		t.Fatal(err)
	}
	assertManagement()
	if err := os.Remove(filepath.Join(root.Path, "pspe.exe")); err != nil {
		t.Fatal(err)
	}
	assertManagement()
	after, _ := os.ReadFile(marker)
	if string(before) != string(after) {
		t.Fatal("identity changed during launcher deployment")
	}
	for _, file := range []string{"portscoop.exe", "psp.exe", "package-data.txt"} {
		data, _ := os.ReadFile(filepath.Join(root.Path, file))
		if string(data) != "preserve" {
			t.Fatal("existing data changed", file)
		}
	}
}
