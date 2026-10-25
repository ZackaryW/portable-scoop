package selfupdate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"portscoop/internal/filelock"
	"portscoop/internal/localipc"
	"portscoop/internal/runtimeinstall"
)

const WorkerOption = "__portscoop_update_worker"
const RecoverOption = "__portscoop_update_recover"
const ProbeOption = "__portscoop_update_probe"

// Probe reports only lifetime eligibility; it accepts no target or command.
func Probe() error { return independent() }

type WindowsWorker struct{}
type handoffRequest struct {
	Nonce    string
	Recovery bool
	PID      uint32
	Created  uint64
	Image    string
}
type acknowledgement struct {
	Accepted bool
	Error    string
}
type workerIdentity struct {
	PID     uint32
	Created uint64
}

func currentProcessIdentity() (uint32, uint64, string, error) {
	c, err := created(windows.CurrentProcess())
	if err != nil {
		return 0, 0, "", err
	}
	p, err := imagePath(windows.CurrentProcess())
	return uint32(os.Getpid()), c, p, err
}

func created(h windows.Handle) (uint64, error) {
	var c, e, k, u windows.Filetime
	err := windows.GetProcessTimes(h, &c, &e, &k, &u)
	return uint64(c.HighDateTime)<<32 | uint64(c.LowDateTime), err
}
func imagePath(h windows.Handle) (string, error) {
	b := make([]uint16, 32768)
	n := uint32(len(b))
	err := windows.QueryFullProcessImageName(h, 0, &b[0], &n)
	return windows.UTF16ToString(b[:n]), err
}
func workerAlive(dir string) bool {
	var w workerIdentity
	if runtimeinstall.Read(dir, "worker.json", &w) != nil {
		return false
	}
	return processAlive(w.PID, w.Created)
}

func processAlive(pid uint32, creation uint64) bool {
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(h)
	c, err := created(h)
	if err != nil || c != creation {
		return false
	}
	status, err := windows.WaitForSingleObject(h, 0)
	return err == nil && status == uint32(windows.WAIT_TIMEOUT)
}
func independent() error {
	var inJob uint32
	proc := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	ok, _, err := proc.Call(uintptr(windows.CurrentProcess()), 0, uintptr(unsafe.Pointer(&inJob)))
	if ok == 0 {
		return err
	}
	if inJob != 0 {
		return fmt.Errorf("external job prevents independent updater lifetime")
	}
	return nil
}
func (WindowsWorker) Handoff(ctx context.Context, tx *Transaction, recovery bool) (Result, error) {
	// Recovery may be called through the retained staged helper when entry points
	// are damaged. The same transaction, root and worker hash are still required.
	if recovery {
		owner, err := runtimeinstall.Lock(ctx, tx.Target, "update", true)
		if err != nil {
			return Result{}, err
		}
		defer owner.Close()
		if workerAlive(tx.Directory) {
			return Result{}, fmt.Errorf("update worker is still active")
		}
		probeCtx, cancel := context.WithTimeout(ctx, time.Second)
		probe, err := filelock.Acquire(probeCtx, filepath.Join(tx.Directory, "worker.lock"))
		cancel()
		if err != nil {
			return Result{}, fmt.Errorf("worker handoff is still owned: %w", err)
		}
		probe.Close()
	}
	h, err := runtimeinstall.Hash(tx.Directory, "worker.exe")
	if err != nil || h != tx.WorkerHash {
		return Result{}, fmt.Errorf("staged worker hash mismatch")
	}
	endpoint, err := localipc.New(tx.Directory)
	if err != nil {
		return Result{}, err
	}
	defer endpoint.Close()
	parentCreated, err := created(windows.CurrentProcess())
	if err != nil {
		return Result{}, err
	}
	parentImage, err := imagePath(windows.CurrentProcess())
	if err != nil {
		return Result{}, err
	}
	request := handoffRequest{tx.Nonce, recovery, uint32(os.Getpid()), parentCreated, parentImage}
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	if err = endpoint.Write(bounded, request); err != nil {
		return Result{}, err
	}
	cmd := exec.Command(filepath.Join(tx.Directory, "worker.exe"), WorkerOption, tx.Directory, endpoint.Directory())
	cmd.Dir = tx.Directory
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB, HideWindow: true}
	// nil streams map to NUL. No parent terminal handles or package kill job.
	if err = cmd.Start(); err != nil {
		return Result{}, fmt.Errorf("independent worker could not start; use --recover: %w", err)
	}
	var ack acknowledgement
	if err = endpoint.Read(bounded, &ack); err != nil {
		// No commit permission was sent; this worker must exit without replacing.
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Result{}, fmt.Errorf("worker startup not accepted; transaction %s retained for --recover: %w", filepath.Base(tx.Directory), err)
	}
	if !ack.Accepted {
		_ = cmd.Wait()
		return Result{}, fmt.Errorf("worker rejected handoff: %s", ack.Error)
	}
	if err = ctx.Err(); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return Result{}, err
	}
	// This permission is the ownership boundary. Never kill or repeat an update
	// automatically after it might have been observed by the worker.
	if err = endpoint.Write(bounded, acknowledgement{Accepted: true}); err != nil {
		_ = cmd.Process.Release()
		return Result{2, fmt.Sprintf("Handoff uncertain for %s; inspect self-update --status. No replacement was confirmed.", filepath.Base(tx.Directory))}, nil
	}
	if err = endpoint.Read(bounded, &ack); err != nil {
		_ = cmd.Process.Release()
		return Result{2, fmt.Sprintf("Handoff uncertain for %s; inspect self-update --status. Do not retry the update.", filepath.Base(tx.Directory))}, nil
	}
	_ = cmd.Process.Release()
	if !ack.Accepted {
		return Result{}, fmt.Errorf("worker could not accept transaction: %s", ack.Error)
	}
	return Result{0, fmt.Sprintf("Accepted transaction %s; replacement will finish after this launcher exits. Run this installation's psp self-update --status.", filepath.Base(tx.Directory))}, nil
}

