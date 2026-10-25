# Design

## Context

See proposal.md for motivation and capability boundaries. This is a greenfield repository containing a license and OpenSpec setup. The neighboring `portable-scoop-old` repository is evidence, not an implementation dependency: its scripts mutate process-level `SCOOP` variables, call a shim containing an absolute engine path, and optionally fall back to system Scoop.

The development machine has a working Go toolchain and PowerShell; no .NET SDK was reported by `dotnet --list-sdks`. Upstream Scoop is PowerShell-based. Its installation flow also writes persistent environment variables and creates shortcuts. A separate process alone cannot encapsulate these host effects.

## Goals / Non-Goals

**Goals:**
- Make target selection explicit and stable across concurrent invocations, disconnected drives, and drive-letter changes.
- Separate identity, host discovery metadata, portable package state, and process execution into independently testable boundaries.
- Keep a portable environment operable without host registration or the central launcher.
- Preserve Scoop package compatibility through a versioned backend with a defined portable policy.

**Non-Goals:**
- A daemon, GUI, application launcher, automatic host/portable application selection, or ambient shell activation.
- A security sandbox for arbitrary PowerShell hooks or third-party applications, or a guarantee that every Scoop package stores all application data portably.
- Shared mutable buckets, package installations, persistence, or download caches between environments in this first version.
- Transparent compatibility with every upstream Scoop command or a general-purpose manifest reimplementation.

## Decisions

### 1. Use Go and Cobra for coordination; keep PowerShell behind the backend boundary

Go packages own environment metadata, registry persistence, discovery, delegation, locking, relocation orchestration, and process lifecycle. Cobra owns Portscoop command/help parsing; it must not reinterpret forwarded Scoop arguments. Configuration uses standard-library JSON with explicit versioned schemas, avoiding a second configuration framework.

Proposed boundaries are `cmd/portscoop`, `internal/environment`, `internal/registry`, `internal/resolver`, `internal/delegation`, `internal/runner`, `internal/relocation`, and `internal/backend/scoop`. A small PowerShell entry script adapts the pinned engine. Core functions take an explicit resolved environment/context rather than reading or changing a process-wide "current environment". Windows volume and process APIs sit behind small platform adapters.

PowerShell alone could meet the same isolation contract with separate processes and disciplined modules. Go is preferred for this growing coordinator: typed metadata, explicit dependencies, direct executable distribution, and testable concurrent operations. C# remains technically suitable but adds a missing SDK here. Reimplementing Scoop would expand compatibility work considerably.

### 2. Each portable root owns an independent installation

```text
<portable-root>/
  portscoop.exe
  psp.exe
  .portscoop/
    environment.json
    state/                  # locks, last repaired root, repair journal
    backend/                # PowerShell adapter and backend version metadata
  scoop/
    config.json
    apps/scoop/current/     # pinned engine; never found through host PATH
    apps/                   # other installed packages
    buckets/
    cache/
    persist/
    shims/
    global/                 # private path, not permission for global installs
    tmp/
```

Release packaging supplies both executable names from the same build. This avoids depending on a PowerShell alias or CMD argument reconstruction. The root is the executable's directory, regardless of which name was used. The layout and controlled engine entry path are fixed in schema v1; metadata cannot inject arbitrary execution paths.

`environment.json` contains a generated installation UUID, metadata schema version, display name, and delegation protocol major. Backend metadata records the pinned upstream revision, patch revision, and verified artifact digests. An absolute last-known root belongs in relocatable operation state, not identity. Unknown schemas fail without editing the marker.

Initialization accepts an empty destination or a directory containing only a verified Portscoop release payload, uses staging inside that destination, and publishes a ready identity only after engine/config/bootstrap checks succeed. It refuses to replace an existing environment or adopt an unrelated non-empty directory. Failure retains a diagnostic or cleans only staging created by that invocation; it does not recursively delete user-provided directories. Portable initialization does not register with the host automatically.

### 3. The host registry holds aliases and discovery hints

Use `%LOCALAPPDATA%\portscoop\registry.json`, with a schema version, entries mapping case-insensitive host aliases to installation IDs, last verified absolute paths, volume identity where available, drive-relative path hints, and an optional default installation ID. Portable state is never written into this registry.

Supported management commands are:

```text
psp env init <path> --name <display-name>
psp env register <path> --name <host-alias>
psp env list
psp env default <host-alias>
psp env unregister <host-alias>
psp --env <host-alias> install <package>
```

Registration verifies an existing ready marker. Re-registering the same ID/alias updates location hints; mapping an existing alias to a different ID fails until explicitly unregistered. Unregistering removes only host metadata and clears the default when its target is no longer registered. It never removes portable files. Registry updates are serialized with a host-local lock and use atomic replacement; corruption fails visibly without silently discarding registrations. Explicit registration can rebind an environment after its containing directory changes.

