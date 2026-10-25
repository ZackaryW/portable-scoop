package scoop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func releaseFixture(t *testing.T) string {
	t.Helper()
	local := managedFixture(t)
	release := testutil.TempDir(t)
	if err := copyTree(filepath.Dir(filepath.Dir(local.Paths.Engine)), filepath.Join(release, "backend", "engine")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"entry.ps1", "request-reader.ps1"} {
		data, err := os.ReadFile(filepath.Join(filepath.Dir(local.Paths.Adapter), name))
		if err != nil {
			t.Fatal(err)
		}
		fixtureFile(t, filepath.Join(release, "backend", "adapter", name), data)
	}
	data, err := os.ReadFile(filepath.Join(local.Paths.Root, MetadataRelative))
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(release, "backend", "backend.json"), data)
	for _, name := range []string{"portscoop.exe", "psp.exe", "pspe.exe"} {
		fixtureFile(t, filepath.Join(release, name), []byte("fixture launcher"))
	}
	fixtureFile(t, filepath.Join(release, "pspe.exe"), []byte("separate execution launcher"))
	metadata := Release{SchemaVersion: 1, PatchRevision: PatchRevision, Artifacts: map[string]string{}}
	if err := filepath.WalkDir(release, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		hash := sha256.Sum256(data)
		relative, _ := filepath.Rel(release, path)
		metadata.Artifacts[filepath.ToSlash(relative)] = hex.EncodeToString(hash[:])
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	encoded, _ := json.Marshal(metadata)
	fixtureFile(t, filepath.Join(release, "release.json"), encoded)
	return release
}

func TestInitializationPublishesUniqueReadyRootsAndPrivateLayout(t *testing.T) {
	release := releaseFixture(t)
	initializer := Initializer{ReleaseRoot: release}
	var previous string
	for _, name := range []string{"work", "personal"} {
		root := filepath.Join(testutil.TempDir(t), name+" (portable)")
		id, err := initializer.Init(context.Background(), root, name)
		if err != nil {
			t.Fatal(err)
		}
		if id.ID == previous {
			t.Fatal("initialization reused an installation identity")
		}
		previous = id.ID
		loaded, paths, err := environment.Load(environment.OSFilesystem{}, root)
		if err != nil || loaded != id {
			t.Fatalf("published identity: %+v %v", loaded, err)
		}
		if _, err := Verify(paths); err != nil {
			t.Fatal(err)
		}
		for _, directory := range []string{paths.State, paths.Apps, paths.Buckets, paths.Cache, paths.Persist, paths.Shims, paths.Global, paths.Temp} {
			if info, err := os.Stat(directory); err != nil || !info.IsDir() {
				t.Fatalf("private directory unavailable: %s %v", directory, err)
			}
		}
		for _, launcher := range []string{"portscoop.exe", "psp.exe", "pspe.exe"} {
			data, _ := os.ReadFile(filepath.Join(root, launcher))
			expected, _ := os.ReadFile(filepath.Join(release, launcher))
			if string(data) != string(expected) {
				t.Fatal("local launcher missing")
			}
		}
		if _, err := initializer.Init(context.Background(), root, name); err == nil {
			t.Fatal("initialized root was replaced")
		}
	}
}

func TestReleaseRequiresVerifiedExecutionArtifact(t *testing.T) {
	for _, kind := range []string{"missing", "altered", "unrecorded"} {
		t.Run(kind, func(t *testing.T) {
			release := releaseFixture(t)
			switch kind {
			case "missing":
				if err := os.Remove(filepath.Join(release, "pspe.exe")); err != nil {
					t.Fatal(err)
				}
			case "altered":
				fixtureFile(t, filepath.Join(release, "pspe.exe"), []byte("altered"))
			case "unrecorded":
				data, _ := os.ReadFile(filepath.Join(release, "release.json"))
				var r Release
				if err := json.Unmarshal(data, &r); err != nil {
					t.Fatal(err)
				}
				delete(r.Artifacts, "pspe.exe")
				data, _ = json.Marshal(r)
				fixtureFile(t, filepath.Join(release, "release.json"), data)
			}
			if _, err := VerifyRelease(release); err == nil {
				t.Fatal("incomplete execution release accepted")
			}
			root := filepath.Join(testutil.TempDir(t), "store")
			if _, err := (Initializer{ReleaseRoot: release}).Init(context.Background(), root, "blocked"); err == nil {
				t.Fatal("invalid execution release published a Store")
			}
			if _, err := os.Stat(filepath.Join(root, environment.MarkerRelative)); !os.IsNotExist(err) {
				t.Fatal("ready marker published", err)
			}
		})
	}
}

