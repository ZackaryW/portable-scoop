package appcli

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"portscoop/internal/application"
	"portscoop/internal/cli"
)

var _ application.Executor = application.ExecuteFunc(nil)

func TestNoReservedApplicationCommands(t *testing.T) {
	for _, name := range []string{"install", "help", "launch"} {
		called := false
		f := Frontend{Directory: func() (string, error) { return `C:\caller`, nil }, Executor: application.ExecuteFunc(func(ctx context.Context, in application.Invocation, streams application.Streams) (int, error) {
			called = true
			if in.Shim != name || in.Directory != `C:\caller` || len(in.Args) != 1 || in.Args[0] != "--help" {
				t.Fatalf("changed invocation: %+v", in)
			}
			return 37, nil
		})}
		var out, diag bytes.Buffer
		if code := f.Run(context.Background(), []string{name, "--help"}, application.Streams{Out: &out, Err: &diag}); code != 37 || !called || diag.Len() != 0 {
			t.Fatalf("code=%d called=%v diag=%s", code, called, diag.String())
		}
	}
	cmd := cli.NewWithOptions("test", cli.Options{})
	cmd.SetArgs([]string{"unregistered-editor"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "unknown command") {
		t.Fatalf("management accepts app fallback: %v", err)
	}
}

func TestFrontendStatusAndDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		status     int
		err        error
		want       int
		diagnostic string
	}{
		{2, nil, 2, ""}, {130, nil, 130, ""}, {0, context.Canceled, 130, "cancelled"}, {0, fmt.Errorf("start failed"), 2, "pspe: start failed"},
	} {
		var diag bytes.Buffer
		f := Frontend{Executor: application.ExecuteFunc(func(context.Context, application.Invocation, application.Streams) (int, error) {
			return tc.status, tc.err
		})}
		if got := f.Run(context.Background(), []string{"app"}, application.Streams{Err: &diag}); got != tc.want || (tc.diagnostic == "" && diag.Len() != 0) || !strings.Contains(diag.String(), tc.diagnostic) {
			t.Fatalf("got %d %q", got, diag.String())
		}
	}
}

func TestHelpVersionDoNotDispatch(t *testing.T) {
	for _, arg := range []string{"--help", "--version"} {
		var out bytes.Buffer
		f := Frontend{Version: "test", Executor: application.ExecuteFunc(func(context.Context, application.Invocation, application.Streams) (int, error) {
			t.Fatal("dispatched informational option")
			return 0, nil
		})}
		if code := f.Run(context.Background(), []string{arg}, application.Streams{Out: &out, Err: &out}); code != 0 || out.Len() == 0 {
			t.Fatalf("%d %q", code, out.String())
		}
	}
}
