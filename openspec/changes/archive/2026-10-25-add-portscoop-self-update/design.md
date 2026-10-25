# Design

## Context

See `proposal.md` for motivation and `specs/runtime-self-update/spec.md` for the command contract.

The implementation is Go 1.25 with separate management (`internal/cli`) and application (`internal/appcli`, `internal/application`) boundaries. `scripts/install.ps1` downloads stable ZIP/checksum assets from `ZackaryW/portable-scoop`; it deliberately rejects non-empty destinations. `scoop.VerifyRelease` decodes `release.json` strictly, hashes the complete distribution, and requires three executables. Installed Host source verification currently permits one ready `store` child as a runtime namespace.

`resolver.Select` treats any adjacent `.portscoop` directory as Store state. Consequently, putting Host updater metadata there would break Host selection. Store initialization copies the three launchers and a separate pinned backend, but does not copy the Host's complete release inventory. `docs/setup.md` already describes compatible Store launcher-only replacement while preserving the backend and data.

`filelock.Acquire` provides exclusive OS-owned locks. `relocation.Coordinator` holds the Store mutation lock for package operations and only through startup for applications. There is no shared installation-use lease or runtime-update gate yet. `runner.Native` intentionally kills package descendants; it cannot own a worker expected to survive launcher exit. The application lifecycle now has explicit handoff and external-job checks that can inform the worker policy.

The implemented `add-pspe-shim-execution` change remains active with qualification unfinished. Its three-launcher deployment is the implementation baseline, not a claim of completed qualification. The user has now deferred further automated tests and VM qualification.

## Goals / Non-Goals

**Goals:** installation-local maintenance; deterministic release selection; an owned, recoverable Windows replacement transaction; retained Store identity/data; explicit handoff and completion status; no dependency on a compiler on the user's machine.

**Non-Goals:** package upgrades; updating all registered Stores; `--env` targeting; arbitrary destination paths; backend/metadata migrations; automatic downgrade; installer overwrite mode; forced process termination; changing PATH or execution policy; weakening external job policies; adding new automated tests or continuing VM qualification in this pass.

## Decisions

### 1. Name the operation Portscoop self-update

Add a dedicated Cobra `self-update` management command with these forms:

```text
psp self-update
psp self-update --check
psp self-update --version v0.2.1
psp self-update --status
psp self-update --recover
```

`portscoop` is equivalent. `update <app>` remains in the package allowlist; do not add a Scoop self-update operation or application fallback. Reject incompatible combinations of check/status/recover/version flags and reject any supplied global `--env`, including an empty explicit selector.

Use `os.Executable()` and adjacent installation state, not CWD or the registry default. A valid Store marker selects that Store. Invalid adjacent `.portscoop` state fails closed. A Host needs a supported verified ownership inventory; an isolated executable without one receives a manual-deployment diagnostic. To update another Store, invoke that Store's management launcher explicitly.

Alternative: update the Host's default Store. This would make `self` depend on registry state and leave users unsure which runtime changed.

### 2. Separate the updater service from package execution

Introduce `internal/selfupdate` with typed target, release, replacement-plan, transaction, and result contracts. Inject release discovery/download, filesystem, locking, clock, and worker-launch dependencies. The Cobra adapter formats states and exit statuses; the service does not classify package commands or invoke Scoop.

Share path validation, atomic JSON publication, and private transport helpers where their contracts fit. Keep package containment unchanged. A worker launch needs an independent lifetime and bounded control acknowledgement, with NUL terminal streams. Both management aliases must support the necessary private worker/probe entry points because either identical executable may be invoked.

### 3. Resolve once, then verify a pinned release

Use the official repository's release API to resolve the latest stable tag or an explicitly requested published tag. Constrain repository, platform, asset names, URL scheme, response sizes, archive sizes, and timeouts. After discovery, construct downloads for that immutable tag rather than continuing to use `latest/download`.

Use a maintained semantic-version parser compatible with Go 1.25. Normalize a leading `v`, reject known downgrades, and make automatic mode refuse an uncomparable development version; an explicit supported target is required in that case. A successful `--check` can report availability from release metadata without downloading/executing the candidate or creating persistent installation state. It must not claim backend compatibility before inspecting the verified payload.

