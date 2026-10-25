# Generic hook operation compiler

The compiler under `internal/compatibility/hooks` is a bounded, non-evaluating language unit. Both `pre_install` and `post_install` use it. It has no application-name dispatch and does not execute PowerShell or inspect host AppData.

The lexer preserves quoted operands, line positions, separators, and block structure. Complete constructs are dispatched to operation units. Unknown commands, dynamic substitutions, unsupported branches, backtick escapes, or executable remainders reject the entire hook plan; recognized actions are not returned after rejection.

Initial units recognize package-local `Rename-Item`, guarded empty persisted-file initialization through `Test-Path`/`New-Item`, and guarded optional AppData directory migration. The optional import must test both an empty destination and an existing host source, copy from that same source into the same declared persistent destination, and contain no extra effects. Optional directory creation must use the matching destination. Both pre-install and post-install plans retain their phase.

Matching is based on validated syntax and operands. Tests vary package, filename, version, and profile names; Bitwarden is not a production exception. Literal version expansion is bounded to checked path operands. Host imports produce a skip diagnostic, not an executable action. Required reads overlapping omitted import output reject the plan, regardless of statement order.

The backend checks each invoked phase against the frozen declaration and executes actions in source order. All-skip hooks retain their declaration and explicit skip decision for deterministic saved-manifest recompilation. Required reads are checked against skipped import output across both phases.

Pre-install uses the physical version directory. Post-install requires the exact Store-owned `current` junction targeting that version. File initialization preserves existing content and supports declared source/destination persistence mappings; post-install additionally verifies the corresponding hard link. Rename never overwrites an existing target and cannot operate on declared persistence. Arbitrary reparse ancestors reject.

Each new operation belongs in its own unit with operand/condition checks, typed actions, effect regions, and positive/rejection tests. Every action must have an independently verified executor before field integration advertises it. Download payload validity and real-application qualification remain separate requirements.

## Local defaults

Separate units recognize single-file `Copy-Item` from a literal `$dir` path,
`New-Item -ItemType Directory` or `ensure` for directories, and `Set-Content` or
`New-Item -ItemType File -Value` for quoted literal content. Each may stand alone
or inside one matching negated `Test-Path` guard. File-copy wildcards, recursive
copies, content arrays, variables, loops and command substitutions remain outside
this bounded grammar. Host copies are handled only by the optional-import unit.

Missing-path guards must name the destination under `$dir`, or its declared
mapping under `$persist_dir`. Mappings can include a file within a persisted
directory. A packaged default cannot overwrite existing saved data. Before
persistence linking, ordinary package entries are replaced atomically when a
literal/copy action requires it; another name for a hard-linked file is not
modified. After linking, writes use the verified physical persistence target.

Literal content accepts ASCII, UTF-8 without BOM, and UTF-16LE (`unicode`, with
BOM). `Set-Content` appends CRLF unless `-NoNewline` is present; `New-Item -Value`
does not. Unspecified `Set-Content` encoding accepts ASCII content only; other
encodings and non-ASCII values with ASCII encoding reject. Single-quoted dollar
signs are literal bytes, while double-quoted variable expansion rejects.
