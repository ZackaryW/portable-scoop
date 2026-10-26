# Proposal

## Why

Fresh Stores lack Main and depend on a host Git installation for bucket operations. Portable manifests such as PotPlayer additionally need bounded collection-based hooks and NSIS extraction, so improving hook parsing alone would not deliver a usable installation workflow.

## What Changes

- Extend the non-evaluating hook compiler with explicit literal escapes, literal arrays, immutable local bindings, bounded `ForEach-Object` expansion, and checked `Join-Path` expressions. Keep operation recognition independent of application identity.
- Add separate typed operations for package-local wildcard cleanup and recursive resource copying; extend default-file initialization to literal line arrays. Preserve guarded configuration defaults and match supported copy collision semantics explicitly.
- Add NSIS-only extraction through a pinned, release-supplied, Store-managed full 7-Zip component. Verify its artifacts, inspect and bound its output, and never execute the installer or use an extractor discovered on host PATH.
- Include the extractor in complete backend provisioning, verification, and self-update ownership. Existing supported archive formats retain their managed Go readers.
- Automatically bootstrap Main plus pinned portable Git and full 7-Zip in new Stores, including a Host's first default Store. Resolve managed helpers exclusively inside the selected Store, with an explicit bootstrap command for existing Stores; remove the Program Files Git prerequisite.
- Treat these tools as managed runtime dependencies, with visible versions and complete artifact ownership, rather than silently bypassing the hooks of ordinary Main packages. Their presence enables supported Store operations; it does not approve arbitrary manifest effects or expand native extraction beyond NSIS.
- Qualify the unchanged PotPlayer manifest as an application fixture, alongside unrelated generic fixtures and existing Bitwarden/SpaceSniffer regressions. Keep static manifest acceptance distinct from verified payload and application behavior.
- Deliver dependency-ordered checkpoints with focused verification before enabling dependent integration.

## Capabilities

### New Capabilities

None. These behaviors extend the existing compatibility and execution contracts.

### Modified Capabilities

- `manifest-compatibility`: Bounded collection-based hooks, checked cleanup and tree copying, and NSIS extraction with complete-operation rejection and confinement.
- `scoop-execution`: Ownership, verification, invocation, and lifecycle management of the pinned NSIS extraction component.
- `environment-management`: Automatic Main and helper bootstrap, readiness, failure recovery, and explicit bootstrap for existing Stores.
- `runtime-self-update`: Include owned helper components in Host-managed backend replacement and recovery while retaining the direct Store launcher-only contract.

## Impact

Touches `internal/compatibility/hooks`, `internal/archive`, Store initialization and CLI commands, the managed backend bridge and PowerShell action executor, backend preparation/release inventories, and Store backend self-update ownership. Adds pinned Git and native extraction dependencies and their redistribution notices; users need no additional system installation.

The user approved this narrow exception to the previous pure-Go-only archive design. It does not authorize arbitrary external extractors, installer execution, general PowerShell evaluation, app-specific handlers, or a release push. Production implementation remains a separate apply step.