func ServeWorker(ctx context.Context, directory, mailbox string) error {
	tx, err := loadTransaction(directory)
	if err != nil {
		return err
	}
	lockCtx, lockCancel := context.WithTimeout(ctx, time.Second)
	workerLock, err := filelock.Acquire(lockCtx, filepath.Join(directory, "worker.lock"))
	lockCancel()
	if err != nil {
		return err
	}
	defer workerLock.Close()
	if filepath.Dir(mailbox) != directory {
		return fmt.Errorf("worker mailbox outside transaction")
	}
	e, err := localipc.Open(mailbox)
	if err != nil {
		return err
	}
	defer e.Close()
	bounded, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	fail := func(err error) error { _ = e.Write(bounded, acknowledgement{Error: err.Error()}); return err }
	if err = independent(); err != nil {
		return fail(err)
	}
	self, err := os.Executable()
	if err != nil {
		return fail(err)
	}
	if !strings.EqualFold(self, filepath.Join(directory, "worker.exe")) {
		return fail(fmt.Errorf("worker must execute the owned staged runtime"))
	}
	h, err := runtimeinstall.Hash(directory, "worker.exe")
	if err != nil || h != tx.WorkerHash {
		return fail(fmt.Errorf("worker hash changed"))
	}
	var request handoffRequest
	if err = e.Read(bounded, &request); err != nil {
		return fail(err)
	}
	if request.Nonce != tx.Nonce {
		return fail(fmt.Errorf("worker nonce mismatch"))
	}
	parent, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, request.PID)
	if err != nil {
		return fail(err)
	}
	defer windows.CloseHandle(parent)
	c, err := created(parent)
	if err != nil || c != request.Created {
		return fail(fmt.Errorf("invoking process identity changed"))
	}
	image, err := imagePath(parent)
	if err != nil || !strings.EqualFold(image, request.Image) {
		return fail(fmt.Errorf("invoking image changed"))
	}
	allowed := strings.EqualFold(image, filepath.Join(tx.Target.Root, "psp.exe")) || strings.EqualFold(image, filepath.Join(tx.Target.Root, "portscoop.exe")) || (request.Recovery && strings.EqualFold(image, self))
	if !allowed {
		return fail(fmt.Errorf("invoking process is not this installation's management runtime"))
	}
	if err = verifySources(tx); err != nil {
		return fail(err)
	}
	if workerAlive(directory) {
		return fail(fmt.Errorf("another worker already owns transaction"))
	}
	if err = e.Write(bounded, acknowledgement{Accepted: true}); err != nil {
		return err
	}
	var permission acknowledgement
	if err = e.Read(bounded, &permission); err != nil || !permission.Accepted {
		return fmt.Errorf("worker commit permission missing")
	}
	ctime, err := created(windows.CurrentProcess())
	if err != nil {
		return fail(err)
	}
	if err = runtimeinstall.Write(directory, "worker.json", workerIdentity{uint32(os.Getpid()), ctime}); err != nil {
		return fail(err)
	}
	j, err := readJournal(tx)
	if err != nil {
		return fail(err)
	}
	j.Phase = "waiting-for-parent"
	if err = writeJournal(tx, j); err != nil {
		return fail(err)
	}
	if err = e.Write(bounded, acknowledgement{Accepted: true}); err != nil {
		return err
	}
	status, err := windows.WaitForSingleObject(parent, 30000)
	if err != nil || status != windows.WAIT_OBJECT_0 {
		j.Phase = "recovery-required"
		j.Detail = "Invoking process did not exit within 30 seconds"
		_ = writeJournal(tx, j)
		return fmt.Errorf("parent exit wait failed")
	}
	if err = commit(context.Background(), directory, request.Recovery); err != nil {
		// Never overwrite a completed/restored result with stale progress.
		if _, pendingErr := runtimeinstall.ReadPending(tx.Target); pendingErr == nil {
			j.Phase = "recovery-required"
			j.Detail = err.Error()
			_ = writeJournal(tx, j)
		}
		return err
	}
	return nil
}

func RecoverStaged(ctx context.Context, directory string) (Result, error) {
	tx, err := loadTransaction(directory)
	if err != nil {
		return Result{}, err
	}
	self, err := os.Executable()
	if err != nil || !strings.EqualFold(self, filepath.Join(directory, "worker.exe")) {
		return Result{}, fmt.Errorf("recovery helper must be the recorded staged executable")
	}
	return (WindowsWorker{}).Handoff(ctx, tx, true)
}
