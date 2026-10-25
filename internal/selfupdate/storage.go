package selfupdate

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"portscoop/internal/atomicfile"
	"portscoop/internal/environment"
	"portscoop/internal/localipc"
	"portscoop/internal/runtimeinstall"
)

type Disk struct{}

func (Disk) Pending(t runtimeinstall.Target) error {
	_, err := runtimeinstall.ReadPending(t)
	return err
}
func (Disk) Abandon(tx *Transaction, cause error) error {
	if err := verifyInstalled(tx, true); err != nil {
		return err
	}
	return finish(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "failed", Detail: "Worker startup failed before replacement: " + cause.Error()})
}

func (Disk) Select(exe string, verify bool) (runtimeinstall.Target, error) {
	name := strings.ToLower(filepath.Base(exe))
	if name != "psp.exe" && name != "portscoop.exe" {
		return runtimeinstall.Target{}, fmt.Errorf("self-update requires an owned psp.exe or portscoop.exe deployment")
	}
	t, err := runtimeinstall.Select(exe, verify)
	if !verify && err != nil && t.ID != "" {
		return t, nil
	} // identity survives partial replacement
	return t, err
}
func (Disk) Ensure(t runtimeinstall.Target) (runtimeinstall.Target, error) {
	t, err := runtimeinstall.EnsureHost(t)
	if err != nil {
		return t, err
	}
	p, err := runtimeinstall.CheckPath(t.Root, runtimeinstall.ControlRelative(t))
	if err != nil {
		return t, err
	}
	err = os.MkdirAll(p, 0700)
	return t, err
}
func copyFile(sourceRoot, source, targetRoot, target string) error {
	s, err := runtimeinstall.CheckPath(sourceRoot, source)
	if err != nil {
		return err
	}
	d, err := runtimeinstall.CheckPath(targetRoot, target)
	if err != nil {
		return err
	}
	f, err := os.Open(s)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("copy source is not a regular file")
	}
	if err = os.MkdirAll(filepath.Dir(d), 0700); err != nil {
		return err
	}
	if _, err = runtimeinstall.CheckPath(targetRoot, target); err != nil {
		return err
	}
	out, err := os.OpenFile(d, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, f)
	if err == nil {
		err = out.Sync()
	}
	closeErr := out.Close()
	if err == nil {
		err = closeErr
	}
	return err
}
func (Disk) Stage(ctx context.Context, t runtimeinstall.Target, r Release, net Downloader, now time.Time) (tx *Transaction, err error) {
	endpoint, err := localipc.New(t.Control())
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			_ = endpoint.Close()
		}
	}()
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		return nil, err
	}
	tx = &Transaction{Schema: 1, Target: t, Directory: endpoint.Directory(), Nonce: hex.EncodeToString(nonce[:]), Version: r.Tag, Created: now}
	var checksum strings.Builder
	if err = net.Download(ctx, r.Checksum, &checksum, 4096); err != nil {
		return nil, err
	}
	parts := strings.Fields(checksum.String())
	if len(parts) != 2 || parts[1] != asset || !runtimeinstall.DigestPattern.MatchString(parts[0]) {
		return nil, fmt.Errorf("invalid release checksum sidecar")
	}
	f, err := os.OpenFile(filepath.Join(tx.Directory, "candidate.zip"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return nil, err
	}
	err = net.Download(ctx, r.Archive, f, ArchiveLimit)
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return nil, err
	}
	h, err := runtimeinstall.Hash(tx.Directory, "candidate.zip")
	if err != nil {
		return nil, err
	}
	if h != parts[0] {
		return nil, fmt.Errorf("release ZIP checksum mismatch")
	}
	if err = extract(tx.Directory); err != nil {
		return nil, err
	}
	_, d, err := runtimeinstall.VerifyDistribution(filepath.Join(tx.Directory, "candidate"), false)
	if err != nil {
		return nil, err
	}
	if normalize(d.Version) != r.Tag {
		return nil, fmt.Errorf("candidate descriptor does not match pinned tag")
	}
	return tx, nil
}
func extract(root string) error {
	return extractArchive(filepath.Join(root, "candidate.zip"), root)
}

