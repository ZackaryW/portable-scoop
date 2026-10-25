//go:build windows

// Package filelock provides OS-owned locks that disappear when a process exits.
package filelock

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sys/windows"
)

type Lock struct {
	handle windows.Handle
	once   sync.Once
	err    error
}

func Acquire(ctx context.Context, path string) (*Lock, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return nil, fmt.Errorf("open lock: %w", err)
	}
	for {
		err = windows.LockFileEx(handle, windows.LOCKFILE_EXCLUSIVE_LOCK|windows.LOCKFILE_FAIL_IMMEDIATELY, 0, 1, 0, &windows.Overlapped{})
		if err == nil {
			return &Lock{handle: handle}, nil
		}
		if !errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			windows.CloseHandle(handle)
			return nil, fmt.Errorf("acquire lock: %w", err)
		}
		timer := time.NewTimer(20 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			windows.CloseHandle(handle)
			return nil, fmt.Errorf("environment or registry busy: %w", ctx.Err())
		case <-timer.C:
		}
	}
}

func (l *Lock) Close() error {
	l.once.Do(func() { l.err = windows.CloseHandle(l.handle) })
	return l.err
}
