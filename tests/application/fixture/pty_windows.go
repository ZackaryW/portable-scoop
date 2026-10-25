package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

type terminalOutput struct {
	mu   sync.Mutex
	text string
}

func (b *terminalOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.text) < 1024*1024 {
		b.text += string(p)
	}
	return len(p), nil
}
func (b *terminalOutput) String() string { b.mu.Lock(); defer b.mu.Unlock(); return b.text }

// runPTY creates a real Windows ConPTY. Pipes here carry terminal protocol, not
// redirected application stdin: the child receives console handles and Ctrl+C.
// API sequence follows Microsoft's Creating a Pseudoconsole Session example.
func runPTY(path string) error {
	c, err := readConfig(path)
	if err != nil {
		return err
	}
	inRead, inWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer inWrite.Close()
	defer inRead.Close()
	outRead, outWrite, err := os.Pipe()
	if err != nil {
		return err
	}
	defer outRead.Close()
	defer outWrite.Close()
	var console windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: 120, Y: 30}, windows.Handle(inRead.Fd()), windows.Handle(outWrite.Fd()), 0, &console); err != nil {
		return err
	}
	var output terminalOutput
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(&output, outRead); close(drained) }()
	defer func() { windows.ClosePseudoConsole(console); _ = outRead.Close(); <-drained }()
	attrs, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return err
	}
	defer attrs.Delete()
	// PSEUDOCONSOLE's lpValue is the opaque HPCON handle itself, not &HPCON.
	ok, _, callErr := kernel32.NewProc("UpdateProcThreadAttribute").Call(uintptr(unsafe.Pointer(attrs.List())), 0, windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, uintptr(console), unsafe.Sizeof(console), 0, 0)
	if ok == 0 {
		return fmt.Errorf("attach ConPTY attribute: %v", callErr)
	}
	si := windows.StartupInfoEx{ProcThreadAttributeList: attrs.List()}
	si.Cb = uint32(unsafe.Sizeof(si))
	self, err := os.Executable()
	if err != nil {
		return err
	}
	app, err := windows.UTF16PtrFromString(self)
	if err != nil {
		return err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine([]string{self, "--fixture-pty-child", path}))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(c.Directory)
	if err != nil {
		return err
	}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(app, line, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT, nil, dir, &si.StartupInfo, &pi); err != nil {
		return err
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	defer windows.TerminateProcess(pi.Process, 2)
	_ = inRead.Close()
	_ = outWrite.Close()
	waitFor := func(text string) error {
		deadline := time.Now().Add(12 * time.Second)
		for time.Now().Before(deadline) {
			if strings.Contains(output.String(), text) {
				return nil
			}
			time.Sleep(10 * time.Millisecond)
		}
		return fmt.Errorf("terminal did not show %q: %s", text, output.String())
	}
	if err := waitFor("PSPE_READY console=true"); err != nil {
		return err
	}
	if _, err := inWrite.WriteString("terminal-input\r"); err != nil {
		return err
	}
	if err := waitFor("PSPE_INPUT=terminal-input"); err != nil {
		return err
	}
	if c.Cancel {
		_, err = inWrite.Write([]byte{3})
	} else {
		_, err = inWrite.WriteString("exit\r")
	}
	if err != nil {
		return err
	}
	if status, err := windows.WaitForSingleObject(pi.Process, 12000); err != nil || status == uint32(windows.WAIT_TIMEOUT) {
		return fmt.Errorf("ConPTY child did not exit: %v: %s", err, output.String())
	}
	var code uint32
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
		return err
	}
	time.Sleep(50 * time.Millisecond)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"console": true, "ctrl_c": c.Cancel, "code": code, "output": output.String()})
}
