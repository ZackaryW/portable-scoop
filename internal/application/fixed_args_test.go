package application

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
)

func TestLocalNativeFixedArgumentsPrecedeCallerArguments(t *testing.T) {
	local, _ := executionStore(t)
	file := filepath.Join(local.Paths.Shims, "tool.shim")
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	putAppFile(t, file, append(data, []byte("args = -test.run=^TestApplicationProcessFixture -- --fixed \"read only\" \"\"\n")...))
	if err := (relocation.Manager{}).Capture(local.Paths); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PSPE_TEST_MODE", "streams")
	t.Setenv("PSPE_TEST_DIR", local.Paths.Root)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var out, diag bytes.Buffer
	p, err := (Local{Verify: func(environment.Paths) error { return nil }}).Start(ctx, local, Invocation{Shim: "tool", Args: []string{"--caller", "雪"}, Directory: local.Paths.Root, Mode: Wait}, Streams{In: bytes.NewBufferString("input"), Out: &out, Err: &diag})
	if err != nil {
		t.Fatal(err)
	}
	code, err := p.Wait(ctx)
	if err != nil || code != 37 {
		t.Fatalf("status=%d error=%v stderr=%s", code, err, &diag)
	}
	var got struct {
		Args  []string
		Input string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("%v: %s", err, &out)
	}
	if !reflect.DeepEqual(got.Args, []string{"--fixed", "read only", "", "--caller", "雪"}) || got.Input != "input" {
		t.Fatalf("got %+v", got)
	}
}
