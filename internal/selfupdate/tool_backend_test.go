package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runtimeinstall"
	"portscoop/internal/testutil"
	"portscoop/internal/toolbundle"
)

func addCandidateTools(t *testing.T, tx *Transaction) map[string]string {
	t.Helper()
	root := filepath.Join(tx.Directory, "candidate")
	var b environment.BackendMetadata
	var r runtimeinstall.Release
	if err := runtimeinstall.Read(root, "backend/backend.json", &b); err != nil {
		t.Fatal(err)
	}
	if err := runtimeinstall.Read(root, "release.json", &r); err != nil {
		t.Fatal(err)
	}
	i := toolbundle.Inventory{Pins: toolbundle.Pins{SchemaVersion: 1}, Artifacts: map[string]string{}}
	for name, required := range map[string][]string{"git": {"cmd/git.exe", "LICENSE.txt"}, "7zip": {"7z.exe", "7z.dll", "License.txt"}} {
		i.Tools = append(i.Tools, toolbundle.Tool{Name: name, Version: "fixture", URL: "https://example.invalid/artifact", SHA256: strings.Repeat("a", 64), Format: "zip", Required: required, Source: "https://example.invalid/source", SourcePackages: "https://example.invalid/sources"})
		for _, p := range required {
			rel := name + "/" + p
			source := "backend/tools/" + rel
			fixtureFile(t, root, source, "new "+rel)
			h, err := runtimeinstall.Hash(root, source)
			if err != nil {
				t.Fatal(err)
			}
			i.Artifacts[rel] = h
			b.Artifacts[".portscoop/tools/"+rel] = h
			r.Artifacts[source] = h
		}
	}
	if err := runtimeinstall.Write(root, "backend/tools/tools.json", i); err != nil {
		t.Fatal(err)
	}
	h, _ := runtimeinstall.Hash(root, "backend/tools/tools.json")
	b.Artifacts[".portscoop/tools/tools.json"] = h
	r.Artifacts["backend/tools/tools.json"] = h
	if err := runtimeinstall.Write(root, "backend/backend.json", b); err != nil {
		t.Fatal(err)
	}
	r.Artifacts["backend/backend.json"], _ = runtimeinstall.Hash(root, "backend/backend.json")
	if err := runtimeinstall.Write(root, "release.json", r); err != nil {
		t.Fatal(err)
	}
	return b.Artifacts
}

func TestIntroduceManagedToolsAndRecover(t *testing.T) {
	for _, recover := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit", true: "recover"}[recover], func(t *testing.T) {
			tx := storeTransaction(t)
			artifacts := addCandidateTools(t, tx)
			fixtureFile(t, tx.Target.Root, "scoop/persist/app/settings", "preserve")
			if err := (Disk{}).Prepare(tx); err != nil {
				t.Fatal(err)
			}
			if err := (Disk{}).Publish(tx); err != nil {
				t.Fatal(err)
			}
			if err := writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
				t.Fatal(err)
			}
			if recover {
				for _, op := range tx.Operations {
					if strings.HasPrefix(op.Path, ".portscoop/tools/") {
						if err := replace(tx, op, false); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			if err := commit(context.Background(), tx.Directory, recover); err != nil {
				t.Fatal(err)
			}
			for p, h := range artifacts {
				if strings.HasPrefix(p, ".portscoop/tools/") {
					if recover {
						if _, e := os.Stat(filepath.Join(tx.Target.Root, p)); !os.IsNotExist(e) {
							t.Fatalf("recovery retained introduced helper: %s", p)
						}
					} else {
						got, e := runtimeinstall.Hash(tx.Target.Root, p)
						if e != nil || got != h {
							t.Fatalf("helper not published %s: %v", p, e)
						}
					}
				}
			}
			b, _ := os.ReadFile(filepath.Join(tx.Target.Root, "scoop/persist/app/settings"))
			if string(b) != "preserve" {
				t.Fatal("user data changed")
			}
		})
	}
}

func TestManagedToolsCannotAdoptUnownedDirectory(t *testing.T) {
	tx := storeTransaction(t)
	addCandidateTools(t, tx)
	fixtureFile(t, tx.Target.Root, ".portscoop/tools/user.dll", "user")
	if err := (Disk{}).Prepare(tx); err == nil {
		t.Fatal("adopted unowned tool directory")
	}
}

