package scoop

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestSavedReinstallBackupsRemainFullyValidated(t *testing.T) {
	for _, name := range []string{"_1.0.old", "_1.0.old(1)", "_1.0.old(12)"} {
		for _, mutation := range []string{"valid", "manifest-version", "hook", "architecture", "source", "missing-manifest"} {
			t.Run(name+"/"+mutation, func(t *testing.T) {
				local := managedFixture(t)
				base := filepath.Join(local.Paths.Apps, "app", name)
				manifest := managedManifest("1.0")
				install := `{"architecture":"64bit","bucket":"main"}`
				switch mutation {
				case "manifest-version":
					manifest = managedManifest("2.0")
				case "hook":
					manifest = []byte(strings.TrimSuffix(string(manifest), "}") + `,"post_uninstall":"Unknown-Command"}`)
				case "architecture":
					install = `{"architecture":"32bit","bucket":"main"}`
				case "source":
					install = `{"architecture":"64bit","bucket":"main","url":"https://example.test/override"}`
				}
				fixtureFile(t, filepath.Join(base, "install.json"), []byte(install))
				if mutation != "missing-manifest" {
					fixtureFile(t, filepath.Join(base, "manifest.json"), manifest)
				}
				err := validateSaved(local.Paths, []string{"app"})
				if (err == nil) != (mutation == "valid") {
					t.Fatalf("saved backup result: %v", err)
				}
			})
		}
	}
	for _, name := range []string{"_1.0", "_1.0.old(0)", "_1.0.old(01)", "_1.0.old(x)", "_1.0.old(1)extra"} {
		t.Run(name, func(t *testing.T) {
			local := managedFixture(t)
			base := filepath.Join(local.Paths.Apps, "app", name)
			fixtureFile(t, filepath.Join(base, "manifest.json"), managedManifest("1.0"))
			fixtureFile(t, filepath.Join(base, "install.json"), []byte(`{"architecture":"64bit","bucket":"main"}`))
			if err := validateSaved(local.Paths, []string{"app"}); err == nil {
				t.Fatal("accepted noncanonical backup")
			}
		})
	}
}
