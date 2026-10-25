# Proposal

## Why

Portable compatibility must be expressed through reusable operations rather than application-specific recipes. Mixed hooks need required Store-local work preserved and only demonstrably optional host-data imports omitted; recognizing an app name or an AppData keyword cannot establish that distinction.

## What Changes

- Keep per-field compatibility entry points, with a shared operation compiler for pre-install and post-install hooks. Each supported operation owns its recognition, checks, and portable replacement or explicit skip decision.
- Initially support checked package-file rename, existing persisted-file initialization, and a bounded optional host-AppData directory-import construct. Validate the complete hook and preserve required-operation order; reject unknown effects and unsafe dependencies on skipped data.
- Make matching independent of package names, vendor names, and whole application scripts. Bitwarden and SpaceSniffer are qualification fixtures alongside unrelated synthetic manifests.
- Support checked direct EXE payloads as inert downloaded package files, never installer execution, so Bitwarden can be qualified through the generic pipeline.
- Expand deterministic download filename handling and safe ZIP-family, 7z, and tar/gzip extraction, prioritizing measured Main/Extras blockers. Add reusable package-local copy and default-file/directory operations after the initial operation compiler is verified.
- **BREAKING**: Advance the managed backend contract for ordered actions and phase-aware execution; incompatible Stores require a fresh matching distribution, not an implicit migration.
- Requalify the generic implementation, rerun manifest coverage, and finish the concise product README with technical material under docs/. Record expansion priorities from measured blockers rather than unsupported coverage promises.

## Coverage goal

Aim for at least **70% full install-preflight acceptance across combined Main and Extras**, including dependency closure, on pinned Windows x64 snapshots. At 4,148 entries the target is 2,904, compared with the restored baseline of 1,626 (39.2%). This is an optimization goal, not a hard completion gate or permission to bypass unsupported behavior. Report achieved counts, remaining blockers, and the gap even if the goal is missed; distinguish declaration acceptance from runtime qualification.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `manifest-compatibility`: Generic operation composition, selective optional-import omission, pre/post phase checks, download/extraction support, and ordered frozen actions.

## Impact

The Go field compiler, managed PowerShell action executor, backend metadata, compiler and execution tests, disposable VM qualification, manifest audit, and documentation. No ordinary host Scoop changes or automatic publishing.

This supersedes the app-specific planning in `add-portable-bitwarden-support`. Implementation commit `574089a` was reverted by `e561421`. Its 1,739-manifest audit and VM run are historical prototype evidence only; they do not qualify the replacement. The restored baseline passed 1,626 of 4,148 full install preflights in the same pinned snapshots. Active OpenSpec artifacts remain excluded from implementation commits.
