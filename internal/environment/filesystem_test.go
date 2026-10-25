package environment_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func TestIndependentRootFixtures(t *testing.T) {
	a, b := testutil.NewRoot(t, "work"), testutil.NewRoot(t, "personal")
	sentinel := testutil.OrdinaryScoopSentinel(t)
	before, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatal(err)
	}
	backend := &testutil.Backend{}
	for _, root := range []testutil.Root{a, b} {
		id, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := backend.Execute(context.Background(), environment.Context{Identity: id, Paths: paths}, []string{"list"}, runner.Streams{}); err != nil {
			t.Fatal(err)
		}
	}
	calls := backend.Calls()
	if len(calls) != 2 || calls[0].Environment.Identity.ID == calls[1].Environment.Identity.ID || calls[0].Environment.Paths.Scoop == calls[1].Environment.Paths.Scoop {
		t.Fatal("roots did not remain independent")
	}
	after, err := os.ReadFile(sentinel)
	if err != nil || string(before) != string(after) {
		t.Fatal("ordinary Scoop sentinel changed")
	}
}

type unavailableFiles struct{ environment.OSFilesystem }

func (unavailableFiles) ReadFile(string) ([]byte, error) { return nil, fs.ErrPermission }

func TestFilesystemFailureDoesNotFallback(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	_, _, err := environment.Load(unavailableFiles{}, root.Path)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected original filesystem failure, got %v", err)
	}
}
