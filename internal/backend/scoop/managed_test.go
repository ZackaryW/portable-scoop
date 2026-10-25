package scoop

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"portscoop/internal/compatibility"
	"portscoop/internal/environment"
	"portscoop/internal/runner"
	"portscoop/internal/testutil"
)

func TestManagedFreezesPortableActionsAndReportsDecisions(t *testing.T) {
	local := managedFixture(t)
	manifest := map[string]any{
		"version": "1.0", "url": "https://example.test/tool.zip", "hash": "sha1:" + strings.Repeat("a", 40),
		"pre_install": []string{`if (-not (Test-Path -Path "$persist_dir\config.xml" -PathType Leaf)) {`, `    New-Item -Path "$dir\config.xml" -ItemType File -Force | Out-Null`, `}`},
		"persist":     "config.xml", "shortcuts": [][]string{{"tool.exe", "My Tool"}},
	}
	data, _ := json.Marshal(manifest)
	path := filepath.Join(local.Paths.Buckets, "main", "bucket", "tool.json")
	fixtureFile(t, path, data)
	process := &captureProcess{run: func(runner.Command) { fixtureFile(t, path, []byte(`{"pre_install":"Write-Host changed"}`)) }}
	var notices bytes.Buffer
	_, err := (Managed{Runner: process}).Execute(context.Background(), local, []string{"install", "tool"}, runner.Streams{Err: &notices})
	if err != nil {
		t.Fatal(err)
	}
	var frozen map[string]FrozenManifest
	if err := json.Unmarshal(process.request.Manifests, &frozen); err != nil {
		t.Fatal(err)
	}
	item := frozen["tool"]
	if len(item.Actions) != 1 || item.Actions[0] != (compatibility.Action{Phase: "pre_install", Kind: "ensure-persisted-file", Path: "config.xml"}) || item.Manifest["shortcuts"] != nil || !strings.Contains(string(item.Manifest["bin"]), "My-Tool") {
		t.Fatalf("bad frozen plan: %+v", item)
	}
	if !strings.Contains(notices.String(), "pre_install rewrite") || !strings.Contains(notices.String(), "shortcuts rewrite") {
		t.Fatal(notices.String())
	}
}

func TestManagedFreezesBitwardenRenameAndSelectiveSkip(t *testing.T) {
	local := managedFixture(t)
	data, err := os.ReadFile("../../compatibility/testdata/bitwarden.json")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local.Paths.Buckets, "extras", "bucket", "bitwarden.json")
	fixtureFile(t, path, data)
	process := &captureProcess{run: func(runner.Command) { fixtureFile(t, path, []byte(`{"pre_install":"unreviewed replacement"}`)) }}
	var notices bytes.Buffer
	_, err = (Managed{Runner: process}).Execute(context.Background(), local, []string{"install", "extras/bitwarden"}, runner.Streams{Err: &notices})
	if err != nil {
		t.Fatal(err)
	}
	var frozen map[string]FrozenManifest
	if err := json.Unmarshal(process.request.Manifests, &frozen); err != nil {
		t.Fatal(err)
	}
	item := frozen["bitwarden"]
	if len(item.Actions) != 1 || item.Actions[0] != (compatibility.Action{Phase: "pre_install", Kind: "rename-package-file", Source: "Bitwarden-Portable-2026.10.1.exe", Path: "Bitwarden.exe"}) {
		t.Fatalf("bad frozen rename: %+v", frozen)
	}
	if !strings.Contains(notices.String(), "skip reviewed optional import from host AppData") {
		t.Fatal(notices.String())
	}
}

func fixtureFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
}

