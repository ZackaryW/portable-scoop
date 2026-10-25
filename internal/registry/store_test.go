package registry

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"portscoop/internal/environment"
)

func addFixtureEntry(d *Document, alias, root string) error {
	id, err := environment.NewIdentity(alias)
	if err != nil {
		return err
	}
	d.Entries[alias] = Entry{ID: id.ID, Path: filepath.Join(root, alias), RelativePath: "tools/" + alias}
	return nil
}

func TestStoreConcurrentWritersAndInterruptedReplace(t *testing.T) {
	root := t.TempDir()
	store := Store{Path: filepath.Join(root, "registry.json")}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	var wg sync.WaitGroup
	failures := make(chan error, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			failures <- store.Update(ctx, func(d *Document) error { return addFixtureEntry(d, fmt.Sprintf("env%d", i), root) })
		}(i)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	d, err := store.Read()
	if err != nil || len(d.Entries) != 20 {
		t.Fatalf("lost update: %d %v", len(d.Entries), err)
	}
	before, _ := os.ReadFile(store.Path)
	store.Replace = func(source, target string) error {
		if info, err := os.Stat(source); err != nil || info.Size() == 0 {
			t.Fatal("replacement wasn't fully staged")
		}
		return errors.New("injected interrupted publication")
	}
	if err := store.Update(ctx, func(d *Document) error { return addFixtureEntry(d, "extra", root) }); err == nil {
		t.Fatal("fault injection did not fail")
	}
	after, _ := os.ReadFile(store.Path)
	if string(before) != string(after) {
		t.Fatal("failed publication changed committed registry")
	}
	staging, _ := filepath.Glob(filepath.Join(root, ".registry-*.tmp"))
	if len(staging) != 0 {
		t.Fatal("uncommitted staging was not cleaned")
	}
}

func TestCorruptRegistryIsNotDiscarded(t *testing.T) {
	for _, raw := range []string{"{", `{"schema_version":99,"entries":{}}`, ""} {
		store := Store{Path: filepath.Join(t.TempDir(), "registry.json")}
		if err := os.WriteFile(store.Path, []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if err := store.Update(context.Background(), func(d *Document) error { return nil }); err == nil {
			t.Fatal("accepted corrupt registry")
		}
		data, _ := os.ReadFile(store.Path)
		if string(data) != raw {
			t.Fatal("corrupt registry was silently replaced")
		}
	}
}

func TestRegistryWriterChild(t *testing.T) {
	path := os.Getenv("PORTSCOOP_REGISTRY_TEST_PATH")
	if path == "" {
		return
	}
	alias := os.Getenv("PORTSCOOP_REGISTRY_TEST_ALIAS")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := (Store{Path: path}).Update(ctx, func(d *Document) error { return addFixtureEntry(d, alias, filepath.Dir(path)) }); err != nil {
		t.Fatal(err)
	}
}

func TestRegistrySerializesDifferentProcesses(t *testing.T) {
	path := filepath.Join(t.TempDir(), "registry.json")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var commands []*exec.Cmd
	for i := 0; i < 5; i++ {
		cmd := exec.Command(executable, "-test.run=^TestRegistryWriterChild$")
		cmd.Env = append(os.Environ(), "PORTSCOOP_REGISTRY_TEST_PATH="+path, fmt.Sprintf("PORTSCOOP_REGISTRY_TEST_ALIAS=child%d", i))
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands = append(commands, cmd)
	}
	for _, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Fatal(err)
		}
	}
	d, err := (Store{Path: path}).Read()
	if err != nil || len(d.Entries) != 5 {
		t.Fatalf("process updates lost: %d %v", len(d.Entries), err)
	}
}
