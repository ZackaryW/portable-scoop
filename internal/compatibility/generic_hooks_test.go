package compatibility

import (
	"portscoop/internal/compatibility/hooks"
	"reflect"
	"strings"
	"testing"
)

func TestGenericHookPhasesAndSkippedImports(t *testing.T) {
	for _, name := range []string{"Aurora", "OtherVendor"} {
		optional := `if (!(Test-Path "$dir\profile\*") -and (Test-Path "$env:APPDATA\` + name + `")) { Copy-Item "$env:APPDATA\` + name + `\*" "$dir\profile" -Recurse }`
		m := fixture(t, map[string]any{"pre_install": []string{`Rename-Item "$dir\original.exe" 'middle.exe'`, optional}, "post_install": `Rename-Item "$dir\middle.exe" 'final.exe'`, "persist": "profile"})
		p, err := Compile(m)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Actions) != 2 || p.Actions[0].Phase != "pre_install" || p.Actions[1].Phase != "post_install" {
			t.Fatal(p.Actions)
		}
		q, err := Compile(p.Manifest)
		if err != nil || !reflect.DeepEqual(q.Actions, p.Actions) {
			t.Fatal("recompilation", err)
		}
		p, err = Compile(fixture(t, map[string]any{"post_install": optional, "persist": "profile"}))
		if err != nil || len(p.Actions) != 0 || p.Manifest["post_install"] == nil {
			t.Fatal(p, err)
		}
	}
}
func TestCrossPhaseDependencies(t *testing.T) {
	if hooks.CheckDependencies(hooks.Plan{SkippedWrites: []string{"profile"}}, hooks.Plan{Reads: []string{"profile/config"}}) == nil {
		t.Fatal("cross-phase imported dependency accepted")
	}
}
func TestConditionSeparatorsCannotBeDiscarded(t *testing.T) {
	source := strings.ReplaceAll(strings.Join(configHook("config.xml"), "\n"), "Test-Path -Path", "Test-Path; -Path")
	if _, err := Compile(fixture(t, map[string]any{"pre_install": source, "persist": "config.xml"})); err == nil {
		t.Fatal("condition statement separator was discarded")
	}
}

func TestMappedPersistenceInitialization(t *testing.T) {
	for _, phase := range []string{"pre_install", "post_install"} {
		m := fixture(t, map[string]any{phase: `if (!(Test-Path "$persist_dir\saved.xml" -PathType Leaf)) { New-Item "$dir\package.xml" -ItemType File -Force | Out-Null }`, "persist": [][]string{{"package.xml", "saved.xml"}}})
		p, err := Compile(m)
		if err != nil || len(p.Actions) != 1 || p.Actions[0].PersistPath != "saved.xml" {
			t.Fatal(p, err)
		}
	}
}
