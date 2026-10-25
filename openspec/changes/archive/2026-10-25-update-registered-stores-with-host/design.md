# Design

## Context

See proposal.md for motivation. Store receipts own three launchers; backend metadata separately inventories Store-relative adapter and engine files. The current worker assumes it is copied from the installation being replaced, which makes delegating to an old Store incapable of upgrading that Store's backend. The registry is shared by the user and has no Host ownership field.

## Goals / Non-Goals

**Goals:** Reuse existing transaction containment, inventory verification, locks, journal and rollback. Support older schema-1 managed backends without reinitialization. Keep direct Store maintenance local.

**Non-Goals:** Package upgrades, arbitrary schema migrations, registry repair, adopting unmanaged files or ordinary Scoop, automatic downgrades, cross-drive atomicity.

## Decisions

1. A Host coordinator snapshots registry entries, groups aliases by UUID and uses existing bounded discovery. It pins one release and caches its downloads for the invocation. Invalid/offline/ambiguous entries produce explicit outcomes; they never select a fallback identity. A current Host still runs this pass. Per-target version comparisons prevent downgrades.
2. Process Stores synchronously before the Host's existing detached handoff. Each Store keeps its own private transaction and locks. This avoids workers waiting for a parent that is still downloading other Stores and permits accurate completed Store reporting. A failed Store does not roll back successful independent Stores. Cancellation stops further work.
3. Build a closed replacement set from launcher receipts and backend inventories. Only `.portscoop/backend/` adapter files and `scoop/apps/scoop/current/` engine files are eligible, with metadata handled separately. Candidate backend paths map to release adapter/engine sources and must match release hashes. Back up removed owned files as well as replaced ones; new paths must not exist. Preserve all other state, including relocation receipts.
4. For coordinated Store transactions, retain a verified snapshot of the controlling Host distribution as recovery-worker provenance. Copy its current launcher as the helper; never execute the downloaded candidate to perform replacement. Independent verification checks the snapshot and worker hash even after the Host updates or disappears. Existing direct transactions keep their previous-runtime provenance. This costs disk space but avoids trusting an unverified external helper or depending on a subsequently updated Host.
5. Host status/recovery enumerate current registrations without changing them. Recover recorded Store transactions synchronously with their own coordination; active owners cannot be recovered. Staged helpers remain an escape hatch for damaged launchers or missing Host registrations. Report aggregate incomplete outcome if any target fails or is unavailable, with individual results retained in target journals.

## Risks / Trade-offs

- Interrupted Store replacement → existing pending gate, exact-operation backups and explicit recovery; never reinitialize a Store.
- Concurrent commands → existing bounded update/runtime/package locks; leave diagnostics when busy.
- Old Store cannot understand new transaction operations → use current Host code and its verified staged helper, not old Store execution.
- Extra staging space per Store → fail before replacement on staging failure; no recursive deletion of installed trees.
- Existing older Host updater does not know this feature → after installing the new Host version, rerun self-update to catch up registered Stores; future invocations coordinate automatically.

## Migration Plan

Keep existing transaction schema fields backward-readable for old transactions; add optional controller provenance for the new Store path. Deploy as a normal release with no registry or Store identity migration. Qualify an older Store in the disposable VM, including backend advancement and preserved data. No push or release is part of this change without fresh authorization.
