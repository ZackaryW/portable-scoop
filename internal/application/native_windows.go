package application

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Native has an application-specific lifecycle. Package runner.Native remains
// responsible for unconditional descendant teardown on package completion.
type Native struct{}

type nativeProcess struct {
	mu                           sync.Mutex
	command                      *exec.Cmd
	job                          windows.Handle
	mode                         Mode
	committed, aborted, rootDone bool
	finished                     chan struct{}
	code                         int
	err                          error
}

var _ Starter = Native{}
var _ Process = (*nativeProcess)(nil)

func (Native) Start(ctx context.Context, spec Command) (Process, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !filepath.IsAbs(spec.Executable) || (spec.Mode != Wait && spec.Mode != Detach) {
		return nil, fmt.Errorf("invalid application process specification")
	}
	for _, s := range append([]string{spec.Executable, spec.Directory, spec.WindowsCommandLine}, spec.Args...) {
		if strings.ContainsRune(s, 0) {
			return nil, fmt.Errorf("NUL in application process specification")
		}
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create application job: %w", err)
	}
	p := &nativeProcess{job: job, mode: spec.Mode, finished: make(chan struct{})}
	// Explicit breakaway is permitted for application delegation descendants.
	// We never change an inherited external job's limits.
	if err := setJobLimits(job, windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE|windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK); err != nil {
		windows.CloseHandle(job)
		return nil, err
	}
	cmd := exec.Command(spec.Executable, spec.Args...)
	cmd.Env, cmd.Dir = spec.Environment, spec.Directory
	cmd.WaitDelay = 500 * time.Millisecond
	flags := uint32(windows.CREATE_SUSPENDED)
	if spec.Mode == Detach {
		flags |= windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_BREAKAWAY_FROM_JOB
		if spec.HiddenConsole {
			flags |= windows.CREATE_NEW_CONSOLE
		} else {
			flags |= windows.DETACHED_PROCESS
		}
		// nil streams deliberately map to NUL, never a launcher terminal/pipe.
	} else {
		cmd.Stdin = spec.Streams.In
		cmd.Stdout, cmd.Stderr = applicationOutput(spec.Streams.Out), applicationOutput(spec.Streams.Err)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: flags, HideWindow: spec.HiddenConsole, CmdLine: spec.WindowsCommandLine}
	p.command = cmd
	if err := cmd.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("start application (an external job may prohibit detached lifetime): %w", err)
	}
	fail := func(err error) (Process, error) {
		_ = cmd.Process.Kill()
		_ = windows.CloseHandle(job)
		_ = cmd.Wait()
		return nil, err
	}
	handle, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE|windows.SYNCHRONIZE, false, uint32(cmd.Process.Pid))
	if err != nil {
		return fail(fmt.Errorf("open application process: %w", err))
	}
	if err := windows.AssignProcessToJobObject(job, handle); err != nil {
		windows.CloseHandle(handle)
		return fail(fmt.Errorf("assign application job: %w", err))
	}
	if err := resumeApplication(uint32(cmd.Process.Pid)); err != nil {
		windows.CloseHandle(handle)
		return fail(err)
	}
	go p.reap(handle)
	if err := ctx.Err(); err != nil {
		_ = p.Abort()
		return nil, err
	}
	return p, nil
}

type forwardedOutput struct{ io.Writer }

func applicationOutput(writer io.Writer) io.Writer {
	if file, ok := writer.(*os.File); ok {
		var mode uint32
		if windows.GetConsoleMode(windows.Handle(file.Fd()), &mode) == nil {
			return writer
		}
		kind, err := windows.GetFileType(windows.Handle(file.Fd()))
		if err == nil && kind == windows.FILE_TYPE_PIPE {
			// Force exec.Cmd to own a forwarding pipe and its bounded drain.
			// Passing the caller's pipe handle directly lets a surviving child
			// keep the caller's pipeline open after both pspe helpers have exited.
			return forwardedOutput{file}
		}
	}
	// Console handles must remain consoles for interactive/TUI applications.
	return writer
}

func setJobLimits(job windows.Handle, flags uint32) error {
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = flags
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	if err != nil {
		return fmt.Errorf("configure application lifetime: %w", err)
	}
	return nil
}

// releaseLocked removes kill-on-close before releasing successful ownership.
func (p *nativeProcess) releaseLocked() error {
	if p.job == 0 {
		return nil
	}
	if err := setJobLimits(p.job, windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK); err != nil {
		return err
	}
	err := windows.CloseHandle(p.job)
	p.job = 0
	return err
}

func (p *nativeProcess) reap(handle windows.Handle) {
	defer close(p.finished)
	_, waitErr := windows.WaitForSingleObject(handle, windows.INFINITE)
	_ = windows.CloseHandle(handle)
	p.mu.Lock()
	p.rootDone = true
	if p.mode == Wait && !p.aborted {
		if err := p.releaseLocked(); err != nil {
			p.err = err
			if p.job != 0 {
				_ = windows.CloseHandle(p.job)
				p.job = 0
			}
		}
	}
	p.mu.Unlock()
	err := p.command.Wait()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.err != nil {
		return
	}
	if waitErr != nil {
		p.err = fmt.Errorf("wait for application: %w", waitErr)
		return
	}
	var status *exec.ExitError
	switch {
	case errors.As(err, &status):
		p.code = status.ExitCode()
	case err == nil || errors.Is(err, exec.ErrWaitDelay):
		p.code = p.command.ProcessState.ExitCode()
	default:
		p.err = fmt.Errorf("wait for application streams: %w", err)
	}
}

func (p *nativeProcess) Wait(ctx context.Context) (int, error) {
	if p.mode != Wait {
		return 0, fmt.Errorf("cannot wait on a detached application")
	}
	select {
	case <-p.finished:
	case <-ctx.Done():
		_ = p.Abort()
		return 0, ctx.Err()
	}
	return p.code, p.err
}

func (p *nativeProcess) Commit(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.mode != Detach {
		return fmt.Errorf("only detached applications have a handoff")
	}
	if p.committed {
		return nil
	}
	if p.aborted {
		return fmt.Errorf("detached startup was aborted")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := p.releaseLocked(); err != nil {
		return err
	}
	p.committed = true
	return nil
}

func (p *nativeProcess) Abort() error {
	p.mu.Lock()
	if p.committed || (p.mode == Wait && p.rootDone) {
		p.mu.Unlock()
		return nil
	}
	p.aborted = true
	if p.job != 0 {
		_ = windows.TerminateJobObject(p.job, 130)
		_ = windows.CloseHandle(p.job)
		p.job = 0
	}
	p.mu.Unlock()
	<-p.finished
	return nil
}

func resumeApplication(pid uint32) error {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPTHREAD, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(snapshot)
	entry := windows.ThreadEntry32{Size: uint32(unsafe.Sizeof(windows.ThreadEntry32{}))}
	for err = windows.Thread32First(snapshot, &entry); err == nil; err = windows.Thread32Next(snapshot, &entry) {
		if entry.OwnerProcessID != pid {
			continue
		}
		thread, err := windows.OpenThread(windows.THREAD_SUSPEND_RESUME, false, entry.ThreadID)
		if err != nil {
			return err
		}
		count, err := windows.ResumeThread(thread)
		_ = windows.CloseHandle(thread)
		if err != nil || count != 1 {
			return fmt.Errorf("resume application thread: suspension count %d: %v", count, err)
		}
		return nil
	}
	return fmt.Errorf("application primary thread was not found: %w", err)
}
