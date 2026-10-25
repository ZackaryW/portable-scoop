package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/runtimeinstall"
)

type fixtureNetwork struct {
	archive                []byte
	resolutions, downloads int
}

func (n *fixtureNetwork) Resolve(context.Context, string) (Release, error) {
	n.resolutions++
	return Release{"v0.3.2-fixture", "archive", "checksum"}, nil
}
func (n *fixtureNetwork) Download(_ context.Context, url string, w io.Writer, _ int64) error {
	n.downloads++
	if url == "checksum" {
		sum := sha256.Sum256(n.archive)
		_, err := fmt.Fprint(w, hex.EncodeToString(sum[:])+" "+asset)
		return err
	}
	_, err := w.Write(n.archive)
	return err
}

type noVolumes struct{}

func (noVolumes) Volumes(context.Context) ([]resolver.Volume, error) { return nil, nil }

type unexpectedWorker struct{ t *testing.T }

type workerFunc func(context.Context, *Transaction, bool) (Result, error)

func (f workerFunc) Handoff(c context.Context, tx *Transaction, r bool) (Result, error) {
	return f(c, tx, r)
}

func (w unexpectedWorker) Handoff(context.Context, *Transaction, bool) (Result, error) {
	w.t.Fatal("unexpected detached worker")
	return Result{}, nil
}

func maintenanceFixture(t *testing.T) (HostMaintenance, *fixtureNetwork, *Transaction, *registry.Document) {
	t.Helper()
	tx := storeTransaction(t)
	data, err := os.ReadFile(writeArchive(t, releaseEntries(t)))
	if err != nil {
		t.Fatal(err)
	}
	net := &fixtureNetwork{archive: data}
	doc := registry.Empty()
	doc.Entries["default"] = registry.Entry{ID: tx.Target.ID, Path: tx.Target.Root}
	doc.Entries["alias"] = doc.Entries["default"]
	doc.DefaultID = tx.Target.ID
	h := HostMaintenance{
		Service:      Service{Discovery: net, Downloads: net, Files: Disk{}, Clock: realClock{}, Locks: NativeLocker{}, Worker: unexpectedWorker{t}},
		ReadRegistry: func() (registry.Document, error) { return doc, nil },
		Resolve:      (resolver.Discovery{Volumes: noVolumes{}}).Resolve,
	}
	return h, net, tx, &doc
}

func TestCurrentHostUpdatesRegisteredStoreOnce(t *testing.T) {
	h, net, tx, doc := maintenanceFixture(t)
	other := storeTransaction(t)
	doc.Entries["other"] = registry.Entry{ID: other.Target.ID, Path: other.Target.Root}
	missing, _ := environment.NewIdentity("missing")
	doc.Entries["offline"] = registry.Entry{ID: missing.ID, Path: filepath.Join(t.TempDir(), "absent")}
	stale, _ := environment.NewIdentity("stale")
	doc.Entries["stale"] = registry.Entry{ID: stale.ID, Path: tx.Target.Root}
	exe := filepath.Join(tx.Controller.Root, "psp.exe")
	before, _ := runtimeinstall.Hash(tx.Controller.Root, "psp.exe")
	r, err := h.Run(context.Background(), exe, Options{})
	if err != nil || r.Code != 2 {
		t.Fatalf("mixed result %#v %v", r, err)
	}
	if strings.Count(r.Message, "Store runtime and managed backend updated") != 2 || !strings.Contains(r.Message, "alias,default") || !strings.Contains(r.Message, "offline") || !strings.Contains(r.Message, "stale") || !strings.Contains(r.Message, "already current") {
		t.Fatal(r.Message)
	}
	if net.resolutions != 1 || net.downloads != 2 {
		t.Fatalf("discovery/downloads: %d/%d", net.resolutions, net.downloads)
	}
	for _, root := range []string{tx.Target.Root, other.Target.Root} {
		updated, err := (Disk{}).Select(filepath.Join(root, "psp.exe"), true)
		if err != nil || updated.Version != "0.3.2-fixture" {
			t.Fatalf("update %v %v", updated, err)
		}
	}
	after, _ := runtimeinstall.Hash(tx.Controller.Root, "psp.exe")
	if before != after {
		t.Fatal("current Host changed")
	}
	if len(doc.Entries) != 5 || doc.DefaultID != tx.Target.ID {
		t.Fatal("registry changed")
	}
	status, err := h.Run(context.Background(), exe, Options{Status: true})
	if err != nil || status.Code != 2 || strings.Count(status.Message, "completed:") != 2 {
		t.Fatalf("status: %#v %v", status, err)
	}
}

