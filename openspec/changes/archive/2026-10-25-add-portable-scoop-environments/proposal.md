# Proposal

## Why

The previous portable Scoop wrappers change shared process environment variables, can fall back to system Scoop, and repair only selected drive-letter references. Portscoop needs a maintainable Windows CLI that can locate and operate several independent portable Scoop environments while ordinary Scoop continues to work.

## What Changes

- Introduce a Go CLI using Cobra, exposed as `portscoop` and `psp`, with the same executable supporting central discovery and direct portable operation.
- Make each portable environment self-contained, with a durable identity, its own Scoop engine, configuration, apps, buckets, downloads, persisted package data, and operation state.
- Add initialization, host registration, listing, default selection, and unregistration for multiple environments. Registration records locations and aliases rather than owning package state.
- Resolve a registered identity from verified location hints when drive letters change; fail clearly for missing or ambiguous targets. Direct invocation resolves the environment adjacent to the executable and needs no central installation.
- Delegate central commands to the verified portable executable through a versioned invocation contract, preserving arguments, streams, cancellation, and exit status.
- Execute a pinned Scoop backend in a separate PowerShell process with explicit environment paths. Adapt Scoop's host integration so supported portable operations do not change persistent user/machine environment variables, Start Menu entries, or ordinary Scoop state.
- Serialize mutations within an environment and perform bounded relocation repair before using paths affected by a move. Different environments remain independently usable.
- Provide release archives and a lightweight setup helper for central or portable deployment; adding the central launcher to user PATH is an explicit setup option.

## Capabilities

### New Capabilities

- `environment-management`: Portable initialization and identity, host aliases and defaults, registry lifecycle, and deployment through both CLI names.
- `environment-resolution`: Deterministic environment selection, relocation-aware discovery, direct local resolution, and central-to-portable delegation.
- `scoop-execution`: Encapsulated package state, child-process invocation, portable host-integration policy, and coexistence with ordinary Scoop.
- `environment-relocation`: Per-environment mutation coordination and repair of managed path artifacts when a portable root moves.

### Modified Capabilities

None. The repository has no existing capability specs or implementation.

## Impact

- New Go module, Cobra dependency, Windows process/volume/locking integration where necessary, a small PowerShell backend adapter, release packaging, documentation, and integration tests.
- Reuse Scoop's manifest ecosystem and pinned engine source. A narrow maintained patch set may be required for persistent environment and shortcut behavior; compatibility must be verified before unrestricted package support is claimed.
- Host state is limited to explicit central deployment/PATH setup and the per-user environment registry. Portable package operations must not borrow system Scoop as a fallback.
- Initial target is Windows 10/11 x64 with Windows PowerShell 5.1; other architectures and PowerShell 7 require explicit validation before being advertised.
- This change does not select between central and portable applications, merge application shims into host PATH, implement a new package engine, automatically migrate the old project, or sandbox arbitrary package scripts and application data.
