package selfupdate

import (
	"context"
	"fmt"
	"path/filepath"

	"portscoop/internal/filelock"
	"portscoop/internal/runtimeinstall"
)

// commitStore uses the executing Host, which is not one of the Store files
// being replaced. Keep the same worker ownership lock used by detached helpers.
func commitStore(ctx context.Context, tx *Transaction, recovery bool) (Result, error) {
	if tx.Target.Kind != "store" {
		return Result{}, fmt.Errorf("synchronous update requires a Store")
	}
	bounded, cancel := context.WithTimeout(ctx, runtimeinstall.BusyTimeout)
	defer cancel()
	l, err := filelock.Acquire(bounded, filepath.Join(tx.Directory, "worker.lock"))
	if err != nil {
		return Result{}, err
	}
	defer l.Close()
	if workerAlive(tx.Directory) || (recovery && processAlive(tx.ParentPID, tx.ParentCreated)) {
		return Result{}, fmt.Errorf("Store transaction still has a live owner")
	}
	pid, created, _, err := currentProcessIdentity()
	if err != nil {
		return Result{}, err
	}
	if err = runtimeinstall.Write(tx.Directory, "worker.json", workerIdentity{pid, created}); err != nil {
		return Result{}, err
	}
	if !recovery {
		if err = writeJournal(tx, Journal{Schema: 1, ID: tx.Target.ID, Nonce: tx.Nonce, Phase: "accepted"}); err != nil {
			return Result{}, err
		}
	}
	if err = commit(ctx, tx.Directory, recovery); err != nil {
		if _, pendingErr := runtimeinstall.ReadPending(tx.Target); pendingErr == nil {
			if j, e := readJournal(tx); e == nil {
				j.Phase = "recovery-required"
				j.Detail = err.Error()
				_ = writeJournal(tx, j)
			}
		}
		return Result{}, err
	}
	if recovery {
		return Result{0, "Previous Store runtime restored."}, nil
	}
	return Result{0, "Store runtime and managed backend updated to " + tx.Version + "."}, nil
}
