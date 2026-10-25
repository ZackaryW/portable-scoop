# Architecture

Portscoop separates an optional Host locator from independent portable Stores. The Host resolves a registered identity and delegates to that Store's launcher. Invoking a Store's launcher directly resolves its own adjacent identity, regardless of the caller's working directory.

```text
Host launcher --> verified identity/location --> Store launcher
                                                     |
Direct Store invocation -----------------------------+
                                                     |
                                                     v
                                           Managed Scoop backend
```

Each Store owns its engine, configuration, apps, buckets, cache, persisted package data, and operation state. Host registrations contain aliases and location hints rather than package data. Identity is independent of drive letter; invocation context is explicit rather than a process-wide selected-environment variable. Ambiguous copies and disconnected defaults fail explicitly. See [registrations](environments.md) and [relocation](relocation.md).

The coordinator uses Go and Cobra. The pinned Scoop engine runs through a PowerShell adapter. `psp` and `portscoop` expose management commands; `pspe` resolves the selected Store's owned shim inventory for application execution.

## Compatibility boundary

Before package mutation, the field compiler checks the selected architecture and full dependency closure. It classifies each field as use, rewrite, skip, or reject and freezes approved declarations and typed actions. The backend never evaluates original hook text. Complete reviewed recipes can retain required effects while explicitly omitting an optional host integration; unknown remainders fail preflight.

Child-process environment isolation protects the calling shell. The adapter also replaces Scoop's persistent host-environment and shortcut integration. These controls do not sandbox application runtime writes. See [field compatibility](compatibility.md), [backend adaptation](backend-compatibility.md), and [process execution](process-execution.md).

## Repository map

| Location | Responsibility |
| --- | --- |
| `cmd/portscoop`, `cmd/pspe` | Management and application entry points |
| `internal/cli`, `internal/appcli` | Command parsing and help |
| `internal/environment` | Store identity and controlled paths |
| `internal/registry`, `internal/resolver` | Host aliases, defaults, location discovery |
| `internal/delegation` | Host-to-Store protocol |
| `internal/application` | Owned shim resolution and app launch |
| `internal/runner` | Child environment, streams, process lifetime |
| `internal/compatibility` | Per-field manifest compiler and typed actions |
| `internal/backend/scoop`, `backend/scoop/adapter` | Verified engine integration and portable replacements |
| `internal/relocation` | Managed path preparation and recovery |
| `internal/runtimeinstall`, `internal/selfupdate` | Runtime ownership, staging, and update handoff |
| `scripts`, `tests` | Build, audit, release, and qualification tooling |
