package compatibility

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func bitwardenFixture(t *testing.T) Manifest {
	return fixture(t, map[string]any{"url": "https://example.test/Bitwarden-Portable-1.0.exe", "pre_install": bitwardenRecipe, "persist": "bitwarden-appdata", "bin": "Bitwarden.exe", "shortcuts": [][]string{{"Bitwarden.exe", "Bitwarden"}}})
}

func TestBitwardenRecipe(t *testing.T) {
	p, err := Compile(bitwardenFixture(t))
	if err != nil {
		t.Fatal(err)
	}
	want := []Action{{Phase: "pre_install", Kind: "rename-package-file", Source: "Bitwarden-Portable-1.0.exe", Path: "Bitwarden.exe"}}
	if !reflect.DeepEqual(p.Actions, want) {
		t.Fatal(p.Actions)
	}
	found := false
	for _, d := range p.Decisions {
		if d.Field == "pre_install" {
			found = d.Disposition == Rewrite && strings.Contains(d.Reason, "skip reviewed optional import")
		}
	}
	if !found {
		t.Fatal("missing selective skip diagnostic")
	}
	again, err := Compile(p.Manifest)
	if err != nil || !reflect.DeepEqual(p.Manifest, again.Manifest) || !reflect.DeepEqual(p.Actions, again.Actions) {
		t.Fatalf("not idempotent: %+v %v", again, err)
	}
}

func TestBitwardenRecipeRejectsUnreviewedEffects(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"remainder":            {"pre_install": append(append([]string{}, bitwardenRecipe...), "Remove-Item $dir")},
		"copy changed":         {"pre_install": strings.Replace(strings.Join(bitwardenRecipe, "\n"), "-Recurse -Force", "-Recurse", 1)},
		"missing persistence":  {"persist": nil},
		"remapped persistence": {"persist": [][]string{{"bitwarden-appdata", "other"}}},
		"wrong payload":        {"url": "https://example.test/other.exe"},
		"version mismatch":     {"version": "2.0"},
		"AppData only":         {"pre_install": `Copy-Item "$env:AppData\Other" "$dir\data"`},
		"post install":         {"post_install": `Copy-Item "$env:AppData\Other" "$dir\data"`},
		"installer":            {"installer": map[string]string{"file": "Bitwarden.exe"}},
	} {
		t.Run(name, func(t *testing.T) {
			m := bitwardenFixture(t)
			for k, v := range extra {
				m[k], _ = json.Marshal(v)
			}
			p, err := Compile(m)
			if err == nil || p.Manifest != nil || p.Actions != nil {
				t.Fatalf("accepted unreviewed recipe: %+v", p)
			}
		})
	}
}

func TestDirectExecutableURLs(t *testing.T) {
	for _, value := range []string{"https://example.test/app.exe", "http://example.test/app-1.0.EXE"} {
		if _, err := Compile(fixture(t, map[string]any{"url": value})); err != nil {
			t.Fatal(err)
		}
	}
	for _, value := range []string{"https://example.test/app.exe?download=1", "https://example.test/app.exe?", "https://example.test/app.exe#/rename.exe", "https://example.test/%61pp.exe", "https://example.test/a%2Fb.exe", "https://example.test/NUL.exe", "https://example.test/a%20b.exe", "file:///app.exe", "https://example.test/app.msi"} {
		if _, err := Compile(fixture(t, map[string]any{"url": value})); err == nil {
			t.Fatal("accepted", value)
		}
	}
	for _, field := range []string{"extract_dir", "extract_to"} {
		if _, err := Compile(fixture(t, map[string]any{"url": "https://example.test/app.exe", field: "."})); err == nil {
			t.Fatal("accepted EXE extraction", field)
		}
	}
}
