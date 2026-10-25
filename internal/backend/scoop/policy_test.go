package scoop

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func fixtureManifest(t *testing.T, extra map[string]any) []byte {
	t.Helper()
	m := map[string]any{"version": "1.0", "url": "https://example.test/tool.zip", "hash": strings.Repeat("a", 64), "bin": "tools/tool.cmd"}
	for key, value := range extra {
		m[key] = value
	}
	data, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCommandPolicy(t *testing.T) {
	for _, args := range [][]string{{"install", "main/tool"}, {"uninstall", "tool"}, {"update", "*"}, {"update", "tool", "--force"}, {"reset", "tool"}, {"cleanup", "*"}, {"list"}, {"search", "git|node"}, {"status"}, {"bucket", "list"}, {"bucket", "add", "main", "https://example.test/bucket.git"}, {"bucket", "remove", "extra"}, {"config", "root_path"}} {
		if _, err := Classify(args); err != nil {
			t.Errorf("rejected %v: %v", args, err)
		}
	}
	for _, args := range [][]string{nil, {"update"}, {"update", "scoop"}, {"install", "scoop"}, {"install", "main/scoop"}, {"install", "tool", "--global"}, {"install", "-gs", "tool"}, {"install", "tool", "--skip-hash-check"}, {"install", "../tool"}, {"install", `C:\tool.json`}, {"install", "tool@1.0"}, {"config", "root_path", "C:\\other"}, {"config", "rm"}, {"bucket", "add", "../main"}, {"bucket", "add", "main", "--upload-pack=bad"}, {"alias", "add", "bad", "script"}, {"scoop", "--", "install", "tool"}} {
		if _, err := Classify(args); err == nil {
			t.Errorf("accepted policy bypass %v", args)
		}
	}
}

func TestManifestSubset(t *testing.T) {
	good := fixtureManifest(t, map[string]any{"env_set": map[string]string{"TOOL_HOME": "$dir", "TOOL_DATA": "$persist_dir/data", "TOOL_FLAG": "1"}, "env_add_path": []string{"tools"}, "persist": []any{"data", []string{"settings", "configuration"}}, "shortcuts": [][]string{{"tools/tool.cmd", "Tool Shortcut"}}})
	manifest, err := DecodeManifest(good)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateManifest(manifest); err != nil {
		t.Fatal(err)
	}
	badFields := []map[string]any{
		{"pre_install": "Remove-Item host"}, {"installer": map[string]string{"file": "setup.exe"}},
		{"uninstaller": map[string]string{"script": "Invoke-Expression data"}},
		{"psmodule": map[string]string{"name": "ToolModule"}},
		{"bin": "../../host.cmd"}, {"bin": [][]string{{"tool.cmd", "../../host"}}},
		{"bin": [][]string{{"tool.cmd", "tool", "; run-command"}}},
		{"persist": [][]string{{"data", "../../host"}}}, {"extract_to": `C:\host`},
		{"env_set": map[string]string{"TOOL_HOME": "$(Get-Content secret)"}},
		{"env_set": map[string]string{"SCOOP_CACHE": "$dir"}},
		{"env_set": map[string]string{"GIT_DIR": "$dir"}},
		{"env_set": map[string]string{"TOOL_HOME": "one", "tool_home": "two"}},
		{"env_set": map[string]string{"TOOL_HOME": "`$dir"}},
		{"hash": "skip"}, {"url": "https://example.test/setup.exe", "installer": map[string]string{"file": "setup.exe"}}, {"version": "nightly"},
	}
	for i, extra := range badFields {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			m, err := DecodeManifest(fixtureManifest(t, extra))
			if err == nil {
				_, err = ValidateManifest(m)
			}
			if err == nil {
				t.Fatal("accepted unsupported manifest")
			}
		})
	}
}

func TestArchitectureOverrideAndCaseAmbiguity(t *testing.T) {
	m, err := DecodeManifest(fixtureManifest(t, map[string]any{"architecture": map[string]any{"64bit": map[string]string{"url": "https://example.test/x64.zip", "hash": strings.Repeat("b", 64)}}}))
	if err != nil {
		t.Fatal(err)
	}
	effective, err := ValidateManifest(m)
	if err != nil {
		t.Fatal(err)
	}
	if value, _ := manifestString(effective["url"]); value != "https://example.test/x64.zip" {
		t.Fatal("architecture was not selected")
	}
	if _, err := DecodeManifest([]byte(`{"version":"1.0","Version":"2.0"}`)); err == nil {
		t.Fatal("accepted ambiguous case-insensitive field")
	}
}

func TestDependencyPreflightIsAllOrNothing(t *testing.T) {
	fixtures := map[string][]byte{
		"app":     fixtureManifest(t, map[string]any{"depends": []string{"library", "utility"}}),
		"library": fixtureManifest(t, nil), "utility": fixtureManifest(t, map[string]any{"depends": "library"}),
	}
	load := func(name string) ([]byte, error) {
		data, ok := fixtures[name]
		if !ok {
			return nil, fmt.Errorf("missing manifest")
		}
		return data, nil
	}
	plan, err := Preflight([]string{"app"}, load)
	if err != nil || len(plan) != 3 || plan[0].Name != "library" || plan[2].Name != "app" {
		t.Fatalf("unexpected dependency plan: %+v %v", plan, err)
	}
	fixtures["library"] = fixtureManifest(t, map[string]any{"pre_install": "bad"})
	if plan, err := Preflight([]string{"app"}, load); err == nil || plan != nil {
		t.Fatal("unsupported dependency produced a usable mutation plan")
	}
	fixtures["library"] = fixtureManifest(t, map[string]any{"depends": "app"})
	if _, err := Preflight([]string{"app"}, load); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatal("dependency cycle was not rejected")
	}
}

func TestBucketAliasesCannotInstallConflictingDeclarations(t *testing.T) {
	fixtures := map[string][]byte{"main/tool": fixtureManifest(t, nil), "tool": fixtureManifest(t, nil), "other/tool": fixtureManifest(t, map[string]any{"version": "2.0"})}
	load := func(name string) ([]byte, error) { return fixtures[name], nil }
	if plan, err := Preflight([]string{"main/tool", "tool"}, load); err != nil || len(plan) != 1 {
		t.Fatalf("identical alias should deduplicate: %v %v", plan, err)
	}
	if _, err := Preflight([]string{"main/tool", "other/tool"}, load); err == nil {
		t.Fatal("conflicting buckets target the same installed app")
	}
}
