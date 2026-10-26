package scoop

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/runtimeinstall"
	"portscoop/internal/testutil"
)

func fixtureMain(_ context.Context, paths environment.Paths) error {
	return os.MkdirAll(filepath.Join(paths.Buckets, "main", ".git"), 0700)
}

type bootstrapGit struct {
	fail   string
	calls  []string
	origin string
}

func (g *bootstrapGit) Run(_ context.Context, c runner.Command) (int, error) {
	index := 0
	for index < len(c.Arguments) && c.Arguments[index] != "-C" {
		index++
	}
	if index+2 >= len(c.Arguments) {
		return 0, fmt.Errorf("missing working directory")
	}
	directory := c.Arguments[index+1]
	args := c.Arguments[index+2:]
	verb := args[0]
	g.calls = append(g.calls, verb)
	if verb == g.fail {
		return 1, nil
	}
	switch verb {
	case "init":
		return 0, os.MkdirAll(filepath.Join(directory, ".git"), 0700)
	case "rev-parse":
		fmt.Fprintln(c.Streams.Out, MainRevision)
	case "config":
		if len(args) > 2 && args[1] == "--local" {
			origin := g.origin
			if origin == "" {
				origin = MainRepository
			}
			fmt.Fprintln(c.Streams.Out, origin)
		}
	}
	return 0, nil
}

func TestMainBootstrapRetryAndIdempotence(t *testing.T) {
	local := managedFixture(t)
	g := &bootstrapGit{fail: "fetch"}
	b := Bootstrapper{Runner: g}
	if err := b.ensureMain(context.Background(), local.Paths); err == nil {
		t.Fatal("accepted failed fetch")
	}
	if _, err := os.Stat(filepath.Join(local.Paths.Buckets, "main")); !os.IsNotExist(err) {
		t.Fatal("published incomplete Main")
	}
	var receipt bootstrapReceipt
	if err := runtimeinstall.Read(local.Paths.Root, bootstrapReceiptPath, &receipt); err != nil || receipt.Complete || receipt.Stage == "" {
		t.Fatalf("missing retry receipt: %#v %v", receipt, err)
	}
	// Model a process interruption that left only its owned staging tree.
	fixtureFile(t, filepath.Join(local.Paths.Buckets, receipt.Stage, "partial"), []byte("partial"))
	g.fail = ""
	if err := b.ensureMain(context.Background(), local.Paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(local.Paths.Buckets, receipt.Stage)); !os.IsNotExist(err) {
		t.Fatal("owned interrupted staging retained")
	}
	main := filepath.Join(local.Paths.Buckets, "main")
	fixtureFile(t, filepath.Join(main, "local-edit.json"), []byte("preserve checkout"))
	before, _ := os.ReadFile(filepath.Join(local.Paths.Root, bootstrapReceiptPath))
	g.calls = nil
	if err := b.ensureMain(context.Background(), local.Paths); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(filepath.Join(local.Paths.Root, bootstrapReceiptPath))
	if string(before) != string(after) || strings.Join(g.calls, ",") != "config" {
		t.Fatal("repeat bootstrap rewrote or fetched Main")
	}
	if data, _ := os.ReadFile(filepath.Join(main, "local-edit.json")); string(data) != "preserve checkout" {
		t.Fatal("reset existing checkout")
	}
}

func TestMainConflictsAndAlteredToolsArePreserved(t *testing.T) {
	for _, kind := range []string{"foreign-origin", "non-git", "altered-tool", "invalid-receipt"} {
		t.Run(kind, func(t *testing.T) {
			local := managedFixture(t)
			g := &bootstrapGit{}
			main := filepath.Join(local.Paths.Buckets, "main")
			fixtureFile(t, filepath.Join(main, "sentinel"), []byte("preserve"))
			if kind != "non-git" {
				if err := os.MkdirAll(filepath.Join(main, ".git"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			switch kind {
			case "foreign-origin":
				g.origin = "https://example.invalid/main"
			case "altered-tool":
				fixtureFile(t, filepath.Join(local.Paths.Root, ToolsRelative, "git/cmd/git.exe"), []byte("changed"))
			case "invalid-receipt":
				runtimeinstall.Write(local.Paths.Root, bootstrapReceiptPath, bootstrapReceipt{Schema: 1, Repository: MainRepository, Revision: MainRevision, Stage: "../../outside"})
			}
			if err := (Bootstrapper{Runner: g}).ensureMain(context.Background(), local.Paths); err == nil {
				t.Fatal("accepted conflict")
			}
			if data, _ := os.ReadFile(filepath.Join(main, "sentinel")); string(data) != "preserve" {
				t.Fatal("changed existing data")
			}
		})
	}
}

func TestInitializationBootstrapFailureNeverPublishesReadiness(t *testing.T) {
	release := releaseFixture(t)
	target := filepath.Join(t.TempDir(), "new store")
	_, err := (Initializer{ReleaseRoot: release, Runner: &captureProcess{}, BootstrapMain: func(context.Context, environment.Paths) error { return fmt.Errorf("offline") }}).Init(context.Background(), target, "fixture")
	if err == nil || !strings.Contains(err.Error(), "bootstrap: offline") {
		t.Fatalf("failure %v", err)
	}
	if _, err = os.Stat(filepath.Join(target, environment.MarkerRelative)); !os.IsNotExist(err) {
		t.Fatal("published readiness on acquisition failure")
	}
	if _, err = os.Stat(filepath.Join(target, "scoop")); !os.IsNotExist(err) {
		t.Fatal("published partial Store")
	}
}

func TestBootstrapTwoStoresAndRelocation(t *testing.T) {
	a, b := managedFixture(t), managedFixture(t)
	bootstrap := Bootstrapper{Runner: &bootstrapGit{}}
	if err := bootstrap.ensureMain(context.Background(), a.Paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(b.Paths.Buckets, "main")); !os.IsNotExist(err) {
		t.Fatal("other Store modified")
	}
	moved := filepath.Join(testutil.TempDir(t), "relocated")
	previous := a.Paths.Root
	if err := os.Rename(previous, moved); err != nil {
		t.Fatal(err)
	}
	paths, err := environment.PathsAt(moved)
	if err != nil {
		t.Fatal(err)
	}
	if err := bootstrap.ensureMain(context.Background(), paths); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(previous); !os.IsNotExist(err) {
		t.Fatal("old root recreated")
	}
}
