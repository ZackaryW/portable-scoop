# Tasks

Automated tests, test authoring, and VM qualification are deferred by the user's instruction. The review, artifact inspection, and compilation checks below do not establish tested runtime behavior. All tasks remain unfinished until implementation is delivered.

## 1. Command and installation targeting

- [x] 1.1 Introduce typed self-update service contracts and injectable discovery, download, filesystem, clock, locking, and worker dependencies; verify the delivered `internal/selfupdate` API has no package dispatch or Scoop execution dependency.
- [x] 1.2 Add equivalent `self-update` commands to the management frontend with `--check`, `--version`, `--status`, and `--recover`; verify command registration and explicit flag-combination checks leave package `update <app>` and `pspe` separate.
- [x] 1.3 Select the managed installation from the invoked executable, validate adjacent Store state or Host ownership, and reject every explicit `--env`; verify the selection code never falls back to CWD, registry default, or another installation.
- [x] 1.4 Document the command name, invocation-local target, flag combinations, and unsupported standalone-binary diagnostic; verify help and setup documentation agree with the command contract.

## 2. Release descriptor and installation ownership

- [x] 2.1 Define the versioned `runtime-update.json` contract and generate it before release hashing; verify package-script inspection shows its digest in the unchanged strict `release.json` envelope and names all three launchers.
- [x] 2.2 Add Store runtime receipts during initialization and successful replacement, with version and owned launcher digests; verify the receipt contract binds the existing Store UUID and does not replace backend metadata.
- [x] 2.3 Add an owned Host identity and update namespace at `.portscoop-host/` plus Store control storage under `.portscoop/state/runtime-update/`; verify path and marker definitions cannot classify a Host as a Store or overwrite unrelated existing controls.
- [x] 2.4 Separate downloaded-payload verification from installed Host source verification and source copying; verify downloaded archives remain strict, recognized controls are validated, and Store initialization excludes update state and unrelated user files.
- [x] 2.5 Document ownership requirements and one-time manual deployment for older runtimes lacking self-update; verify the instructions retain the empty-destination installer contract and distinguish Host distribution ownership from Store launcher receipts.

## 3. Release discovery and verified staging

- [x] 3.1 Add a bounded HTTPS client for the official repository's stable-release and explicit-tag metadata; verify the client fixes platform, repository, allowed assets, redirects, limits, and timeouts before constructing tag-pinned downloads.
- [x] 3.2 Implement semantic-version selection, automatic already-current behavior, known-downgrade rejection, and explicit targeting for uncomparable development builds; verify the decision branches and selected version-library compatibility with Go 1.25.
- [x] 3.3 Implement metadata-only `--check` with current/candidate/kind reporting; verify its call path neither creates persistent installation ownership nor downloads or executes candidate code and does not claim unverified Store compatibility.
- [x] 3.4 Download into private same-volume staging and verify checksum, safe extraction, full inventory, descriptor/tag/platform agreement, three executable hashes, and identical management binaries; verify every failure returns before replacement ownership is accepted.
- [x] 3.5 Document discovery, pinned assets, checksum trust, development-version handling, and check-only limitations; verify the published examples and diagnostics match the implemented selection and staging rules.

## 4. Fixed replacement planning

- [x] 4.1 Build Host plans from verified old ownership and supported distribution namespaces, including coherent source payload and inventory replacements/removals; verify plans preserve the default `store` child, registrations, persistent environment, and unrelated files while rejecting drift and new-file collisions.
- [x] 4.2 Build Store plans containing only the three launchers and owned runtime receipts; verify candidate compatibility is checked against the retained backend contract before handoff and no backend, package, shim, or path-state mutation enters the plan.
- [x] 4.3 Bind normalized relative operations to the canonical installation identity, expected hashes, and private transaction owner; verify plan construction rejects redirected paths, traversal, unsupported schemas, and ownership mismatches.
- [x] 4.4 Document Host versus Store replacement scope and migration-required outcomes; verify examples explain that updating one installation leaves its default and other registered Stores unchanged.

## 5. Runtime coordination and startup gates

- [x] 5.1 Add shared installation-use leases and exclusive replacement leases while retaining the existing Store operation lock; verify the lock implementation supports bounded cancellation and OS-owned release on process exit.
- [x] 5.2 Integrate leases and identity-bound pending-state checks into management and application startup; verify acquisition order is consistent and application lifetime does not extend the Store mutation lock beyond process creation.
- [x] 5.3 Serialize updater ownership and let the worker reacquire exclusive runtime coordination followed by the Store operation lock; verify it reloads identity, plan, pending nonce, source ownership, and staged hashes after the caller exits instead of transferring caller locks.
- [x] 5.4 Keep status/recovery reachable during pending or interrupted transactions and keep independent roots independently coordinated; verify startup gates distinguish active, failed, and recovery-required state without adopting stale lock files as permanent owners.
- [x] 5.5 Document wait bounds, busy diagnostics, application behavior, and interrupted-startup gates; verify the documentation states that unrelated processes are never force-terminated.

