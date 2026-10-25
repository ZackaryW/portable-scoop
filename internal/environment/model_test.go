package environment

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityRoundTripAndRejections(t *testing.T) {
	i, err := NewIdentity("work")
	if err != nil {
		t.Fatal(err)
	}
	other, _ := NewIdentity("work")
	if i.ID == other.ID {
		t.Fatal("identities were reused")
	}
	data, _ := json.Marshal(i)
	if decoded, err := DecodeIdentity(data); err != nil || decoded != i {
		t.Fatalf("round trip: %+v %v", decoded, err)
	}
	for name, change := range map[string]func(*Identity){
		"version": func(i *Identity) { i.SchemaVersion++ }, "id": func(i *Identity) { i.ID = "../other" },
		"protocol": func(i *Identity) { i.ProtocolMajor++ }, "incomplete": func(i *Identity) { i.Ready = false },
	} {
		t.Run(name, func(t *testing.T) {
			bad := i
			change(&bad)
			raw, _ := json.Marshal(bad)
			if _, err := DecodeIdentity(raw); err == nil {
				t.Fatal("accepted invalid identity")
			}
		})
	}
	for _, bad := range []string{string(data) + ` {}`, strings.TrimSuffix(string(data), "}") + `,"root":"C:\\other"}`} {
		if _, err := DecodeIdentity([]byte(bad)); err == nil {
			t.Fatal("accepted extra identity data")
		}
	}
}

func TestPathsCannotBeRedirectedByIdentity(t *testing.T) {
	root := t.TempDir()
	p, err := PathsAt(root)
	if err != nil {
		t.Fatal(err)
	}
	if p.Engine != filepath.Join(root, filepath.FromSlash(EngineEntryRelative)) {
		t.Fatal("engine escaped fixed layout")
	}
	if _, err := PathsAt("relative"); err == nil {
		t.Fatal("accepted relative root")
	}
}

func TestBackendRejectsUnsafeArtifactPaths(t *testing.T) {
	b := BackendMetadata{SchemaVersion: 1, ProtocolMajor: 1, UpstreamRevision: strings.Repeat("a", 40), PatchRevision: "v1", Artifacts: map[string]string{EngineEntryRelative: strings.Repeat("b", 64), AdapterRelative: strings.Repeat("c", 64)}}
	data, _ := json.Marshal(b)
	if _, err := DecodeBackend(data); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../host.ps1", `C:\host.ps1`, `\\server\share\script.ps1`, `scoop/../../host.ps1`, `scoop/entry.ps1:stream`, `scoop/folder./file`} {
		b.Artifacts[path] = strings.Repeat("b", 64)
		if err := b.Validate(); err == nil {
			t.Errorf("accepted %q", path)
		}
		delete(b.Artifacts, path)
	}
	b.SchemaVersion++
	if err := b.Validate(); err == nil {
		t.Fatal("accepted unknown backend schema")
	}
}
