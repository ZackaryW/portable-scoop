# Design

## Context

See `proposal.md` for motivation and `specs/shim-execution/spec.md` for the invocation contract.

The current repository provides a Go/Cobra management frontend in `internal/cli`. `scripts/build.ps1` builds `cmd/portscoop` and copies its executable as `psp.exe`. Release verification requires those two launchers, and Store initialization copies only those two executable names.

`resolver.Select` already distinguishes adjacent Store invocation from Host selection through an explicit alias or configured default. `delegation.Client` uses a strict package handshake and forwards only classified package operations. Its metadata decoder rejects unknown JSON fields, so adding execution capability fields to the existing handshake would break older consumers.

`runner.Native` starts suspended processes, assigns a kill-on-close Windows Job Object, resumes them, and closes the job when the root exits. This is intentional for package operations. It would terminate an application created by a launcher that has just completed. Windows job inheritance also means that changing only the innermost app runner cannot detach an app from a package-contained delegation ancestor. The [Windows job documentation](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects) describes these lifetime and breakaway constraints.

`relocation.Manager` owns and validates the generated shim inventory, including native shim executables and their metadata, PowerShell and CMD wrappers, links, and portable environment state. `relocation.Coordinator.Do` currently holds the mutation lock through the entire backend call. The backend environment builder deliberately removes inherited Git settings and supplies private Git configuration; that behavior belongs to package operations, not arbitrary application CLIs.

## Goals / Non-Goals

**Goals:**
- Separate application execution from management while sharing Store selection and preparation primitives.
- Preserve the received argument vector and foreground terminal contract across direct and delegated execution.
- Give detached applications an explicit startup handoff and an independent lifetime.
- Keep launch validation and process creation coordinated without locking a Store for the duration of an app session.
- Keep package protocol compatibility, containment, and manifest policy unchanged.

**Non-Goals:**
- Exporting every application name to host PATH, activating a persistent shell, or adding `psp <app>` fallback.
- Arbitrary paths, shell command strings, file associations, elevation, a launch daemon, or a process-manager service.
- Broadening supported package manifests or treating application runtime effects as sandboxed.
- Recovering arguments that CMD or PowerShell transformed before `pspe` received them.

## Decisions

### 1. Build a dedicated frontend and executable

Add `cmd/pspe` and a separate execution frontend, preferably `internal/appcli`, with a reusable execution service in `internal/application`. Share identity, registry, selection, discovery, and launcher-version plumbing rather than importing package dispatch into the new interface. Use injectable launch/process contracts so tests need only temporary fixtures.

Keep `portscoop.exe` and `psp.exe` identical management binaries. Build `pspe.exe` separately and include its own digest in the release inventory. Update required artifacts and Store provisioning to copy and verify the third executable before publishing readiness. The new executable need not share the management binaries' digest.

Alternative considered: dispatch by executable basename inside the management binary. A separate entry point makes the public command boundary and tests explicit and avoids exposing package handlers through `pspe` by accident.

### 2. Parse the prefix once and preserve the application tail

The frontend consumes only prefix launcher options, then produces a typed invocation containing selector, shim name, application argument slice, requested mode, and caller directory. Stop parsing at the first positional shim name. Prefix `--` ends launcher options; a `--` after the shim remains an application argument. Launcher help/version are recognized only before a shim.

Compute automatic mode from the application argument slice: zero arguments means detached, otherwise wait. `--wait` and `--detach` override it and are mutually exclusive. Reject unknown prefix options, missing option values, missing shim names, and NUL-bearing values before launch. Names such as `install`, `help`, and `launch` remain usable shim names rather than subcommands.

Alternative considered: `pspe run` and `pspe launch` subcommands. Prefix mode flags avoid reserving more application names and preserve the agreed `pspe <shim>` form.

### 3. Reuse Store resolution and validate a launch descriptor