Reuse the ZIP/checksum contract: `portscoop-windows-amd64.zip` and its `.sha256` sidecar. Verify the checksum, safe extraction, complete release inventory, required executable digests, identical management pair, and update descriptor before accepting replacement. HTTPS and checksums follow the existing publisher trust model; they are not an independent signing system.

### 4. Add a descriptor without changing the strict release envelope

Create `runtime-update.json` before `scripts/package.ps1` computes the release artifact inventory. Include its digest in the existing `release.json.artifacts` map. Keep the current top-level release envelope unchanged, since adding fields would break strict older decoders.

The descriptor records its own schema/update protocol, release version/tag, Windows architecture, supported target kinds, required three-launcher set, and the backend contracts supported by Store launcher-only updates. Cross-check its version with the pinned release. Require it for self-update candidates; an older archive without this contract is not silently adopted.

Persist a Store runtime receipt during initialization and successful update, recording version and owned launcher digests. Host ownership comes from its complete verified release inventory and descriptor. Reject mixed/unowned runtime states rather than treating arbitrary same-named files as update targets. An installation predating self-update support needs a one-time verified manual deployment; the old executable cannot acquire a command it does not contain.

### 5. Keep Host and Store replacement sets different

For a Host, use the owned old inventory and a fixed supported distribution namespace to plan replacements/additions/removals. Include binaries, backend source payload, shipped scripts/docs, descriptors, and the matching `release.json`. Do not infer overwrite permission from candidate filenames alone. Preserve the ready `store` child, Host update control namespace, and unrelated files outside the owned set. Drift in an owned file or a collision with a new candidate file stops preparation.

For a Store, the replacement set is exactly `portscoop.exe`, `psp.exe`, and `pspe.exe`, plus its updater receipt/journal. Check that the candidate explicitly supports the retained backend's schema/protocol/upstream/patch contract. Keep engine/adapter files and their inventory untouched. This avoids implicitly migrating shim implementations or changing installed package behavior. A candidate requiring another backend contract fails before handoff.

Alternative: replace a Store's backend along with its launchers. That can change the interpretation of existing shims, path state, and manifests and requires a separate migration design.

### 6. Use portable owned control namespaces

Use `.portscoop-host/` for Host identity, runtime-use lease, updater state, and per-transaction staging. Give it a versioned owned marker and stable Host installation UUID when persistent update state is first needed. Do not create `.portscoop` on a Host.

Use `.portscoop/state/runtime-update/` and the existing Store UUID for Store update state. Put staging/backups on the same local supported volume as the target. Protect private transaction payloads with current-user ACLs. Store normalized relative replacement paths and digests; pin the canonical installation and expected identity separately.

Split installed-source verification from downloaded-payload verification. The latter remains strict. Installed Host verification can recognize validated control namespaces and preserved unrelated files while still rejecting untracked executable/backend content inside managed namespaces. Store initialization must copy only distribution source artifacts, never Host update state or user files.

### 7. Coordinate use, pending state, and worker commit

Add a shared installation-use lease for runtime-dependent management and application commands; the worker requires its exclusive counterpart before replacement. Store commits additionally acquire the existing operation lock. Use one lock order: update ownership, exclusive runtime lease, then Store operation lock. Normal Store operations acquire shared runtime use before the operation lock. Preserve the short application-startup mutation-lock duration already implemented.

Download and verify before taking exclusive replacement coordination. Publish an exclusive, identity-bound pending transaction before worker handoff. New runtime-dependent commands that see pending state wait within the documented bound or explain the pending/recovery state; status/recovery can inspect it without using the mutable backend. An invocation that passed the gate before pending publication retains its shared lease, so the worker waits for it.

Do not transfer a process-owned file lock from caller to worker. The caller exits after accepted handoff; the worker obtains its own locks and reloads target identity, pending nonce, source ownership, and staged hashes under those locks. Competing self-update requests cannot adopt or replace that pending transaction. Existing unrelated sessions are never force-killed.

### 8. Finish through a private Windows worker

