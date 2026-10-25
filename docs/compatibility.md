# Manifest compatibility

Portscoop compiles a manifest before allowing the managed backend to mutate a Store. Every field has its own source unit in `internal/compatibility/`: the handler inspects the effective declaration and returns **use**, **rewrite**, **skip**, or **reject**, with normalized data or typed actions. Shared helpers implement JSON, path, binding, and argument checks. Handlers perform no I/O and never run scripts.

Architecture selection precedes effect inspection. All branch objects must be structurally valid; only the selected Windows x64 effects are compiled. Unknown effective fields are rejected. Field failures are reported in sorted order, and no executable plan is returned for a rejected manifest. Dependency preflight remains all-or-nothing.

## Current field behavior

| Fields | Behavior |
| --- | --- |
| `version`, `url`, `hash` | Fixed versions; HTTP(S) ZIPs without filename fragments; SHA-256, SHA-1, SHA-512, or MD5 with exact digest lengths. Release/backend integrity remains SHA-256. |
| `extract_dir`, `extract_to` | Checked package-relative paths, including `.`. |
| `bin` | Store-owned shims; literal fixed arguments for native EXE/COM targets. Script-wrapper arguments, dynamic expansion, and absolute host paths in fixed arguments are rejected. |
| `shortcuts` | Rewrite target/name/optional native arguments into launch aliases. Spaces in names become hyphens. No host Start Menu entry; icon overrides are unsupported. Identical aliases deduplicate; conflicting aliases reject. |
| `persist` | Store-owned persistence paths or source/destination pairs. |
| `env_add_path`, `env_set` | Checked paths and child-process environment declarations; reserved host/control variables remain blocked. |
| `depends` | Store-local dependency resolution. |
| `pre_install` | Exact recognized persisted-file initialization pattern becomes a typed file operation. Everything else rejects. |
| `post_install`, `pre_uninstall`, `post_uninstall`, `installer`, `uninstaller`, `innosetup`, `cookie`, `psmodule` | Explicitly unsupported until a portable implementation exists. Module integration is no longer silently ignored. |
| `description`, `homepage`, `license`, `notes`, `suggest` | Declarative metadata retained without script evaluation. |
| `checkver`, `autoupdate`, `$schema`, `_comment`, `##` | Skipped as installation-irrelevant metadata; not evaluated. |

The first hook recipe recognizes the whole `Test-Path` / `New-Item` pattern used by SpaceSniffer to initialize `SpaceSnifferConfig.xml`. It requires the same simple filename under `$dir` and `$persist_dir` and a matching `persist` declaration. Additional statements, different commands, paths, or missing persistence reject. The executor creates the file before persistence linking and leaves existing persisted content intact.

Adding support means changing the corresponding field handler, adding its typed action executor if needed, and verifying both accepted behavior and rejection boundaries. Do not add generic script evaluation or partial script matching. Manifest acceptance is not application-runtime sandboxing or proof that an application writes no files outside its Store.
