package relocation_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
	"portscoop/internal/testutil"
)

func contextFixture(t *testing.T) environment.Context {
	t.Helper()
	root := testutil.NewRoot(t, "work")
	paths, _ := environment.PathsAt(root.Path)
	return environment.Context{Identity: root.Identity, Paths: paths}
}

func TestSameRootSerializesPreparationAndStableReads(t *testing.T) {
	local := contextFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	coordinator := relocation.Coordinator{BusyTimeout: 100 * time.Millisecond, Prepare: func(environment.Paths) error { close(entered); <-release; return nil }}
	go func() {
		_, err := coordinator.Do(context.Background(), local, func() (int, error) { return 0, nil })
		finished <- err
	}()
	<-entered
	called := false
	_, err := (relocation.Coordinator{BusyTimeout: 50 * time.Millisecond}).Do(context.Background(), local, func() (int, error) { called = true; return 0, nil })
	if err == nil || !strings.Contains(err.Error(), "busy") || called {
		t.Fatalf("error=%v called=%v", err, called)
	}
	close(release)
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
	_, err = (relocation.Coordinator{}).Do(context.Background(), local, func() (int, error) { return 0, nil })
	if err != nil {
		t.Fatal("released filename remained busy:", err)
	}
}

func TestDifferentRootsDoNotShareOperationLock(t *testing.T) {
	first, second := contextFixture(t), contextFixture(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan error, 1)
	go func() {
		_, err := (relocation.Coordinator{}).Do(context.Background(), first, func() (int, error) { close(entered); <-release; return 0, nil })
		finished <- err
	}()
	<-entered
	_, err := (relocation.Coordinator{BusyTimeout: 50 * time.Millisecond}).Do(context.Background(), second, func() (int, error) { return 0, nil })
	close(release)
	if err != nil {
		t.Fatal(err)
	}
	if err := <-finished; err != nil {
		t.Fatal(err)
	}
}

func TestPreparationFailureBlocksExecutionAndReleasesLock(t *testing.T) {
	local := contextFixture(t)
	called := false
	_, err := (relocation.Coordinator{Prepare: func(environment.Paths) error { return os.ErrPermission }}).Do(context.Background(), local, func() (int, error) { called = true; return 0, nil })
	if err == nil || called {
		t.Fatalf("error=%v called=%v", err, called)
	}
	if _, err := os.Stat(filepath.Join(local.Paths.State, "operation.lock")); err != nil {
		t.Fatal(err)
	}
	_, err = (relocation.Coordinator{}).Do(context.Background(), local, func() (int, error) { return 0, nil })
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterruptedCapturePreservesCancellationStatus(t *testing.T) {
	local := contextFixture(t)
	_, err := (relocation.Coordinator{Capture: func(environment.Paths) error { return os.ErrPermission }}).Do(context.Background(), local, func() (int, error) { return 0, context.Canceled })
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "inspection") {
		t.Fatalf("cancellation was hidden by capture failure: %v", err)
	}
}
