# Tasks

## 1. Dedicated frontend and invocation contract

- [x] 1.1 Add a typed application invocation and injectable execution interfaces under the new execution boundary; verify compile-time/interface tests keep package dispatch out of this API.
- [x] 1.2 Implement prefix-only parsing for selector, modes, help/version, and `--`; verify automatic mode, explicit overrides, argument-tail preservation, missing values, and conflicting options with table-driven tests.
- [x] 1.3 Add `cmd/pspe` and its frontend; verify shims named `install`, `help`, and `launch` are not reserved, and existing management commands still reject application-name fallback.
- [x] 1.4 Add command help and an execution usage document covering bare launch, argument-based waiting, overrides, Store selection, and caller-shell argument limitations; verify examples against parser tests.

## 2. Owned shim resolution and child environment

- [x] 2.1 Expose reusable owned-shim descriptors from the validated relocation inventory; verify native metadata, equivalent wrapper families, conflicting definitions, unsupported fixed arguments, and unrecorded edits with temporary-root fixtures.
- [x] 2.2 Resolve logical names only inside the selected Store and validate canonical package targets; verify path/device-name rejection, escaped links, missing targets, case handling, and no ordinary-Scoop or host-PATH fallback.
- [x] 2.3 Add a distinct application environment builder that applies supported portable state and filters foreign root/transport settings; verify caller environment preservation, relocated values, malformed/reserved state rejection, and unchanged backend environment tests.
- [x] 2.4 Preserve legitimate application Git configuration without introducing backend bucket restrictions; verify a controlled Git/config fixture observes caller settings while backend tests still observe private settings.
- [x] 2.5 Document qualified shim families, deterministic resolution, runtime prerequisites, and explicit unsupported-handler diagnostics; verify each documented category has a descriptor/error fixture.

## 3. Application lifecycle and format handlers

- [x] 3.1 Add Windows native application start/wait contracts separate from `runner.Native`; verify arguments, caller directory, stdout/stderr, numeric status, launch failures, and cancellation using compiled fixtures.
- [x] 3.2 Add foreground process ownership that cancels the managed invocation but does not apply package teardown on ordinary launcher completion; verify child cancellation, normal launcher-child survival, bounded stream draining, and all existing package lifecycle tests.
- [x] 3.3 Implement detached startup ownership and its acknowledgement commit boundary; verify success only after creation/handoff, pre-commit failure cleanup, no inherited terminal pipes, post-handoff helper exit survival, and no duplicate retry on control-channel failure.
- [ ] 3.4 Qualify native GUI and console target creation in direct and nested-job contexts; verify GUI windows are not hidden, helper consoles are suppressed, restrictive external jobs produce diagnostics, and acknowledged apps survive Host completion/cancellation.
- [x] 3.5 Add and qualify a profile-free Windows PowerShell handler with owned typed argument transport; verify Unicode, empty received values, quotes, named options, input/exit behavior, detached payload consumption, and invocation-file cleanup before advertising it.
- [x] 3.6 Implement and qualify the explicit CMD/BAT argument codec/handler rather than generic native quoting; verify percent/exclamation characters, quotes, spaces, empty received values, redirection/metacharacters, named options, and both modes before advertising support. Keep unqualified cases explicit failures.
- [x] 3.7 Document handler support and lifecycle semantics alongside these handlers; verify documentation distinguishes launch acknowledgement from application completion and does not claim application sandboxing.

## 4. Coordination and Host-to-Store execution

- [x] 4.1 Factor coordinated launch preparation from the existing per-Store coordinator; verify preparation and creation exclude concurrent mutations, busy waits remain bounded, and no application lifetime retains the mutation lock or adopts application edits into the shim inventory.
- [x] 4.2 Connect direct Store execution to existing identity/selection and relocation preparation; verify unrelated caller directories, moved roots, invalid adjacent metadata, explicit cross-identity selectors, and failed preparation using temporary roots.
- [x] 4.3 Add a separate versioned execution handshake and validated invocation envelope; verify expected identity, bounds, malformed/unsupported requests, missing `pspe.exe`, and unchanged strict package-handshake compatibility.
- [x] 4.4 Implement bounded owner-restricted local execution control/acknowledgement transport separate from application streams; verify disconnect and cancellation cleanup, private payload handling, and no network listener or raw-command bypass.
- [x] 4.5 Wire Host selection, discovery, hint refresh, execution dispatch, and mode-specific lifecycle through the new protocol; verify direct/central argument, stream, directory, exit, and detached-survival equivalence.
- [x] 4.6 Extend environment/process documentation for the execution protocol, lock duration, failure boundaries, and older-Store diagnostics; verify its examples against the new delegation and coordinator tests.

## 5. Release, setup, and existing Store compatibility

- [x] 5.1 Build and package `pspe.exe` beside the management launchers; verify management binaries remain identical, all three files have recorded release digests, and missing/altered execution artifacts fail verification.
- [x] 5.2 Copy and verify `pspe.exe` during Store initialization and Host default-Store bootstrap before readiness; verify standalone, Host-created, and subsequent Stores all receive the execution entry point without changing UUID/default-preservation behavior.
- [ ] 5.3 Extend installer/setup qualification for the third executable and opt-in Host PATH integration; verify Store shims are not added to host PATH and existing installer mode/first-Store choices remain intact.
- [x] 5.4 Document compatible execution-launcher deployment and rollback for existing Stores; verify temporary legacy/mixed-version fixtures preserve identities, registrations, packages, and otherwise compatible management operations.

## 6. End-to-end qualification

- [ ] 6.1 Run the relevant Go tests, vet, PowerShell bootstrap checks, and strict OpenSpec validation; record exact commands/results and confirm the package command allowlist and containment regressions still pass.
- [ ] 6.2 Qualify a fresh local release in the disposable Windows VM with no Go compiler, one Host, and two Stores; verify CMD and PowerShell launch/argument modes, Store selection, native and advertised script handlers, and directory/drive relocation using only owned fixtures.
- [ ] 6.3 Exercise interactive input in a real console/PTY fixture or observable guest console session; record evidence separately from redirected-input tests, including `--wait` with no app arguments and Ctrl+C behavior.
- [ ] 6.4 Exercise detached acknowledgement, helper termination, GUI visibility, restrictive jobs, foreground application-launcher children, and concurrent package operations with observable process fixtures; retain logs and clean up only test-owned processes.
- [ ] 6.5 Record artifact hashes, runtime/handler qualification scope, and before/after ordinary-Scoop and persistent-environment sentinels; verify the evidence corresponds to the tested local payload and no developer Scoop data changed.

## Workflow follow-up

Archive disposition (2026-10-25): The user reported manual verification and
explicitly approved archiving with the seven remaining tasks unchecked. The
checklist remains 24/31; archive status does not claim those qualification tasks
were completed or independently verified by the agent.

- Keep the active change artifacts out of implementation commits until the user authorizes their inclusion or archive.
- Review and archive only after implementation and qualification are complete.
- Publication is separate from local implementation and requires explicit approval for each individual push.
