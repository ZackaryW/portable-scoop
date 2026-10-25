# Portscoop documentation

## Use Portscoop

| Guide | What it covers |
| --- | --- |
| [Internet installation](internet-install.md) | PowerShell bootstrap, current-folder defaults, Host/Store selection, scripted setup |
| [Setup and multiple Stores](setup.md) | Prerequisites, first default Store, independent environments, matching distributions |
| [Environment registrations](environments.md) | Aliases, identity, selection, and location hints |
| [Application execution](application-execution.md) | `pspe`, argument forwarding, detached and terminal modes |
| [Self-update](self-update.md) | Installation-local runtime updates and compatibility limits |
| [Relocation](relocation.md) | Moving a Store, managed paths, and recovery |
| [Manifest coverage](manifest-coverage.md) | Measured preflight acceptance, pinned snapshots, accepted names |

## Understand the implementation

| Guide | What it covers |
| --- | --- |
| [Architecture](architecture.md) | Host delegation, Store ownership, repository components |
| [Manifest compatibility](compatibility.md) | Per-field use/rewrite/skip/reject decisions and reviewed hook operations |
| [Hook operations](hook-operations.md) | Generic grammar, selective import omission, local defaults and lifecycle paths |
| [Download plans](downloads.md) | Frozen filenames, queries, fragments, checksums and paired extraction |
| [Archive extraction](archive-extraction.md) | Managed readers, content checks, resource limits and dependency attribution |
| [Portable policy](portable-policy.md) | Supported commands, dependency preflight, isolation boundaries |
| [Backend compatibility](backend-compatibility.md) | Pinned Scoop adapter, patched behavior, verification |
| [Process execution](process-execution.md) | Streams, exit codes, cancellation, and process lifetime |
| [Generic qualification](generic-qualification.md) | Current VM application checks, build provenance and limits |
| [Historical Bitwarden prototype](bitwarden-qualification.md) | Reverted prototype evidence, retained for context |
| [Development](development.md) | Build, test, package, and change tracking |

The [main specifications](../openspec/specs/) describe synchronized behavior contracts. Active changes under `openspec/changes/` may describe newer behavior pending synchronization; release availability and test evidence are documented separately.
