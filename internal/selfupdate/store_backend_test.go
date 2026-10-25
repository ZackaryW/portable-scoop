package selfupdate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/localipc"
	"portscoop/internal/runtimeinstall"
)

func fixtureDistribution(t *testing.T, root string) {
	t.Helper()
	for _, e := range releaseEntries(t) {
		if strings.HasSuffix(e.name, "/") {
			continue
		}
		fixtureFile(t, root, e.name, e.content)
	}
}

func fixtureFile(t *testing.T, root, name, content string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func storeTransaction(t *testing.T) *Transaction {
	t.Helper()
	host, root := t.TempDir(), t.TempDir()
	fixtureDistribution(t, host)
	h, err := (Disk{}).Select(filepath.Join(host, "psp.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	h, err = (Disk{}).Ensure(h)
	if err != nil {
		t.Fatal(err)
	}
	r, d, err := runtimeinstall.VerifyDistribution(host, true)
	if err != nil {
		t.Fatal(err)
	}
	id, err := environment.NewIdentity("fixture")
	if err != nil {
		t.Fatal(err)
	}
	if err = runtimeinstall.Write(root, environment.MarkerRelative, id); err != nil {
		t.Fatal(err)
	}
	for _, n := range runtimeinstall.Launchers {
		if err = copyFile(host, n, root, n); err != nil {
			t.Fatal(err)
		}
	}
	var b environment.BackendMetadata
	if err = runtimeinstall.Read(host, "backend/backend.json", &b); err != nil {
		t.Fatal(err)
	}
	b.PatchRevision = "older-backend"
	for p := range b.Artifacts {
		fixtureFile(t, root, p, "old "+p)
		b.Artifacts[p], err = runtimeinstall.Hash(root, p)
		if err != nil {
			t.Fatal(err)
		}
	}
	fixtureFile(t, root, ".portscoop/backend/obsolete.ps1", "old owned adapter")
	b.Artifacts[".portscoop/backend/obsolete.ps1"], _ = runtimeinstall.Hash(root, ".portscoop/backend/obsolete.ps1")
	if err = runtimeinstall.Write(root, backendMetadata, b); err != nil {
		t.Fatal(err)
	}
	d.Version = "0.3.0"
	if err = runtimeinstall.Write(root, runtimeinstall.ReceiptName, runtimeinstall.NewReceipt(id.ID, d, r)); err != nil {
		t.Fatal(err)
	}
	target, err := (Disk{}).Select(filepath.Join(root, "psp.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	target, err = (Disk{}).Ensure(target)
	if err != nil {
		t.Fatal(err)
	}
	e, err := localipc.New(target.Control())
	if err != nil {
		t.Fatal(err)
	}
	fixtureDistribution(t, filepath.Join(e.Directory(), "candidate"))
	return &Transaction{Schema: 1, Target: target, Controller: &h, Directory: e.Directory(), Nonce: strings.Repeat("a", 64), Version: "v0.3.2-fixture"}
}

func TestManagedStoreUpgradeAndRecovery(t *testing.T) {
	for _, recover := range []bool{false, true} {
		t.Run(map[bool]string{false: "upgrade", true: "interrupted-recovery"}[recover], func(t *testing.T) {
			tx := storeTransaction(t)
			preserved := []string{environment.MarkerRelative, "scoop/apps/example/current/app.exe", "scoop/persist/example/data", "scoop/buckets/main/app.json", "scoop/cache/a", "scoop/shims/app.exe", ".portscoop/state/paths.json", ".portscoop/state/unowned.txt"}
			hashes := map[string]string{}
			for _, p := range preserved {
				if p != environment.MarkerRelative {
					fixtureFile(t, tx.Target.Root, p, "user state")
				}
				hashes[p], _ = runtimeinstall.Hash(tx.Target.Root, p)
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
			if recover {
				// Simulate interruption after metadata, receipt and adapter replacements.
				for _, op := range tx.Operations[:len(tx.Operations)/2] {
					if err := replace(tx, op, false); err != nil {
						t.Fatal(err)
					}
				}
			}
			if err := commit(context.Background(), tx.Directory, recover); err != nil {
				t.Fatal(err)
			}
			if err := verifyInstalled(tx, recover); err != nil {
				t.Fatal(err)
			}
			for p, want := range hashes {
				got, err := runtimeinstall.Hash(tx.Target.Root, p)
				if err != nil || got != want {
					t.Fatalf("changed user state %s: %v", p, err)
				}
			}
			if _, err := runtimeinstall.ReadPending(tx.Target); !os.IsNotExist(err) {
				t.Fatalf("pending remains: %v", err)
			}
			var b environment.BackendMetadata
			if err := runtimeinstall.Read(tx.Target.Root, backendMetadata, &b); err != nil {
				t.Fatal(err)
			}
			want := "fixture-v1"
			if recover {
				want = "older-backend"
			}
			if b.PatchRevision != want {
				t.Fatalf("backend %s", b.PatchRevision)
			}
		})
	}
}

func TestManagedStoreRejectsUnownedOrAlteredArtifacts(t *testing.T) {
	for _, kind := range []string{"drift", "collision", "outside", "unowned", "traversal", "candidate-digest", "controller"} {
		t.Run(kind, func(t *testing.T) {
			tx := storeTransaction(t)
			var b environment.BackendMetadata
			if err := runtimeinstall.Read(tx.Target.Root, backendMetadata, &b); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "unowned":
				fixtureFile(t, tx.Target.Root, "scoop/apps/scoop/current/extra.ps1", "unowned library")
			case "drift":
				fixtureFile(t, tx.Target.Root, environment.AdapterRelative, "user edited")
			case "collision":
				delete(b.Artifacts, ".portscoop/backend/obsolete.ps1")
				// A required candidate entry exists but is not owned by the old inventory.
				fixtureFile(t, tx.Target.Root, ".portscoop/backend/new.ps1", "user data")
				candidate := filepath.Join(tx.Directory, "candidate")
				var next environment.BackendMetadata
				runtimeinstall.Read(candidate, "backend/backend.json", &next)
				fixtureFile(t, candidate, "backend/adapter/new.ps1", "candidate")
				next.Artifacts[".portscoop/backend/new.ps1"], _ = runtimeinstall.Hash(candidate, "backend/adapter/new.ps1")
				runtimeinstall.Write(candidate, "backend/backend.json", next)
				refreshFixtureInventory(t, candidate)
			case "outside":
				fixtureFile(t, tx.Target.Root, "scoop/persist/secret", "user data")
				b.Artifacts["scoop/persist/secret"], _ = runtimeinstall.Hash(tx.Target.Root, "scoop/persist/secret")
			case "traversal":
				b.Artifacts[".portscoop/backend/../../escape"] = strings.Repeat("a", 64)
			case "candidate-digest":
				candidate := filepath.Join(tx.Directory, "candidate")
				var next environment.BackendMetadata
				runtimeinstall.Read(candidate, "backend/backend.json", &next)
				next.Artifacts[environment.AdapterRelative] = strings.Repeat("a", 64)
				runtimeinstall.Write(candidate, "backend/backend.json", next)
				refreshFixtureInventory(t, candidate)
			case "controller":
				tx.Controller.Version = "changed"
			}
			if err := runtimeinstall.Write(tx.Target.Root, backendMetadata, b); err != nil {
				t.Fatal(err)
			}
			if err := (Disk{}).Prepare(tx); err == nil {
				t.Fatal("accepted invalid ownership")
			}
			if _, err := runtimeinstall.VerifyReceipt(tx.Target.Root, tx.Target.ID); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestManagedStoreReplacementFailureRollsBack(t *testing.T) {
	tx := storeTransaction(t)
	if err := (Disk{}).Prepare(tx); err != nil {
		t.Fatal(err)
	}
	if err := (Disk{}).Publish(tx); err != nil {
		t.Fatal(err)
	}
	if err := writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
		t.Fatal(err)
	}
	// An open Windows file prevents rename after earlier operations committed.
	f, err := os.Open(filepath.Join(tx.Target.Root, environment.EngineEntryRelative))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err = commit(context.Background(), tx.Directory, false); err == nil {
		t.Fatal("replacement unexpectedly succeeded")
	}
	if err = verifyInstalled(tx, true); err != nil {
		t.Fatal(err)
	}
	j, err := readJournal(tx)
	if err != nil || j.Phase != "failed" || j.Applied == 0 {
		t.Fatalf("rollback journal %#v %v", j, err)
	}
	if _, err = runtimeinstall.ReadPending(tx.Target); !os.IsNotExist(err) {
		t.Fatalf("rollback left pending: %v", err)
	}
}

func TestManagedStoreRejectsChangedControllerSnapshot(t *testing.T) {
	tx := storeTransaction(t)
	if err := (Disk{}).Prepare(tx); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, tx.Directory, "controller/psp.exe", "changed helper provenance")
	if err := verifySources(tx); err == nil {
		t.Fatal("accepted altered controller")
	}
}

func refreshFixtureInventory(t *testing.T, root string) {
	t.Helper()
	var r runtimeinstall.Release
	if err := runtimeinstall.Read(root, "release.json", &r); err != nil {
		t.Fatal(err)
	}
	err := filepath.WalkDir(root, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if e.Name() == runtimeinstall.HostControl {
				return filepath.SkipDir
			}
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		rel = filepath.ToSlash(rel)
		if rel != "release.json" {
			r.Artifacts[rel], err = runtimeinstall.Hash(root, rel)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = runtimeinstall.Write(root, "release.json", r); err != nil {
		t.Fatal(err)
	}
}