func installOldTools(t *testing.T, tx *Transaction) map[string]string {
	t.Helper()
	artifacts := addCandidateTools(t, tx)
	var old environment.BackendMetadata
	if err := runtimeinstall.Read(tx.Target.Root, backendMetadata, &old); err != nil {
		t.Fatal(err)
	}
	for p := range artifacts {
		if strings.HasPrefix(p, ".portscoop/tools/") {
			fixtureFile(t, tx.Target.Root, p, "old "+p)
			old.Artifacts[p], _ = runtimeinstall.Hash(tx.Target.Root, p)
		}
	}
	if err := runtimeinstall.Write(tx.Target.Root, backendMetadata, old); err != nil {
		t.Fatal(err)
	}
	return old.Artifacts
}

func TestExistingManagedToolsRollbackAndRelocation(t *testing.T) {
	for _, mode := range []string{"interruption", "locked-dll", "relocated"} {
		t.Run(mode, func(t *testing.T) {
			tx := storeTransaction(t)
			old := installOldTools(t, tx)
			preserved := map[string]string{}
			for _, p := range []string{environment.MarkerRelative, "scoop/apps/app/current/data", "scoop/persist/app/settings", "scoop/buckets/main/app.json"} {
				if p != environment.MarkerRelative {
					fixtureFile(t, tx.Target.Root, p, "preserve")
				}
				preserved[p], _ = runtimeinstall.Hash(tx.Target.Root, p)
			}
			if mode == "relocated" {
				previous := tx.Target.Root
				destination := filepath.Join(testutil.TempDir(t), "moved store")
				rel, err := filepath.Rel(previous, tx.Directory)
				if err != nil {
					t.Fatal(err)
				}
				if err = os.Rename(previous, destination); err != nil {
					t.Fatal(err)
				}
				tx.Target.Root = destination
				tx.Directory = filepath.Join(destination, rel)
			}
			if err := (Disk{}).Prepare(tx); err != nil {
				t.Fatal(err)
			}
			if err := (Disk{}).Publish(tx); err != nil {
				t.Fatal(err)
			}
			if err := writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
				t.Fatal(err)
			}
			if mode == "locked-dll" {
				f, err := os.Open(filepath.Join(tx.Target.Root, ".portscoop/tools/7zip/7z.dll"))
				if err != nil {
					t.Fatal(err)
				}
				err = commit(context.Background(), tx.Directory, false)
				f.Close()
				if err == nil {
					t.Fatal("locked DLL replacement succeeded")
				}
			} else {
				for _, op := range tx.Operations {
					if strings.HasPrefix(op.Path, ".portscoop/tools/") {
						if err := replace(tx, op, false); err != nil {
							t.Fatal(err)
						}
					}
				}
				if err := commit(context.Background(), tx.Directory, true); err != nil {
					t.Fatal(err)
				}
			}
			for p, want := range old {
				got, err := runtimeinstall.Hash(tx.Target.Root, p)
				if err != nil || got != want {
					t.Fatalf("prior runtime changed: %s %v", p, err)
				}
			}
			for p, want := range preserved {
				got, err := runtimeinstall.Hash(tx.Target.Root, p)
				if err != nil || got != want {
					t.Fatalf("user state changed: %s %v", p, err)
				}
			}
		})
	}
}

func TestCandidateHelperInventoryMustAgree(t *testing.T) {
	for _, kind := range []string{"missing-dll", "unlisted-plugin", "backend-only", "old-descriptor"} {
		t.Run(kind, func(t *testing.T) {
			tx := storeTransaction(t)
			addCandidateTools(t, tx)
			root := filepath.Join(tx.Directory, "candidate")
			switch kind {
			case "missing-dll":
				if err := os.Remove(filepath.Join(root, "backend/tools/7zip/7z.dll")); err != nil {
					t.Fatal(err)
				}
			case "unlisted-plugin":
				fixtureFile(t, root, "backend/tools/7zip/plugin.dll", "unowned")
				refreshFixtureInventory(t, root)
			case "backend-only":
				var b environment.BackendMetadata
				if err := runtimeinstall.Read(root, "backend/backend.json", &b); err != nil {
					t.Fatal(err)
				}
				b.Artifacts[".portscoop/tools/7zip/plugin.dll"] = strings.Repeat("a", 64)
				if err := runtimeinstall.Write(root, "backend/backend.json", b); err != nil {
					t.Fatal(err)
				}
				refreshFixtureInventory(t, root)
			case "old-descriptor":
				var d runtimeinstall.Descriptor
				if err := runtimeinstall.Read(root, runtimeinstall.DescriptorName, &d); err != nil {
					t.Fatal(err)
				}
				d.Backends[0].Patch = "older-contract"
				if err := runtimeinstall.Write(root, runtimeinstall.DescriptorName, d); err != nil {
					t.Fatal(err)
				}
				refreshFixtureInventory(t, root)
			}
			if err := (Disk{}).Prepare(tx); err == nil {
				t.Fatal("accepted inconsistent helper bundle")
			}
		})
	}
}
