# Generic hook operation compiler

The independently tested compiler under `internal/compatibility/hooks` is a bounded, non-evaluating language unit. Live field/backend integration is a later checkpoint. It has no application-name dispatch and does not execute PowerShell or inspect host AppData.

The lexer preserves quoted operands, line positions, separators, and block structure. Complete constructs are dispatched to operation units. Unknown commands, dynamic substitutions, unsupported branches, backtick escapes, or executable remainders reject the entire hook plan; recognized actions are not returned after rejection.

Initial units recognize package-local `Rename-Item`, guarded empty persisted-file initialization through `Test-Path`/`New-Item`, and guarded optional AppData directory migration. The optional import must test both an empty destination and an existing host source, copy from that same source into the same declared persistent destination, and contain no extra effects. Optional directory creation must use the matching destination. Both pre-install and post-install plans retain their phase.

Matching is based on validated syntax and operands. Tests vary package, filename, version, and profile names; Bitwarden is not a production exception. Literal version expansion is bounded to checked path operands. Host imports produce a skip diagnostic, not an executable action. Required reads overlapping omitted import output reject the plan, regardless of statement order.

Each new operation belongs in its own unit with operand/condition checks, typed actions, effect regions, and positive/rejection tests. Every action must have an independently verified executor before field integration advertises it. Runtime path ownership, phase-specific persistence, download payload validity, and integration qualification remain separate requirements.
