package resolver_test

import (
	"os"
	"path/filepath"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/testutil"
)

func selectionRegistry(a, b testutil.Root) registry.Document {
	d := registry.Empty()
	d.Entries["work"] = registry.Entry{ID: a.Identity.ID, Path: a.Path, RelativePath: "tools/work"}
	d.Entries["personal"] = registry.Entry{ID: b.Identity.ID, Path: b.Path, RelativePath: "tools/personal"}
	d.DefaultID = b.Identity.ID
	return d
}

func TestSelectionPrecedenceAndLocalIndependence(t *testing.T) {
	a, b := testutil.NewRoot(t, "work"), testutil.NewRoot(t, "personal")
	d := selectionRegistry(a, b)
	read := func() (registry.Document, error) { return d, nil }
	central := filepath.Join(t.TempDir(), "psp.exe")
	selected, err := resolver.Select(nil, central, read, "WORK")
	if err != nil || selected.ID != a.Identity.ID || selected.Local {
		t.Fatalf("explicit selection failed: %+v %v", selected, err)
	}
	selected, err = resolver.Select(nil, central, read, "")
	if err != nil || selected.ID != b.Identity.ID {
		t.Fatalf("default selection failed: %+v %v", selected, err)
	}
	selected, err = resolver.Select(nil, filepath.Join(a.Path, "psp.exe"), func() (registry.Document, error) {
		t.Fatal("local invocation consulted host registry")
		return registry.Document{}, nil
	}, "")
	if err != nil || !selected.Local || selected.Paths.Root != a.Path {
		t.Fatalf("direct local selection failed: %+v %v", selected, err)
	}
	if _, err := resolver.Select(nil, filepath.Join(a.Path, "psp.exe"), read, "personal"); err == nil {
		t.Fatal("portable invocation targeted another identity")
	}
	if selected, err := resolver.Select(nil, filepath.Join(a.Path, "psp.exe"), read, "work"); err != nil || selected.ID != a.Identity.ID {
		t.Fatal("self selection was rejected")
	}
}

func TestCentralDoesNotInventDefaultOrInferWorkingDirectory(t *testing.T) {
	a, b := testutil.NewRoot(t, "work"), testutil.NewRoot(t, "personal")
	d := selectionRegistry(a, b)
	d.DefaultID = ""
	delete(d.Entries, "personal")
	t.Chdir(a.Path)
	central := filepath.Join(t.TempDir(), "portscoop.exe")
	if _, err := resolver.Select(nil, central, func() (registry.Document, error) { return d, nil }, ""); err == nil {
		t.Fatal("single registration or working directory implicitly became default")
	}
}

func TestCorruptOrIncompleteAdjacentStateDoesNotFallback(t *testing.T) {
	for _, raw := range []string{"{", "missing"} {
		a := testutil.NewRoot(t, "work")
		marker := filepath.Join(a.Path, filepath.FromSlash(environment.MarkerRelative))
		if raw == "missing" {
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
		} else {
			if err := os.WriteFile(marker, []byte(raw), 0600); err != nil {
				t.Fatal(err)
			}
		}
		if _, err := resolver.Select(nil, filepath.Join(a.Path, "psp.exe"), func() (registry.Document, error) {
			t.Fatal("invalid local marker fell back to registry")
			return registry.Empty(), nil
		}, ""); err == nil {
			t.Fatal("accepted invalid local state")
		}
	}
}
