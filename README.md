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

## Install

In 64-bit Windows PowerShell 5.1, run:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/get.ps1 | Invoke-Expression
```

Create and enter an empty folder before running the command. Choose **Host** for
a central launcher with optional user PATH integration, or **Store** for an
independent portable environment. Press Enter at the destination prompt to
install directly in your current folder. Store initialization requires local
NTFS. No Go compiler or existing
Scoop installation is required. Repeat Store setup for additional environments.
Host setup offers to create and select a first default Store in its `store`
subfolder. Accept Yes to start using that Store through the Host immediately.
See [internet installation](docs/internet-install.md) for scripted destinations,
version selection, and registration with a Host.

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

Current behavior contracts live in [the main specifications](openspec/specs/).
The completed proposal, design, delta specifications, and implementation tasks
are preserved in [the archived change](openspec/changes/archive/2026-10-25-add-portable-scoop-environments/).
Tests use temporary roots and explicit injected dependencies. They must never
discover or modify a developer's real Scoop installation.
# Launch applications

Use `pspe editor` for detached launch, `pspe rg -n hello .` for terminal execution,
or explicit `--wait`/`--detach` before the shim name. Select a registered Store
with `pspe --env work tool --help`. See [execution usage and handler scope](docs/application-execution.md)
and [compatible Store updates](docs/setup.md).

