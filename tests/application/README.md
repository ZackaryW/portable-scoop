# Application execution qualification

Run repository checks on Windows with Go 1.25:

```powershell
go test ./...
go vet ./...
powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File tests/installer/bootstrap-tests.ps1
openspec validate add-pspe-shim-execution --strict
```

All Go fixtures use owned temporary roots and processes. They do not install
packages into ordinary Scoop. `TestForegroundChildCannotRetainCallersOutputPipe`
separately checks actual `*os.File` pipes; an `io.Writer` buffer alone does not
cover a real shell's inherited pipe handles.

Build the release and a qualification bundle on the development machine:

```powershell
./scripts/package.ps1 -Version 0.2.0
./tests/application/build-bundle.ps1 `
  -ReleaseArchive ./dist/portscoop-0.2.0-windows-amd64.zip `
  -Destination './.work/vmware-testing/execution-qualification'
```

Choose fresh output/version names when rerunning. The builder retains its source
artifacts, compiles console/GUI/ConPTY/job fixtures and standalone Go test suites,
and copies the exact local release with its checksum. No compiler is needed in
the guest. Transfer the resulting ZIP and `qualify-release.ps1` through the
restricted VMware bridge. Run the script only inside the selected disposable VM:

```powershell
powershell.exe -NoProfile -NonInteractive -WindowStyle Hidden `
  -ExecutionPolicy Bypass -File .\qualify-release.ps1 `
  -Bundle .\execution-qualification.zip -Results .\execution-results.json `
  -LegacyRelease 'C:\Users\Public\PortscoopLegacy\release'
```

The test harness probes its job context. Detached-success cases are skipped in a
development shell whose external job prohibits breakaway or remains after it; refusal is the
expected product behavior there. The guest success qualification requires no
remaining external job and runs the same cases without those skips. Residual-job
rejection is tested with an explicit restrictive nested hierarchy separately.

VMware guest execution must use an elevated PowerShell session on the logged-in
interactive desktop for native GUI observations and the DiskPart VHD checks. The
script checks elevation before creating fixtures, guards against running outside VMware, creates
a fresh `C:\Users\Public\PortscoopExecution-<UUID>` root, uses private Host metadata,
and checks ordinary-Scoop and persistent-environment sentinels before/after.
Stage an extracted compatible older release (for example, v0.1.0) in the guest
and pass its directory as `-LegacyRelease`. It must contain the original
`portscoop.exe` and `psp.exe`; only copies inside the new fixture Store are modified.

The suite installs owned ZIP fixtures through the real backend, exercises direct
and Host execution in CMD/PowerShell, script handlers, caller arguments/streams,
application Git settings, multiple Stores, directory relocation, and reassignment
of a newly created NTFS VHD. It detaches only its own VHD in cleanup. App cleanup
checks recorded PID/start time and image scope before stopping test-owned processes.

Interactive evidence comes from a **real Windows ConPTY**, independently of
redirected-input fixtures. The app verifies `GetConsoleMode` on both input and
output, receives a terminal line, and then exits normally or receives a terminal
Ctrl+C. Both direct and Host `--wait` cases use **no application arguments**.
An attached bootstrap opens `CONIN$`/`CONOUT$` explicitly before starting `pspe`,
so a redirected parent of the harness cannot substitute its own pipe handles.
The fixture follows Microsoft's [pseudoconsole session API sequence](https://learn.microsoft.com/en-us/windows/console/creating-a-pseudoconsole-session).

GUI evidence inspects visible top-level windows on the interactive desktop and
checks for extra visible helper consoles. Nested-job fixtures run native console
and GUI apps with permitted and prohibited breakaway. The compiled application
suite covers pre-commit disconnect, lost acknowledgement without retry, exact
received empty/quote/Unicode vectors, cancellation, and child survival. The
unchanged package runner suite checks its existing teardown policy separately.

Results, command logs, observed argument JSON, ConPTY output, windows, and process
observations remain in the guest root for review. No completed application release
qualification report is checked in yet: the earlier exploratory VM run stopped
at DiskPart because the session lacked elevation. Local repository checks passing
does not establish full VM qualification. Development archives
are local artifacts; publication and each Git push require separate explicit approval.
