package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/testutil"
)

func TestRegistrationLifecyclePreservesPortableFiles(t *testing.T) {
	work, personal := testutil.NewRoot(t, "work"), testutil.NewRoot(t, "personal")
	marker := filepath.Join(work.Path, filepath.FromSlash(environment.MarkerRelative))
	before, _ := os.ReadFile(marker)
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
	ctx := context.Background()
	for alias, root := range map[string]string{"WORK": work.Path, "personal": personal.Path, "second-work": work.Path} {
		if err := m.Register(ctx, root, alias); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SetDefault(ctx, "WoRk"); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(ctx, personal.Path, "work"); err == nil {
		t.Fatal("alias silently changed identity")
	}
	if err := m.Unregister(ctx, "WORK"); err != nil {
		t.Fatal(err)
	}
	d, err := m.Store.Read()
	if err != nil || d.DefaultID != work.Identity.ID {
		t.Fatalf("default was cleared while another alias remained: %v", err)
	}
	if err := m.Unregister(ctx, "second-work"); err != nil {
		t.Fatal(err)
	}
	d, err = m.Store.Read()
	if err != nil || d.DefaultID != "" || len(d.Entries) != 1 {
		t.Fatalf("default/registration cleanup failed: %+v %v", d, err)
	}
	after, _ := os.ReadFile(marker)
	if string(before) != string(after) {
		t.Fatal("registration modified portable identity")
	}
}

func TestSameIdentityRegistrationRefreshesLocation(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}}
	if err := m.Register(context.Background(), root.Path, "work"); err != nil {
		t.Fatal(err)
	}
	moved := filepath.Join(testutil.TempDir(t), "moved (work)")
	if err := os.Rename(root.Path, moved); err != nil {
		t.Fatal(err)
	}
	if err := m.Register(context.Background(), moved, "WORK"); err != nil {
		t.Fatal(err)
	}
	d, err := m.Store.Read()
	if err != nil || d.Entries["work"].Path != moved || d.Entries["work"].ID != root.Identity.ID {
		t.Fatalf("registration did not rebind: %+v %v", d, err)
	}
}