func TestHostCheckDoesNotCreateUpdateState(t *testing.T) {
	h, net, tx, _ := maintenanceFixture(t)
	before, err := os.ReadFile(filepath.Join(tx.Target.Root, runtimeinstall.ReceiptName))
	if err != nil {
		t.Fatal(err)
	}
	r, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{Check: true})
	if err != nil || r.Code != 0 || !strings.Contains(r.Message, "0.3.0 -> v0.3.2-fixture available") {
		t.Fatalf("%#v %v", r, err)
	}
	if net.resolutions != 1 || net.downloads != 0 {
		t.Fatal("check downloaded code")
	}
	after, _ := os.ReadFile(filepath.Join(tx.Target.Root, runtimeinstall.ReceiptName))
	if string(before) != string(after) {
		t.Fatal("check mutated Store")
	}
	if _, err = runtimeinstall.ReadPending(tx.Target); !os.IsNotExist(err) {
		t.Fatalf("pending %v", err)
	}
}

func TestHostRecoveryAndLiveOwner(t *testing.T) {
	h, _, tx, _ := maintenanceFixture(t)
	if err := (Disk{}).Prepare(tx); err != nil {
		t.Fatal(err)
	}
	if err := (Disk{}).Publish(tx); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(tx.Controller.Root, "psp.exe")
	r, err := h.Run(context.Background(), exe, Options{Recover: true})
	if err != nil || r.Code != 2 || !strings.Contains(r.Message, "live owner") {
		t.Fatalf("live: %#v %v", r, err)
	}
	tx.ParentPID = 0
	tx.ParentCreated = 0
	if err := runtimeinstall.Write(tx.Directory, "plan.json", tx); err != nil {
		t.Fatal(err)
	}
	for _, op := range tx.Operations[:len(tx.Operations)/2] {
		if err := replace(tx, op, false); err != nil {
			t.Fatal(err)
		}
	}
	r, err = h.Run(context.Background(), exe, Options{Recover: true})
	if err != nil || r.Code != 0 || !strings.Contains(r.Message, "Previous Store runtime restored") || !strings.Contains(r.Message, "No pending update transaction") {
		t.Fatalf("recovery: %#v %v", r, err)
	}
	if err := verifyInstalled(tx, true); err != nil {
		t.Fatal(err)
	}
}

func TestDirectStoreDoesNotReadRegistry(t *testing.T) {
	h, _, tx, _ := maintenanceFixture(t)
	h.ReadRegistry = func() (registry.Document, error) {
		t.Fatal("direct Store read registry")
		return registry.Document{}, nil
	}
	r, err := h.Run(context.Background(), filepath.Join(tx.Target.Root, "psp.exe"), Options{Check: true})
	if err != nil || r.Code != 0 {
		t.Fatalf("%#v %v", r, err)
	}
	_, err = h.Run(context.Background(), filepath.Join(tx.Target.Root, "psp.exe"), Options{})
	if err == nil || !strings.Contains(err.Error(), "use a current Host") {
		t.Fatalf("direct backend migration accepted: %v", err)
	}
}

func TestHostRejectsAmbiguousIdentityAndCancellation(t *testing.T) {
	h, net, tx, doc := maintenanceFixture(t)
	clone := t.TempDir()
	if err := copyFile(tx.Target.Root, environment.MarkerRelative, clone, environment.MarkerRelative); err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, clone, "portscoop.exe", "another copy")
	doc.Entries["clone"] = registry.Entry{ID: tx.Target.ID, Path: clone}
	r, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{})
	if err != nil || r.Code != 2 || !strings.Contains(r.Message, "ambiguous installation identity") || net.downloads != 0 {
		t.Fatalf("%#v %v", r, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r, err = h.Run(ctx, filepath.Join(tx.Controller.Root, "psp.exe"), Options{Check: true})
	if err != nil || r.Code != 130 {
		t.Fatalf("cancellation: %#v %v", r, err)
	}
}

func TestHostNeverDowngradesNewerStore(t *testing.T) {
	h, net, tx, _ := maintenanceFixture(t)
	var receipt runtimeinstall.Receipt
	if err := runtimeinstall.Read(tx.Target.Root, runtimeinstall.ReceiptName, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.Version = "0.9.0"
	if err := runtimeinstall.Write(tx.Target.Root, runtimeinstall.ReceiptName, receipt); err != nil {
		t.Fatal(err)
	}
	r, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{})
	if err != nil || r.Code != 0 || !strings.Contains(r.Message, "0.9.0 is already current") || net.downloads != 0 {
		t.Fatalf("%#v %v", r, err)
	}
	r, err = h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{Version: "v0.3.2-fixture"})
	if err != nil || r.Code != 2 || !strings.Contains(r.Message, "downgrading") || net.downloads != 0 {
		t.Fatalf("%#v %v", r, err)
	}
}

