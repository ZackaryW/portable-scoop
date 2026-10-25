package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestAuditCountsDependencyClosureAndBlockerCombinations(t *testing.T) {
	manifest := func(extra map[string]any) []byte {
		m := map[string]any{"version": "1.0", "url": "https://example.test/tool.zip", "hash": strings.Repeat("a", 64)}
		for k, v := range extra {
			m[k] = v
		}
		b, _ := json.Marshal(m)
		return b
	}
	data := map[string][]byte{
		"main/ok": manifest(nil), "main/bad": manifest(map[string]any{"pre_install": "Unknown-Command", "installer": map[string]string{"file": "setup.exe"}}),
		"extras/needsbad": manifest(map[string]any{"depends": "bad"}), "extras/needsok": manifest(map[string]any{"depends": "ok"}),
		"extras/missing": manifest(map[string]any{"depends": "other/absent"}), "extras/invalid": []byte(`{`),
	}
	buckets := []bucketResult{{Name: "main"}, {Name: "extras"}}
	got := analyze(data, buckets)
	if got[0].Total != 2 || got[0].ManifestAccepted != 1 || got[0].InstallPreflightAccepted != 1 || got[0].BlockerCombinations["installer+pre_install"] != 1 {
		t.Fatalf("main: %+v", got[0])
	}
	if got[1].Total != 4 || got[1].ManifestAccepted != 3 || got[1].InstallPreflightAccepted != 1 || len(got[1].ClosureBlocked) != 2 || got[1].BlockerCombinations["invalid_json"] != 1 {
		t.Fatalf("extras: %+v", got[1])
	}
	for _, b := range got {
		if len(b.Accepted) != b.InstallPreflightAccepted {
			t.Fatal("accepted list differs from count")
		}
	}
	first, _ := json.Marshal(got)
	for i := 0; i < 10; i++ {
		again := analyze(data, []bucketResult{{Name: "main"}, {Name: "extras"}})
		encoded, _ := json.Marshal(again)
		if !reflect.DeepEqual(encoded, first) {
			t.Fatal("nondeterministic report")
		}
	}
}
