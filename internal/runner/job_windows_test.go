package runner_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"portscoop/internal/filelock"
	"portscoop/internal/runner"
)

func TestJobMemoryChild(t *testing.T) {
	if os.Getenv("PORTSCOOP_MEMORY_CHILD") != "1" {
		return
	}
	small, err := windows.VirtualAlloc(0, 1<<20, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err != nil {
		t.Fatal("small allocation failed", err)
	}
	defer windows.VirtualFree(small, 0, windows.MEM_RELEASE)
	large, err := windows.VirtualAlloc(0, 512<<20, windows.MEM_COMMIT|windows.MEM_RESERVE, windows.PAGE_READWRITE)
	if err == nil {
		windows.VirtualFree(large, 0, windows.MEM_RELEASE)
		t.Fatal("allocation exceeded job memory cap")
	}
	fmt.Fprint(os.Stdout, "allocation constrained")
}

func TestJobMemoryLimitAppliesBeforeExecution(t *testing.T) {
	exe, _ := os.Executable()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var output bytes.Buffer
	code, err := (runner.Native{}).Run(ctx, runner.Command{Executable: exe, Arguments: []string{"-test.run=^TestJobMemoryChild$"}, Environment: append(os.Environ(), "PORTSCOOP_MEMORY_CHILD=1"), MemoryLimit: 256 << 20, Streams: runner.Streams{Out: &output, Err: &output}})
	if err != nil || code != 0 || !strings.Contains(output.String(), "allocation constrained") {
		t.Fatalf("memory constraint: code=%d err=%v output=%s", code, err, &output)
	}
}

func lifecycleEnvironment(mode, directory string) []string {
	var result []string
	for _, value := range os.Environ() {
		if !strings.HasPrefix(value, "PORTSCOOP_LIFECYCLE_") {
			result = append(result, value)
		}
	}
	return append(result, "PORTSCOOP_LIFECYCLE_MODE="+mode, "PORTSCOOP_LIFECYCLE_DIR="+directory)
}

func TestLifecycleChild(t *testing.T) {
	mode := os.Getenv("PORTSCOOP_LIFECYCLE_MODE")
	if mode == "" {
		return
	}
	directory := os.Getenv("PORTSCOOP_LIFECYCLE_DIR")
	executable, _ := os.Executable()
	args := []string{"-test.run=^TestLifecycleChild$"}
	if mode == "owner" {
		_, err := (runner.Native{}).Run(context.Background(), runner.Command{Executable: executable, Arguments: args, Environment: lifecycleEnvironment("worker", directory)})
		if err != nil {
			t.Fatal(err)
		}
		return
	}
	if mode == "worker" {
		lock, err := filelock.Acquire(context.Background(), filepath.Join(directory, "operation.lock"))
		if err != nil {
			t.Fatal(err)
		}
		defer lock.Close()
		child := exec.Command(executable, args...)
		child.Env = lifecycleEnvironment("grandchild", directory)
		if err := child.Start(); err != nil {
			t.Fatal(err)
		}
		defer child.Process.Kill()
	}
	if err := os.WriteFile(filepath.Join(directory, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		t.Fatal(err)
	}
	time.Sleep(60 * time.Second)
}

func processAlive(pid int) bool {
	handle, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	status, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}

func waitLifecycle(t *testing.T, directory string) []int {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		var pids []int
		for _, name := range []string{"worker", "grandchild"} {
			data, _ := os.ReadFile(filepath.Join(directory, name+".pid"))
			if pid, err := strconv.Atoi(string(data)); err == nil {
				pids = append(pids, pid)
			}
		}
		if len(pids) == 2 {
			t.Cleanup(func() {
				for _, pid := range pids {
					if processAlive(pid) {
						p, _ := os.FindProcess(pid)
						_ = p.Kill()
					}
				}
			})
			return pids
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("managed child tree did not become ready")
	return nil
}

func assertLifecycleStopped(t *testing.T, directory string, pids []int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		alive := false
		for _, pid := range pids {
			alive = alive || processAlive(pid)
		}
		if !alive {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			lock, err := filelock.Acquire(ctx, filepath.Join(directory, "operation.lock"))
			if err != nil {
				t.Fatal("dead owner kept operation lock:", err)
			}
			_ = lock.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("managed descendants survived:", pids)
}

func TestCancellationKillsDescendantsAndReleasesLock(t *testing.T) {
	directory := t.TempDir()
	executable, _ := os.Executable()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		_, err := (runner.Native{}).Run(ctx, runner.Command{Executable: executable, Arguments: []string{"-test.run=^TestLifecycleChild$"}, Environment: lifecycleEnvironment("worker", directory)})
		finished <- err
	}()
	pids := waitLifecycle(t, directory)
	cancel()
	select {
	case err := <-finished:
		if err != context.Canceled {
			t.Fatalf("cancellation error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled runner did not return")
	}
	assertLifecycleStopped(t, directory, pids)
}

func TestAbruptOwnerTerminationKillsDescendantsAndReleasesLock(t *testing.T) {
	directory := t.TempDir()
	executable, _ := os.Executable()
	owner := exec.Command(executable, "-test.run=^TestLifecycleChild$")
	owner.Env = lifecycleEnvironment("owner", directory)
	if err := owner.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = owner.Process.Kill() })
	pids := waitLifecycle(t, directory)
	if err := owner.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	if err := owner.Wait(); err == nil {
		t.Fatal(fmt.Sprint("killed owner returned success"))
	}
	assertLifecycleStopped(t, directory, pids)
}
