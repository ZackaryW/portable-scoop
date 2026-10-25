package application

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/windows"
	"portscoop/internal/testutil"
)

func appFixtureEnv(mode, dir string) []string {
	var env []string
	for _, v := range os.Environ() {
		if !strings.HasPrefix(v, "PSPE_TEST_") {
			env = append(env, v)
		}
	}
	return append(env, "PSPE_TEST_MODE="+mode, "PSPE_TEST_DIR="+dir)
}

func TestApplicationProcessFixture(t *testing.T) {
	mode := os.Getenv("PSPE_TEST_MODE")
	if mode == "" {
		return
	}
	dir := os.Getenv("PSPE_TEST_DIR")
	exe, _ := os.Executable()
	if mode == "nested-restrictive-owner" {
		p, err := (Native{}).Start(context.Background(), Command{Executable: exe, Args: []string{"-test.run=^TestApplicationProcessFixture$"}, Environment: appFixtureEnv("restrictive-owner", dir), Mode: Wait})
		if err != nil {
			panic(err)
		}
		code, err := p.Wait(context.Background())
		if err != nil {
			panic(err)
		}
		os.Exit(code)
	}
	if mode == "restrictive-owner" {
		p, err := (Native{}).Start(context.Background(), Command{Executable: exe, Args: []string{"-test.run=^TestApplicationProcessFixture$"}, Environment: appFixtureEnv("survivor", dir), Mode: Detach})
		result := "accepted"
		if err != nil {
			result = err.Error()
		} else {
			if err := p.Commit(context.Background()); err != nil {
				result = err.Error()
			}
		}
		if err := os.WriteFile(filepath.Join(dir, "detach-result.txt"), []byte(result), 0600); err != nil {
			panic(err)
		}
		os.Exit(0)
	}
	if mode == "streams" || mode == "batch-detached" {
		input, _ := io.ReadAll(os.Stdin)
		cwd, _ := os.Getwd()
		var captured []string
		for i := 0; i < 16; i++ {
			captured = append(captured, os.Getenv("PSPE_TEST_CAPTURE_"+strconv.Itoa(i)))
		}
		var output io.Writer = os.Stdout
		if mode == "batch-detached" {
			file, err := os.Create(filepath.Join(dir, "batch-detached.json"))
			if err != nil {
				panic(err)
			}
			defer file.Close()
			output = file
		}
		_ = json.NewEncoder(output).Encode(struct {
			Input, Cwd string
			Args       []string
			Captured   []string
		}{string(input), cwd, os.Args[3:], captured})
		if mode == "batch-detached" {
			_ = os.WriteFile(filepath.Join(dir, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
			time.Sleep(30 * time.Second)
			os.Exit(0)
		}
		fmtWrite(os.Stderr, "application stderr\n")
		os.Exit(37)
	}
	if mode == "launcher" || mode == "tree" {
		child := exec.Command(exe, "-test.run=^TestApplicationProcessFixture$")
		child.Env = appFixtureEnv("survivor", dir)
		child.Stdout, child.Stderr = os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			panic(err)
		}
		waitPID(t, dir, "survivor")
		if mode == "launcher" {
			os.Exit(23)
		}
	}
	if mode == "owner" || mode == "pending-owner" {
		p, err := (Native{}).Start(context.Background(), Command{Executable: exe, Args: []string{"-test.run=^TestApplicationProcessFixture$"}, Environment: appFixtureEnv("survivor", dir), Mode: Detach})
		if err != nil {
			panic(err)
		}
		if mode == "owner" {
			if err := p.Commit(context.Background()); err != nil {
				panic(err)
			}
			os.Exit(0)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, mode+".pid"), []byte(strconv.Itoa(os.Getpid())), 0600); err != nil {
		panic(err)
	}
	time.Sleep(60 * time.Second)
}

func fmtWrite(w io.Writer, text string) { _, _ = io.WriteString(w, text) }

func waitPID(t *testing.T, dir, name string) int {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		data, _ := os.ReadFile(filepath.Join(dir, name+".pid"))
		pid, err := strconv.Atoi(string(data))
		if err == nil {
			return pid
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("%s did not start", name)
	return 0
}
func appAlive(pid int) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE, false, uint32(pid))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	s, err := windows.WaitForSingleObject(h, 0)
	return err == nil && s == uint32(windows.WAIT_TIMEOUT)
}
func cleanupApp(t *testing.T, pid int) {
	t.Helper()
	t.Cleanup(func() {
		if appAlive(pid) {
			p, _ := os.FindProcess(pid)
			_ = p.Kill()
		}
	})
}
func assertAppDead(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !appAlive(pid) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("owned process %d survived", pid)
}
func fixtureCommand(t *testing.T, mode string, execution Mode) (Command, string) {
	t.Helper()
	dir := testutil.TempDir(t)
	exe, _ := os.Executable()
	return Command{Executable: exe, Args: []string{"-test.run=^TestApplicationProcessFixture$"}, Environment: appFixtureEnv(mode, dir), Directory: dir, Mode: execution}, dir
}

