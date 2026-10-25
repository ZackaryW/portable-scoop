# Development

Development requires Go 1.25 or newer and PowerShell. Users installing a binary distribution do not need Go. The distribution target is Windows x64 with Windows PowerShell 5.1 FullLanguage and local NTFS.

From the repository root:

```powershell
go mod download
go build ./...
.\scripts\build.ps1 -Version dev
.\bin\portscoop.exe --help
.\bin\psp.exe --version
.\bin\pspe.exe --help
go test ./...
go vet ./...
.\scripts\package.ps1 -Version 0.3.1-local1
```

Choose a new local version for each package output; packaging refuses to overwrite an existing artifact. The build helper sets `CGO_ENABLED=0`, targets Windows x64, writes all three launchers to `bin/`, and restores the caller's build variables and working directory. Build products are ignored by Git. These commands install nothing into ordinary Scoop or user PATH.

Packaging prepares the pinned backend, emits runtime/backend integrity metadata, verifies the complete distribution, and creates the archive and release assets under `dist/`. Backend patches and launchers must match. Copying new executables over an incompatible Store is insufficient; see [setup](setup.md).

Packaging validates the finished ZIP before creating release assets. To check an
archive separately without installing it:

```powershell
go run ./scripts/verify-release-archive -archive C:\releases\portscoop-windows-amd64.zip
```

## Verification

Tests use owned temporary roots and injected dependencies. They must never discover or modify a developer's ordinary Scoop installation. Use a disposable Windows VM for real application qualification. See [generic qualification](generic-qualification.md) for the scripts and [manifest coverage](manifest-coverage.md) for the separate static audit workflow.

When editing PowerShell fixture scripts, run their Go wrapper with `-count=1` so
Go's test cache cannot hide a script-only change. To include prepared-backend
HTTP/hash and artifact checks, set `PORTSCOOP_TEST_BACKEND_PAYLOAD` to the packaged
`backend` directory before running `go test -count=1 ./internal/backend/scoop`.

Run focused tests while editing the relevant component, then the regression suite and packaging checks. A preflight audit does not replace application qualification. Self-update and remaining `pspe` scenarios have separate qualification scopes; passing the existing suite does not imply those unimplemented/deferred tests were run.

## Change tracking

Synchronized contracts live in [openspec/specs](../openspec/specs/). Active proposals, delta specs, designs, and task lists live under `openspec/changes/`; completed changes are retained under `openspec/changes/archive/`. Keep implementation evidence and task status accurate, and distinguish local development builds from published versions.
