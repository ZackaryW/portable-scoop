package runner

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Assignment happens while the primary thread is suspended: no user code can
// spawn an uncontained descendant between CreateProcess and job assignment.
func startContained(process *exec.Cmd) (func(), error) {
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create process job: %w", err)
	}
	var once sync.Once
	closeJob := func() { once.Do(func() { _ = windows.CloseHandle(job) }) }
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		closeJob()
		return nil, fmt.Errorf("configure process job: %w", err)
	}
	process.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_SUSPENDED, HideWindow: true}
	if err := process.Start(); err != nil {
		closeJob()
		return nil, err
	}
	fail := func(err error) (func(), error) {
		_ = process.Process.Kill()
		closeJob()
		_ = process.Wait()
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(process.Process.Pid))
	if err != nil {
		return fail(fmt.Errorf("open managed process: %w", err))
	}
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		_ = windows.CloseHandle(handle)
		return fail(fmt.Errorf("contain managed process: %w", err))
	}
	if err := resumePrimaryThread(uint32(process.Process.Pid)); err != nil {
		_ = windows.CloseHandle(handle)
		return fail(err)
	}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		// Close the job as soon as its root exits, before exec.Wait joins any
		// stream-copy goroutines whose handles descendants might have inherited.
		_, _ = windows.WaitForSingleObject(handle, windows.INFINITE)
		closeJob()
		_ = windows.CloseHandle(handle)
	}()
	return func() { closeJob(); <-finished }, nil
}

func resumePrimaryThread(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return fmt.Errorf("enumerate suspended process thread: %w", err)
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return fmt.Errorf("open suspended process thread: %w", err)
		}
		count, err := windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if err != nil || count != 1 {
			return fmt.Errorf("resume managed process: suspension count %d: %v", count, err)
		}
		return nil
	}
	return fmt.Errorf("suspended primary thread for process %d was not found: %w", pid, err)
}
