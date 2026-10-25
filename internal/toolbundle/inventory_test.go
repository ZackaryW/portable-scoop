package toolbundle

import (
	"strings"
	"testing"
)

func TestInventoryRejectsIncompleteOrEscapingFiles(t *testing.T) {
	for _, kind := range []string{"valid", "notice", "escape", "unknown-tool", "case", "digest", "backslash"} {
		t.Run(kind, func(t *testing.T) {
			h := strings.Repeat("a", 64)
			i := Inventory{Pins: Pins{SchemaVersion: 1, Tools: []Tool{{Name: "git", Version: "fixture", URL: "https://example.invalid/git.zip", SHA256: h, Format: "zip", Required: []string{"git.exe", "LICENSE"}, Source: "https://example.invalid/source", SourcePackages: "https://example.invalid/sources"}}}, Artifacts: map[string]string{"git/git.exe": h, "git/LICENSE": h}}
			switch kind {
			case "notice":
				delete(i.Artifacts, "git/LICENSE")
			case "escape":
				i.Artifacts["git/../../outside"] = h
			case "unknown-tool":
				i.Artifacts["other/plugin.dll"] = h
			case "case":
				i.Artifacts["git/GIT.EXE"] = h
			case "digest":
				i.Artifacts["git/git.exe"] = "invalid"
			case "backslash":
				i.Artifacts[`git\extra.dll`] = h
			}
			err := i.Validate()
			if (err == nil) != (kind == "valid") {
				t.Fatalf("validation: %v", err)
			}
		})
	}
}
