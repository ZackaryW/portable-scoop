package selfupdate

import (
	"archive/zip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"portscoop/internal/environment"
	"portscoop/internal/runtimeinstall"
)

type archiveEntry struct{ name, content string }

func writeArchive(t *testing.T, entries []archiveEntry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "release.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	z := zip.NewWriter(f)
	for _, entry := range entries {
		w, err := z.Create(entry.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(entry.content)); err != nil {
			t.Fatal(err)
		}
	}
	if err = z.Close(); err != nil {
		t.Fatal(err)
	}
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func releaseEntries(t *testing.T) []archiveEntry {
	t.Helper()
	marshal := func(v any) string {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	digest := func(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }
	upstream := strings.Repeat("a", 40)
	files := map[string]string{
		"portscoop.exe": "inert management fixture", "psp.exe": "inert management fixture",
		"pspe.exe": "inert execution fixture", "backend/adapter/entry.ps1": "# adapter",
		"backend/engine/bin/scoop.ps1": "# engine", "docs/nested/guide.md": "fixture documentation",
		"backend/engine/.hidden": "hidden payload",
	}
	files["backend/backend.json"] = marshal(environment.BackendMetadata{
		SchemaVersion: 1, ProtocolMajor: 1, UpstreamRevision: upstream, PatchRevision: "fixture-v1",
		Artifacts: map[string]string{environment.AdapterRelative: digest(files["backend/adapter/entry.ps1"]), environment.EngineEntryRelative: digest(files["backend/engine/bin/scoop.ps1"])},
	})
	files["runtime-update.json"] = marshal(runtimeinstall.Descriptor{
		Schema: 1, Protocol: 1, Version: "0.3.2-fixture", Platform: "windows-amd64",
		Kinds: []string{"host", "store"}, Launchers: runtimeinstall.Launchers,
		Backends: []runtimeinstall.Contract{{Schema: 1, Protocol: 1, Upstream: upstream, Patch: "fixture-v1"}},
	})
	inventory := map[string]string{}
	for name, content := range files {
		inventory[name] = digest(content)
	}
	files["release.json"] = marshal(runtimeinstall.Release{SchemaVersion: 1, PatchRevision: "fixture-v1", Artifacts: inventory})
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := []archiveEntry{{"docs/", ""}}
	for _, name := range names {
		entries = append(entries, archiveEntry{name, files[name]})
	}
	return entries
}

func TestVerifyArchiveUsesUpdaterChecks(t *testing.T) {
	for _, kind := range []string{"canonical", "backslash", "duplicate", "traversal", "missing", "changed"} {
		t.Run(kind, func(t *testing.T) {
			entries := releaseEntries(t)
			switch kind {
			case "backslash":
				for i := range entries {
					entries[i].name = strings.ReplaceAll(entries[i].name, "/", `\`)
				}
			case "duplicate":
				entries = append(entries, archiveEntry{"DOCS/NESTED/GUIDE.MD", "collision"})
			case "traversal":
				entries = append(entries, archiveEntry{"../outside.txt", "escape"})
			case "missing", "changed":
				for i := range entries {
					if entries[i].name == "pspe.exe" {
						if kind == "missing" {
							entries = append(entries[:i], entries[i+1:]...)
						} else {
							entries[i].content = "altered"
						}
						break
					}
				}
			}
			archive := writeArchive(t, entries)
			temporary := t.TempDir()
			t.Setenv("TMP", temporary)
			t.Setenv("TEMP", temporary)
			t.Setenv("TMPDIR", temporary)
			err := VerifyArchive(archive)
			if (err == nil) != (kind == "canonical") {
				t.Fatalf("%s: %v", kind, err)
			}
			if kind == "backslash" && !strings.Contains(err.Error(), "noncanonical ZIP path") {
				t.Fatal(err)
			}
			remaining, err := os.ReadDir(temporary)
			if err != nil || len(remaining) != 0 {
				t.Fatalf("temporary files left behind: %v %v", remaining, err)
			}
		})
	}
}
