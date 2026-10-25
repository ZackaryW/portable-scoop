package application

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/localipc"
	"portscoop/internal/registry"
	"portscoop/internal/relocation"
	"portscoop/internal/resolver"
	"portscoop/internal/testutil"
)

// The compiled test executable doubles as an isolated Store helper, without Go
// builds inside a Store or any access to the developer's Scoop installation.
func TestMain(m *testing.M) {
	if os.Getenv("PSPE_CONTROL_TEST") == "1" && len(os.Args) == 3 && os.Args[1] == ControlOption {
		exe, _ := os.Executable()
		code, err := ServeControl(context.Background(), os.Args[2], exe, "fixture", Local{Verify: func(environment.Paths) error { return nil }}, Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
		if err != nil {
			fmt.Fprintln(os.Stderr, "pspe fixture:", err)
			os.Exit(2)
		}
		os.Exit(code)
	}
	os.Exit(m.Run())
}

func putAppFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func executionStore(t *testing.T) (environment.Context, string) {
	t.Helper()
	root := testutil.NewRoot(t, "execution")
	id, paths, err := environment.Load(environment.OSFilesystem{}, root.Path)
	if err != nil {
		t.Fatal(err)
	}
	exe, _ := os.Executable()
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	putAppFile(t, filepath.Join(paths.Root, "pspe.exe"), data)
	putAppFile(t, filepath.Join(paths.Root, "portscoop.exe"), []byte("management fixture"))
	target := filepath.Join(paths.Apps, "fixture", "1", "fixture.exe")
	putAppFile(t, target, data)
	stub := make([]byte, 256)
	copy(stub, "MZ")
	binary.LittleEndian.PutUint32(stub[60:64], 64)
	copy(stub[64:], "PE\x00\x00")
	putAppFile(t, filepath.Join(filepath.Dir(filepath.Dir(paths.Engine)), "supporting", "shims", "kiennq", "shim.exe"), stub)
	putAppFile(t, filepath.Join(paths.Shims, "tool.exe"), stub)
	putAppFile(t, filepath.Join(paths.Shims, "tool.shim"), []byte("path = \""+target+"\"\n"))
	if err := os.MkdirAll(paths.State, 0700); err != nil {
		t.Fatal(err)
	}
	if err := (relocation.Manager{}).Capture(paths); err != nil {
		t.Fatal(err)
	}
	return environment.Context{Identity: id, Paths: paths}, target
}

func invocationForFixture(dir string, mode Mode) Invocation {
	return Invocation{Shim: "tool", Args: []string{"-test.run=^TestApplicationProcessFixture$", "--", "", "雪", `a"b`, "--env=app", "&|<>%!"}, Mode: mode, Directory: dir}
}

func TestDirectAndCentralExecutionPreserveContract(t *testing.T) {
	local, _ := executionStore(t)
	t.Setenv("PSPE_CONTROL_TEST", "1")
	t.Setenv("PSPE_TEST_MODE", "streams")
	store := registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}
	manager := registry.Manager{Store: store}
	if err := manager.Register(context.Background(), local.Paths.Root, "work"); err != nil {
		t.Fatal(err)
	}
	if err := manager.SetDefault(context.Background(), "work"); err != nil {
		t.Fatal(err)
	}
	caller := testutil.TempDir(t)
	for _, central := range []bool{false, true} {
		r := Router{Executable: filepath.Join(local.Paths.Root, "pspe.exe"), RegistryPath: store.Path, Volumes: testutil.Volumes{}, Local: Local{Verify: func(environment.Paths) error { return nil }}, Client: Client{TempDirectory: testutil.TempDir(t)}}
		in := invocationForFixture(caller, Wait)
		if central {
			r.Executable = filepath.Join(testutil.TempDir(t), "pspe.exe")
			in.Selector = "work"
		}
		var out, diag bytes.Buffer
		code, err := r.Execute(context.Background(), in, Streams{In: strings.NewReader("caller input"), Out: &out, Err: &diag})
		if err != nil || code != 37 {
			t.Fatalf("central=%v %d %v stderr=%s", central, code, err, &diag)
		}
		var got struct {
			Args       []string
			Input, Cwd string
		}
		if err := json.Unmarshal(out.Bytes(), &got); err != nil {
			t.Fatalf("%s %v", &out, err)
		}
		if !reflect.DeepEqual(got.Args, in.Args[2:]) || got.Input != "caller input" || got.Cwd != caller || diag.String() != "application stderr\n" {
			t.Fatalf("central=%v %+v stderr=%s", central, got, &diag)
		}
	}
}