Copy the currently trusted management executable into protected transaction staging and verify that copy. Run it through a private updater endpoint; do not execute an unverified downloaded executable. The endpoint accepts only an owned transaction descriptor with fixed installation-relative operations, expected identity, protocol, and nonce. It is not raw command or arbitrary path forwarding.

The worker verifies the plan and acknowledges pending ownership, then waits on a verified process handle for the invoking launcher to exit. It does not inherit terminal streams, so it cannot retain a calling shell's pipeline. Independent launch must fail before handoff if an external job cannot release the worker. Use a bounded metadata/control contract rather than the package runner's kill-on-close lifetime.

Because the original loaded launcher must exit, its return code reports handoff acceptance, not final replacement success. Print the transaction ID and `psp self-update --status` instruction. The worker records completion/failure in the portable receipt. A later ordinary startup sees completed state or waits/reports busy/recovery; it does not silently use mixed files. Never start a second worker automatically after an ambiguous acknowledgement.

Alternative: overwrite the running binary or change the public entry points into permanent bootstrap stubs. The first relies on file-sharing behavior that is not established here; the second would redesign deployment beyond this command.

### 9. Journal fixed replacements and recover explicitly

Persist the plan and verified backups before changing runtime files. Journal phases such as staged, accepted, waiting-for-parent, committing, completed, failed, and recovery-required, together with each applied relative replacement. Recheck paths for reparse points and containment at commit/recovery; journal ownership is not permission to traverse arbitrary paths.

A failed commit restores the recorded old runtime where possible, removes only newly added owned candidate files, and preserves diagnostics. Only a coherent verified replacement or restoration clears the startup gate. If interruption/storage loss prevents that, retain recovery-required state and use `--recover` to reconcile the recorded transaction. Unsupported schema, identity mismatch, or damaged backups require inspection rather than guessed repair. Document use of the retained staged helper if the normal launcher entry points are unavailable.

`--status` returns 0 for completed/absent state, 3 for pending work, and 2 for failed/recovery-required state. Successful checks/no-ops/accepted handoffs return 0; pre-handoff cancellation is 130 and other failures are 2. Recovery handoff also reports acceptance separately from its final receipt.

## Risks / Trade-offs

- Windows executable sharing and independent worker lifetime → wait for caller exit, hold owned coordination, preserve backups, and reject unqualified external jobs; no forced process termination.
- An older unaware launcher can bypass new coordination → require a coherent supported current runtime/receipt and retain operating-system file-use diagnostics; do not treat mixed manual deployments as supported.
- Host metadata accidentally looks like Store state → use a distinct `.portscoop-host` namespace and keep resolver semantics explicit.
- A rolling latest release can mix artifacts → pin one tag before every asset download and cross-check the update descriptor.
- A Store launcher upgrade requires a new backend → reject it as migration-required; retain the existing backend byte-for-byte in this change.
- Partial replacement or removable-drive interruption → fixed relative paths, an identity-bound journal, verified backups, startup gating, and explicit recovery without alternate-root discovery.
- A returned handoff code can be mistaken for completed success → print pending state and provide a final status query.
- Testing is deferred → implementation and documentation must label the updater unqualified; do not reuse earlier `pspe` evidence as updater verification.

## Migration Plan

1. Ship the descriptor and runtime receipts with the first self-update-capable release. Document the one-time manual upgrade for older versions lacking the command.
2. Add the command/service, owned namespaces, coordination, worker, and status/recovery paths while retaining the empty-destination installer behavior.
3. Update Hosts as complete distributions and compatible Stores as launcher-only transactions; preserve UUIDs, default selection, package data, and the existing backend.
4. Retain rollback metadata until a coherent candidate or prior runtime has been recorded. No automatic fleet update or backend migration is introduced.
5. Keep both active changes unarchived. The relocation delta composes the implemented `pspe` launch rules with runtime updates; when syncing later, preserve both changes' scenarios rather than overwriting one with the other.

Automated tests, test authoring, and VM qualification are deferred by the user's current instruction. Packaging/building the implementation and structural OpenSpec checks are separate from a claim of tested runtime behavior. Every Git push and any release publication still require fresh explicit authorization.
