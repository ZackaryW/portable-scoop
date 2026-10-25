// This executable is a disposable Windows qualification fixture, not a shipped
// Portscoop command. It writes only requested fixture paths and its own children.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

type observation struct {
	Args        []string `json:"args"`
	Cwd, Input  string
	PID         int
	Environment map[string]string
	HandleTypes map[string]uint32
}

func report(value any) {
	var output io.Writer = os.Stdout
	if path := os.Getenv("PSPE_FIXTURE_OUTPUT"); path != "" {
		file, err := os.Create(path)
		if err != nil {
			panic(err)
		}
		defer file.Close()
		output = file
	}
	if err := json.NewEncoder(output).Encode(value); err != nil {
		panic(err)
	}
}
func baseObservation() observation {
	cwd, _ := os.Getwd()
	o := observation{Args: os.Args[1:], Cwd: cwd, PID: os.Getpid(), Environment: map[string]string{}, HandleTypes: map[string]uint32{}}
	for _, key := range []string{"SCOOP", "SCOOP_GLOBAL", "PORTSCOOP_ROOT", "PSPE_STORE_HOME", "GIT_CONFIG_GLOBAL", "GIT_SSH_COMMAND"} {
		o.Environment[key] = os.Getenv(key)
	}
	for key, file := range map[string]*os.File{"stdin": os.Stdin, "stdout": os.Stdout, "stderr": os.Stderr} {
		kind, _ := windows.GetFileType(windows.Handle(file.Fd()))
		o.HandleTypes[key] = kind
	}
	return o
}
func main() {
	if len(os.Args) > 2 && os.Args[1] == "--fixture-job-probe" {
		cmd := exec.Command(os.Args[2], "--portscoop-probe-job")
		cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_BREAKAWAY_FROM_JOB, HideWindow: true}
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		_, _ = os.Stdout.Write(out)
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "--fixture-pty-child" {
		os.Exit(runPTYChild(os.Args[2]))
	}
	if len(os.Args) > 2 && os.Args[1] == "--fixture-job" {
		if err := runJob(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "--fixture-pty" {
		if err := runPTY(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(2)
		}
		return
	}
	if len(os.Args) > 2 && os.Args[1] == "--fixture-windows" {
		pid, _ := strconv.Atoi(os.Args[2])
		_ = json.NewEncoder(os.Stdout).Encode(inspectWindows(uint32(pid)))
		return
	}
	mode := os.Getenv("PSPE_FIXTURE_MODE")
	if mode == "interactive" {
		var cm uint32
		inputConsole := windows.GetConsoleMode(windows.Handle(os.Stdin.Fd()), &cm) == nil
		outputConsole := windows.GetConsoleMode(windows.Handle(os.Stdout.Fd()), &cm) == nil
		fmt.Printf("PSPE_READY console=%t input_console=%t output_console=%t pid=%d\n", inputConsole && outputConsole, inputConsole, outputConsole, os.Getpid())
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := scanner.Text()
			fmt.Println("PSPE_INPUT=" + line)
			if line == "exit" {
				return
			}
		}
		return
	}
	if mode == "launcher" {
		exe, _ := os.Executable()
		child := exec.Command(exe, "--fixture-child")
		for _, item := range os.Environ() {
			if !strings.HasPrefix(item, "PSPE_FIXTURE_MODE=") {
				child.Env = append(child.Env, item)
			}
		}
		child.Env = append(child.Env, "PSPE_FIXTURE_MODE=linger")
		child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := child.Start(); err != nil {
			panic(err)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(os.Getenv("PSPE_FIXTURE_OUTPUT")); err == nil {
				os.Exit(23)
			}
			time.Sleep(10 * time.Millisecond)
		}
		panic("launcher child did not record readiness")
	}
	o := baseObservation()
	if mode != "linger" && mode != "gui" {
		input, _ := io.ReadAll(os.Stdin)
		o.Input = string(input)
	}
	report(o)
	if mode == "gui" {
		title, _ := windows.UTF16PtrFromString("PSPE GUI fixture " + strconv.Itoa(os.Getpid()))
		text, _ := windows.UTF16PtrFromString("Disposable Portscoop GUI qualification")
		_, _, _ = user32.NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(text)), uintptr(unsafe.Pointer(title)), 0)
		return
	}
	if mode == "linger" {
		time.Sleep(45 * time.Second)
		return
	}
	fmt.Fprintln(os.Stderr, "fixture stderr")
	code := 37
	if s := os.Getenv("PSPE_FIXTURE_EXIT"); s != "" {
		code, _ = strconv.Atoi(s)
	}
	os.Exit(code)
}