Use the existing resolver precedence, bounded discovery, ambiguity checks, and location refresh. Direct Store invocation remains independent of Host metadata. An explicit selector resolving to another identity remains invalid for a Store-local executable.

Under launch preparation coordination, reload the expected identity, validate the layout and compatible managed backend, run relocation preparation, and resolve only the selected Store's owned shim inventory. Decode known shim forms into a typed descriptor containing the original target and a qualified handler; do not infer a command from arbitrary wrapper text. Resolve target links and validate the owning package boundary.

Group generated representations of one alias as one shim family. Equivalent wrapper representations can select a documented handler; inconsistent target definitions fail as ambiguous. Reject path-like names, device names, unrecorded or changed shim artifacts, unsupported fixed shim argument strings, and escaped targets. A target absent from the selected Store never triggers PATH lookup or another Store search.

Native EXE/COM targets use their validated target path directly, avoiding intermediate shim process lifetime assumptions. PowerShell targets use an explicit profile-free Windows PowerShell handler and typed argument transport. CMD/BAT targets require an explicit Windows batch handler and a dedicated quoting/transport codec; they must not be treated as normal native executables. The [Go execution contract](https://pkg.go.dev/os/exec#Command) specifically distinguishes CMD/batch parsing from ordinary Windows argument parsing.

Qualification is per handler. Native executable launch is required for initial readiness. Windows PowerShell and batch handlers must pass their argument and lifecycle fixtures before being advertised; unsupported formats or argument forms fail explicitly rather than evaluating a reconstructed command string. Python, Java, and shell wrappers require an explicit Store-bound runtime handler and otherwise report the missing support. Existing ownership recognition alone is not permission to execute every wrapper format.

### 4. Build an application environment without backend side effects

Extract reusable filtering/normalization primitives from `runner.BuildEnvironment` while retaining all existing backend tests and behavior. The application builder filters foreign Scoop/Store roots and stale Portscoop transport variables, prepends the selected Store's shims to the child PATH, and applies the validated `.portscoop/state/environment.json` values after relocation.

Preserve the existing supported portable variable semantics and reject reserved-root or malformed state rather than allowing state files to redirect the selected Store. Keep the caller's directory for application-relative arguments. Do not introduce the package runner's `GIT_CONFIG_*`, disabled prompts, hooks, templates, or credential-helper restrictions into an application's environment; legitimate caller/application Git configuration remains available. Do not mutate process-global or persistent host environment values.

Alternative considered: launch applications through the Scoop adapter. That would retain package policy dispatch and backend-only runtime restrictions in the application path.

### 5. Add an execution-specific delegation protocol

Keep the existing package handshake and its protocol unchanged. Host `pspe` checks the selected Store's `pspe.exe` through a distinct hidden execution handshake. Its versioned response identifies the expected Store and supported execution protocol/modes. A missing executable or unsupported execution protocol produces an upgrade diagnostic; it does not disable otherwise compatible management operations.

Forward a bounded, versioned invocation with expected identity, shim name, mode, received arguments, and caller directory. The Store rechecks identity and resolves the shim locally. Use an invocation-private control channel for handshake and launch acknowledgement, separate from foreground application streams. New internal endpoints accept only a valid execution envelope and do not provide raw command forwarding.

Use owner-restricted local IPC or equivalent owned transport, not a network listener. Bound negotiation and startup waits; reject malformed, mismatched, oversized, or disconnected requests. Do not automatically retry a possibly started application after an acknowledgement failure. Keep argument data out of diagnostics unless the user explicitly requests diagnostic detail.

Alternative considered: extend the package handshake with optional fields. Strict decoding in existing consumers makes that incompatible, and the package runner's lifetime policy would still be wrong for application delegation.

### 6. Separate application lifetime from package containment

Introduce explicit application start/wait/handoff contracts instead of weakening `runner.Native`. Both Host execution delegation and the Store's application runner use the new policy so no package-owned kill-on-close ancestor silently retains the app.

For foreground execution, connect caller streams and console input, wait for the application root, preserve its exit status, and terminate the owned foreground invocation on cancellation. Normal completion of an application launcher must not trigger package teardown of its application child. Stream ownership and draining must be bounded so a surviving descendant cannot keep the Portscoop helper waiting indefinitely. Keep package cancellation and abrupt-owner cleanup tests unchanged.

For detached execution, use a pending startup state with owned cleanup until the application has been created successfully and the bounded acknowledgement handoff is committed. Only then release launch ownership. Detach terminal handles, leave normal GUI windows visible, and avoid extra helper consoles. Return 0 for the committed launch; do not claim later application status. If startup is cancelled or fails before commit, stop uncommitted children and remove their transport artifacts.

Define and test the acknowledgement boundary using real parent/helper/application fixtures. Temporary job and process handles must not accidentally transfer kill-on-close ownership to the post-handoff app. If an inherited external job prevents the requested lifetime, return a launch diagnostic rather than quietly changing global containment policy. Once a detached handoff is committed, normal Host/Store helper exit and subsequent Host cancellation must not terminate that application.

### 7. Limit coordination to preparation and creation

Factor a launch-preparation path from `relocation.Coordinator` rather than calling its existing `Do` around a long-running app. Reuse the same Store lock and busy timeout for identity checks, relocation, inventory validation, environment snapshot, and process creation. Complete handler startup sufficiently to establish its validated target before releasing that lock, then wait or finish detached handoff outside the mutation critical section.

Do not mark an application session as an ongoing package mutation or capture arbitrary application changes as newly owned shim artifacts. Running apps remain subject to ordinary Windows file-use restrictions; this does not promise that an app can update its own loaded executable. Another Store retains its independent lock.

Alternative considered: retain the Store lock throughout execution. Interactive shells and GUI apps can live for hours and would block unrelated package operations in the same Store.

## Risks / Trade-offs

- Argument-count defaults can detach an interactive CLI or wait for a GUI launcher -> document `--wait` and `--detach` prominently and test both overrides.
- CMD/BAT and PowerShell wrappers have different parsing and runtime rules -> require explicit handlers and round-trip fixtures covering quotes, empty values, percent/exclamation expansion, Unicode, redirection characters, and named options. Do not advertise an unqualified handler.
- Nested Windows jobs can defeat detachment despite correct local process creation -> test direct and Host-delegated startup, parent exit, acknowledgement failure, cancellation, and restrictive external jobs before declaring detached support ready.
- An acknowledgement can be lost after a process may have started -> define the commit boundary, give an explicit diagnostic, and never duplicate the application through automatic retry.
- Application processes can retain inherited pipes or request files -> use separate control IPC, deliberately assign stream handles, and transfer temporary-payload cleanup to the consuming handler until it acknowledges consumption.
- Sharing backend environment helpers can impose hidden Git behavior on applications -> retain separate builders with focused regression coverage for normal application Git configuration.
- A running app can hold package files open -> release the preparation lock, preserve operating-system errors, and avoid promising runtime file isolation or application sandboxing.
- Older Stores have no execution executable -> keep management protocol 1 intact and fail only the new execution request with a compatible-upgrade message.

## Migration Plan

1. Build and verify a release containing all three executables; update fresh Store initialization, the Host-created default Store, and setup/package inventory checks.
2. Qualify a fresh Host and two independent Stores in the disposable Windows VM without Go installed. Use compiled native/GUI and script fixtures, private Host metadata, and before/after Scoop and persistent-environment sentinels.
3. Document compatible launcher replacement for existing Stores: preserve UUIDs, package data, persistence, and registry entries, and add `pspe.exe` from the verified release. Never copy an unrelated Store identity or silently modify an older root during launch.
4. If rolling back, restore the previous compatible management launchers and payload while preserving portable data; remove only the newly deployed execution entry point when appropriate. Missing execution support remains an explicit error for newer Hosts.
