# Generic hook operation compiler

The compiler under `internal/compatibility/hooks` is a bounded, non-evaluating language unit. Both `pre_install` and `post_install` use it. It has no application-name dispatch and does not execute PowerShell or inspect host AppData.

The lexer preserves quoted operands, literal/variable segments, line positions,
separators, and block structure. Unknown commands, command substitutions,
unsupported branches, or executable remainders reject the entire hook plan;
recognized actions are not returned after rejection.

Initial units recognize package-local `Rename-Item`, guarded empty persisted-file initialization through `Test-Path`/`New-Item`, and guarded optional AppData directory migration. The optional import must test both an empty destination and an existing host source, copy from that same source into the same declared persistent destination, and contain no extra effects. Optional directory creation must use the matching destination. Both pre-install and post-install plans retain their phase.

Matching is based on validated syntax and operands, without application-name
exceptions. Literal version expansion uses the checked manifest version. Host
imports produce a skip diagnostic, not an executable action. Required reads
overlapping omitted import output reject the plan, regardless of statement order.

The backend checks each invoked phase against the frozen declaration and executes actions in source order. All-skip hooks retain their declaration and explicit skip decision for deterministic saved-manifest recompilation. Required reads are checked against skipped import output across both phases.

Pre-install uses the physical version directory. Post-install requires the exact Store-owned `current` junction targeting that version. File initialization preserves existing content and supports declared source/destination persistence mappings; post-install additionally verifies the corresponding hard link. Rename never overwrites an existing target and cannot operate on declared persistence. Arbitrary reparse ancestors reject.

Each new operation belongs in its own unit with operand/condition checks, typed actions, effect regions, and positive/rejection tests. Every action must have an independently verified executor before field integration advertises it. Download payload validity and real-application qualification remain separate requirements.

## Local defaults

Separate units recognize single-file `Copy-Item` from a literal `$dir` path,
`New-Item -ItemType Directory` or `ensure` for directories, and `Set-Content` or
`New-Item -ItemType File -Value` for quoted literal content. Each may stand alone
or inside one matching negated `Test-Path` guard. File-copy wildcards, recursive
copies and command substitutions remain outside this bounded grammar. Host
copies are handled only by the optional-import unit.

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
signs are literal bytes. Double-quoted strings may reference checked immutable
bindings; unresolved variables reject.

## Literal collections and bindings

The compiler accepts comma-separated strings, multiline `@(...)` string arrays,
single-assignment local variables, and a finite literal-list `ForEach-Object`
block. Each iteration gets its own `$_` and local bindings. Nested loops,
dynamic collections, method/property access, and reassignment reject. `Join-Path`
accepts a checked package/persistence root and a literal relative child, with
positional or `-Path`/`-ChildPath` arguments.

Double quotes support backtick-dollar, backtick-backtick, and backtick-double-quote.
Escaped dollars stay literal and are never reinterpreted as variables. This does
not by itself expand the filename rules of an individual operation.

`Set-Content` accepts literal line arrays: each line ends with CRLF, or values are
concatenated with `-NoNewline`. Quoted `info` and `Write-Host` messages with resolved
literal interpolation are diagnostic-only. They cannot hide an unsupported body.
Existing guarded defaults keep their persistence-preservation behavior.

Limits are 64 KiB of source, nesting depth 32, 256 collection elements, 4096
expanded actions, and 1 MiB of expanded literal data. An additional 16,384-node
work bound limits non-action expansion. One invalid iteration rejects every
action; diagnostics retain its source line and iteration number.