func TestInitializationRejectsUnrelatedDirectoryWithoutChangingFiles(t *testing.T) {
	root := testutil.TempDir(t)
	sentinel := filepath.Join(root, "my-file.txt")
	fixtureFile(t, sentinel, []byte("preserve"))
	_, err := (Initializer{ReleaseRoot: releaseFixture(t)}).Init(context.Background(), root, "work")
	if err == nil || !strings.Contains(err.Error(), "existing files were preserved") {
		t.Fatalf("error=%v", err)
	}
	data, _ := os.ReadFile(sentinel)
	if string(data) != "preserve" {
		t.Fatal("unrelated file changed")
	}
	if _, err := os.Stat(filepath.Join(root, environment.MarkerRelative)); !os.IsNotExist(err) {
		t.Fatal("failed root became ready")
	}
}

func TestInitializationFailurePreservesVerifiedRelease(t *testing.T) {
	release := releaseFixture(t)
	target := filepath.Join(testutil.TempDir(t), "portable")
	if err := copyTree(release, target); err != nil {
		t.Fatal(err)
	}
	_, err := (Initializer{ReleaseRoot: release, Runner: &captureProcess{}, BeforePublish: func() error { return fmt.Errorf("injected publication failure") }}).Init(context.Background(), target, "work")
	if err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatalf("error=%v", err)
	}
	if _, err := VerifyRelease(target); err != nil {
		t.Fatal("preexisting release damaged:", err)
	}
	staging, _ := filepath.Glob(filepath.Join(target, ".portscoop-init-*"))
	if len(staging) != 0 {
		t.Fatal("owned staging was not cleaned")
	}
	if _, err := os.Stat(filepath.Join(target, "scoop")); !os.IsNotExist(err) {
		t.Fatal("failed initialization published package state")
	}
}

type unsupportedRuntime struct{}

func (unsupportedRuntime) Run(_ context.Context, c runner.Command) (int, error) {
	_, err := c.Streams.Out.Write([]byte(`{"version":"5.1.1","language":"ConstrainedLanguage","x64":true,"junction":"Junction"}`))
	return 0, err
}

func TestInitializationRejectsUnsupportedRuntimeBeforeReadiness(t *testing.T) {
	root := filepath.Join(testutil.TempDir(t), "root")
	_, err := (Initializer{ReleaseRoot: releaseFixture(t), Runner: unsupportedRuntime{}}).Init(context.Background(), root, "work")
	if err == nil || !strings.Contains(err.Error(), "FullLanguage") {
		t.Fatalf("error=%v", err)
	}
	if _, err := os.Stat(filepath.Join(root, environment.MarkerRelative)); !os.IsNotExist(err) {
		t.Fatal("unsupported runtime published readiness")
	}
}

func TestHostWithDefaultStoreCanInitializeMoreStores(t *testing.T) {
	release := releaseFixture(t)
	initializer := Initializer{ReleaseRoot: release}
	store := filepath.Join(release, environment.DefaultStoreDirectory)
	id, err := initializer.Init(context.Background(), store, "Default")
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(store, "user-data.txt"), []byte("preserve"))
	if _, err := VerifyRelease(release); err == nil {
		t.Fatal("download verification accepted runtime Store data")
	}
	other := filepath.Join(testutil.TempDir(t), "other")
	if _, err := initializer.Init(context.Background(), other, "Other"); err != nil {
		t.Fatal("default Store blocked subsequent initialization:", err)
	}
	loaded, _, err := environment.Load(environment.OSFilesystem{}, store)
	if err != nil || loaded != id {
		t.Fatalf("default Store changed: %+v %v", loaded, err)
	}
	if _, err := os.Stat(filepath.Join(other, environment.DefaultStoreDirectory)); !os.IsNotExist(err) {
		t.Fatal("runtime Store was copied into another Store")
	}
	fixtureFile(t, filepath.Join(release, "backend", "engine", "bin", "scoop.ps1"), []byte("tampered"))
	if _, err := initializer.Init(context.Background(), filepath.Join(testutil.TempDir(t), "blocked"), "Blocked"); err == nil || !strings.Contains(err.Error(), "artifact changed") {
		t.Fatalf("runtime Store hid Host payload tampering: %v", err)
	}
}

func TestHostSourceRejectsUnreadyOrRedirectedDefaultStore(t *testing.T) {
	for _, kind := range []string{"unready", "file", "link"} {
		t.Run(kind, func(t *testing.T) {
			release := releaseFixture(t)
			store := filepath.Join(release, environment.DefaultStoreDirectory)
			switch kind {
			case "unready":
				fixtureFile(t, filepath.Join(store, "unrelated.txt"), []byte("preserve"))
			case "file":
				fixtureFile(t, store, []byte("preserve"))
			case "link":
				outside := testutil.NewRoot(t, "outside")
				if err := os.Symlink(outside.Path, store); err != nil {
					t.Skip("directory symlinks unavailable:", err)
				}
			}
			if _, err := verifyRelease(release, true); err == nil {
				t.Fatal("invalid default Store accepted as a runtime directory")
			}
		})
	}
}
