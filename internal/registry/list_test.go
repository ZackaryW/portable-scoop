package registry_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/testutil"
)

type slowFiles struct {
	environment.OSFilesystem
	slow    string
	release <-chan struct{}
}

func (s slowFiles) Canonical(path string) (string, error) {
	if path == s.slow {
		<-s.release
	}
	return s.OSFilesystem.Canonical(path)
}

func TestListRetainsDisconnectedIdentityAndBoundsInspection(t *testing.T) {
	work, slow := testutil.NewRoot(t, "work"), testutil.NewRoot(t, "slow")
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(t.TempDir(), "registry.json")}}
	for alias, root := range map[string]string{"work": work.Path, "slow": slow.Path} {
		if err := m.Register(context.Background(), root, alias); err != nil {
			t.Fatal(err)
		}
	}
	if err := m.SetDefault(context.Background(), "work"); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(m.Store.Path)
	release := make(chan struct{})
	defer close(release)
	m.Files = slowFiles{slow: slow.Path, release: release}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	start := time.Now()
	rows, err := m.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if time.Since(start) > time.Second {
		t.Fatal("listing waited for unavailable media")
	}
	if len(rows) != 2 || rows[0].Alias != "slow" || !strings.Contains(rows[0].Status, "deadline") || rows[1].Status != "available" || !rows[1].Default {
		t.Fatalf("unexpected availability: %+v", rows)
	}
	after, _ := os.ReadFile(m.Store.Path)
	if string(before) != string(after) {
		t.Fatal("list changed registry metadata")
	}
}

func TestListReportsWrongIdentityAtSavedLocation(t *testing.T) {
	root := testutil.NewRoot(t, "work")
	m := registry.Manager{Store: registry.Store{Path: filepath.Join(t.TempDir(), "registry.json")}}
	if err := m.Register(context.Background(), root.Path, "work"); err != nil {
		t.Fatal(err)
	}
	other, _ := environment.NewIdentity("other")
	testutil.WriteIdentity(t, root.Path, other)
	rows, err := m.List(context.Background())
	if err != nil || rows[0].Status != "identity-mismatch" {
		t.Fatalf("wrong identity not reported: %+v %v", rows, err)
	}
}
