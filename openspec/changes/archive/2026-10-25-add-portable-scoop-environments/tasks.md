# Tasks

## 1. Coordinator foundation

- [x] 1.1 Create the Go module, Cobra entry point, internal package boundaries from design.md, and Windows x64 build command; verify `go build ./...` and help/version smoke checks succeed.
- [x] 1.2 Define versioned portable identity, registry, backend metadata, and resolved-context contracts with explicit paths and no global selected-environment variable; verify JSON fixtures reject unsupported versions and invalid path-bearing data.
- [x] 1.3 Add injectable filesystem/volume/backend boundaries and test fixture helpers for multiple roots; verify fixtures can model two independent identities and an ordinary-Scoop sentinel without touching the user's installed Scoop.
- [x] 1.4 Document build prerequisites, repository layout, and the central/portable execution split; verify the documented build produces a runnable CLI using the installed Go toolchain.

## 2. Pin and qualify the Scoop backend contract

- [x] 2.1 Select and record a concrete upstream Scoop revision and artifact digests; inspect config precedence, bootstrap, dependency lookup, environment writers, shortcuts/PSModule integration, shims, and junctions at that revision; verify a compatibility note names the exact integration/patch sites and records source references.
- [x] 2.2 Build isolated probe fixtures for root-local config, manifest environment settings, shortcut metadata, and supported plain-archive package operations; verify observed file and persistent-environment changes identify every host effect requiring adaptation.
- [x] 2.3 Define the initial supported command/manifest policy, dependency-closure preflight rules, and immutable engine update contract; verify policy fixtures distinguish supported packages from hooks/installers or flags that would bypass isolation.
- [x] 2.4 Record Windows PowerShell 5.1, Windows x64, and local NTFS prerequisites plus the supported package subset in backend documentation; verify each claimed feature is backed by a probe or explicitly awaits its implementation task.

## 3. Host registry and management commands

- [x] 3.1 Implement host registry storage with schema validation, a host-local update lock, and atomic replacement; verify concurrent writers preserve both updates and interrupted/corrupt writes never silently reset registrations.
- [x] 3.2 Implement `env register`, `env default`, and `env unregister` with case-insensitive aliases, same-identity hint refresh, alias-conflict rejection, and last-alias default cleanup; verify the environment-management lifecycle scenarios with fixture roots.
- [x] 3.3 Implement `env list` with identities, aliases, default status, and bounded availability diagnostics; verify disconnected storage stays registered and does not prevent listing available entries.
- [x] 3.4 Document registry location, explicit defaults, registration after directory moves, and data-preserving unregistration; verify CLI help/examples match the implemented commands.

## 4. Selection, drive discovery, and delegation

- [x] 4.1 Implement explicit alias, adjacent-root, and host-default selection with portable cross-identity rejection and no working-directory inference; verify selection tests cover missing defaults, unrelated working directories, corrupt adjacent markers, and two registered roots.
- [x] 4.2 Implement canonical candidate generation using saved paths, volume hints, and drive-relative paths on supported available volumes; verify bounded unavailable-volume handling, changed drive letters, wrong identities at cached paths, and same-drive directory moves requiring re-registration.
- [x] 4.3 Reject multiple distinct roots with the same identity while deduplicating equivalent canonical paths; verify a cached match never hides a duplicate in the remaining bounded candidates.
- [x] 4.4 Implement the read-only portable handshake and expected-identity/protocol delegation contract with a local-only execution endpoint; verify fake-launcher tests reject protocol mismatch, changed identity, and recursive central dispatch.
- [x] 4.5 Document selection precedence, duplicate-copy diagnostics, discovery limits, and launcher compatibility errors; verify examples demonstrate central targeting and independent direct invocation.

## 5. Windows process execution and command transport

- [x] 5.1 Implement explicit per-call child environment construction, portable root/cache/global/config/temp paths, removal of inherited Scoop activation paths, and profile-free PowerShell resolution; verify a fake backend sees only the selected Scoop context and parent variables remain unchanged on success/failure.
- [x] 5.2 Implement fixed-script argument transport into Windows PowerShell 5.1 without interpolating user data into script source; verify round trips for empty arguments, quotes, Unicode, spaces, parentheses, and shell metacharacters through direct and delegated execution.
- [x] 5.3 Preserve caller working directory, input/output/error streams, and backend exit codes; verify a fake backend observes identical command contracts through the central and portable launchers.
- [x] 5.4 Implement Windows process-tree lifecycle and cancellation with an OS-backed containment mechanism; verify cancellation and abrupt parent termination leave no managed child processes running and permit later lock acquisition.
- [x] 5.5 Document exit-status conventions, cancellation behavior, and the distinction between process isolation and host-effect adaptation; verify diagnostics distinguish launch/validation failures from backend failures.

