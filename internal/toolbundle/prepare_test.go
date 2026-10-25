package toolbundle

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fixture(t *testing.T) (Pins, string, string) {
	t.Helper()
	root := t.TempDir()
	cache := filepath.Join(root, "cache")
	if err := os.Mkdir(cache, 0700); err != nil {
		t.Fatal(err)
	}
	var b bytes.Buffer
	w := zip.NewWriter(&b)
	for _, name := range []string{"bin/tool.exe", "LICENSE.txt", "unused.exe"} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(name))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	h := sha256.Sum256(b.Bytes())
	digest := hex.EncodeToString(h[:])
	if err := os.WriteFile(filepath.Join(cache, digest), b.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	p := Pins{SchemaVersion: 1, Tools: []Tool{{Name: "fixture", Version: "1", URL: "https://example.invalid/tool.zip", SHA256: digest, Format: "zip", Source: "https://example.invalid/source", SourcePackages: "https://example.invalid/source.tar", Required: []string{"bin/tool.exe", "LICENSE.txt"}, Retain: []string{"bin/tool.exe", "LICENSE.txt"}}}}
	return p, cache, filepath.Join(root, "bundle")
}
func TestPreparePublishesCompleteInventoryAndRetainsNotices(t *testing.T) {
	p, cache, dest := fixture(t)
	r, err := Prepare(context.Background(), p, cache, dest)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Artifacts) != 2 || r.Artifacts["fixture/LICENSE.txt"] == "" || r.Artifacts["fixture/bin/tool.exe"] == "" {
		t.Fatalf("incomplete inventory: %+v", r)
	}
	if _, err = os.Stat(filepath.Join(dest, "fixture/unused.exe")); !os.IsNotExist(err) {
		t.Fatal("retained unrequested executable")
	}
	if _, err = os.Stat(filepath.Join(dest, "tools.json")); err != nil {
		t.Fatal(err)
	}
	if _, err = Prepare(context.Background(), p, cache, dest); err == nil {
		t.Fatal("overwrote existing bundle")
	}
}
func TestPrepareRejectsDigestMismatchWithoutPublishing(t *testing.T) {
	p, cache, dest := fixture(t)
	file := filepath.Join(cache, p.Tools[0].SHA256)
	os.WriteFile(file, []byte("altered"), 0600)
	if _, err := Prepare(context.Background(), p, cache, dest); err == nil || !strings.Contains(err.Error(), "SHA-256") {
		t.Fatalf("%v", err)
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("published bad bundle")
	}
	b, _ := os.ReadFile(file)
	if string(b) != "altered" {
		t.Fatal("silently replaced cache")
	}
}
func TestPrepareMissingRequiredDoesNotPublish(t *testing.T) {
	p, cache, dest := fixture(t)
	p.Tools[0].Required = append(p.Tools[0].Required, "missing.dll")
	p.Tools[0].Retain = nil
	if _, err := Prepare(context.Background(), p, cache, dest); err == nil {
		t.Fatal("accepted incomplete seed")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("published incomplete bundle")
	}
}
func TestPinsRejectUnsafeOrIncompleteSelection(t *testing.T) {
	for _, mutate := range []func(*Tool){func(t *Tool) { t.Name = "../escape" }, func(t *Tool) { t.URL = "http://example.invalid" }, func(t *Tool) { t.Retain = []string{"bin/tool.exe"} }, func(t *Tool) { t.Retain = append(t.Retain, "../escape") }, func(t *Tool) { t.SHA256 = "bad" }} {
		p, _, _ := fixture(t)
		mutate(&p.Tools[0])
		if p.Validate() == nil {
			t.Fatal("invalid pin accepted")
		}
	}
}
func TestPrepareCancelledBeforePublication(t *testing.T) {
	p, cache, dest := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Prepare(ctx, p, cache, dest); err == nil {
		t.Fatal("cancelled preparation succeeded")
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatal("published cancelled bundle")
	}
}
