package scoop

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"portscoop/internal/compatibility"
	"portscoop/internal/environment"
	"portscoop/internal/runner"
)

func extractionFixture(t *testing.T) (environment.Context, string, FrozenManifest) {
	t.Helper()
	local := managedFixture(t)
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	w, _ := z.Create("prefix/tool.txt")
	w.Write([]byte("hello"))
	z.Close()
	payloads := [][]byte{[]byte("inert executable"), b.Bytes(), b.Bytes()}
	names := []string{"inert.exe", "one.zip", "two.nupkg"}
	var urls, hashes []string
	for i, n := range names {
		urls = append(urls, "https://example.invalid/get?download=1#/"+n)
		hashes = append(hashes, fmt.Sprintf("%x", sha256.Sum256(payloads[i])))
		fixtureFile(t, filepath.Join(local.Paths.Apps, "app", "1", n), payloads[i])
	}
	data, _ := json.Marshal(map[string]any{"version": "1", "url": urls, "hash": hashes, "extract_dir": []any{nil, "prefix", "prefix"}, "extract_to": []any{nil, "one", "two"}})
	var m Manifest
	json.Unmarshal(data, &m)
	p, err := compatibility.Compile(m)
	if err != nil {
		t.Fatal(err)
	}
	frozen := FrozenManifest{Manifest: p.Manifest, Downloads: p.Downloads}
	request := filepath.Join(local.Paths.Temp, "portscoop-request-fixture.json")
	writeExtractRequest(t, local, request, frozen)
	return local, request, frozen
}
func writeExtractRequest(t *testing.T, local environment.Context, path string, frozen FrozenManifest) {
	t.Helper()
	ms, _ := json.Marshal(map[string]FrozenManifest{"app": frozen})
	b, _ := json.Marshal(runner.Request{SchemaVersion: 1, ExpectedID: local.Identity.ID, Arguments: []string{"install", "app"}, Manifests: ms})
	fixtureFile(t, path, b)
}
func TestFrozenExtractionPairing(t *testing.T) {
	local, request, _ := extractionFixture(t)
	if err := extractRequest(context.Background(), local.Paths, local.Identity.ID, request, "app"); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"one/tool.txt", "two/tool.txt", "inert.exe"} {
		if _, err := os.Stat(filepath.Join(local.Paths.Apps, "app", "1", n)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(local.Paths.Apps, "app", "1", "one.zip")); !os.IsNotExist(err) {
		t.Fatal("archive not removed after success")
	}
}
func TestFrozenExtractionRejectsBeforePublication(t *testing.T) {
	for _, mutation := range []string{"plan", "identity", "transport", "hash", "content", "collision", "cancel", "name"} {
		t.Run(mutation, func(t *testing.T) {
			local, request, frozen := extractionFixture(t)
			ctx := context.Background()
			id := local.Identity.ID
			app := "app"
			switch mutation {
			case "plan":
				frozen.Downloads[1].ExtractTo = "escape"
				writeExtractRequest(t, local, request, frozen)
			case "identity":
				id = "wrong"
			case "transport":
				request = filepath.Join(local.Paths.Root, "foreign.json")
			case "hash":
				fixtureFile(t, filepath.Join(local.Paths.Apps, "app", "1", "two.nupkg"), []byte("changed"))
			case "content":
				bytes := []byte("MZ not a ZIP")
				fixtureFile(t, filepath.Join(local.Paths.Apps, "app", "1", "two.nupkg"), bytes)
				var hashes []string
				json.Unmarshal(frozen.Manifest["hash"], &hashes)
				hashes[2] = fmt.Sprintf("%x", sha256.Sum256(bytes))
				frozen.Manifest["hash"], _ = json.Marshal(hashes)
				p, _ := compatibility.Compile(frozen.Manifest)
				frozen.Downloads = p.Downloads
				writeExtractRequest(t, local, request, frozen)
			case "collision":
				fixtureFile(t, filepath.Join(local.Paths.Apps, "app", "1", "two", "tool.txt"), []byte("sentinel"))
			case "cancel":
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case "name":
				app = "../app"
			}
			if err := extractRequest(ctx, local.Paths, id, request, app); err == nil {
				t.Fatal("accepted mutation")
			}
			if _, err := os.Stat(filepath.Join(local.Paths.Apps, "app", "1", "one", "tool.txt")); !os.IsNotExist(err) {
				t.Fatal("published before validating full download set")
			}
			if _, err := os.Stat(filepath.Join(local.Paths.Apps, "app", "1", "one.zip")); err != nil {
				t.Fatal("removed archive after failure")
			}
		})
	}
}
