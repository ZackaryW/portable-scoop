package runner_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portscoop/internal/runner"
)

func TestNativeChild(t *testing.T) {
	if os.Getenv("PORTSCOOP_RUNNER_TEST") != "streams" {
		return
	}
	input, _ := io.ReadAll(os.Stdin)
	cwd, _ := os.Getwd()
	_ = json.NewEncoder(os.Stdout).Encode(struct {
		Input, Cwd string
		Arguments  []string
	}{string(input), cwd, os.Args[3:]})
	_, _ = io.WriteString(os.Stderr, "backend diagnostic\n")
	os.Exit(37)
}

func TestNativeStreamsDirectoryArgumentsAndExit(t *testing.T) {
	executable, _ := os.Executable()
	directory := filepath.Join(t.TempDir(), "caller (space)")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	arguments := []string{"", `value "quoted"`, "π 你好", "$(); & |"}
	var out, diagnostic bytes.Buffer
	code, err := (runner.Native{}).Run(context.Background(), runner.Command{
		Executable: executable, Arguments: append([]string{"-test.run=^TestNativeChild$", "--"}, arguments...),
		Environment: append(os.Environ(), "PORTSCOOP_RUNNER_TEST=streams"), Directory: directory,
		Streams: runner.Streams{In: strings.NewReader("stdin data"), Out: &out, Err: &diagnostic},
	})
	if code != 37 || err != nil {
		t.Fatalf("status=%d error=%v stderr=%s", code, err, &diagnostic)
	}
	var received struct {
		Input, Cwd string
		Arguments  []string
	}
	if err := json.Unmarshal(out.Bytes(), &received); err != nil {
		t.Fatal(err)
	}
	if received.Input != "stdin data" || received.Cwd != directory || !reflect.DeepEqual(arguments, received.Arguments) || diagnostic.String() != "backend diagnostic\n" {
		t.Fatalf("received=%+v stderr=%s", received, &diagnostic)
	}
}

func TestNativeLaunchFailureIsSeparateFromBackendStatus(t *testing.T) {
	code, err := (runner.Native{}).Run(context.Background(), runner.Command{Executable: filepath.Join(t.TempDir(), "missing.exe")})
	if code != 0 || err == nil || !strings.Contains(err.Error(), "launch") {
		t.Fatalf("status=%d error=%v", code, err)
	}
}
