package compatibility

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, extra map[string]any) Manifest {
	t.Helper()
	m := map[string]any{"version": "1.0", "url": "https://example.test/app.zip", "hash": strings.Repeat("a", 64), "bin": "app.exe"}
	for k, v := range extra {
		m[k] = v
	}
	data, _ := json.Marshal(m)
	out, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func configHook(name string) []string {
	return []string{`if (-not (Test-Path -Path "$persist_dir\` + name + `" -PathType Leaf)) {`, `    New-Item -Path "$dir\` + name + `" -ItemType File -Force | Out-Null`, `}`}
}

func TestCompileDeclarativeFieldsAndSelectedArchitecture(t *testing.T) {
	m := fixture(t, map[string]any{"architecture": map[string]any{"64bit": map[string]any{"hash": strings.Repeat("b", 64)}, "arm64": map[string]any{"pre_install": "unsupported"}}, "persist": []any{"data", []string{"settings", "config"}}, "env_set": map[string]string{"APP_DATA": "$persist_dir/data"}, "env_add_path": ".", "extract_to": ".", "checkver": map[string]string{"script": "never evaluated"}, "##": "comment"})
	p, err := Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	if string(p.Manifest["hash"]) != `"`+strings.Repeat("b", 64)+`"` || p.Manifest["architecture"] != nil || p.Manifest["checkver"] != nil {
		t.Fatalf("bad compiled declaration: %+v", p)
	}
	if len(p.Decisions) != len(m) {
		t.Fatalf("missing per-field decision: %+v", p.Decisions)
	}
	for _, d := range p.Decisions {
		if d.Field == "checkver" && d.Disposition != Skip {
			t.Fatal(d)
		}
	}
}

func TestFailuresAreStableAndHaveNoExecutablePlan(t *testing.T) {
	m := fixture(t, map[string]any{"zzz": true, "installer": map[string]string{"file": "setup.exe"}, "psmodule": map[string]string{"name": "Module"}})
	var previous string
	for i := 0; i < 20; i++ {
		p, err := Compile(m)
		if err == nil || p.Manifest != nil || p.Actions != nil {
			t.Fatal("rejected manifest yielded executable plan")
		}
		if previous != "" && previous != err.Error() {
			t.Fatal("unstable diagnostics")
		}
		previous = err.Error()
		if !strings.Contains(previous, "installer:") || !strings.Contains(previous, "psmodule:") || !strings.Contains(previous, "zzz:") {
			t.Fatal(previous)
		}
	}
}

func TestDigestAlgorithmsAndInvalidInputs(t *testing.T) {
	for _, hash := range []string{strings.Repeat("a", 64), "sha256:" + strings.Repeat("b", 64), "sha1:" + strings.Repeat("c", 40), "sha512:" + strings.Repeat("d", 128), "md5:" + strings.Repeat("e", 32)} {
		if _, err := Compile(fixture(t, map[string]any{"hash": hash})); err != nil {
			t.Fatal(hash, err)
		}
	}
	for _, hash := range []any{"sha1:" + strings.Repeat("a", 64), "skip", "sha999:" + strings.Repeat("a", 64), nil, []string{strings.Repeat("a", 64), strings.Repeat("b", 64)}} {
		if _, err := Compile(fixture(t, map[string]any{"hash": hash})); err == nil {
			t.Fatalf("accepted digest %v", hash)
		}
	}
}

func TestPortableRewritesAndIdempotence(t *testing.T) {
	m := fixture(t, map[string]any{"hash": "sha1:" + strings.Repeat("a", 40), "pre_install": configHook("SpaceSnifferConfig.xml"), "persist": "SpaceSnifferConfig.xml", "bin": "SpaceSniffer.exe", "shortcuts": [][]string{{"SpaceSniffer.exe", "SpaceSniffer"}, {"SpaceSniffer.exe", "Disk View", `--mode "read only"`}}})
	p, err := Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Actions, []Action{{Phase: "pre_install", Kind: "ensure-persisted-file", Path: "SpaceSnifferConfig.xml"}}) {
		t.Fatal(p.Actions)
	}
	var bins [][]string
	_ = json.Unmarshal(p.Manifest["bin"], &bins)
	if len(bins) != 2 || bins[1][1] != "Disk-View" || p.Manifest["shortcuts"] != nil {
		t.Fatal(bins)
	}
	again, err := Compile(p.Manifest)
	if err != nil || !reflect.DeepEqual(again.Manifest, p.Manifest) || !reflect.DeepEqual(again.Actions, p.Actions) {
		t.Fatalf("not idempotent: %+v %v", again, err)
	}
}

func TestUnsafeAndUnsupportedEffects(t *testing.T) {
	for name, extra := range map[string]map[string]any{
		"hook remainder":         {"pre_install": append(configHook("config.xml"), "Remove-Item elsewhere"), "persist": "config.xml"},
		"hook traversal":         {"pre_install": configHook("../config.xml"), "persist": "config.xml"},
		"hook missing persist":   {"pre_install": configHook("config.xml")},
		"hook different persist": {"pre_install": configHook("config.xml"), "persist": [][]string{{"config.xml", "elsewhere.xml"}}},
		"alias collision":        {"bin": "app.exe", "shortcuts": [][]string{{"other.exe", "app"}}},
		"alias args collision":   {"bin": [][]string{{"app.exe", "app", "--a"}}, "shortcuts": [][]string{{"app.exe", "app", "--b"}}},
		"shortcut icon":          {"shortcuts": [][]string{{"app.exe", "app", "", "icon.ico"}}},
		"shortcut path":          {"shortcuts": [][]string{{"../app.exe", "app"}}},
		"script args":            {"bin": [][]string{{"app.cmd", "app", "--literal"}}},
		"dynamic args":           {"bin": [][]string{{"app.exe", "app", "$dir"}}},
		"device alias":           {"bin": [][]string{{"app.exe", "NUL"}}},
		"device zero alias":      {"bin": [][]string{{"app.exe", "COM0"}}},
		"device config":          {"pre_install": configHook("COM0.xml"), "persist": "COM0.xml"},
		"traversal":              {"persist": "../host"},
		"path expression":        {"env_add_path": "$(Get-Content secret)"},
		"environment expression": {"env_set": map[string]string{"X": "$(Get-Content secret)"}},
		"global env":             {"env_set": map[string]string{"SCOOP": "$dir"}},
		"null URL":               {"url": nil},
		"reserved version":       {"version": "current"},
		"missing architecture":   {"architecture": map[string]any{"arm64": map[string]any{}}},
		"nested architecture":    {"architecture": map[string]any{"64bit": map[string]any{"architecture": map[string]any{}}}},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(fixture(t, extra)); err == nil {
				t.Fatal("accepted unsupported effects")
			}
		})
	}
}

func TestDecodeRejectsDuplicateKeys(t *testing.T) {
	for _, data := range []string{`{"version":"1","version":"2"}`, `{"version":"1","Version":"2"}`, `null`, `{} {}`} {
		if _, err := Decode([]byte(data)); err == nil {
			t.Fatal(data)
		}
	}
}
