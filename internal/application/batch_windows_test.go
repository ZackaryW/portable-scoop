package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func batchFixture(t *testing.T, mode string) (Command, string, string) {
	t.Helper()
	spec, dir := fixtureCommand(t, mode, Wait)
	target := filepath.Join(dir, "batch fixture.cmd")
	exe, _ := os.Executable()
	var capture string
	for i := 0; i < 16; i++ {
		capture += "@set \"PSPE_TEST_CAPTURE_" + strconv.Itoa(i) + "=%~1\"\r\n@shift\r\n"
	}
	if err := os.WriteFile(target, []byte(capture+"@\""+exe+"\" -test.run=^TestApplicationProcessFixture$ -- %*\r\n@exit /b %errorlevel%\r\n"), 0600); err != nil {
		t.Fatal(err)
	}
	return spec, dir, target
}

func TestBatchCodecRoundTripInputExitAndMetaCharacters(t *testing.T) {
	spec, dir, target := batchFixture(t, "streams")
	args := []string{"", "space value", "雪 π", "%PATH%", "100%", "!important!", "^caret", "&|<>()", "--key=value", "--env", "trailing\\"}
	spec.Args = args
	var out, diag bytes.Buffer
	spec.Streams = Streams{In: strings.NewReader("batch stdin"), Out: &out, Err: &diag}
	p, err := startBatch(context.Background(), Native{}, target, spec)
	if err != nil {
		t.Fatal(err)
	}
	code, err := p.Wait(context.Background())
	if err != nil || code != 37 {
		t.Fatalf("%d %v %s", code, err, &diag)
	}
	var got struct {
		Args       []string
		Captured   []string
		Input, Cwd string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%s %v", &out, err)
	}
	// Batch parameters use CMD's %~1 semantics. A batch file forwarding %*
	// into a native executable is responsible for that additional CRT parse.
	if !reflect.DeepEqual(got.Captured[:len(args)], args) || !reflect.DeepEqual(got.Args[:len(args)-1], args[:len(args)-1]) || got.Input != "batch stdin" || got.Cwd != dir || !strings.Contains(diag.String(), "application stderr") {
		t.Fatalf("got=%+v want=%q stderr=%s", got, args, &diag)
	}
}

func TestBatchRejectsUnqualifiedFormsBeforeStart(t *testing.T) {
	spec, _, target := batchFixture(t, "streams")
	for _, arg := range []string{`embedded"quote`, "line\nbreak", "line\rbreak", "NUL\x00value"} {
		spec.Args = []string{arg}
		if _, err := batchCommand(target, spec); err == nil || !strings.Contains(err.Error(), "not qualified") {
			t.Fatalf("accepted %q: %v", arg, err)
		}
	}
	spec.Args = make([]string, 500)
	for i := range spec.Args {
		spec.Args[i] = "argument"
	}
	if _, err := batchCommand(target, spec); err == nil {
		t.Fatal("unbounded batch command accepted")
	}
}

func TestBatchDetachedSurvivesAndNeedsNoInvocationFiles(t *testing.T) {
	spec, dir, target := batchFixture(t, "batch-detached")
	spec.Mode = Detach
	spec.Args = []string{"", "%PATH%", "!data!", "--key=value", "&|<>"}
	p, err := startBatch(context.Background(), Native{}, target, spec)
	if err != nil {
		t.Fatal(err)
	}
	defer p.Abort()
	if err := p.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	pid := waitPID(t, dir, "batch-detached")
	cleanupApp(t, pid)
	cleanupApp(t, p.(*nativeProcess).command.Process.Pid)
	if !appAlive(pid) {
		t.Fatal("detached batch child did not survive")
	}
	data, err := os.ReadFile(filepath.Join(dir, "batch-detached.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got struct{ Args, Captured []string }
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Args, spec.Args) || !reflect.DeepEqual(got.Captured[:len(spec.Args)], spec.Args) {
		t.Fatalf("detached arguments=%+v", got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "portscoop-execution-") {
			t.Fatal("batch retained invocation transport")
		}
	}
}
