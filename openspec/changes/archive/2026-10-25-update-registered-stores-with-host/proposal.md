# Proposal

## Why

Updating a Host currently leaves registered Stores on older launchers and managed backends, so a current Host can still reject newly supported manifests. Users expect Host maintenance to bring their reachable portable environments up to date too.

## What Changes

- **BREAKING**: Host self-update includes all registered Stores, independent of default selection, and checks them even when the Host is current.
- Update each reachable identity once; report unavailable, mismatched, busy, newer, and failed entries without blocking other identities.
- Upgrade Store launchers and inventory-owned backend files together while preserving UUIDs, applications, configuration, buckets, caches, shims and persistence.
- Pin one release, retain independent Store transactions and recovery, and aggregate check/status/recovery output.
- Direct Store invocation remains scoped to that Store with its existing compatible launcher update contract.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `runtime-self-update`: Host-wide registered Store maintenance, managed backend replacement, per-target outcomes and recovery.

## Impact

The self-update service, registry integration, transaction ownership validation, CLI help and maintenance documentation change. No package upgrades, registry migration, persistent environment changes, arbitrary backend scripts or ordinary Scoop operations are introduced. Verification uses isolated fixtures and the disposable Windows VM.
