package scoop

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"portscoop/internal/compatibility"
	"portscoop/internal/runner"
)

func frozenDiagnostic(t *testing.T, data []byte) FrozenManifest {
	t.Helper()
	m, err := DecodeManifest(data)
	if err != nil {
		t.Fatal(err)
	}
	p, err := compatibility.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	return FrozenManifest{Manifest: p.Manifest, Decisions: p.Decisions, Actions: p.Actions, Downloads: p.Downloads}
}

func TestPinnedPotPlayerDiagnosticFormatting(t *testing.T) {
	data, err := os.ReadFile("../../compatibility/testdata/potplayer-261021.json")
	if err != nil {
		t.Fatal(err)
	}
	frozen := frozenDiagnostic(t, data)
	var normal, detailed bytes.Buffer
	reportCompatibility(&normal, map[string]FrozenManifest{"potplayer": frozen}, false)
	reportCompatibility(&detailed, map[string]FrozenManifest{"potplayer": frozen}, true)
	want := "Portable compatibility: potplayer\n" +
		"  pre_install rewrite:\n" +
		"    Clean package files: 1\n" +
		"    Initialize missing settings: 4\n" +
		"  post_install rewrite:\n" +
		"    Refresh resource folders: 7\n" +
		"    Clean package files: 7\n" +
		"  shortcuts rewrite: Store-owned launch aliases; host Start Menu integration omitted\n" +
		"Use --verbose for individual compatibility decisions.\n"
	if normal.String() != want {
		t.Fatalf("unexpected normal report:\n%s", normal.String())
	}
	for _, field := range frozen.Decisions {
		for _, op := range field.Operations {
			line := fmt.Sprintf("    operation %d, line %d: %s - %s\n", op.Position, op.Line, op.Disposition, op.Reason)
			if !strings.Contains(detailed.String(), line) {
				t.Fatalf("lost detailed operation: %s", line)
			}
		}
	}
	if !strings.Contains(detailed.String(), "autoupdate skip:") || !strings.Contains(detailed.String(), "checkver skip:") {
		t.Fatal("detailed report lost metadata decisions")
	}
	t.Log("Default output:\n" + normal.String())
}

func TestOptionalImportOmissionsRemainVisible(t *testing.T) {
	optional := `if (!(Test-Path "$dir\profile\*") -and (Test-Path "$env:APPDATA\Aurora")) { Copy-Item "$env:APPDATA\Aurora\*" "$dir\profile" -Recurse }`
	for _, mixed := range []bool{false, true} {
		hook := optional
		if mixed {
			hook = `Rename-Item "$dir\original.exe" 'final.exe'; ` + hook
		}
		frozen := frozenDiagnostic(t, fixtureManifest(t, map[string]any{"pre_install": hook, "persist": "profile"}))
		var out bytes.Buffer
		reportCompatibility(&out, map[string]FrozenManifest{"aurora": frozen}, false)
		if !strings.Contains(out.String(), "skipped (line 1): skip optional host AppData import into profile") {
			t.Fatalf("import omission hidden:\n%s", out.String())
		}
		if strings.Contains(out.String(), "Rename package files: 1") != mixed {
			t.Fatalf("rename summary incorrect:\n%s", out.String())
		}
	}
}

func TestCompatibilityOutputOrderingAndSilentMetadata(t *testing.T) {
	frozen := frozenDiagnostic(t, fixtureManifest(t, map[string]any{"shortcuts": [][]string{{"tool.exe", "Tool Launch"}}}))
	var a, b bytes.Buffer
	reportCompatibility(&a, map[string]FrozenManifest{"zebra": frozen, "alpha": frozen}, false)
	reportCompatibility(&b, map[string]FrozenManifest{"alpha": frozen, "zebra": frozen}, false)
	if a.String() != b.String() || strings.Index(a.String(), "alpha") > strings.Index(a.String(), "zebra") {
		t.Fatal("unstable package ordering")
	}
	metadata := frozenDiagnostic(t, fixtureManifest(t, map[string]any{"checkver": "1.0", "autoupdate": map[string]any{"url": "https://example.test/$version.zip"}}))
	a.Reset()
	b.Reset()
	reportCompatibility(&a, map[string]FrozenManifest{"tool": metadata}, false)
	reportCompatibility(&b, map[string]FrozenManifest{"tool": frozenDiagnostic(t, fixtureManifest(t, nil))}, false)
	if a.String() != b.String() {
		t.Fatal("routine metadata produced noise")
	}
}

func TestVerboseChangesOnlyPresentation(t *testing.T) {
	local := managedFixture(t)
	fixtureFile(t, filepath.Join(local.Paths.Buckets, "main/bucket/tool.json"), fixtureManifest(t, map[string]any{"checkver": "1.0"}))
	var requests []runner.Request
	for _, verbose := range []bool{false, true} {
		process := &captureProcess{}
		var out bytes.Buffer
		args := []string{"install", "tool"}
		if verbose {
			args = append(args, "--verbose")
		}
		_, err := (Managed{Runner: process}).Execute(context.Background(), local, args, runner.Streams{Err: &out})
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(out.String(), "checkver skip") != verbose {
			t.Fatalf("wrong report for verbose=%t: %s", verbose, out.String())
		}
		requests = append(requests, process.request)
	}
	if !reflect.DeepEqual(requests[0], requests[1]) || !reflect.DeepEqual(requests[1].Arguments, []string{"install", "tool"}) {
		t.Fatal("verbose changed the engine request")
	}
}