## 6. Portable backend adaptation and initialization

- [x] 6.1 Implement and version the minimal pinned Scoop adapter/patch set identified in group 2, routing supported environment settings into portable state and child-process application, suppressing host shortcut/module registration, and confining dependency lookup; verify environment and ordinary-Scoop sentinels remain unchanged across install/update/uninstall fixtures.
- [x] 6.2 Implement command/flag policy enforcement and dependency-closure manifest preflight for install/update/uninstall, including installed uninstall metadata; verify unsupported hooks, global flags, root overrides, raw command bypasses, and engine self-update are rejected before prohibited effects or installation mutation.
- [x] 6.3 Implement backend artifact/version verification and explicit controlled engine invocation; verify missing, altered, or incompatible backends fail without calling a system `scoop` sentinel on PATH.
- [x] 6.4 Implement `env init` provisioning from verified release/engine artifacts with staged readiness, private state directories, filesystem/runtime checks, and failure cleanup confined to newly created staging; verify unique identities, initialized local launchers, unrelated non-empty directory rejection, and preservation of existing files after injected failure.
- [x] 6.5 Document supported commands, environment-setting behavior, package limitations, backend pins/patches, and provisioning recovery; verify a documented compatible fixture package can be installed, listed, updated, and uninstalled locally without host registration.

## 7. Per-environment locking and relocation

- [x] 7.1 Implement canonical-root OS-backed mutation locks and stable-state read coordination with bounded busy behavior; verify same-root serialization, different-root concurrency, and lock release after killed owners without relying on deleting a lock filename.
- [x] 7.2 Persist last-prepared root and repair journal state under the operation lock; verify preparation and backend launch share the protected interval and failed preparation prevents backend execution.
- [x] 7.3 Implement an owned-artifact catalog and repair/rebuild support for qualified shim formats, current-version/persist links, and adapter-controlled path state; verify drive-letter and same-drive root moves while unrelated same-drive paths remain untouched.
- [x] 7.4 Enforce canonical repair-target/link containment and reject unknown artifact formats or unsupported relocation; verify traversal/reparse-point fixtures cannot write outside the portable root or launch a backend after a rejected repair.
- [x] 7.5 Implement idempotent repair recovery and commit prepared-root state only after complete success; verify fault injection after partial changes resumes correctly and preserves the prior committed root on failure.
- [x] 7.6 Document relocation scope, supported package/link formats, busy diagnostics, and interrupted-operation recovery; verify direct invocation of a moved fixture root follows the documented recovery path without host registration.

## 8. Deployment and user-facing documentation

- [x] 8.1 Package Windows x64 release archives containing equivalent `portscoop.exe` and `psp.exe`, backend metadata/adaptation payload, and verified artifact information; verify both names behave equivalently from CMD and PowerShell without a Go toolchain on the target.
- [x] 8.2 Add a lightweight PowerShell setup helper for central deployment or portable initialization with explicit optional central user-PATH integration; verify setup adds only its requested launcher path and portable mode makes no host PATH/registry changes.
- [x] 8.3 Add setup/unsetup, compatible-release rollback, two-environment usage, and ordinary-Scoop coexistence documentation; verify worked examples cover default selection, explicit targeting, disconnected drives, and removal of host registration without deleting portable data.

## 9. Integrated qualification

- [x] 9.1 Run a Windows integration matrix with two distinct portable roots and an isolated ordinary-Scoop fixture; verify install/update/uninstall in one root leaves the other root, ordinary Scoop, persistent environment variables, shortcuts, and host module registrations unchanged.
- [x] 9.2 Exercise central delegation and direct portable execution after drive-letter and same-drive directory moves, plus disconnected, duplicated, corrupt, and incompatible targets; verify every environment-resolution and relocation scenario has observed passing coverage.
- [x] 9.3 Exercise concurrent same-root and different-root commands, cancellation, interrupted registry replacement, and interrupted repair; verify stable diagnostics, no leaked managed processes/locks, and successful documented recovery.
- [x] 9.4 Qualify the release on a clean supported Windows x64/PowerShell 5.1 environment without Go, with both CLI names and paths containing spaces/parentheses; verify prerequisites and the advertised compatibility subset match actual results.
- [x] 9.5 Run `go test ./...`, the Windows backend/integration suites, build/package smoke checks, and strict OpenSpec validation; verify all pass and record evidence before marking implementation tasks complete.
