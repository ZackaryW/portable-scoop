package resolver_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/testutil"
)

func driveFixture(t *testing.T, volume, relative string, id environment.Identity) string {
	t.Helper()
	root := filepath.Join(volume, relative)
	testutil.WriteIdentity(t, root, id)
	if err := os.WriteFile(filepath.Join(root, "portscoop.exe"), []byte("fixture launcher"), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestRediscoveryRejectsWrongCachedIdentityAndFindsMovedRoot(t *testing.T) {
	oldVolume, newVolume := testutil.TempDir(t), testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	other, _ := environment.NewIdentity("other")
	relative := filepath.Join("tools", "work")
	cached := driveFixture(t, oldVolume, relative, other)
	moved := driveFixture(t, newVolume, relative, id)
	s := resolver.Selection{ID: id.ID, Hints: []registry.Entry{{ID: id.ID, Path: cached, RelativePath: relative, VolumeID: "portable-volume"}}}
	d := resolver.Discovery{Volumes: testutil.Volumes{{Root: newVolume, ID: "portable-volume"}, {Root: oldVolume, ID: "different-volume"}}}
	target, err := d.Resolve(context.Background(), s)
	if err != nil || target.Paths.Root != moved || target.VolumeID != "portable-volume" {
		t.Fatalf("relocated identity was not found: %+v %v", target, err)
	}
}

type blockedCandidate struct {
	environment.OSFilesystem
	blocked string
	release <-chan struct{}
}

func (b blockedCandidate) Canonical(path string) (string, error) {
	if path == b.blocked {
		<-b.release
	}
	return b.OSFilesystem.Canonical(path)
}

func TestUnavailableCandidateDoesNotPreventBoundedDiscovery(t *testing.T) {
	volume := testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	relative := "work"
	root := driveFixture(t, volume, relative, id)
	blocked := filepath.Join(testutil.TempDir(t), relative)
	release := make(chan struct{})
	defer close(release)
	d := resolver.Discovery{Files: blockedCandidate{blocked: blocked, release: release}, Volumes: testutil.Volumes{{Root: volume, ID: "volume"}}, ProbeTimeout: 100 * time.Millisecond}
	selection := resolver.Selection{ID: id.ID, Hints: []registry.Entry{{ID: id.ID, Path: blocked, RelativePath: relative}}}
	start := time.Now()
	target, err := d.Resolve(context.Background(), selection)
	if err != nil || target.Paths.Root != root || time.Since(start) > time.Second {
		t.Fatalf("unavailable media blocked discovery: %+v %v", target, err)
	}
}

func TestContainingDirectoryMoveNeedsRefreshedHint(t *testing.T) {
	volume := testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	driveFixture(t, volume, "new-folder", id)
	selection := resolver.Selection{ID: id.ID, Hints: []registry.Entry{{ID: id.ID, Path: filepath.Join(volume, "old-folder"), RelativePath: "old-folder"}}}
	if _, err := (resolver.Discovery{Volumes: testutil.Volumes{{Root: volume}}}).Resolve(context.Background(), selection); err == nil || !strings.Contains(err.Error(), "env register") {
		t.Fatal("discovery searched outside bounded hints")
	}
}

func TestCachedMatchCannotHideDuplicateIdentity(t *testing.T) {
	firstVolume, secondVolume := testutil.TempDir(t), testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	first := driveFixture(t, firstVolume, "work", id)
	second := driveFixture(t, secondVolume, "work", id)
	selection := resolver.Selection{ID: id.ID, Hints: []registry.Entry{{ID: id.ID, Path: first, RelativePath: "work"}}}
	_, err := (resolver.Discovery{Volumes: testutil.Volumes{{Root: firstVolume, ID: "first"}, {Root: secondVolume, ID: "second"}}}).Resolve(context.Background(), selection)
	if err == nil || !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), first) || !strings.Contains(err.Error(), second) {
		t.Fatalf("duplicate cached identity was not rejected: %v", err)
	}
}

type equivalentCandidate struct {
	environment.OSFilesystem
	alias, root string
}

func (f equivalentCandidate) Canonical(path string) (string, error) {
	if path == f.alias {
		return f.root, nil
	}
	return f.OSFilesystem.Canonical(path)
}

func TestEquivalentCanonicalPathsAreOneCandidate(t *testing.T) {
	volume := testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	root := driveFixture(t, volume, "work", id)
	alias := filepath.Join(testutil.TempDir(t), "alias")
	selection := resolver.Selection{ID: id.ID, Hints: []registry.Entry{{ID: id.ID, Path: root, RelativePath: "work"}, {ID: id.ID, Path: alias, RelativePath: "work"}}}
	target, err := (resolver.Discovery{Files: equivalentCandidate{alias: alias, root: root}, Volumes: testutil.Volumes{}}).Resolve(context.Background(), selection)
	if err != nil || target.Paths.Root != root {
		t.Fatalf("equivalent paths were treated as duplicate roots: %+v %v", target, err)
	}
}

func TestVerifiedRefreshPreservesDefaultAndOtherIdentities(t *testing.T) {
	volume := testutil.TempDir(t)
	id, _ := environment.NewIdentity("work")
	root := driveFixture(t, volume, "work", id)
	other := testutil.NewRoot(t, "personal")
	store := registry.Store{Path: filepath.Join(testutil.TempDir(t), "registry.json")}
	if err := store.Update(context.Background(), func(d *registry.Document) error {
		d.Entries["work"] = registry.Entry{ID: id.ID, Path: filepath.Join(volume, "old"), RelativePath: "old"}
		d.Entries["personal"] = registry.Entry{ID: other.Identity.ID, Path: other.Path, RelativePath: "personal"}
		d.DefaultID = id.ID
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	_, paths, err := environment.Load(environment.OSFilesystem{}, root)
	if err != nil {
		t.Fatal(err)
	}
	if err := resolver.Refresh(context.Background(), store, resolver.Target{Identity: id, Paths: paths, VolumeID: "verified-volume"}); err != nil {
		t.Fatal(err)
	}
	d, err := store.Read()
	if err != nil || d.DefaultID != id.ID || d.Entries["work"].Path != root || d.Entries["work"].VolumeID != "verified-volume" || d.Entries["personal"].Path != other.Path {
		t.Fatalf("refresh changed unrelated selection state: %+v %v", d, err)
	}
}