func TestNativeApplicationStreamsArgsDirectoryStatus(t *testing.T) {
	spec, dir := fixtureCommand(t, "streams", Wait)
	args := []string{"", `value "quoted"`, "雪 π", "% ! ^ & | < >", "--env=app", `trailing\`}
	spec.Args = append(spec.Args, append([]string{"--"}, args...)...)
	var out, diag bytes.Buffer
	spec.Streams = Streams{In: strings.NewReader("input data"), Out: &out, Err: &diag}
	p, err := (Native{}).Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	code, err := p.Wait(context.Background())
	if err != nil || code != 37 {
		t.Fatalf("status=%d err=%v", code, err)
	}
	var got struct {
		Input, Cwd string
		Args       []string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Input != "input data" || got.Cwd != dir || !reflect.DeepEqual(got.Args, args) || diag.String() != "application stderr\n" {
		t.Fatalf("%+v diag=%s", got, &diag)
	}
}

func TestNativeApplicationStartFailure(t *testing.T) {
	if _, err := (Native{}).Start(context.Background(), Command{Executable: filepath.Join(testutil.TempDir(t), "missing.exe"), Mode: Wait}); err == nil {
		t.Fatal("missing executable succeeded")
	}
}

func TestForegroundCancellationStopsTree(t *testing.T) {
	spec, dir := fixtureCommand(t, "tree", Wait)
	p, err := (Native{}).Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Abort()
	parent, child := waitPID(t, dir, "tree"), waitPID(t, dir, "survivor")
	cleanupApp(t, parent)
	cleanupApp(t, child)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := p.Wait(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	assertAppDead(t, parent)
	assertAppDead(t, child)
}

func TestForegroundNormalLauncherPreservesChildAndBoundsDrain(t *testing.T) {
	spec, dir := fixtureCommand(t, "launcher", Wait)
	var out bytes.Buffer
	spec.Streams.Out = &out
	p, err := (Native{}).Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	child := waitPID(t, dir, "survivor")
	cleanupApp(t, child)
	start := time.Now()
	code, err := p.Wait(context.Background())
	if err != nil || code != 23 || time.Since(start) > 2*time.Second {
		t.Fatalf("%d %v elapsed=%s", code, err, time.Since(start))
	}
	if !appAlive(child) {
		t.Fatal("normal completion killed application child")
	}
}

func TestForegroundChildCannotRetainCallersOutputPipe(t *testing.T) {
	spec, dir := fixtureCommand(t, "launcher", Wait)
	readSide, writeSide, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer readSide.Close()
	defer writeSide.Close()
	spec.Streams.Out = writeSide
	p, err := (Native{}).Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	child := waitPID(t, dir, "survivor")
	cleanupApp(t, child)
	code, err := p.Wait(context.Background())
	if err != nil || code != 23 {
		t.Fatalf("%d %v", code, err)
	}
	_ = writeSide.Close()
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, readSide); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("application descendant retained the caller's output pipe after launcher exit")
	}
	if !appAlive(child) {
		t.Fatal("normal completion killed the child to release output")
	}
}

func TestDetachedPendingAbortAndCommittedLifetime(t *testing.T) {
	requireDetachedCapability(t)
	for _, commit := range []bool{false, true} {
		t.Run(strconv.FormatBool(commit), func(t *testing.T) {
			spec, dir := fixtureCommand(t, "survivor", Detach)
			var out bytes.Buffer
			spec.Streams = Streams{In: strings.NewReader("must not inherit"), Out: &out, Err: &out}
			p, err := (Native{}).Start(context.Background(), spec)
			if err != nil {
				t.Fatal(err)
			}
			defer p.Abort()
			pid := waitPID(t, dir, "survivor")
			cleanupApp(t, pid)
			if commit {
				if err := p.Commit(context.Background()); err != nil {
					t.Fatal(err)
				}
			}
			if err := p.Abort(); err != nil {
				t.Fatal(err)
			}
			if commit {
				if !appAlive(pid) {
					t.Fatal("committed application did not survive")
				}
			} else {
				assertAppDead(t, pid)
			}
			if out.Len() != 0 {
				t.Fatal("detached child retained launcher streams")
			}
		})
	}
}

func TestDetachedSurvivesHelperNormalExit(t *testing.T) {
	requireDetachedCapability(t)
	spec, dir := fixtureCommand(t, "owner", Wait)
	p, err := (Native{}).Start(context.Background(), spec)
	if err != nil {
		t.Fatal(err)
	}
	code, err := p.Wait(context.Background())
	if err != nil || code != 0 {
		t.Fatalf("%d %v", code, err)
	}
	pid := waitPID(t, dir, "survivor")
	cleanupApp(t, pid)
	if !appAlive(pid) {
		t.Fatal("application died with nested helper job")
	}
}

func requireDetachedCapability(t *testing.T) {
	t.Helper()
	_, err := probeExternalJob(context.Background())
	var external *ExternalJobError
	if errors.As(err, &external) {
		t.Skipf("this test host retains an external job (%v); success is qualified in the disposable VM", err)
	}
	if err != nil {
		t.Fatal(err)
	}
}

func TestDetachedRejectsResidualRestrictiveAncestorBeforeUserCode(t *testing.T) {
	dir := testutil.TempDir(t)
	exe, _ := os.Executable()
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(job)
	if err := setJobLimits(job, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestApplicationProcessFixture$")
	cmd.Env = appFixtureEnv("nested-restrictive-owner", dir)
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED, HideWindow: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Kill()
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		t.Fatal(err)
	}
	if err := resumeApplication(uint32(cmd.Process.Pid)); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(fmt.Errorf("nested job fixture: %w", err))
	}
	data, err := os.ReadFile(filepath.Join(dir, "detach-result.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "external job") {
		t.Fatalf("unsafe detached acknowledgement under restrictive ancestor: %s", data)
	}
	if _, err := os.Stat(filepath.Join(dir, "survivor.pid")); !os.IsNotExist(err) {
		t.Fatal("unqualified child executed before external-job rejection", err)
	}
}
