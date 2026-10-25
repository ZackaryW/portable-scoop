package registry_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/testutil"
)

func firstStoreFixture(ctx context.Context, root, name string) (environment.Identity, error) {
	if err := ctx.Err(); err != nil {
		return environment.Identity{}, err
	}
	id, err := environment.NewIdentity(name)
	if err != nil {
		return id, err
	}
	marker := filepath.Join(root, filepath.FromSlash(environment.MarkerRelative))
	if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
		return id, err
	}
	data, err := json.Marshal(id)
	if err == nil {
		err = os.WriteFile(marker, data, 0600)
	}
	return id, err
}

func TestBootstrapPublishesFirstDefaultAndPreservesExistingSelection(t *testing.T) {
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
	root := filepath.Join(testutil.TempDir(t), "store")
	created, err := m.Bootstrap(context.Background(), root, firstStoreFixture)
	if err != nil || !created {
		t.Fatalf("bootstrap: created=%v error=%v", created, err)
	}
	d, err := m.Store.Read()
	if err != nil || len(d.Entries) != 1 || d.DefaultID != d.Entries["default"].ID || d.Entries["default"].Path != root {
		t.Fatalf("first default was not published: %+v %v", d, err)
	}
	before, _ := os.ReadFile(m.Store.Path)
	created, err = m.Bootstrap(context.Background(), filepath.Join(testutil.TempDir(t), "other"), func(context.Context, string, string) (environment.Identity, error) {
		t.Fatal("existing default triggered another initialization")
		return environment.Identity{}, nil
	})
	after, _ := os.ReadFile(m.Store.Path)
	if err != nil || created || string(before) != string(after) {
		t.Fatalf("existing default changed: %v %v", created, err)
	}
}

func TestBootstrapRejectsAliasCollisionBeforeInitialization(t *testing.T) {
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
	other := testutil.NewRoot(t, "existing")
	if err := m.Register(context.Background(), other.Path, "default"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.Store.Path)
	created, err := m.Bootstrap(context.Background(), filepath.Join(testutil.TempDir(t), "store"), func(context.Context, string, string) (environment.Identity, error) {
		t.Fatal("alias collision triggered initialization")
		return environment.Identity{}, nil
	})
	after, _ := os.ReadFile(m.Store.Path)
	if created || err == nil || !strings.Contains(err.Error(), "already registered") || string(before) != string(after) {
		t.Fatalf("alias collision not preserved: %v %v", created, err)
	}
}

func TestBootstrapFailuresDoNotPublishFalseDefault(t *testing.T) {
	for _, failPublication := range []bool{false, true} {
		t.Run(map[bool]string{false: "initialization", true: "publication"}[failPublication], func(t *testing.T) {
			m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
			root := filepath.Join(testutil.TempDir(t), "store")
			initialize := firstStoreFixture
			if failPublication {
				m.Store.Replace = func(string, string) error { return errors.New("publication failed") }
			} else {
				initialize = func(context.Context, string, string) (environment.Identity, error) {
					return environment.Identity{}, errors.New("initialization failed")
				}
			}
			created, err := m.Bootstrap(context.Background(), root, initialize)
			d, readErr := m.Store.Read()
			if err == nil || readErr != nil || d.DefaultID != "" || len(d.Entries) != 0 || created != failPublication {
				t.Fatalf("failure published metadata: %v %v %+v %v", created, err, d, readErr)
			}
			if failPublication {
				if _, _, err := environment.Load(environment.OSFilesystem{}, root); err != nil {
					t.Fatal("ready Store was removed after registry failure:", err)
				}
			}
		})
	}
}

func TestConcurrentHostBootstrapInitializesOnlyOneStore(t *testing.T) {
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
	parent := testutil.TempDir(t)
	var calls atomic.Int32
	initialize := func(ctx context.Context, root, name string) (environment.Identity, error) {
		calls.Add(1)
		time.Sleep(50 * time.Millisecond)
		return firstStoreFixture(ctx, root, name)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var workers sync.WaitGroup
	var successes atomic.Int32
	errors := make(chan error, 2)
	for _, folder := range []string{"first", "second"} {
		workers.Add(1)
		go func(folder string) {
			defer workers.Done()
			created, err := m.Bootstrap(ctx, filepath.Join(parent, folder), initialize)
			if created {
				successes.Add(1)
			}
			errors <- err
		}(folder)
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	if calls.Load() != 1 || successes.Load() != 1 {
		t.Fatalf("competing Hosts both initialized: calls=%d successes=%d", calls.Load(), successes.Load())
	}
}
