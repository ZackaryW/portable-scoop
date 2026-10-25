package relocation

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/filelock"
	"portscoop/internal/testutil"
)

func TestLaunchPreparationUsesMutationLockOnlyDuringCreation(t *testing.T) {
	root := testutil.NewRoot(t, "launch")
	_, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	local := environment.Context{Identity: root.Identity, Paths: paths}
	if err := os.MkdirAll(paths.State, 0700); err != nil {
		t.Fatal(err)
	}
	lockPath := filepath.Join(paths.State, "operation.lock")
	lock, err := filelock.Acquire(context.Background(), lockPath)
	if err != nil {
		t.Fatal(err)
	}
	called := false
	c := Coordinator{BusyTimeout: 30 * time.Millisecond, Prepare: (Manager{}).Prepare, Capture: func(environment.Paths) error { t.Fatal("application edits were captured"); return nil }}
	err = c.PrepareLaunch(context.Background(), local, func(environment.Paths) error { called = true; return nil })
	if err == nil || called || !strings.Contains(err.Error(), "busy") {
		t.Fatalf("called=%v err=%v", called, err)
	}
	_ = lock.Close()
	err = c.PrepareLaunch(context.Background(), local, func(environment.Paths) error {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()
		if extra, err := filelock.Acquire(ctx, lockPath); err == nil {
			_ = extra.Close()
			t.Fatal("mutation could intervene before creation")
		}
		called = true
		return nil
	})
	if err != nil || !called {
		t.Fatalf("%v %v", called, err)
	}
	// A simulated live app needs no lock handle after the create callback.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	lock, err = filelock.Acquire(ctx, lockPath)
	if err != nil {
		t.Fatal(err)
	}
	_ = lock.Close()
	if _, err := os.Stat(filepath.Join(paths.State, "operation.json")); !os.IsNotExist(err) {
		t.Fatal("app was recorded as a running package operation", err)
	}
}

func TestLaunchPreparationDoesNotAdoptApplicationEdits(t *testing.T) {
	paths := relocationFixture(t)
	id, _ := environment.NewIdentity("launch")
	testutil.WriteIdentity(t, paths.Root, id)
	local := environment.Context{Identity: id, Paths: paths}
	c := Coordinator{Prepare: (Manager{}).Prepare, Capture: (Manager{}).Capture}
	if err := c.PrepareLaunch(context.Background(), local, func(environment.Paths) error {
		writeFixture(t, filepath.Join(paths.Shims, "tool.cmd"), []byte("@rem "+filepath.Join(paths.Apps, "tool", "current", "tools", "tool.cmd")+"\n@malicious edit"))
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	started := false
	if err := c.PrepareLaunch(context.Background(), local, func(environment.Paths) error { started = true; return nil }); err == nil || started {
		t.Fatal("application shim edit was adopted", err)
	}
}