func managedFixture(t *testing.T) environment.Context {
	t.Helper()
	root := testutil.TempDir(t)
	paths, _ := environment.PathsAt(root)
	id, _ := environment.NewIdentity("fixture")
	marker, _ := json.Marshal(id)
	fixtureFile(t, filepath.Join(root, environment.MarkerRelative), marker)
	metadata := environment.BackendMetadata{SchemaVersion: 1, ProtocolMajor: 1, UpstreamRevision: UpstreamRevision, PatchRevision: PatchRevision, Artifacts: map[string]string{}}
	for _, path := range []string{environment.EngineEntryRelative, environment.AdapterRelative, ".portscoop/backend/request-reader.ps1", "scoop/apps/scoop/current/lib/system.ps1", "scoop/apps/scoop/current/lib/manifest.ps1"} {
		data := []byte("fixture " + path)
		fixtureFile(t, filepath.Join(root, path), data)
		hash := sha256.Sum256(data)
		metadata.Artifacts[path] = hex.EncodeToString(hash[:])
	}
	encoded, _ := json.Marshal(metadata)
	fixtureFile(t, filepath.Join(root, MetadataRelative), encoded)
	fixtureFile(t, paths.Config, []byte("{}"))
	if err := os.MkdirAll(paths.Buckets, 0700); err != nil {
		t.Fatal(err)
	}
	return environment.Context{Identity: id, Paths: paths, WorkingDirectory: root}
}

func managedManifest(version string) []byte {
	return []byte(fmt.Sprintf(`{"version":%q,"url":"https://example.test/package.zip","hash":%q,"bin":"tool.exe"}`, version, strings.Repeat("a", 64)))
}

type captureProcess struct {
	calls   int
	request runner.Request
	run     func(runner.Command)
}

func (p *captureProcess) Run(_ context.Context, c runner.Command) (int, error) {
	if len(c.Arguments) > 0 && c.Arguments[len(c.Arguments)-1] == runtimeProbe {
		_, err := c.Streams.Out.Write([]byte(`{"version":"5.1.26100.1","language":"FullLanguage","x64":true,"junction":"Junction"}`))
		return 0, err
	}
	p.calls++
	if p.run != nil {
		p.run(c)
	}
	for _, value := range c.Environment {
		if strings.HasPrefix(value, "PORTSCOOP_REQUEST_FILE=") {
			data, err := os.ReadFile(strings.TrimPrefix(value, "PORTSCOOP_REQUEST_FILE="))
			if err != nil {
				return 0, err
			}
			if err := json.Unmarshal(data, &p.request); err != nil {
				return 0, err
			}
		}
	}
	return 17, nil
}

func TestManagedFreezesPreflightedDependencyClosure(t *testing.T) {
	local := managedFixture(t)
	app := filepath.Join(local.Paths.Buckets, "main", "bucket", "app.json")
	fixtureFile(t, app, []byte(fmt.Sprintf(`{"version":"1.0","url":"https://example.test/app.zip","hash":%q,"depends":"dep"}`, strings.Repeat("a", 64))))
	fixtureFile(t, filepath.Join(filepath.Dir(app), "dep.json"), managedManifest("3.0"))
	process := &captureProcess{run: func(runner.Command) { fixtureFile(t, app, []byte(`{"pre_install":"forbidden"}`)) }}
	code, err := (Managed{Runner: process}).Execute(context.Background(), local, []string{"install", "app"}, runner.Streams{})
	if err != nil || code != 17 || process.calls != 1 {
		t.Fatalf("status=%d error=%v calls=%d", code, err, process.calls)
	}
	var frozen map[string]FrozenManifest
	if err := json.Unmarshal(process.request.Manifests, &frozen); err != nil {
		t.Fatal(err)
	}
	if len(frozen) != 2 || string(frozen["app"].Manifest["version"]) != `"1.0"` || frozen["dep"].Bucket != "main" {
		t.Fatalf("incorrect frozen closure: %+v", frozen)
	}
	files, _ := filepath.Glob(filepath.Join(local.Paths.Temp, "portscoop-request-*"))
	if len(files) != 0 {
		t.Fatal("managed request leaked")
	}
}

func TestManagedRejectsDependencyHookBeforeAnyMutation(t *testing.T) {
	local := managedFixture(t)
	fixtureFile(t, filepath.Join(local.Paths.Buckets, "main", "bucket", "app.json"), []byte(fmt.Sprintf(`{"version":"1","url":"https://example.test/app.zip","hash":%q,"depends":"dep"}`, strings.Repeat("a", 64))))
	fixtureFile(t, filepath.Join(local.Paths.Buckets, "main", "bucket", "dep.json"), []byte(`{"version":"1","pre_install":"Write-Host forbidden"}`))
	process := &captureProcess{}
	_, err := (Managed{Runner: process}).Execute(context.Background(), local, []string{"install", "app"}, runner.Streams{})
	if err == nil || !strings.Contains(err.Error(), "pre_install") || process.calls != 0 {
		t.Fatalf("error=%v calls=%d", err, process.calls)
	}
	if _, err := os.Stat(local.Paths.Temp); !os.IsNotExist(err) {
		t.Fatal("preflight mutated temporary state")
	}
}

