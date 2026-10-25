package scoop

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"portscoop/internal/environment"
	"portscoop/internal/filelock"
	"portscoop/internal/runner"
)

const runtimeProbe = `$ErrorActionPreference='Stop'; $directory=$env:PORTSCOOP_RUNTIME_PROBE; $target=Join-Path $directory 'target'; $link=Join-Path $directory 'link'; New-Item -ItemType Directory -Path $target | Out-Null; New-Item -ItemType Junction -Path $link -Target $target | Out-Null; @{version=$PSVersionTable.PSVersion.ToString();language=$ExecutionContext.SessionState.LanguageMode.ToString();x64=[Environment]::Is64BitProcess;junction=(Get-Item -LiteralPath $link).LinkType} | ConvertTo-Json -Compress`

func CheckRuntime(ctx context.Context, paths environment.Paths, process runner.ProcessRunner) error {
	volume := make([]uint16, windows.MAX_PATH+1)
	path, err := windows.UTF16PtrFromString(paths.Root)
	if err != nil {
		return err
	}
	if err = windows.GetVolumePathName(path, &volume[0], uint32(len(volume))); err != nil {
		return fmt.Errorf("portable volume: %w", err)
	}
	if kind := windows.GetDriveType(&volume[0]); kind != windows.DRIVE_FIXED && kind != windows.DRIVE_REMOVABLE {
		return fmt.Errorf("portable storage must be a local NTFS volume")
	}
	fileSystem := make([]uint16, 32)
	if err = windows.GetVolumeInformation(&volume[0], nil, 0, nil, nil, nil, &fileSystem[0], uint32(len(fileSystem))); err != nil {
		return err
	}
	if windows.UTF16ToString(fileSystem) != "NTFS" {
		return fmt.Errorf("portable storage requires NTFS")
	}
	powerShell, err := runner.PowerShell(os.Environ(), nil)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(paths.Temp, 0700); err != nil {
		return err
	}
	directory, err := os.MkdirTemp(paths.Temp, "portscoop-runtime-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(directory)
	lock, err := filelock.Acquire(ctx, filepath.Join(directory, "probe.lock"))
	if err != nil {
		return fmt.Errorf("portable storage locking: %w", err)
	}
	defer lock.Close()
	if process == nil {
		process = runner.Native{}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var out, diagnostic bytes.Buffer
	code, err := process.Run(ctx, runner.Command{Executable: powerShell, Arguments: []string{"-NoProfile", "-NonInteractive", "-Command", runtimeProbe}, Directory: paths.Root, Environment: append(os.Environ(), "PORTSCOOP_RUNTIME_PROBE="+directory), Streams: runner.Streams{Out: &out, Err: &diagnostic}})
	if err != nil {
		return fmt.Errorf("runtime prerequisite probe: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("Windows PowerShell prerequisite/link check failed (%d): %s", code, diagnostic.String())
	}
	var state struct {
		Version, Language, Junction string
		X64                         bool
	}
	if err = json.Unmarshal(out.Bytes(), &state); err != nil {
		return fmt.Errorf("invalid runtime prerequisite report: %w", err)
	}
	if state.Version[:min(4, len(state.Version))] != "5.1." || state.Language != "FullLanguage" || !state.X64 || state.Junction != "Junction" {
		return fmt.Errorf("requires Windows x64, Windows PowerShell 5.1 FullLanguage, and NTFS junction support: %+v", state)
	}
	return nil
}