func TestCentralDetachedAcknowledgementSurvivesHelpersAndLaterCancel(t *testing.T) {
	local, _ := executionStore(t)
	dir := testutil.TempDir(t)
	t.Setenv("PSPE_CONTROL_TEST", "1")
	t.Setenv("PSPE_TEST_MODE", "survivor")
	t.Setenv("PSPE_TEST_DIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	in := invocationForFixture(dir, Detach)
	transport := testutil.TempDir(t)
	code, err := (Client{TempDirectory: transport}).Forward(ctx, resolver.Target{Identity: local.Identity, Paths: local.Paths}, in, Streams{})
	if err != nil || code != 0 {
		t.Fatalf("%d %v", code, err)
	}
	pid := waitPID(t, dir, "survivor")
	cleanupApp(t, pid)
	cancel()
	time.Sleep(20 * time.Millisecond)
	if !appAlive(pid) {
		t.Fatal("acknowledged app died with helpers/caller cancellation")
	}
	entries, _ := os.ReadDir(transport)
	if len(entries) != 0 {
		t.Fatal("Host control transport retained", entries)
	}
}

func TestCentralCancellationStopsStoreAndApplicationTree(t *testing.T) {
	local, _ := executionStore(t)
	dir := testutil.TempDir(t)
	t.Setenv("PSPE_CONTROL_TEST", "1")
	t.Setenv("PSPE_TEST_MODE", "tree")
	t.Setenv("PSPE_TEST_DIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := (Client{TempDirectory: testutil.TempDir(t)}).Forward(ctx, resolver.Target{Identity: local.Identity, Paths: local.Paths}, invocationForFixture(dir, Wait), Streams{})
		done <- err
	}()
	parent, child := waitPID(t, dir, "tree"), waitPID(t, dir, "survivor")
	cleanupApp(t, parent)
	cleanupApp(t, child)
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("%v", err)
		}
	case <-time.After(4 * time.Second):
		t.Fatal("cancellation did not return")
	}
	assertAppDead(t, parent)
	assertAppDead(t, child)
}

func TestDirectExecutionAfterMoveAndInvalidState(t *testing.T) {
	local, _ := executionStore(t)
	old := local.Paths.Root
	moved := old + " moved"
	if err := os.Rename(old, moved); err != nil {
		t.Fatal(err)
	}
	defer os.Rename(moved, old)
	local.Paths, _ = environment.PathsAt(moved)
	t.Setenv("PSPE_TEST_MODE", "streams")
	var out bytes.Buffer
	r := Router{Executable: filepath.Join(moved, "pspe.exe"), RegistryPath: filepath.Join(moved, "nonexistent.json"), Local: Local{Verify: func(environment.Paths) error { return nil }}}
	code, err := r.Execute(context.Background(), invocationForFixture(testutil.TempDir(t), Wait), Streams{In: strings.NewReader(""), Out: &out})
	if err != nil || code != 37 {
		t.Fatalf("%d %v", code, err)
	}
	data, _ := os.ReadFile(filepath.Join(local.Paths.Shims, "tool.shim"))
	if strings.Contains(string(data), old+string(filepath.Separator)) {
		t.Fatal("shim was not relocated")
	}
	putAppFile(t, filepath.Join(local.Paths.Root, environment.MarkerRelative), []byte("broken"))
	if _, err := r.Execute(context.Background(), invocationForFixture(testutil.TempDir(t), Wait), Streams{}); err == nil || !strings.Contains(err.Error(), "adjacent") {
		t.Fatal("invalid adjacent state fell back", err)
	}
}

func TestExecutionEnvelopeValidationAndMissingSupport(t *testing.T) {
	local, _ := executionStore(t)
	in := invocationForFixture(testutil.TempDir(t), Wait)
	for _, change := range []func(*Envelope){func(e *Envelope) { e.Protocol++ }, func(e *Envelope) { e.ExpectedID = "wrong" }, func(e *Envelope) { e.Invocation.Selector = "other" }, func(e *Envelope) { e.Invocation.Mode = "unknown" }, func(e *Envelope) { e.Invocation.Args = []string{"NUL\x00"} }, func(e *Envelope) { e.Invocation.Shim = `C:\app.exe` }, func(e *Envelope) { e.Invocation.Args = []string{strings.Repeat("x", 65537)} }} {
		e := Envelope{ExecutionProtocol, local.Identity.ID, in}
		change(&e)
		if err := e.Validate(local.Identity.ID); err == nil {
			t.Fatalf("accepted %+v", e)
		}
	}
	if err := os.Remove(filepath.Join(local.Paths.Root, "pspe.exe")); err != nil {
		t.Fatal(err)
	}
	if _, err := (Client{}).Forward(context.Background(), resolver.Target{Identity: local.Identity, Paths: local.Paths}, in, Streams{}); err == nil || !strings.Contains(err.Error(), "compatible execution support") {
		t.Fatal(err)
	}
}

func TestMalformedControlCannotStartApplications(t *testing.T) {
	local, _ := executionStore(t)
	server, err := localipc.New(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := ServeControl(ctx, server.Directory(), filepath.Join(local.Paths.Root, "pspe.exe"), "fixture", Local{}, Streams{})
		done <- err
	}()
	if err := server.Write(ctx, HelloRequest{ExecutionProtocol, local.Identity.ID}); err != nil {
		t.Fatal(err)
	}
	var hello ExecutionHello
	if err := server.Read(ctx, &hello); err != nil {
		t.Fatal(err)
	}
	if err := server.Write(ctx, map[string]any{"execution_protocol": ExecutionProtocol, "expected_id": local.Identity.ID, "executable": `C:\Windows\System32\cmd.exe`}); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatal(err)
	}
}

