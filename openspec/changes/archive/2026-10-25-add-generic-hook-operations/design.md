# Design

## Context

The restored implementation has per-field Go handlers, a pure compiler, frozen actions, and one whole-hook persisted-file initializer. The PowerShell adapter permits exactly one pre-install action. Post-install hooks and non-ZIP payloads are rejected. See proposal.md for motivation and the 70% goal.

The app-specific prototype was reverted. Its artifacts are retained outside active changes at `.work/superseded-openspec/add-portable-bitwarden-support`; its runtime tests and 1,739-entry audit cannot qualify this implementation. The remaining README/docs drafts must be refreshed before they are committed as current documentation.

Baseline investigation on the pinned 4,148-entry snapshots found 971 manifests blocked only by url, 317 by pre_install plus url, 253 by pre_install alone, and 42 by post_install alone. Counts describe own-manifest failures and overlap with other feature estimates; they are not promised full-preflight gains. The 70% target is 2,904 full dependency-preflight successes, compared with baseline 1,626.

## Goals / Non-Goals

**Goals:** Reusable operation units, complete-hook accounting, explicit selective skips, phase-aware execution, broader portable payload handling, and measured progress toward 70% combined Main/Extras acceptance.

**Non-goals:** Application-name dispatch, arbitrary PowerShell interpretation, importing host profiles, installer/custom-action execution, global integration, ordinary Scoop helper reuse, backward Store migration, and asserting that preflight proves runtime portability. Reaching 70% never authorizes weakening these boundaries.

## Decisions

### Keep fields as entry points; compose operation units

`pre_install` and `post_install` delegate to one bounded operation compiler. Retain one unit per feature: package rename, package copy, persisted/default file initialization, directory creation, and optional host-data import. Operation units receive normalized syntax plus manifest context and produce a typed action or an explicit skip/reject decision. Production code has no vendor/app-name branching or full app-script fingerprints.

Use a non-evaluating lexer and bounded construct recognizers in Go. Track source positions, quote boundaries, comments, block nesting, and separators so apparent matches inside strings or unrecognized blocks cannot become actions. Recognizers must consume complete constructs; leftover executable tokens reject the entire manifest. Support a documented subset, not all PowerShell grammar. Unsupported loops, dynamic expressions, commands, and expansions remain errors. Literal `$version`, `$dir`, and `$persist_dir` substitutions are allowed only in separately validated path operands; `$env:APPDATA` is allowed only in the optional-import recognizer and is never resolved against the host.

The first slice composes the existing empty-config initializer, checked rename, and optional directory import. The second slice adds bounded local copy and literal defaults driven by observed blocker examples. Tests use unrelated package/profile names, reordered independent constructs, multiple actions, and negative tails. Bitwarden and SpaceSniffer are input fixtures, never special production handlers.

### Prove omission at the operation level

The initial optional-import construct is a guarded directory migration: host source exists AND package profile is empty, optional creation of the same destination, then recursive copy from that source into that destination. Require a checked source suffix, matching destination operands, and matching declared persistence. Reject else branches, host mutation, extra statements, unconditional copy, and source/destination mismatches. These checks establish that a fresh profile is already a valid outcome when the host source is absent; no host filesystem inspection is needed.

Record each construct's read/write regions while compiling. If another required operation consumes a region supplied by the skipped import, reject rather than assuming that imported data exists. Unknown effects also reject. Required local work is retained in source order. A hook consisting only of recognized safe skips can be omitted entirely, with diagnostics, instead of generating a dummy action. The whole hook must be accounted for even though its operations are classified separately.

### Freeze ordered actions and validate lifecycle layout

Extend the frozen plan with ordered actions and operation diagnostics. Keep the field-level disposition; identify operation position, disposition, and reason without inventing per-application metadata. The adapter checks the frozen manifest and invoked phase, then executes approved actions in order. Recompilation of stored declarations must be deterministic.

