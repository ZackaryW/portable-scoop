package scoop

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/environment"
)

func TestManagedHelpersAreStoreOwned(t *testing.T) {
	local := managedFixture(t)
	for name, relative := range helperExecutables {
		path, err := Helper(local.Paths, name)
		if err != nil || path != filepath.Join(local.Paths.Root, ToolsRelative, filepath.FromSlash(relative)) {
			t.Fatalf("helper %s: %s %v", name, path, err)
		}
	}
	if _, err := Helper(local.Paths, "arbitrary.exe"); err == nil {
		t.Fatal("arbitrary helper accepted")
	}
}

func TestManagedHelperIntegrity(t *testing.T) {
	for _, kind := range []string{"missing", "altered", "extra-plugin", "outside-namespace", "inventory-mismatch"} {
		t.Run(kind, func(t *testing.T) {
			local := managedFixture(t)
			dll := filepath.Join(local.Paths.Root, ToolsRelative, "7zip/7z.dll")
			switch kind {
			case "missing":
				if err := os.Remove(dll); err != nil {
					t.Fatal(err)
				}
			case "altered":
				fixtureFile(t, dll, []byte("changed"))
			case "extra-plugin":
				fixtureFile(t, filepath.Join(local.Paths.Root, ToolsRelative, "7zip/Codecs/unowned.dll"), []byte("plugin"))
			default:
				var b environment.BackendMetadata
				path := filepath.Join(local.Paths.Root, MetadataRelative)
				data, _ := os.ReadFile(path)
				if err := json.Unmarshal(data, &b); err != nil {
					t.Fatal(err)
				}
				if kind == "outside-namespace" {
					b.Artifacts[".portscoop/elsewhere/7z.exe"] = strings.Repeat("a", 64)
				} else {
					delete(b.Artifacts, ToolsRelative+"/7zip/7z.dll")
				}
				data, _ = json.Marshal(b)
				fixtureFile(t, path, data)
			}
			if _, err := Verify(local.Paths); err == nil {
				t.Fatal("invalid helper inventory accepted")
			}
		})
	}
}