func TestExplicitHostDowngradeStopsBeforeStores(t *testing.T) {
	h, _, tx, _ := maintenanceFixture(t)
	var d runtimeinstall.Descriptor
	if err := runtimeinstall.Read(tx.Controller.Root, runtimeinstall.DescriptorName, &d); err != nil {
		t.Fatal(err)
	}
	d.Version = "0.9.0"
	if err := runtimeinstall.Write(tx.Controller.Root, runtimeinstall.DescriptorName, d); err != nil {
		t.Fatal(err)
	}
	refreshFixtureInventory(t, tx.Controller.Root)
	// The fixture helper inventories the complete tree; controls are not payload.
	var inventory runtimeinstall.Release
	runtimeinstall.Read(tx.Controller.Root, "release.json", &inventory)
	for p := range inventory.Artifacts {
		if strings.HasPrefix(p, ".portscoop-host/") {
			delete(inventory.Artifacts, p)
		}
	}
	runtimeinstall.Write(tx.Controller.Root, "release.json", inventory)
	h.ReadRegistry = func() (registry.Document, error) { t.Fatal("downgrade read Stores"); return registry.Document{}, nil }
	_, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{Version: "v0.3.2-fixture"})
	if err == nil || !strings.Contains(err.Error(), "downgrading") {
		t.Fatalf("%v", err)
	}
}

func TestBusyStoreLeavesRecoverableTransaction(t *testing.T) {
	h, _, tx, _ := maintenanceFixture(t)
	lease, err := runtimeinstall.Lock(context.Background(), tx.Target, "runtime", false)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	start := time.Now()
	r, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{})
	if err != nil || r.Code != 2 || !strings.Contains(r.Message, "busy") {
		t.Fatalf("busy: %#v %v", r, err)
	}
	if time.Since(start) > 12*time.Second {
		t.Fatal("unbounded busy wait")
	}
	if _, err = runtimeinstall.ReadPending(tx.Target); err != nil {
		t.Fatalf("missing recovery record: %v", err)
	}
	if _, err = runtimeinstall.VerifyReceipt(tx.Target.Root, tx.Target.ID); err != nil {
		t.Fatal(err)
	}
}

func TestHostHandoffFollowsCompletedStoreUpdates(t *testing.T) {
	h, net, tx, _ := maintenanceFixture(t)
	var descriptor runtimeinstall.Descriptor
	if err := runtimeinstall.Read(tx.Controller.Root, runtimeinstall.DescriptorName, &descriptor); err != nil {
		t.Fatal(err)
	}
	descriptor.Version = "0.3.1"
	if err := runtimeinstall.Write(tx.Controller.Root, runtimeinstall.DescriptorName, descriptor); err != nil {
		t.Fatal(err)
	}
	refreshFixtureInventory(t, tx.Controller.Root)
	called := false
	h.Service.Worker = workerFunc(func(_ context.Context, hostTx *Transaction, recover bool) (Result, error) {
		called = true
		store, err := (Disk{}).Select(filepath.Join(tx.Target.Root, "psp.exe"), true)
		if err != nil || store.Version != "0.3.2-fixture" {
			t.Fatalf("Store not completed before Host handoff: %v %v", store, err)
		}
		if hostTx.Target.Kind != "host" || recover {
			t.Fatal("incorrect handoff target")
		}
		return Result{0, "Accepted Host transaction; inspect --status."}, nil
	})
	r, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{})
	if err != nil || r.Code != 0 || !called || !strings.Contains(r.Message, "Accepted Host transaction") {
		t.Fatalf("%#v %v", r, err)
	}
	if net.resolutions != 1 || net.downloads != 2 {
		t.Fatalf("discovery/downloads %d/%d", net.resolutions, net.downloads)
	}
	status, err := h.Run(context.Background(), filepath.Join(tx.Controller.Root, "psp.exe"), Options{Status: true})
	if err != nil || status.Code != 3 || !strings.Contains(status.Message, "completed:") {
		t.Fatalf("pending aggregate %#v %v", status, err)
	}
}

type cancellingStorage struct {
	Storage
	cancel     context.CancelFunc
	executable string
	reads      int
}

func (s *cancellingStorage) Select(exe string, verify bool) (runtimeinstall.Target, error) {
	if exe == s.executable {
		s.reads++
		if s.reads == 2 {
			s.cancel()
			return runtimeinstall.Target{}, context.Canceled
		}
	}
	return s.Storage.Select(exe, verify)
}
func TestCancellationDuringFinalHostUpdateReturns130(t *testing.T) {
	h, _, tx, _ := maintenanceFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	exe := filepath.Join(tx.Controller.Root, "psp.exe")
	h.Service.Files = &cancellingStorage{Storage: Disk{}, cancel: cancel, executable: exe}
	r, err := h.Run(ctx, exe, Options{Check: true})
	if err != nil || r.Code != 130 {
		t.Fatalf("%#v %v", r, err)
	}
}
