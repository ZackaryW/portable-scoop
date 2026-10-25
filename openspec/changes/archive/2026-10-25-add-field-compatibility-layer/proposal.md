# Proposal

## Why

The monolithic ZIP allowlist rejects portable behavior such as SpaceSniffer's configuration initialization, while accepting fields whose required effects are silently omitted. Portscoop needs per-field compatibility decisions and portable action replacements before the managed Scoop backend executes.

## What Changes

- Introduce one compatibility handler per manifest field, with use, rewrite, skip, or reject decisions and deterministic diagnostics.
- Resolve architecture before evaluating effects; reject unknown fields and unknown scripts, and check the complete dependency closure before mutation.
- Freeze rewritten declarations and typed lifecycle actions into the backend request. Recognize the complete SpaceSniffer configuration initialization pattern without executing its PowerShell text.
- Support declared SHA-1/SHA-256/SHA-512/MD5 download hashes separately from release integrity validation; retain ZIP-only extraction in this increment.
- Rewrite shortcut declarations into Store-owned launch aliases, and support literal fixed arguments for native executable shims. Unsupported script-wrapper arguments remain explicit rejections.
- **BREAKING**: Reject PowerShell module declarations instead of silently ignoring required module discovery. Other unsupported field handlers remain explicit and independently extensible.
- Version the backend contract without preserving compatibility with older Portscoop releases or Store formats. Breaking releases may require a fresh Store from a matching complete distribution; legacy support and Store migration are not requirements. Scoop manifest compatibility remains the goal within portable behavior. Self-update is outside this change.

## Capabilities

### New Capabilities
- `manifest-compatibility`: Per-field inspection, action rewriting, diagnostics, and a frozen portable execution plan.

### Modified Capabilities
- `scoop-execution`: Execute approved portable replacements, visibly skip optional integration, and reject unsupported required effects before dispatch.

## Impact

Go manifest policy and preflight, frozen request construction, pinned PowerShell adapter/build metadata, owned shim resolution and launch argument composition, compatibility tests and documentation. No new external runtime dependency, host Scoop changes, or network publication.