## 6. Independent Windows worker handoff

- [x] 6.1 Stage and hash a copy of the currently trusted management executable and add private worker/probe entry points to both identical management names; verify endpoints accept only a supported owned transaction and cannot forward arbitrary commands or target paths.
- [x] 6.2 Launch the worker with private bounded acknowledgement, NUL terminal streams, and an independent Windows lifetime; verify the implementation checks residual external job restrictions and fails before accepted handoff when independent lifetime is unavailable.
- [x] 6.3 Bind the worker to a verified invoking-process handle and wait for that process to exit before replacing loaded files; verify caller shutdown releases its leases and no replacement begins while relying on the caller's ownership.
- [x] 6.4 Report accepted or uncertain handoff with transaction identity and status instructions; verify ambiguous acknowledgement cannot automatically launch a duplicate worker and cancellation cleans up only uncommitted startup.
- [x] 6.5 Document accepted handoff separately from completed replacement, including calling-shell and external-job limitations; verify examples do not equate launcher exit code 0 with a successful later worker result.

## 7. Journal, replacement, and recovery

- [x] 7.1 Persist the fixed plan, verified backups, phase transitions, and per-file progress before runtime mutations; verify the journal records enough ownership and digest information to distinguish old, candidate, and newly added files.
- [x] 7.2 Implement coordinated replacement with containment rechecks, coherent inventory/receipt publication, and bounded file-use failures; verify failure paths restore recorded owned files and remove only new transaction-owned additions.
- [x] 7.3 Implement status queries and explicit recovery of the recorded transaction, including worker handoff when needed; verify unsupported schemas, identity mismatches, damaged backups, and disconnected media produce diagnostics without selecting another root.
- [x] 7.4 Implement outcome codes and persistent final results: success/accepted 0, pending status 3, failure/recovery-required 2, and pre-handoff cancellation 130; verify each public result maps explicitly and only verified coherence clears startup gating.
- [x] 7.5 Document rollback, interrupted-media recovery, retained staged-helper recovery when entry points are unavailable, and receipt retention; verify every recovery example refers to the same installation and owned transaction.

## 8. Integration delivery without qualification

- [x] 8.1 Compile both frontends and the updater worker for Windows amd64 using the repository build command; verify compilation succeeds and the management aliases remain identical, without running or adding tests.
- [x] 8.2 Review the final implementation against all three delta specs and inspect the packaged descriptor/inventory/launcher set; record remaining limitations and explicitly label runtime qualification deferred rather than claiming the updater is tested.

## Workflow follow-up

### Delivery evidence (2026-10-25)

- Implemented installation-local command selection, descriptor/receipt ownership,
  bounded GitHub discovery and verified ZIP staging, fixed Host/Store plans,
  shared/exclusive leases, Windows worker handoff, journaling, status and rollback.
- Source review covered all three delta specs, including identity reload after
  caller exit, invocation-local selection, and the separate Store operation lock.
- Repository packaging built both Windows amd64 frontends and identical management
  aliases. The final local review package is
  `dist/portscoop-0.0.0-selfupdate-review.20261025.3-windows-amd64.zip`.
  Its strict release verification passed; the update descriptor is inventoried,
  names all three launchers, and records the matching portable-fields-v1.0 backend.
- Go module inspection confirms x/mod v0.29.0 requires Go 1.24, compatible with the
  project's Go 1.25. Source builds and strict OpenSpec validation passed.
- Automated tests, test authoring, and VM qualification remain deferred. No live
  runtime update or interrupted-media recovery was exercised. Restrictive external
  jobs fail handoff; incompatible Store backends require a fresh Store. This is an
  implementation delivery, not a runtime qualification or published release.
- The user's current instruction authorizes syncing and archiving completed
  changes. It supersedes earlier workflow notes to keep this change active after
  implementation; incomplete pspe qualification remains active separately.

- Resume automated tests and disposable-VM qualification only after the user lifts the current deferral; do not reuse prior application-launch evidence as self-update verification.
- Keep `add-pspe-shim-execution` and this change active until their remaining work and review requirements are satisfied. Preserve both changes' launch and replacement scenarios when synchronizing specs later.
- Keep active OpenSpec artifacts out of incremental implementation commits under the existing commit instruction.
- Obtain fresh explicit authorization for every Git push and for release publication; planning or local implementation does not authorize either.