Pre-install operates on the physical version directory before persistence linking. Post-install resolves only the verified Store-owned current alias to that version directory, and uses checked persistence mappings for initializer/copy destinations. Arbitrary reparse paths remain forbidden. Rename is limited to package files outside declared persistence and is non-overwriting. Copy/default handlers preserve existing persisted content. Filesystem failures can still interrupt installation; compiler acceptance is not a transactional rollback guarantee.

### Separate URL naming from payload execution

Compile each download into its network URL, local basename, digest, and handling strategy. Accept checked Scoop filename fragments; strip them from the network request. When a query could affect upstream filename heuristics, use an explicit compiled name rather than allowing the query to select a path. Reject traversal, encoded separator ambiguity, device names, alternate streams, and duplicate destinations. Pair multiple downloads with their hashes and extraction declarations deterministically.

EXEs remain inert files unless their declared handling selects a supported archive reader. A misleading .7z suffix does not establish the content's format. Never execute an installer to obtain files.

### Own extraction instead of inheriting machine tools

Introduce a managed extraction unit callable by the adapted backend through the verified Portscoop runtime and a checked frozen request. It accepts only the expected Store staging paths and declared payloads. Use Go standard ZIP/tar/gzip readers and a pinned pure-Go 7z reader, keeping the existing no-CGO distribution model. The [sevenzip project's primary documentation](https://github.com/bodgit/sevenzip) establishes a pure-Go reader with member enumeration and 7z self-extracting archive support; pin an exact reviewed version and license in implementation. This does not establish support for arbitrary NSIS, Inno, or MSI packages.

Validate member paths, link types, case collisions, extraction selectors, and resource bounds in private staging before publishing files. Preserve supported extract_dir/extract_to semantics, including paired multi-download declarations. ZIP-format nupkg is a container, not an instruction to execute NuGet installation scripts. Unsupported actual formats fail clearly. No PATH fallback, ordinary Scoop helper, or user-installed extractor is used.

### Measure toward 70% without changing the meaning of support

Use the existing audit tool with pinned Main and Extras revisions, record per-bucket and combined totals, and count full install preflight including dependency closure. Add blocker combinations so priorities reflect interacting failures rather than summed field counts. Measure after each coherent capability checkpoint. Broader URL naming and archive support come before the next local-operation vocabulary because payload failures currently dominate.

If the included stages do not reach 70%, report the achieved fraction and remaining ranked opportunities, such as extraction-only MSI/Inno support and shortcut/bin alias collisions. Do not silently implement new unsafe semantics, label skipped required effects supported, or hold an otherwise complete tested change indefinitely for a numeric threshold. Those larger candidates need their own reviewed contract before implementation. The final report includes both the attempted target and the actual result.

## Risks / Trade-offs

- Bounded grammar covers fewer syntactic variants than PowerShell itself → add operation-level forms only with positive and rejection tests, and report unsupported constructs precisely.
- Optional import supplies hidden prerequisites → conservative read/write dependency checks and complete-hook rejection where independence cannot be established.
- Post-install persistence uses junctions → verify exact Store ownership and operate on checked physical paths, with phase-specific adversarial tests.
- Archive readers add dependency and resource risk → exact dependency pin, integrity checks, bounded extraction, malicious-member fixtures, and no arbitrary external helper execution.
- Static acceptance can rise without successful app execution → separate preflight metrics from representative real installs/launches and report actual-format failures.
- 70% may exceed safe coverage available in these stages → retain the target, publish the gap and prioritized blockers, never relax portability to meet it.

## Delivery and migration

Advance backend metadata and release contracts together after the action/extraction interface is finalized. Fresh Stores are acceptable. The effective memory decision `3e6c669:1`, checked against docs/setup.md and current verification, permits breaking Store compatibility; it does not permit bypassing integrity checks. The selective-skip principle recorded at `574089a:1` remains applicable, while that commit's app-specific code was reverted.

Apply in dependency-ordered, independently verified checkpoints. Keep each checkpoint coherent before proceeding to dependent work; use focused checks first and broad checks for integration. Within the existing commit authorization, validate messages through zmem and stage only explicit implementation paths, excluding active OpenSpec. No pushes are authorized. README stays product-focused; technical contracts, methodology, architecture, and evidence live under docs/.