func TestSavedUninstallHooksAndOtherVersionsRejected(t *testing.T) {
	for _, command := range []string{"install", "uninstall", "update", "reset", "cleanup"} {
		t.Run(command, func(t *testing.T) {
			local := managedFixture(t)
			fixtureFile(t, filepath.Join(local.Paths.Apps, "app", "0.9", "scoop-manifest.json"), []byte(`{"post_uninstall":"forbidden"}`))
			process := &captureProcess{}
			_, err := (Managed{Runner: process}).Execute(context.Background(), local, []string{command, "app"}, runner.Streams{})
			if err == nil || !strings.Contains(err.Error(), "post_uninstall") || process.calls != 0 {
				t.Fatalf("error=%v calls=%d", err, process.calls)
			}
		})
	}
}

func TestManagedPolicyRejectsBypassesWithoutLaunching(t *testing.T) {
	for _, args := range [][]string{{"install", "app", "--global"}, {"update"}, {"update", "scoop"}, {"config", "root_path", "X:\\other"}, {"alias", "add"}, {"install", "https://example.test/app.json"}, {"install", "app", "--skip-hash-check"}} {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			process := &captureProcess{}
			_, err := (Managed{Runner: process}).Execute(context.Background(), environment.Context{}, args, runner.Streams{})
			if err == nil || process.calls != 0 {
				t.Fatalf("error=%v calls=%d", err, process.calls)
			}
		})
	}
}

func TestBackendVerificationFailsClosed(t *testing.T) {
	for _, mutation := range []string{"missing", "changed", "extra", "incompatible"} {
		t.Run(mutation, func(t *testing.T) {
			local := managedFixture(t)
			switch mutation {
			case "missing":
				if err := os.Remove(local.Paths.Engine); err != nil {
					t.Fatal(err)
				}
			case "changed":
				fixtureFile(t, filepath.Join(filepath.Dir(filepath.Dir(local.Paths.Engine)), "lib", "system.ps1"), []byte("altered"))
			case "extra":
				fixtureFile(t, filepath.Join(filepath.Dir(local.Paths.Adapter), "extra.ps1"), []byte("unverified"))
			case "incompatible":
				path := filepath.Join(local.Paths.Root, MetadataRelative)
				data, _ := os.ReadFile(path)
				fixtureFile(t, path, []byte(strings.ReplaceAll(string(data), PatchRevision, "unknown-patch")))
			}
			process := &captureProcess{}
			_, err := (Managed{Runner: process}).Execute(context.Background(), local, []string{"list"}, runner.Streams{})
			if err == nil || process.calls != 0 {
				t.Fatalf("error=%v calls=%d", err, process.calls)
			}
		})
	}
}

func TestBackendVerificationAcceptsCompleteRecordedPayload(t *testing.T) {
	local := managedFixture(t)
	if _, err := Verify(local.Paths); err != nil {
		t.Fatal(err)
	}
}

func TestPreparedPayloadMatchesRecordedArtifacts(t *testing.T) {
	payload := os.Getenv("PORTSCOOP_TEST_BACKEND_PAYLOAD")
	if payload == "" {
		t.Skip("set PORTSCOOP_TEST_BACKEND_PAYLOAD to qualify a generated backend")
	}
	local := managedFixture(t)
	for _, pair := range [][2]string{{"engine", filepath.Dir(filepath.Dir(local.Paths.Engine))}, {"adapter", filepath.Dir(local.Paths.Adapter)}} {
		if err := filepath.WalkDir(filepath.Join(payload, pair[0]), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			relative, err := filepath.Rel(filepath.Join(payload, pair[0]), path)
			if err != nil {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			fixtureFile(t, filepath.Join(pair[1], relative), data)
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	data, err := os.ReadFile(filepath.Join(payload, "backend.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, filepath.Join(local.Paths.Root, MetadataRelative), data)
	if _, err := Verify(local.Paths); err != nil {
		t.Fatal(err)
	}
}
