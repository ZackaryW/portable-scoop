// Package testutil creates isolated roots and injectable backend fixtures.
// It is imported by tests only; helpers never discover host Scoop installations.
package testutil

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
)

type Root struct {
	Path     string
	Identity environment.Identity
}

func NewRoot(t testing.TB, name string) Root {
	t.Helper()
	root := t.TempDir()
	id, err := environment.NewIdentity(name)
	if err != nil {
		t.Fatal(err)
	}
	WriteIdentity(t, root, id)
	return Root{root, id}
}

func WriteIdentity(t testing.TB, root string, id environment.Identity) {
	t.Helper()
	marker := filepath.Join(root, filepath.FromSlash(environment.MarkerRelative))
	if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
		t.Fatal(err)
	}
	data, err := json.MarshalIndent(id, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(marker, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func OrdinaryScoopSentinel(t testing.TB) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ordinary-scoop-sentinel.txt")
	if err := os.WriteFile(path, []byte("ordinary Scoop must remain unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

type Volumes []resolver.Volume

func (v Volumes) Volumes(ctx context.Context) ([]resolver.Volume, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return append([]resolver.Volume(nil), v...), nil
}

type Call struct {
	Environment environment.Context
	Arguments   []string
}

type Backend struct {
	mu       sync.Mutex
	calls    []Call
	ExitCode int
	Err      error
}

func (b *Backend) Execute(ctx context.Context, env environment.Context, args []string, _ runner.Streams) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls = append(b.calls, Call{env, append([]string(nil), args...)})
	return b.ExitCode, b.Err
}

func (b *Backend) Calls() []Call {
	b.mu.Lock()
	defer b.mu.Unlock()
	result := make([]Call, len(b.calls))
	for i, call := range b.calls {
		result[i] = Call{call.Environment, append([]string(nil), call.Arguments...)}
	}
	return result
}
