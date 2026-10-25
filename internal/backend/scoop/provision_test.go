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
	for _, name := range []string{"portscoop.exe", "psp.exe"} {
		fixtureFile(t, filepath.Join(release, name), []byte("fixture launcher"))
	}
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
		for _, launcher := range []string{"portscoop.exe", "psp.exe"} {
			data, _ := os.ReadFile(filepath.Join(root, launcher))
			if string(data) != "fixture launcher" {
				t.Fatal("local launcher missing")
			}
		}
		if _, err := initializer.Init(context.Background(), root, name); err == nil {
			t.Fatal("initialized root was replaced")
		}
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