func TestDirectStoreRejectsCrossIdentitySelection(t *testing.T) {
	local, _ := executionStore(t)
	other, _ := executionStore(t)
	store := registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}
	if err := (registry.Manager{Store: store}).Register(context.Background(), other.Paths.Root, "other"); err != nil {
		t.Fatal(err)
	}
	r := Router{Executable: filepath.Join(local.Paths.Root, "pspe.exe"), RegistryPath: store.Path, Local: Local{Verify: func(environment.Paths) error { t.Fatal("cross-identity execution reached startup"); return nil }}}
	in := invocationForFixture(testutil.TempDir(t), Wait)
	in.Selector = "other"
	if _, err := r.Execute(context.Background(), in, Streams{}); err == nil || !strings.Contains(err.Error(), "different environment identity") {
		t.Fatal(err)
	}
}

func TestDisconnectBeforeCommitCleansPendingApplication(t *testing.T) {
	local, _ := executionStore(t)
	dir := testutil.TempDir(t)
	t.Setenv("PSPE_CONTROL_TEST", "1")
	t.Setenv("PSPE_TEST_MODE", "survivor")
	t.Setenv("PSPE_TEST_DIR", dir)
	server, err := localipc.New(testutil.TempDir(t))
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Write(ctx, HelloRequest{ExecutionProtocol, local.Identity.ID}); err != nil {
		t.Fatal(err)
	}
	helper, err := (Native{}).Start(ctx, Command{Executable: filepath.Join(local.Paths.Root, "pspe.exe"), Args: []string{ControlOption, server.Directory()}, Environment: os.Environ(), Mode: Wait, Directory: dir})
	if err != nil {
		t.Fatal(err)
	}
	defer helper.Abort()
	var hello ExecutionHello
	if err := server.Read(ctx, &hello); err != nil {
		t.Fatal(err)
	}
	if err := server.Write(ctx, Envelope{ExecutionProtocol, local.Identity.ID, invocationForFixture(dir, Detach)}); err != nil {
		t.Fatal(err)
	}
	var status startupMessage
	if err := server.Read(ctx, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != "ready" {
		t.Fatal(status)
	}
	pid := waitPID(t, dir, "survivor")
	cleanupApp(t, pid)
	if err := server.Close(); err != nil {
		t.Fatal(err)
	}
	code, err := helper.Wait(ctx)
	if err != nil || code != 2 {
		t.Fatalf("helper=%d %v", code, err)
	}
	assertAppDead(t, pid)
}

type protocolStarter struct {
	calls int
	serve func(*localipc.Endpoint)
}
type protocolProcess struct{}

func (protocolProcess) Wait(context.Context) (int, error) { return 0, nil }
func (protocolProcess) Commit(context.Context) error      { return nil }
func (protocolProcess) Abort() error                      { return nil }
func (p *protocolStarter) Start(ctx context.Context, command Command) (Process, error) {
	p.calls++
	endpoint, err := localipc.Open(command.Args[1])
	if err != nil {
		return nil, err
	}
	go func() { defer endpoint.Close(); p.serve(endpoint) }()
	return protocolProcess{}, nil
}

func TestExecutionHandshakeAndLostAcknowledgementNeverRetry(t *testing.T) {
	for _, failure := range []string{"identity", "version", "modes", "lost acknowledgement"} {
		t.Run(failure, func(t *testing.T) {
			local, _ := executionStore(t)
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			starter := &protocolStarter{serve: func(peer *localipc.Endpoint) {
				var request HelloRequest
				if err := peer.Read(ctx, &request); err != nil {
					return
				}
				hello := ExecutionHello{ExecutionProtocol, local.Identity.ID, "fixture", []Mode{Wait, Detach}, ""}
				switch failure {
				case "identity":
					hello.ID = "wrong"
				case "version":
					hello.Protocol++
				case "modes":
					hello.Modes = []Mode{Wait}
				}
				if err := peer.Write(ctx, hello); err != nil {
					return
				}
				if failure != "lost acknowledgement" {
					return
				}
				var envelope Envelope
				if err := peer.Read(ctx, &envelope); err != nil {
					return
				}
				if err := peer.Write(ctx, startupMessage{State: "ready"}); err != nil {
					return
				}
				var commit startupMessage
				_ = peer.Read(ctx, &commit)
			}}
			_, err := (Client{Starter: starter, TempDirectory: testutil.TempDir(t)}).Forward(ctx, resolver.Target{Identity: local.Identity, Paths: local.Paths}, invocationForFixture(testutil.TempDir(t), Detach), Streams{})
			if err == nil || starter.calls != 1 {
				t.Fatalf("calls=%d error=%v", starter.calls, err)
			}
			if failure == "lost acknowledgement" && !strings.Contains(err.Error(), "may be running; not retried") {
				t.Fatal(err)
			}
		})
	}
}
