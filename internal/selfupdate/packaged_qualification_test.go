package selfupdate

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/runtimeinstall"
)

type packagedNetwork struct{ path, version string }

func (n packagedNetwork) Resolve(context.Context, string) (Release, error) {
	return Release{"v" + n.version, "archive", "checksum"}, nil
}
func (n packagedNetwork) Download(_ context.Context, url string, w io.Writer, limit int64) error {
	f, err := os.Open(n.path)
	if err != nil {
		return err
	}
	defer f.Close()
	if url == "checksum" {
		h := sha256.New()
		if _, err = io.Copy(h, f); err != nil {
			return err
		}
		_, err = fmt.Fprintf(w, "%x %s", h.Sum(nil), asset)
		return err
	}
	count, err := io.Copy(w, io.LimitReader(f, limit+1))
	if count > limit {
		return fmt.Errorf("fixture archive limit")
	}
	return err
}

func qualificationTree(t *testing.T, root string) map[string]string {
	t.Helper()
	m := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		h, err := runtimeinstall.Hash(root, rel)
		if err == nil {
			m[rel] = h
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Prepared by tests/selfupdate/qualify-managed-tools.ps1 in a disposable VM.
// Transport is local and pinned; transaction preparation, replacement, recovery,
// Host coordination and subsequent package execution use production code/files.
func TestPackagedManagedToolQualification(t *testing.T) {
	root := os.Getenv("PORTSCOOP_PACKAGED_UPDATE_FIXTURE")
	if root == "" {
		t.Skip("requires prepared disposable VM fixture")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	disk := Disk{}
	host, err := disk.Select(filepath.Join(root, "host", "psp.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	host, err = disk.Ensure(host)
	if err != nil {
		t.Fatal(err)
	}
	net := packagedNetwork{filepath.Join(root, "candidate.zip"), host.Version}
	release, _ := net.Resolve(ctx, "")
	store := filepath.Join(root, "old-store")
	target, err := disk.Select(filepath.Join(store, "psp.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	target, err = disk.Ensure(target)
	if err != nil {
		t.Fatal(err)
	}
	mainBefore := qualificationTree(t, filepath.Join(store, "scoop", "buckets", "main"))
	persistBefore := qualificationTree(t, filepath.Join(store, "scoop", "persist"))
	stage := func(target runtimeinstall.Target, controller *runtimeinstall.Target) *Transaction {
		tx, err := disk.Stage(ctx, target, release, net, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		tx.Controller = controller
		return tx
	}
	// A direct Store cannot quietly adopt a backend/helper migration.
	tx := stage(target, nil)
	if err = disk.Prepare(tx); err == nil || !strings.Contains(err.Error(), "current Host self-update") {
		t.Fatalf("migration diagnostic: %v", err)
	}
	if err = disk.Discard(tx); err != nil {
		t.Fatal(err)
	}
	t.Log("direct Store rejected required backend migration")
	// Inject interruption after actual helper publication, then recover using
	// the production journal and complete previous runtime inventory.
	tx = stage(target, &host)
	if err = disk.Prepare(tx); err != nil {
		t.Fatal(err)
	}
	if err = disk.Publish(tx); err != nil {
		t.Fatal(err)
	}
	if err = writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
		t.Fatal(err)
	}
	changed := 0
	for _, op := range tx.Operations {
		if strings.HasPrefix(op.Path, ".portscoop/tools/") {
			if err = replace(tx, op, false); err != nil {
				t.Fatal(err)
			}
			changed++
			if changed == 3 {
				break
			}
		}
	}
	if changed != 3 {
		t.Fatal("no real helper introduction exercised")
	}
	if err = commit(ctx, tx.Directory, true); err != nil {
		t.Fatal(err)
	}
	if err = verifyInstalled(tx, true); err != nil {
		t.Fatal(err)
	}
	t.Log("interrupted real helper introduction restored the complete old runtime")
	doc := registry.Empty()
	doc.Entries["old"] = registry.Entry{ID: target.ID, Path: store}
	doc.DefaultID = target.ID
	h := HostMaintenance{Service: Service{Discovery: net, Downloads: net, Files: disk, Clock: realClock{}, Locks: NativeLocker{}, Worker: unexpectedWorker{t}}, ReadRegistry: func() (registry.Document, error) { return doc, nil }, Resolve: (resolver.Discovery{Volumes: noVolumes{}}).Resolve}
	result, err := h.Run(ctx, filepath.Join(host.Root, "psp.exe"), Options{})
	if err != nil || result.Code != 0 || !strings.Contains(result.Message, "Store runtime and managed backend updated") {
		t.Fatalf("Host update: %+v %v", result, err)
	}
	if !reflect.DeepEqual(mainBefore, qualificationTree(t, filepath.Join(store, "scoop", "buckets", "main"))) || !reflect.DeepEqual(persistBefore, qualificationTree(t, filepath.Join(store, "scoop", "persist"))) {
		t.Fatal("Host migration changed user state")
	}
	t.Log("Host-coordinated migration installed complete helpers and preserved Main/persistence")
	// A compatible direct Store transaction owns only launchers and receipt.
	compatible := filepath.Join(root, "compatible-store")
	direct, err := disk.Select(filepath.Join(compatible, "psp.exe"), true)
	if err != nil {
		t.Fatal(err)
	}
	direct, err = disk.Ensure(direct)
	if err != nil {
		t.Fatal(err)
	}
	helpersBefore := qualificationTree(t, filepath.Join(compatible, ".portscoop", "tools"))
	tx = stage(direct, nil)
	if err = disk.Prepare(tx); err != nil {
		t.Fatal(err)
	}
	for _, op := range tx.Operations {
		if strings.HasPrefix(op.Path, ".portscoop/tools/") || strings.HasPrefix(op.Path, "scoop/") {
			t.Fatal("direct update owns backend", op.Path)
		}
	}
	if err = disk.Publish(tx); err != nil {
		t.Fatal(err)
	}
	if err = writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
		t.Fatal(err)
	}
	if err = commit(ctx, tx.Directory, false); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(helpersBefore, qualificationTree(t, filepath.Join(compatible, ".portscoop", "tools"))) {
		t.Fatal("direct update changed retained helpers")
	}
	t.Log("compatible direct Store transaction retained complete helper inventory")
	var payload bytes.Buffer
	z := zip.NewWriter(&payload)
	w, _ := z.Create("check.cmd")
	w.Write([]byte("@echo migrated package works\r\n"))
	z.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(payload.Bytes()) }))
	defer server.Close()
	manifest := fmt.Sprintf(`{"version":"1","url":%q,"hash":"%x","bin":"check.cmd"}`, server.URL+"/fixture.zip", sha256.Sum256(payload.Bytes()))
	if err = os.WriteFile(filepath.Join(store, "scoop", "buckets", "main", "bucket", "migration-check.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, filepath.Join(store, "psp.exe"), "install", "main/migration-check")
	if out, err := command.CombinedOutput(); err != nil {
		t.Fatalf("migrated package install: %v\n%s", err, out)
	}
	command = exec.CommandContext(ctx, filepath.Join(store, "pspe.exe"), "check", "--qualification")
	if out, err := command.CombinedOutput(); err != nil || !strings.Contains(string(out), "migrated package works") {
		t.Fatalf("migrated shim: %v\n%s", err, out)
	}
	t.Log("migrated Store installed an unchanged local fixture and ran its shim")
}

// Repair a failed application qualification through real Host maintenance,
// retaining the existing apps and persistence for the resumed lifecycle check.
func TestUpdateQualifiedApplicationStores(t *testing.T) {
	root := os.Getenv("PORTSCOOP_APPLICATION_UPDATE_FIXTURE")
	if root == "" {
		t.Skip("requires existing disposable VM application fixture")
	}
	host := filepath.Join(root, "maintenance-host")
	_, descriptor, err := runtimeinstall.VerifyDistribution(host, true)
	if err != nil {
		t.Fatal(err)
	}
	net := packagedNetwork{filepath.Join(root, "candidate.zip"), descriptor.Version}
	doc := registry.Empty()
	for _, name := range []string{"moved store", "second"} {
		target, err := (Disk{}).Select(filepath.Join(root, name, "psp.exe"), true)
		if err != nil {
			t.Fatal(err)
		}
		doc.Entries[strings.ReplaceAll(name, " ", "-")] = registry.Entry{ID: target.ID, Path: target.Root}
	}
	h := HostMaintenance{Service: Service{Discovery: net, Downloads: net, Files: Disk{}, Clock: realClock{}, Locks: NativeLocker{}, Worker: unexpectedWorker{t}}, ReadRegistry: func() (registry.Document, error) { return doc, nil }, Resolve: (resolver.Discovery{Volumes: noVolumes{}}).Resolve}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	result, err := h.Run(ctx, filepath.Join(host, "psp.exe"), Options{})
	if err != nil || result.Code != 0 || strings.Count(result.Message, "Store runtime and managed backend updated") != 2 {
		t.Fatalf("application Store maintenance: %+v %v", result, err)
	}
	t.Log(result.Message)
}
