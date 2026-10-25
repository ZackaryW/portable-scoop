# Design

## Context

See proposal.md for motivation. Current Go validation in internal/backend/scoop/policy.go has an allowlist and rejects every script. Plan freezes declarations for a pinned PowerShell backend. That backend suppresses shortcuts and modules, and relocation owns native shim metadata but pspe rejects fixed arguments. Saved versions are checked before mutation. The existing root lock and frozen request remain the execution boundary.

## Goals / Non-Goals

**Goals:** A small per-field handler contract; deterministic inspection; executable portable replacements with no arbitrary PowerShell evaluation; compatibility reports available to callers and concise CLI notices; preservation across install/update/reset/uninstall and relocation.

**Non-Goals:** Full Scoop parity in one change, a general PowerShell translator, runtime application sandboxing, non-ZIP extraction/helper bootstrap, portable module discovery, backward compatibility with older Portscoop releases or Store formats, legacy support, Store migration, and self-update. Fixed arguments are supported for native EXE/COM targets only; script wrapper arguments and dynamic placeholder arguments require separate qualified implementations.

## Decisions

1. **One Go source unit per field under internal/compatibility.** A registry maps field names to handlers; shared helpers provide strict JSON, path, binding and argument checks. A handler receives the immutable effective declaration and returns a decision plus typed actions or normalized values. Metadata-only units can share a no-execution helper. This avoids either a monolithic switch or a separate plugin protocol for every small field.
2. **Compile before execute.** Select and structurally validate architecture first; inspect effective fields in lexical order, aggregate field failures, then validate cross-field constraints and merge aliases deterministically. Compile the full dependency closure before dispatch. Retain recognized original hook declarations for saved-manifest recompilation, but never execute them: the frozen request carries typed actions separately. Recompilation is idempotent, including shortcut/bin deduplication.
3. **Separate interpretation from effects.** Existing checked declarative behavior continues through the pinned backend. Custom initialization is a typed `ensure-persisted-file` action at pre_install with only a simple validated filename. Its recognizer matches the entire known three-line SpaceSniffer pattern, allowing only whitespace/newline variation and equal captured filenames. Require a matching persistence declaration. No partial matching, command-name allowlist, scriptblock construction, or dynamic evaluation.
4. **Check filesystem effects at execution.** The replacement executor checks the package and persistence paths are inside the expected Store layout, rejects reparse ancestors and non-file existing destinations, and never overwrites existing persisted data. The action uses no absolute paths, so existing persistence relocation handles it. Runtime containment protects this action; it is not advertised as a sandbox for the launched application.
5. **Shortcut rewriting uses existing shim ownership.** Derive a stable alias from the display name, replace spaces with hyphens, validate the resulting logical name, and reject unsupported icon/argument forms. Merge identical aliases; reject differing definitions. Remove shortcut metadata from the effective backend declaration after compilation; retain decisions for notices. Existing uninstall/reset ownership then works through bin. Native fixed arguments use a bounded literal Windows command-line parser shared by compatibility and shim resolution; reject variables/control characters and script wrappers with arguments. Native shim metadata retains the literal argument string, and pspe parses it into argv before appending caller arguments.
6. **Honest compatibility status.** Record decisions for every field. Print meaningful rewrite/skip notices after successful preflight, sorted by package/field. Unsupported hooks, installer, uninstaller, innosetup, cookie, and psmodule have their own explicit handlers. Descriptive/updater metadata is kept or removed as appropriate without evaluation. Hash parsing is independent of release SHA-256 patterns.
7. **Backend revision is explicit; backward compatibility is not required.** Change the adapter patch revision together with build metadata. A launcher must still use its matching verified backend. Unsupported older Stores fail before execution; the design need not retain their formats, provide legacy execution paths, or migrate them. Breaking releases may require a fresh Store initialized from the complete distribution. Scoop manifest compatibility remains constrained by portability. Self-update remains a separate change and does not imply Store migration support.

## Risks / Trade-offs

- Exact hook recognition is intentionally narrow → reject unrecognized variants; add separately reviewed recipes later.
- SHA-1/MD5 provide weaker upstream checksums → use only when declared by the manifest, never for release/backend integrity.
- Shortcut names can collide → deterministic preflight rejection within a manifest; preserve existing backend ownership behavior between packages.
- Existing module manifests become rejected → explain the missing portable implementation rather than report a working installation.
- Unit-level adapter tests are not full application portability certification → run controlled temporary filesystem tests and existing Go/bootstrap suites; reserve real application execution for the disposable VM.

## Deployment Policy

Ship the launcher and revised backend in one complete distribution. Initialize a fresh Store for a breaking release and for qualification. Backward compatibility, in-place migration, and Store downgrade support are not release requirements. Retain an old Store separately when its data is still needed; this policy does not authorize deleting it. Returning to an earlier release means using that release with its own matching Store, not opening a newer Store with older binaries.
