package relocation

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/testutil"
)

func writeFixture(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func relocationFixture(t *testing.T) environment.Paths {
	t.Helper()
	root := filepath.Join(testutil.TempDir(t), "old (portable)")
	paths, _ := environment.PathsAt(root)
	for _, path := range []string{paths.State, paths.Shims, filepath.Join(paths.Apps, "tool", "1.0", "tools"), filepath.Join(paths.Persist, "tool", "data")} {
		if err := os.MkdirAll(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	version := filepath.Join(paths.Apps, "tool", "1.0")
	writeFixture(t, filepath.Join(version, "scoop-manifest.json"), []byte(`{"persist":["data","settings.ini"]}`))
	writeFixture(t, filepath.Join(version, "tools", "tool.cmd"), []byte("@echo package"))
	writeFixture(t, filepath.Join(version, "user-config.txt"), []byte(root))
	writeFixture(t, filepath.Join(paths.Persist, "tool", "settings.ini"), []byte("persisted user settings"))
	if err := os.Link(filepath.Join(paths.Persist, "tool", "settings.ini"), filepath.Join(version, "settings.ini")); err != nil {
		t.Fatal(err)
	}
	if err := createJunction(filepath.Join(version, "data"), filepath.Join(paths.Persist, "tool", "data")); err != nil {
		t.Fatal(err)
	}
	if err := createJunction(filepath.Join(paths.Apps, "tool", "current"), version); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(paths.Apps, "tool", "current", "tools", "tool.cmd")
	writeFixture(t, filepath.Join(paths.Shims, "tool.cmd"), []byte("@rem "+target+"\r\n@\""+target+"\"  %*\r\n"))
	writeFixture(t, filepath.Join(paths.Shims, "tool"), []byte("#!/bin/sh\n# "+target+"\nMSYS2_ARG_CONV_EXCL=/C cmd.exe /C \""+target+"\"  \"$@\""))
	state := map[string]any{"schema_version": 1, "values": map[string]string{"PATH": paths.Shims + ";" + filepath.Join(paths.Apps, "tool", "current", "tools"), "TOOL_HOME": filepath.Join(paths.Apps, "tool", "current"), "UNRELATED": filepath.VolumeName(root) + `\unrelated\user-data`}}
	data, _ := json.Marshal(state)
	writeFixture(t, filepath.Join(paths.State, "environment.json"), data)
	if err := (Manager{}).Capture(paths); err != nil {
		t.Fatal(err)
	}
	return paths
}

func moveFixture(t *testing.T, paths environment.Paths) environment.Paths {
	t.Helper()
	destination := filepath.Join(filepath.Dir(paths.Root), "new (portable)")
	if err := os.Rename(paths.Root, destination); err != nil {
		t.Fatal(err)
	}
	result, _ := environment.PathsAt(destination)
	return result
}

func TestDirectoryRelocationRepairsOnlyOwnedArtifacts(t *testing.T) {
	old := relocationFixture(t)
	moved := moveFixture(t, old)
	if err := (Manager{}).Prepare(moved); err != nil {
		t.Fatal(err)
	}
	shim, _ := os.ReadFile(filepath.Join(moved.Shims, "tool.cmd"))
	if strings.Contains(string(shim), old.Root) || !strings.Contains(string(shim), moved.Root) {
		t.Fatal("shim was not repaired")
	}
	for _, pair := range [][2]string{{filepath.Join(moved.Apps, "tool", "current"), filepath.Join(moved.Apps, "tool", "1.0")}, {filepath.Join(moved.Apps, "tool", "1.0", "data"), filepath.Join(moved.Persist, "tool", "data")}} {
		target, err := os.Readlink(pair[0])
		if err != nil || target != pair[1] {
			t.Fatalf("link=%s target=%s error=%v", pair[0], target, err)
		}
	}
	left, _ := os.Stat(filepath.Join(moved.Apps, "tool", "1.0", "settings.ini"))
	right, _ := os.Stat(filepath.Join(moved.Persist, "tool", "settings.ini"))
	if !os.SameFile(left, right) {
		t.Fatal("persisted hardlink changed")
	}
	config, _ := os.ReadFile(filepath.Join(moved.Apps, "tool", "1.0", "user-config.txt"))
	if string(config) != old.Root {
		t.Fatal("arbitrary application config was rewritten")
	}
	var state struct {
		Values map[string]string `json:"values"`
	}
	data, _ := os.ReadFile(filepath.Join(moved.State, "environment.json"))
	_ = json.Unmarshal(data, &state)
	if state.Values["TOOL_HOME"] != filepath.Join(moved.Apps, "tool", "current") || state.Values["UNRELATED"] != filepath.VolumeName(old.Root)+`\unrelated\user-data` {
		t.Fatalf("environment repair=%+v", state.Values)
	}
	if err := (Manager{}).Prepare(moved); err != nil {
		t.Fatal("repeat preparation:", err)
	}
}

func TestInterruptedRepairResumesWithoutCommittingFalseRoot(t *testing.T) {
	old := relocationFixture(t)
	moved := moveFixture(t, old)
	err := (Manager{AfterArtifact: func(index int) error {
		if index == 0 {
			return fmt.Errorf("injected interruption")
		}
		return nil
	}}).Prepare(moved)
	if err == nil {
		t.Fatal("injected repair did not fail")
	}
	var state Prepared
	if err := readJSON(moved.Root, filepath.Join(moved.State, "prepared.json"), &state); err != nil {
		t.Fatal(err)
	}
	if state.Root != old.Root {
		t.Fatal("partial repair committed new root")
	}
	if _, err := os.Stat(filepath.Join(moved.State, "repair.json")); err != nil {
		t.Fatal("repair journal missing")
	}
	if err := (Manager{}).Prepare(moved); err != nil {
		t.Fatal(err)
	}
	if err := readJSON(moved.Root, filepath.Join(moved.State, "prepared.json"), &state); err != nil || state.Root != moved.Root {
		t.Fatalf("committed=%s error=%v", state.Root, err)
	}
	if _, err := os.Stat(filepath.Join(moved.State, "repair.json")); !os.IsNotExist(err) {
		t.Fatal("completed journal remained")
	}
}

func TestInterruptedJunctionReplacementRecreatesMissingLink(t *testing.T) {
	old := relocationFixture(t)
	moved := moveFixture(t, old)
	if err := os.Remove(filepath.Join(moved.Apps, "tool", "current")); err != nil {
		t.Fatal(err)
	}
	if err := (Manager{}).Prepare(moved); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moved.Apps, "tool", "current", "tools", "tool.cmd")); err != nil {
		t.Fatal(err)
	}
}

func TestExternalLinksAndUnknownFormatsAreRejected(t *testing.T) {
	for _, variant := range []string{"external-link", "changed-shim", "traversal-catalog", "reparse-parent"} {
		t.Run(variant, func(t *testing.T) {
			old := relocationFixture(t)
			outside := testutil.TempDir(t)
			sentinel := filepath.Join(outside, "sentinel.txt")
			writeFixture(t, sentinel, []byte("preserve"))
			moved := moveFixture(t, old)
			switch variant {
			case "external-link":
				path := filepath.Join(moved.Apps, "tool", "current")
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := createJunction(path, outside); err != nil {
					t.Fatal(err)
				}
			case "changed-shim":
				writeFixture(t, filepath.Join(moved.Shims, "tool.cmd"), []byte("unknown format"))
			case "traversal-catalog":
				var state Prepared
				if err := readJSON(moved.Root, filepath.Join(moved.State, "prepared.json"), &state); err != nil {
					t.Fatal(err)
				}
				state.Artifacts[0].Path = "../sentinel.txt"
				if err := writeJSON(filepath.Join(moved.State, "prepared.json"), state); err != nil {
					t.Fatal(err)
				}
			case "reparse-parent":
				if err := os.Rename(moved.Shims, moved.Shims+".saved"); err != nil {
					t.Fatal(err)
				}
				if err := createJunction(moved.Shims, outside); err != nil {
					t.Fatal(err)
				}
			}
			if err := (Manager{}).Prepare(moved); err == nil {
				t.Fatal("unsafe repair accepted")
			}
			data, _ := os.ReadFile(sentinel)
			if string(data) != "preserve" {
				t.Fatal("external file changed")
			}
		})
	}
}

func TestExactRootReplacementPreservesSiblingAndSameDrivePaths(t *testing.T) {
	from := `F:\tools\work`
	to := `G:\portable\work`
	input := `F:\tools\work\apps;F:\tools\work-old\file;F:\unrelated\data;"F:\TOOLS\WORK"`
	want := `G:\portable\work\apps;F:\tools\work-old\file;F:\unrelated\data;"G:\portable\work"`
	if got := relocateText(input, from, to); got != want {
		t.Fatalf("got %s", got)
	}
}

func TestRecordedDriveLetterMoveRepairsOwnedPathState(t *testing.T) {
	// Model a disconnected previous drive without mounting or changing any
	// host drive letters. Real guest drive reassignment is release qualification.
	root := filepath.Join(testutil.TempDir(t), "portable")
	paths, _ := environment.PathsAt(root)
	old := `Z:\portscoop-fixture\work`
	for _, directory := range []string{paths.Shims, paths.State, paths.Apps} {
		if err := os.MkdirAll(directory, 0700); err != nil {
			t.Fatal(err)
		}
	}
	target := old + `\scoop\apps\tool\1.0\tool.cmd`
	data := []byte("@rem " + target + "\r\n@\"" + target + "\"  %*\r\n")
	writeFixture(t, filepath.Join(paths.Shims, "tool.cmd"), data)
	if err := writeJSON(filepath.Join(paths.State, "prepared.json"), Prepared{SchemaVersion: 1, Root: old, Artifacts: []Artifact{{Path: "scoop/shims/tool.cmd", Kind: "shim", Content: data}}}); err != nil {
		t.Fatal(err)
	}
	if err := (Manager{}).Prepare(paths); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(filepath.Join(paths.Shims, "tool.cmd"))
	if strings.Contains(string(updated), old) || !strings.Contains(string(updated), root) {
		t.Fatal("recorded drive root was not repaired")
	}
}

func TestRedirectedManagedDirectoriesBlockBackendBeforeWrites(t *testing.T) {
	for _, directory := range []string{"cache", "tmp", "global", "buckets"} {
		t.Run(directory, func(t *testing.T) {
			paths := relocationFixture(t)
			outside := testutil.TempDir(t)
			path := filepath.Join(paths.Scoop, directory)
			if err := createJunction(path, outside); err != nil {
				t.Fatal(err)
			}
			id, _ := environment.NewIdentity("fixture")
			data, _ := json.Marshal(id)
			writeFixture(t, filepath.Join(paths.Root, environment.MarkerRelative), data)
			called := false
			_, err := (Coordinator{}).Do(context.Background(), environment.Context{Identity: id, Paths: paths}, func() (int, error) { called = true; return 0, nil })
			if err == nil || called {
				t.Fatalf("error=%v called=%v", err, called)
			}
			entries, _ := os.ReadDir(outside)
			if len(entries) != 0 {
				t.Fatal("redirected directory was modified")
			}
		})
	}
}
