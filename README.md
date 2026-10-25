# Portscoop

Portscoop (`psp`) manages independent portable Scoop environments on Windows.
An optional central launcher locates a registered environment and delegates to
its portable launcher. A portable launcher resolves its own root and works
without central registration. Regular Scoop remains a separate installation.

This experimental implementation supports portable initialization, explicit host
registrations, central delegation, and a constrained pinned Scoop backend. Release
qualification is tracked in OpenSpec; treat development archives accordingly.
See [setup and two environments](docs/setup.md),
[internet installation](docs/internet-install.md),
[environment registrations](docs/environments.md), and the
[supported package policy](docs/portable-policy.md).

## Build

Development requires Go 1.25 or newer and PowerShell to run the build helper.
Go is only a development dependency. The initial distribution target is Windows
x64; package-backend qualification will target Windows PowerShell 5.1 and local
NTFS storage before additional runtimes/filesystems are advertised.

From the repository root:

```powershell
go mod download
go build ./...
.\scripts\build.ps1 -Version dev
.\bin\portscoop.exe --help
.\bin\psp.exe --version
go test ./...
.\scripts\package.ps1 -Version 0.1.0-dev
```

The helper builds with `CGO_ENABLED=0` for Windows x64 and writes both executable
names to `bin/`. It restores the calling shell's build variables and working
directory. Build products are ignored by Git. It installs nothing into your
existing Scoop or user PATH.

## Architecture

```text
Central launcher --> verified identity/location --> Portable launcher
                                                       |
Direct portable invocation ----------------------------+
                                                       |
                                                       v
                                            Isolated Scoop backend
```

Each portable root owns its engine, configuration, apps, buckets, cache,
persisted package data, and operation state. Host registrations store aliases
and location hints rather than package data. Identity is independent of drive
letter, and invocation context is passed explicitly rather than kept in a
process-wide selected-environment variable.

| Location | Responsibility |
| --- | --- |
| `cmd/portscoop` | Executable entry point |
| `internal/cli` | Portscoop command parsing and help |
| `internal/environment` | Identity, controlled paths, invocation context |
| `internal/registry` | Host aliases, defaults, location metadata |
| `internal/resolver` | Selection and volume discovery |
| `internal/delegation` | Central-to-portable protocol |
| `internal/runner` | Child environment, streams, process lifetime |
| `internal/backend/scoop` | Pinned Scoop compatibility boundary |
| `internal/relocation` | Managed path preparation after a move |
| `internal/testutil` | Temporary environment/backend fixtures for tests |
| `scripts` | Build and release tooling |

The coordinator is Go with Cobra. Scoop remains behind a PowerShell adapter.
Child-process isolation protects the calling shell; backend adaptation is also
required to prevent Scoop's persistent host environment and shortcut writes.
Portable operation is not a sandbox for arbitrary package scripts or
application-runtime data writes.

See [process execution](docs/process-execution.md) for argument transport, exit
statuses, cancellation, and process-tree containment.

## Change tracking

The active plan is `openspec/changes/add-portable-scoop-environments/`, containing
the proposal, design, four capability specifications, and implementation tasks.
Tests use temporary roots and explicit injected dependencies. They must never
discover or modify a developer's real Scoop installation.