func extractArchive(archivePath, root string) error {
	z, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer z.Close()
	if len(z.File) > 20000 {
		return fmt.Errorf("too many archive entries")
	}
	seen := map[string]bool{}
	var total uint64
	for _, e := range z.File {
		n := strings.TrimSuffix(e.Name, "/")
		key := strings.ToLower(n)
		if seen[key] || strings.Contains(n, `\`) {
			return fmt.Errorf("duplicate or noncanonical ZIP path")
		}
		seen[key] = true
		if e.Mode()&(os.ModeSymlink|os.ModeIrregular) != 0 {
			return fmt.Errorf("ZIP reparse entry")
		}
		p, err := runtimeinstall.CheckPath(root, "candidate/"+n)
		if err != nil {
			return err
		}
		if e.FileInfo().IsDir() {
			if err = os.MkdirAll(p, 0700); err != nil {
				return err
			}
			continue
		}
		if e.UncompressedSize64 > uint64(ArchiveLimit) || total > 2*1024*1024*1024-e.UncompressedSize64 {
			return fmt.Errorf("expanded archive exceeds bound")
		}
		total += e.UncompressedSize64
		if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
			return err
		}
		in, err := e.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(p, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			in.Close()
			return err
		}
		nbytes, e1 := io.Copy(out, io.LimitReader(in, int64(e.UncompressedSize64)+1))
		e2 := out.Close()
		in.Close()
		if e1 != nil {
			return e1
		}
		if e2 != nil {
			return e2
		}
		if uint64(nbytes) != e.UncompressedSize64 {
			return fmt.Errorf("ZIP size mismatch")
		}
	}
	return nil
}
func (Disk) Prepare(tx *Transaction) error {
	t := tx.Target
	candidate := filepath.Join(tx.Directory, "candidate")
	r, d, err := runtimeinstall.VerifyDistribution(candidate, false)
	if err != nil {
		return err
	}
	old := map[string]string{}
	next := map[string]string{}
	if t.Kind == "host" {
		previous, _, err := runtimeinstall.VerifyDistribution(t.Root, true)
		if err != nil {
			return err
		}
		old = previous.Artifacts
		next = r.Artifacts
		old["release.json"], err = runtimeinstall.Hash(t.Root, "release.json")
		if err != nil {
			return err
		}
		next["release.json"], err = runtimeinstall.Hash(candidate, "release.json")
		if err != nil {
			return err
		}
	} else {
		receipt, err := runtimeinstall.VerifyReceipt(t.Root, t.ID)
		if err != nil {
			return err
		}
		old = receipt.Artifacts
		var b environment.BackendMetadata
		if err = runtimeinstall.Read(t.Root, ".portscoop/backend/backend.json", &b); err != nil {
			return err
		}
		if err = b.Validate(); err != nil {
			return err
		}
		tx.BackendHash, err = runtimeinstall.Hash(t.Root, ".portscoop/backend/backend.json")
		if err != nil {
			return err
		}
		contract := runtimeinstall.Contract{Schema: b.SchemaVersion, Protocol: b.ProtocolMajor, Upstream: b.UpstreamRevision, Patch: b.PatchRevision}
		compatible := false
		for _, c := range d.Backends {
			if c == contract {
				compatible = true
			}
		}
		if tx.Controller != nil {
			previous, err := backendInventory(t.Root, backendMetadata, false, runtimeinstall.Release{})
			if err != nil {
				return err
			}
			updated, err := backendInventory(candidate, "backend/backend.json", true, r)
			if err != nil {
				return err
			}
			addInventory(old, previous)
			addInventory(next, updated)
		} else if !compatible {
			return fmt.Errorf("candidate needs a different backend; use a current Host self-update to update this registered Store")
		}
		for _, n := range runtimeinstall.Launchers {
			next[n] = r.Artifacts[n]
		}
		if err = runtimeinstall.Write(tx.Directory, "receipt.json", runtimeinstall.NewReceipt(t.ID, d, r)); err != nil {
			return err
		}
		old[runtimeinstall.ReceiptName], err = runtimeinstall.Hash(t.Root, runtimeinstall.ReceiptName)
		if err != nil {
			return err
		}
		next[runtimeinstall.ReceiptName], err = runtimeinstall.Hash(tx.Directory, "receipt.json")
		if err != nil {
			return err
		}
	}
	keys := map[string]bool{}
	for p := range old {
		keys[p] = true
	}
	for p := range next {
		keys[p] = true
	}
	names := []string{}
	for p := range keys {
		names = append(names, p)
	}
	sort.Strings(names)
	for _, p := range names {
		h, err := runtimeinstall.Hash(t.Root, p)
		if old[p] == "" {
			if !os.IsNotExist(err) {
				return fmt.Errorf("new artifact collides with existing data: %s", p)
			}
		} else if err != nil || h != old[p] {
			return fmt.Errorf("owned artifact changed: %s", p)
		}
		op := Operation{p, old[p], next[p]}
		tx.Operations = append(tx.Operations, op)
		if op.Old != "" {
			if err = copyFile(t.Root, p, tx.Directory, "backup/"+p); err != nil {
				return err
			}
			if h, err = runtimeinstall.Hash(tx.Directory, "backup/"+p); err != nil || h != op.Old {
				return fmt.Errorf("backup verification failed: %s", p)
			}
		}
	}
	if tx.Controller != nil {
		err = prepareController(tx)
	} else {
		err = copyFile(t.Root, "psp.exe", tx.Directory, "worker.exe")
	}
	if err != nil {
		return err
	}
	tx.WorkerHash, err = runtimeinstall.Hash(tx.Directory, "worker.exe")
	if err != nil {
		return err
	}
	if tx.Controller == nil && tx.WorkerHash != old["psp.exe"] {
		return fmt.Errorf("worker does not match current owned launcher")
	}
	tx.ParentPID, tx.ParentCreated, tx.ParentImage, err = currentProcessIdentity()
	if err != nil {
		return err
	}
	return nil
}
func (Disk) Publish(tx *Transaction) error {
	if err := runtimeinstall.Write(tx.Directory, "plan.json", tx); err != nil {
		return err
	}
	if err := writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "staged"}); err != nil {
		return err
	}
	p := runtimeinstall.Pending{Schema: 1, ID: tx.Target.ID, Transaction: filepath.Base(tx.Directory), Nonce: tx.Nonce}
	return runtimeinstall.Write(tx.Target.Root, runtimeinstall.ControlRelative(tx.Target)+"/pending.json", p)
}
func (Disk) Discard(tx *Transaction) error {
	if tx == nil {
		return nil
	}
	e, err := localipc.Open(tx.Directory)
	if err != nil {
		return err
	}
	_ = e // validates private owner and DACL
	if filepath.Dir(tx.Directory) != tx.Target.Control() {
		return fmt.Errorf("invalid staging parent")
	}
	return os.RemoveAll(tx.Directory)
}
func (Disk) Load(t runtimeinstall.Target) (*Transaction, error) {
	p, err := runtimeinstall.ReadPending(t)
	if err != nil {
		return nil, err
	}
	return loadTransaction(filepath.Join(t.Control(), p.Transaction))
}
func loadTransaction(directory string) (*Transaction, error) {
	if _, err := localipc.Open(directory); err != nil {
		return nil, err
	}
	var tx Transaction
	if err := runtimeinstall.Read(directory, "plan.json", &tx); err != nil {
		return nil, err
	}
	if tx.Schema != 1 || tx.Directory != directory || filepath.Dir(directory) != tx.Target.Control() || !environment.ValidID(tx.Target.ID) || !runtimeinstall.DigestPattern.MatchString(tx.Nonce) || !runtimeinstall.DigestPattern.MatchString(tx.WorkerHash) || len(tx.Operations) == 0 {
		return nil, fmt.Errorf("invalid update transaction")
	}
	if tx.Target.Kind != "host" && tx.Target.Kind != "store" {
		return nil, fmt.Errorf("invalid update target kind")
	}
	canonical, err := filepath.EvalSymlinks(tx.Target.Root)
	if err != nil {
		return nil, err
	}
	if canonical != tx.Target.Root {
		return nil, fmt.Errorf("update installation moved; restore the recorded location before recovery")
	}
	current, err := (Disk{}).Select(filepath.Join(tx.Target.Root, "psp.exe"), false)
	if err != nil {
		return nil, err
	}
	if current.ID != tx.Target.ID || current.Kind != tx.Target.Kind {
		return nil, fmt.Errorf("update installation identity changed")
	}
	p, err := runtimeinstall.ReadPending(tx.Target)
	if err != nil {
		return nil, err
	}
	if p.Nonce != tx.Nonce || p.Transaction != filepath.Base(directory) {
		return nil, fmt.Errorf("transaction does not own pending state")
	}
	seen := map[string]bool{}
	for _, op := range tx.Operations {
		key := strings.ToLower(op.Path)
		if seen[key] || (op.Old == "" && op.New == "") || (op.Old != "" && !runtimeinstall.DigestPattern.MatchString(op.Old)) || (op.New != "" && !runtimeinstall.DigestPattern.MatchString(op.New)) {
			return nil, fmt.Errorf("invalid replacement digest or duplicate path")
		}
		seen[key] = true
		if tx.Target.Kind == "host" {
			if !runtimeinstall.DistributionPath(op.Path) {
				return nil, fmt.Errorf("operation outside distribution")
			}
		} else {
			_, backendPath := backendSource(op.Path)
			allowed := op.Path == runtimeinstall.ReceiptName || (tx.Controller != nil && backendPath)
			for _, n := range runtimeinstall.Launchers {
				allowed = allowed || op.Path == n
			}
			if !allowed {
				return nil, fmt.Errorf("operation outside Store runtime")
			}
		}
		if _, err := runtimeinstall.CheckPath(tx.Target.Root, op.Path); err != nil {
			return nil, err
		}
	}
	return &tx, nil
}
func writeJournal(tx *Transaction, j Journal) error {
	return runtimeinstall.Write(tx.Directory, "journal.json", j)
}
func readJournal(tx *Transaction) (Journal, error) {
	var j Journal
	err := runtimeinstall.Read(tx.Directory, "journal.json", &j)
	if err == nil && (j.Schema != 1 || j.ID != tx.Target.ID || j.Nonce != tx.Nonce || j.Applied < 0 || j.Applied > len(tx.Operations)) {
		err = fmt.Errorf("untrusted update journal")
	}
	return j, err
}
func (Disk) Status(t runtimeinstall.Target) (Result, error) {
	tx, err := (Disk{}).Load(t)
	if os.IsNotExist(err) {
		var last Journal
		err = runtimeinstall.Read(t.Root, runtimeinstall.ControlRelative(t)+"/result.json", &last)
		if os.IsNotExist(err) {
			return Result{0, "No update transaction."}, nil
		}
		if err != nil {
			return Result{}, err
		}
		if last.Schema != 1 || last.ID != t.ID {
			return Result{}, fmt.Errorf("invalid update result")
		}
		code := 0
		if last.Phase != "completed" {
			code = 2
		}
		return Result{code, last.Phase + ": " + last.Detail}, nil
	}
	if err != nil {
		return Result{}, err
	}
	j, err := readJournal(tx)
	if err != nil {
		return Result{}, err
	}
	code := 3
	active := workerAlive(tx.Directory) || (j.Phase == "staged" && processAlive(tx.ParentPID, tx.ParentCreated))
	if j.Phase == "failed" || j.Phase == "recovery-required" || !active {
		code = 2
	}
	if code == 2 && j.Phase != "failed" && j.Phase != "recovery-required" {
		j.Detail = "No live owner; explicit recovery required. " + j.Detail
	}
	return Result{code, fmt.Sprintf("%s: %s (%s). %s", filepath.Base(tx.Directory), j.Phase, tx.Version, j.Detail)}, nil
}

// replace publishes from a same-directory temporary file. Only fixed owned
// files are ever removed; there is no recursive deletion of installed trees.
func replace(tx *Transaction, op Operation, rollback bool) error {
	expected, wanted, source := op.Old, op.New, candidatePath(op.Path)
	if rollback {
		expected, wanted, source = op.New, op.Old, "backup/"+op.Path
	}
	actual, err := runtimeinstall.Hash(tx.Target.Root, op.Path)
	if os.IsNotExist(err) {
		actual = ""
	} else if err != nil {
		return err
	}
	if actual != expected && actual != wanted && !(rollback && actual == "") {
		return fmt.Errorf("replacement destination drift: %s", op.Path)
	}
	if actual == wanted {
		return nil
	}
	p, err := runtimeinstall.CheckPath(tx.Target.Root, op.Path)
	if err != nil {
		return err
	}
	if wanted == "" {
		return os.Remove(p)
	}
	h, err := runtimeinstall.Hash(tx.Directory, source)
	if err != nil || h != wanted {
		return fmt.Errorf("replacement source hash mismatch: %s", op.Path)
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(p), "portscoop-replace-*.tmp")
	if err != nil {
		return err
	}
	name := temp.Name()
	temp.Close()
	os.Remove(name)
	defer os.Remove(name)
	rel, err := filepath.Rel(tx.Target.Root, name)
	if err != nil {
		return err
	}
	if err = copyFile(tx.Directory, source, tx.Target.Root, rel); err != nil {
		return err
	}
	if _, err = runtimeinstall.CheckPath(tx.Target.Root, op.Path); err != nil {
		return err
	}
	return atomicfile.Replace(name, p)
}
func candidatePath(p string) string {
	if p == runtimeinstall.ReceiptName {
		return "receipt.json"
	}
	if source, ok := backendSource(p); ok {
		return "candidate/" + source
	}
	return "candidate/" + p
}
