# Manifest compatibility

Portscoop compiles a manifest before allowing the managed backend to mutate a Store. Every field has its own source unit in `internal/compatibility/`: the handler inspects the effective declaration and returns **use**, **rewrite**, **skip**, or **reject**, with normalized data or typed actions. Shared helpers implement JSON, path, binding, and argument checks. Handlers perform no I/O and never run scripts.

Architecture selection precedes effect inspection. All branch objects must be structurally valid; only the selected Windows x64 effects are compiled. Unknown effective fields are rejected. Field failures are reported in sorted order, and no executable plan is returned for a rejected manifest. Dependency preflight remains all-or-nothing.

## Current field behavior

| Fields | Behavior |
| --- | --- |
| `version`, `url`, `hash` | Fixed versions; checked HTTP(S) ZIP and inert EXE download plans with filename fragments and query handling; SHA-256, SHA-1, SHA-512, or MD5 with exact digest lengths. Release/backend integrity remains SHA-256. |
| `extract_dir`, `extract_to` | Checked package-relative paths, including `.`. |
| `bin` | Store-owned shims; literal fixed arguments for native EXE/COM targets. Script-wrapper arguments, dynamic expansion, and absolute host paths in fixed arguments are rejected. |
| `shortcuts` | Rewrite target/name/optional native arguments into launch aliases. Spaces in names become hyphens. No host Start Menu entry; icon overrides are unsupported. Identical aliases deduplicate; conflicting aliases reject. |
| `persist` | Store-owned persistence paths or source/destination pairs. |
| `env_add_path`, `env_set` | Checked paths and child-process environment declarations; reserved host/control variables remain blocked. |
| `depends` | Store-local dependency resolution. |
| `pre_install`, `post_install` | Compose checked package rename, persisted-file initialization, and selectively omitted optional AppData imports. Complete construct accounting and ordered frozen actions; see [hook operations](hook-operations.md). |
| `pre_uninstall`, `post_uninstall`, `installer`, `uninstaller`, `innosetup`, `cookie`, `psmodule` | Explicitly unsupported until a portable implementation exists. Module integration is no longer silently ignored. |
| `description`, `homepage`, `license`, `notes`, `suggest` | Declarative metadata retained without script evaluation. |
| `checkver`, `autoupdate`, `$schema`, `_comment`, `##` | Skipped as installation-irrelevant metadata; not evaluated. |

Hook recognition uses generic operation syntax and checked operands, never application names. Optional host imports are omitted only when their guards and complete body establish an independent optional migration into declared persistence. Unknown statements reject the whole hook, and required operations reading omitted data reject. Existing persisted content is preserved.

Adding support means changing the corresponding field handler, adding its typed action executor if needed, and verifying both accepted behavior and rejection boundaries. Do not add generic script evaluation or partial script matching. Manifest acceptance is not application-runtime sandboxing or proof that an application writes no files outside its Store.

## Backend compatibility

Portscoop does not guarantee backward compatibility with older Portscoop releases or Store formats. Breaking releases may require a fresh Store. Legacy execution paths and Store migration are not requirements; Scoop manifest compatibility remains the goal within the portable behavior supported by each field.

These replacements require backend patch `portable-operations-v1.0`. Both backend metadata and release inventory name that revision. A launcher must use its matching verified backend. An incompatible Store fails verification before execution: copying new launcher executables into it is insufficient. Use a complete matching distribution to initialize a fresh Store, and retain the old Store separately if its data is still needed. Self-update does not imply support for migrating incompatible Stores.