// A native parent with redirected stdio can pass those handles even when its
// child is associated with ConPTY. Open the child's actual console devices and
// pass those handles to pspe, as an interactive console shell would.
func runPTYChild(path string) int {
	c, err := readConfig(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	input, err := os.OpenFile("CONIN$", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer input.Close()
	output, err := os.OpenFile("CONOUT$", os.O_RDWR, 0)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	defer output.Close()
	interrupts := make(chan os.Signal, 4)
	signal.Notify(interrupts, os.Interrupt)
	defer signal.Stop(interrupts)
	cmd := exec.Command(c.Executable, c.Args...)
	cmd.Dir = c.Directory
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, output, output
	err = cmd.Run()
	if status, ok := err.(*exec.ExitError); ok {
		return status.ExitCode()
	}
	if err != nil {
		fmt.Fprintln(output, err)
		return 2
	}
	return 0
}

type window struct {
	Handle       uintptr
	PID          uint32
	Visible      bool
	Title, Class string
}

func inspectWindows(pid uint32) []window {
	var result []window
	callback := windows.NewCallback(func(hwnd, unused uintptr) uintptr {
		var owner uint32
		_, _, _ = user32.NewProc("GetWindowThreadProcessId").Call(hwnd, uintptr(unsafe.Pointer(&owner)))
		if pid != 0 && owner != pid {
			return 1
		}
		title := make([]uint16, 512)
		class := make([]uint16, 128)
		_, _, _ = user32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&title[0])), uintptr(len(title)))
		_, _, _ = user32.NewProc("GetClassNameW").Call(hwnd, uintptr(unsafe.Pointer(&class[0])), uintptr(len(class)))
		visible, _, _ := user32.NewProc("IsWindowVisible").Call(hwnd)
		result = append(result, window{hwnd, owner, visible != 0, windows.UTF16ToString(title), windows.UTF16ToString(class)})
		return 1
	})
	_, _, _ = user32.NewProc("EnumWindows").Call(callback, 0)
	return result
}

type launchConfig struct {
	Executable             string
	Args                   []string
	Directory              string
	AllowBreakaway, Cancel bool
}

func readConfig(path string) (launchConfig, error) {
	var c launchConfig
	data, err := os.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(data, &c)
	}
	return c, err
}
func jobLimits(job windows.Handle, flags uint32) error {
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = flags
	_, err := windows.SetInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)))
	return err
}

func runJob(path string) error {
	c, err := readConfig(path)
	if err != nil {
		return err
	}
	job, err := windows.CreateJobObject(nil, nil)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(job)
	flags := uint32(windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE)
	if c.AllowBreakaway {
		flags |= windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK
	}
	if err := jobLimits(job, flags); err != nil {
		return err
	}
	var output strings.Builder
	// Run the selected pspe process suspended before assigning the external job.
	app, err := windows.UTF16PtrFromString(c.Executable)
	if err != nil {
		return err
	}
	line, err := windows.UTF16PtrFromString(windows.ComposeCommandLine(append([]string{c.Executable}, c.Args...)))
	if err != nil {
		return err
	}
	dir, err := windows.UTF16PtrFromString(c.Directory)
	if err != nil {
		return err
	}
	log, err := os.Create(filepath.Join(c.Directory, "job-output.txt"))
	if err != nil {
		return err
	}
	defer log.Close()
	nul, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer nul.Close()
	if err := windows.SetHandleInformation(windows.Handle(nul.Fd()), windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return err
	}
	if err := windows.SetHandleInformation(windows.Handle(log.Fd()), windows.HANDLE_FLAG_INHERIT, windows.HANDLE_FLAG_INHERIT); err != nil {
		return err
	}
	si := windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfo{})), Flags: windows.STARTF_USESTDHANDLES | windows.STARTF_USESHOWWINDOW, StdInput: windows.Handle(nul.Fd()), StdOutput: windows.Handle(log.Fd()), StdErr: windows.Handle(log.Fd()), ShowWindow: windows.SW_HIDE}
	var pi windows.ProcessInformation
	if err := windows.CreateProcess(app, line, nil, nil, true, windows.CREATE_SUSPENDED, nil, dir, &si, &pi); err != nil {
		return err
	}
	defer windows.CloseHandle(pi.Thread)
	defer windows.CloseHandle(pi.Process)
	if err := windows.AssignProcessToJobObject(job, pi.Process); err != nil {
		_ = windows.TerminateProcess(pi.Process, 2)
		return err
	}
	if _, err := windows.ResumeThread(pi.Thread); err != nil {
		return err
	}
	if status, err := windows.WaitForSingleObject(pi.Process, 20000); err != nil || status == uint32(windows.WAIT_TIMEOUT) {
		_ = windows.TerminateJobObject(job, 2)
		return fmt.Errorf("external job fixture timed out: %v", err)
	}
	var code uint32
	if err := windows.GetExitCodeProcess(pi.Process, &code); err != nil {
		return err
	}
	data, _ := os.ReadFile(log.Name())
	output.Write(data)
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"code": code, "output": output.String(), "allow_breakaway": c.AllowBreakaway})
}
