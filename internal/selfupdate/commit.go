package selfupdate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/filelock"
	"portscoop/internal/runtimeinstall"
)

func verifySources(tx *Transaction) error {
	r, d, err := runtimeinstall.VerifyDistribution(filepath.Join(tx.Directory, "candidate"), false)
	if err != nil {
		return err
	}
	if normalize(d.Version) != tx.Version {
		return fmt.Errorf("staged version differs from transaction")
	}
	old, next := map[string]string{}, map[string]string{}
	if tx.Target.Kind == "host" {
		previous, _, err := runtimeinstall.VerifyDistribution(filepath.Join(tx.Directory, "backup"), false)
		if err != nil {
			return err
		}
		old = previous.Artifacts
		next = r.Artifacts
		old["release.json"], err = runtimeinstall.Hash(tx.Directory, "backup/release.json")
		if err != nil {
			return err
		}
		next["release.json"], err = runtimeinstall.Hash(tx.Directory, "candidate/release.json")
		if err != nil {
			return err
		}
	} else {
		h, err := runtimeinstall.Hash(tx.Target.Root, ".portscoop/backend/backend.json")
		if err != nil || h != tx.BackendHash {
			return fmt.Errorf("retained Store backend changed")
		}
		var backend environment.BackendMetadata
		if err = runtimeinstall.Read(tx.Target.Root, ".portscoop/backend/backend.json", &backend); err != nil {
			return err
		}
		compatible := false
		for _, c := range d.Backends {
			if c == (runtimeinstall.Contract{Schema: backend.SchemaVersion, Protocol: backend.ProtocolMajor, Upstream: backend.UpstreamRevision, Patch: backend.PatchRevision}) {
				compatible = true
			}
		}
		if !compatible {
			return fmt.Errorf("candidate does not support the retained backend")
		}
		var before, after runtimeinstall.Receipt
		if err = runtimeinstall.Read(tx.Directory, "backup/"+runtimeinstall.ReceiptName, &before); err != nil {
			return err
		}
		if err = runtimeinstall.Read(tx.Directory, "receipt.json", &after); err != nil {
			return err
		}
		if before.Schema != 1 || after.Schema != 1 || before.ID != tx.Target.ID || after.ID != tx.Target.ID || normalize(after.Version) != tx.Version || len(before.Artifacts) != 3 || len(after.Artifacts) != 3 {
			return fmt.Errorf("invalid transaction receipt")
		}
		for _, n := range runtimeinstall.Launchers {
			old[n] = before.Artifacts[n]
			next[n] = r.Artifacts[n]
			if after.Artifacts[n] != next[n] {
				return fmt.Errorf("candidate receipt hash mismatch")
			}
		}
		old[runtimeinstall.ReceiptName], err = runtimeinstall.Hash(tx.Directory, "backup/"+runtimeinstall.ReceiptName)
		if err != nil {
			return err
		}
		next[runtimeinstall.ReceiptName], err = runtimeinstall.Hash(tx.Directory, "receipt.json")
		if err != nil {
			return err
		}
	}
	keys := map[string]bool{}
	for n := range old {
		keys[n] = true
	}
	for n := range next {
		keys[n] = true
	}
	if len(keys) != len(tx.Operations) {
		return fmt.Errorf("transaction operation set differs from inventories")
	}
	for _, op := range tx.Operations {
		if !keys[op.Path] || op.Old != old[op.Path] || op.New != next[op.Path] {
			return fmt.Errorf("operation is not owned by verified inventories")
		}
		if op.Old != "" {
			h, err := runtimeinstall.Hash(tx.Directory, "backup/"+op.Path)
			if err != nil || h != op.Old {
				return fmt.Errorf("backup damaged: %s", op.Path)
			}
		}
		if op.New != "" {
			h, err := runtimeinstall.Hash(tx.Directory, candidatePath(op.Path))
			if err != nil || h != op.New {
				return fmt.Errorf("candidate damaged: %s", op.Path)
			}
		}
	}
	if old["psp.exe"] != tx.WorkerHash {
		return fmt.Errorf("worker is not the verified previous runtime")
	}
	return nil
}
func verifyInstalled(tx *Transaction, rollback bool) error {
	for _, op := range tx.Operations {
		want := op.New
		if rollback {
			want = op.Old
		}
		h, err := runtimeinstall.Hash(tx.Target.Root, op.Path)
		if want == "" && os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		if h != want {
			return fmt.Errorf("runtime coherence failed: %s", op.Path)
		}
	}
	return nil
}
func finish(tx *Transaction, j Journal) error {
	if err := writeJournal(tx, j); err != nil {
		return err
	}
	if err := runtimeinstall.Write(tx.Target.Root, runtimeinstall.ControlRelative(tx.Target)+"/result.json", j); err != nil {
		return err
	}
	p, err := runtimeinstall.ReadPending(tx.Target)
	if err != nil {
		return err
	}
	if p.Nonce != tx.Nonce {
		return fmt.Errorf("pending owner changed")
	}
	path, err := runtimeinstall.CheckPath(tx.Target.Root, runtimeinstall.ControlRelative(tx.Target)+"/pending.json")
	if err != nil {
		return err
	}
	return os.Remove(path)
}
func commit(ctx context.Context, directory string, recover bool) error {
	tx, err := loadTransaction(directory)
	if err != nil {
		return err
	}
	owner, err := runtimeinstall.Lock(ctx, tx.Target, "update", true)
	if err != nil {
		return err
	}
	defer owner.Close()
	use, err := runtimeinstall.Lock(ctx, tx.Target, "runtime", true)
	if err != nil {
		return err
	}
	defer use.Close()
	// In-place conversion of a fresh distribution to a Store uses this lock
	// before it has a runtime identity or shared lease.
	initPath, err := runtimeinstall.CheckPath(tx.Target.Root, ".portscoop-init.lock")
	if err != nil {
		return err
	}
	initCtx, initCancel := context.WithTimeout(ctx, 5*time.Second)
	initLock, err := filelock.Acquire(initCtx, initPath)
	initCancel()
	if err != nil {
		return err
	}
	defer initLock.Close()
	if tx.Target.Kind == "store" {
		path, err := runtimeinstall.CheckPath(tx.Target.Root, ".portscoop/state/operation.lock")
		if err != nil {
			return err
		}
		bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
		lock, err := filelock.Acquire(bounded, path)
		cancel()
		if err != nil {
			return err
		}
		defer lock.Close()
	}
	// Reload every identity and immutable operation under worker-owned locks.
	tx, err = loadTransaction(directory)
	if err != nil {
		return err
	}
	if err = verifySources(tx); err != nil {
		return err
	}
	j, err := readJournal(tx)
	if err != nil {
		return err
	}
	if recover {
		j.Phase = "recovering"
		if err = writeJournal(tx, j); err != nil {
			return err
		}
		for i := len(tx.Operations) - 1; i >= 0; i-- {
			if err = replace(tx, tx.Operations[i], true); err != nil {
				j.Phase = "recovery-required"
				j.Detail = err.Error()
				_ = writeJournal(tx, j)
				return err
			}
		}
		if err = verifyInstalled(tx, true); err != nil {
			return err
		}
		j.Phase = "failed"
		j.Detail = "Previous runtime restored by explicit recovery."
		return finish(tx, j)
	}
	if j.Phase != "accepted" && j.Phase != "waiting-for-parent" {
		return fmt.Errorf("transaction cannot enter commit from %s", j.Phase)
	}
	if err = verifyInstalled(tx, true); err != nil {
		return err
	}
	j.Phase = "committing"
	if err = writeJournal(tx, j); err != nil {
		return err
	}
	for i, op := range tx.Operations {
		if err = replace(tx, op, false); err != nil {
			break
		}
		j.Applied = i + 1
		if err = writeJournal(tx, j); err != nil {
			break
		}
	}
	if err == nil {
		err = verifyInstalled(tx, false)
	}
	if err == nil {
		j.Phase = "completed"
		j.Detail = "Updated to " + tx.Version
		return finish(tx, j)
	}
	j.Phase = "recovery-required"
	j.Detail = err.Error()
	_ = writeJournal(tx, j)
	for i := len(tx.Operations) - 1; i >= 0; i-- {
		if rollbackErr := replace(tx, tx.Operations[i], true); rollbackErr != nil {
			j.Detail += "; rollback: " + rollbackErr.Error()
			_ = writeJournal(tx, j)
			return err
		}
	}
	if rollbackErr := verifyInstalled(tx, true); rollbackErr != nil {
		return rollbackErr
	}
	j.Phase = "failed"
	j.Detail += "; previous runtime restored"
	if finalErr := finish(tx, j); finalErr != nil {
		return finalErr
	}
	return err
}