A release archive and PowerShell setup helper support central deployment to a user-chosen host directory or portable initialization at a chosen root. Only an explicit central setup option adds the central directory to user PATH. Package commands and portable setup do not modify host PATH. Undo instructions remove only the installed launcher files, matching PATH entry, and requested registry entries.

### 4. Resolve one identity per invocation, then delegate once

Selection order is: explicit `--env` alias; otherwise a valid marker adjacent to the executable; otherwise the explicitly configured host default. A portable executable with an explicit selector may target itself but rejects a different identity, keeping portable operation independent of the host registry. A central launcher requires a selector or configured default, even when only one environment is registered. It never chooses whichever drive happens to be available. The working directory does not implicitly select an environment.

For central resolution, evaluate registered paths and the recorded drive-relative paths on available local fixed/removable volumes, prioritizing matching volume identity. This is a bounded candidate search, not recursive scanning of every drive. Skip unavailable volumes with bounded diagnostics. Validate ready metadata, ID, and the portable executable at candidate roots, canonicalize roots to deduplicate equivalent paths, and reject multiple distinct roots advertising the requested identity. A cached path alone does not establish identity. Check all bounded candidates before choosing; do not return the first apparent match. UUIDs identify installations but do not authenticate executable provenance.

After a drive-letter-only change, update the registry only after a unique target is verified. A containing-directory move requires registration of the new path; arbitrary full-disk search is excluded. A copied root retains the same ID and is ambiguous when both copies are found. Supporting an independent clone command is deferred; diagnostics explain that an independently initialized environment needs its own identity.

The central executable queries a read-only internal handshake on the resolved portable executable. The response contains the identity and protocol major. It then forwards the original argument vector, with Portscoop selectors removed, through a versioned internal execution mode carrying the expected identity. The portable executable rechecks its adjacent marker and executes locally; the delegated mode cannot enter central discovery. An identity/protocol mismatch fails before Scoop starts. Use inherited standard streams and propagate the backend exit code; launcher validation errors have a documented separate code. Windows process-tree cancellation must reach the delegated launcher and backend, using a Job Object or an equivalent validated platform mechanism.

### 5. Multi-environment isolation includes persistent host effects

Create one resolved context per call containing identity, absolute root, package paths, backend pin, runtime, policy, and original working directory. Construct a child environment explicitly, replacing inherited `SCOOP`, `SCOOP_GLOBAL`, and `SCOOP_CACHE`, choosing the root-local config mechanism verified against the pin, setting per-root temporary paths, and removing inherited Scoop activation paths. Keep ordinary unrelated environment settings; this is coexistence, not virtualization. Resolve the PowerShell executable explicitly and launch without profiles. Preserve the caller's working directory for manifest arguments; use the resolved root for managed state.

Use a fixed backend script and argument transport verified with Windows PowerShell 5.1 for spaces, quotes, empty strings, Unicode, and shell metacharacters. Never interpolate user arguments into executable PowerShell source or a CMD command string. Go `os/exec` supports explicit child environment and streams, but crossing into PowerShell still needs a tested argument protocol.

Before integrating package operations, inspect and probe the selected upstream revision's config lookup, environment writers, shortcut/PSModule integration, dependency selection, shim creation, junction creation, and bootstrap behavior. The chosen strategy is a pinned backend plus the smallest versioned patch/adapter needed to route supported environment changes into portable metadata and child-process state, suppress host shortcuts/module registration, and keep dependency/package lookup inside the target root. Avoid transient monkey-patching of functions loaded into the caller's PowerShell session. If upstream has no stable extension points, keep a reviewed patch set with explicit upstream revision and regression fixtures. This bounded compatibility task determines patch sites, not whether isolation is required.

The initial supported surface is install, uninstall, app update, list, search, status, bucket list/add/remove, reset, cleanup, and configuration inspection. Document the mapping to upstream command spelling. Recognize commands before backend execution and reject unsupported entry points, `--global`/`-g`, root-changing config writes, and unrestricted engine self-update. Portscoop updates the pinned engine only through a compatible release. With Cobra, reserve `env` and help/version handling and otherwise preserve recognized backend arguments; do not add an unrestricted raw-command escape hatch that bypasses policy.

Supported packages must use vetted portable behavior. Manifests needing arbitrary install/uninstall hooks, external installers, host registration, or unsupported integration are rejected before their hooks/installers execute. Preflight covers the dependency closure and updates/uninstalls as well as direct installs. The first release can support a deliberately narrow package subset; broad support must be earned with fixtures. Application runtime writes outside the root remain outside the guarantee and are documented. Merely enabling Scoop's `use_isolated_path` is insufficient: upstream still manages persistent environment settings.

### 6. Coordinate mutations and repair only owned relocation artifacts

Take an OS-backed exclusive mutation lock anchored in the canonical environment root before backend writes, bootstrap changes, or relocation. Different roots have independent locks. Reads that require stable state either take a compatible lock or report the environment busy; upstream "read" commands that write caches are treated as mutations. The lock is released by process termination, and a leftover filename is not proof of a held lock. A failed/cancelled operation leaves an explicit status when inspection or repair is required.

On every backend invocation compare the current canonical root with the last successfully prepared root. The same lock covers preparation and command execution, avoiding a window between repair and use. A fresh environment records its root only when bootstrap preparation succeeds.

Own a catalog of generated shims, current-version junctions, persist links, and adapter-controlled config/environment paths supported by the pinned engine. Repair replaces verified references to the exact previous root prefix with the new root; it never substitutes every matching drive letter. Resolve and validate link destinations for owned artifacts, permit only expected targets inside the root, and refuse external links or unknown formats. Rebuild managed artifacts from installed manifests/state when that is safer than text replacement. Do not rewrite downloaded manifests, arbitrary app config, user files, or executables. Records of owned paths must not be able to escape the canonical root through `..` or reparse points.

Journal repair progress before changing artifacts. Publish the new last-prepared root only after all required changes succeed. On interruption, retain the journal and previous root; rerunning preparation completes/reconciles changes idempotently before any backend launch. A repair failure blocks package execution with actionable diagnostics. No broad rollback claim is made for arbitrary interrupted Scoop package operations.

### 7. Validate behavior across two roots and ordinary Scoop

Unit fixtures cover selection, drive candidates, corrupt metadata, duplicate IDs, protocol mismatch, argument vectors, and exact-root relocation. Windows integration fixtures initialize two distinct roots and an independent ordinary-Scoop fixture, use paths containing spaces and parentheses, move one root, and verify the other two are unaffected. Use isolated user/VM fixtures for engine integration so tests can observe persistent environment and shortcut effects without touching the developer's real Scoop installation. Fake backends cover failure, streams, cancellation, and lock contention before network-dependent engine tests.

Release qualification includes a clean Windows x64 machine without Go installed, Windows PowerShell 5.1, both CLI names from CMD and PowerShell, direct portable execution without registry state, concurrent different-root operations, disconnected media, and interrupted repair. A Go compiler is a development dependency only.

## Risks / Trade-offs

- [Upstream changes bypass portable policy] -> Pin and verify engine artifacts; maintain a minimal patch inventory and contract tests; block engine self-update.
- [Package scripts or applications write host state] -> Enforce a documented supported manifest subset before execution; explicitly exclude arbitrary scripts/application sandboxing from the portability claim.
- [Raw copies share identity] -> Reject discovered ambiguity; require independently initialized identities for distinct environments.
- [Moved app internals remain absolute] -> Repair only supported owned artifacts and reject/report packages whose relocation contract is not satisfied; avoid generic filesystem rewriting.
- [Windows argument/cancellation behavior differs from Unix] -> Qualify the Windows transport and process tree explicitly, including PowerShell 5.1 and both entry names.
- [Media/filesystem failure prevents atomic writes or supported links] -> Check capabilities at init, fail without publishing a ready marker, and retain repair journals; initially qualify local NTFS roots rather than claiming all filesystems/UNC support.
- [Pinned engines duplicate storage] -> Accept independent engines/cache/buckets for predictable behavior; consider content-addressed download sharing separately.

## Migration Plan

1. Build and test the coordinator and metadata/delegation contracts with fake backends.
2. Complete the pinned-engine compatibility probe and portable adaptation before enabling real package operations.
3. Qualify initialization, isolation, relocation, and multi-root concurrency against the capability scenarios.
4. Publish Windows x64 archives with both CLI names, backend metadata/digests, and central/portable setup instructions.
5. Initialize new environments and register them explicitly. Leave the old project and ordinary Scoop untouched; automatic import is deferred.

Launcher rollback restores the previous compatible release without changing identity. Engine rollback requires a compatible backend/state version; refuse unsupported state downgrades. Host uninstall removes only the launcher deployment, its explicitly installed PATH entry, and requested registry metadata. Portable package state is preserved unless the user separately requests its deletion.

## Evidence

- Local baseline: `portable-scoop-old/scoop.ps1`, `scoop-runner.ps1`, `install-scoop.ps1`; current repository has no application source.
- [Go process execution](https://pkg.go.dev/os/exec) and [Cobra](https://github.com/spf13/cobra).
- [Scoop core configuration/environment behavior](https://github.com/ScoopInstaller/Scoop/blob/master/lib/core.ps1) and [installation integration](https://github.com/ScoopInstaller/Scoop/blob/master/lib/install.ps1). These moving branches inform the investigation; implementation must record a concrete upstream pin.
