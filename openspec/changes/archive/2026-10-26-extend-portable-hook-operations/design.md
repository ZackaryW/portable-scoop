# Design

## Context

See `proposal.md` for motivation and the four capability deltas for behavior. Investigation at `117771f` found:

- `internal/compatibility/hooks` already separates parsing, operand checks, operation recognizers, and a frozen typed plan. Its lexer rejects backticks and comma-separated quoted expressions; its executor supports single-file defaults but not recursive copies or cleanup.
- `backend/scoop/adapter/system.ps1` validates physical version paths and the exact post-install current/persistence mappings. Those checks must remain authoritative as operations grow.
- `internal/archive` supplies bounded staged extraction using standard-library readers and `bodgit/sevenzip`. `#/dl.7z` already permits ZIP signature dispatch; an NSIS marker was observed in the first MiB of PotPlayer 261021 x64. Full payload extraction has not been qualified.
- `Get-HelperPath` currently selects Program Files Git; `Initializer.Init` publishes an empty bucket directory. Backend verification and self-update own only the engine and adapter namespaces.
- Current Main Git and 7-Zip manifests contain effects beyond this change's hook grammar; the x64 7-Zip manifest uses MSI. Bootstrapping by invoking these manifests unchanged would recreate the compatibility and extraction dependency cycle.

The user approved pinned Store-managed native extraction for NSIS only, then requested automatic Main/Git/7-Zip bootstrap. This supersedes the pure-Go-only restriction recalled as valid in zmem `6353557:1` only for this narrow managed component. Complete syntax/effect checking and prohibition of arbitrary helpers remain in force.

## Goals / Non-Goals

**Goals:** Make a fresh Store independently usable; keep helper ownership explicit; compile bounded hooks into reviewable operations; preserve lifecycle/path checks; qualify actual downloads separately from static manifest acceptance.

**Non-Goals:** General PowerShell interpretation, native extraction for arbitrary formats, executing NSIS/MSI installers, unrestricted Git commands through package management, or application-specific hook handlers. This change does not implement the full ordinary Main Git/7-Zip manifests. Managed tools are identified as runtime components, not falsely reported as those packages.

## Decisions

### 1. Bootstrap trusted tools before Main

Ship pinned portable Git and full 7-Zip with the release's managed backend. The distribution contains normalized tool files plus a version/source/digest inventory and notices. Provision them to `.portscoop/tools/<tool>/...`; they have no global installation or registry setup. Add an explicit closed mapping from release `backend/tools/` to that namespace in provisioning, backend verification, and self-update. Inventory every loadable dependency; reject extra executable/plugin artifacts. Do not use `7za` or `7zr` as substitutes for the NSIS-capable full distribution.

Release preparation obtains fixed upstream artifacts or builds from pinned sources, validates SHA-256, and prepares the normalized tool bundle without running installers. The first implementation checkpoint must demonstrate this recipe and NSIS support in the disposable VM before downstream extraction work. A failed probe blocks that integration; it does not authorize a host-tool fallback. Record exact versions, source revisions, hashes, and redistribution materials during that checkpoint rather than inventing hashes in this plan.

Once tools exist in the initialization stage, use the private Git to clone the official Main repository at a release-recorded revision. This removes the Git/Main dependency cycle. The resulting Main is an ordinary Git bucket which users can explicitly update later; self-update never resets it. Initial setup needs network access for Main unless a verified cached acquisition is available, and reports acquisition failure before readiness.

New direct Stores and requested Host default Stores share this path. Host-only installation creates no Store. Add `psp env bootstrap` using existing Store selection/locking: on an updated older Store it verifies runtime-provisioned helpers and installs an absent official Main, reports versions, and is idempotent. A differently sourced Main or missing or modified inventoried tool is a conflict, not permission to overwrite. Stage Main, validate before publication, retain a small owned bootstrap receipt so interrupted work is distinguishable from user content, and preserve the previous usable state on failure. Existing initialization publication/readiness semantics remain in place.

The existing Host `env bootstrap` first-Store behavior remains: without an explicit selector it preserves a reachable default, or uses the existing missing-default recovery rules to create the Host's Store. The extended command then prepares the selected Store's Main. A direct Store or explicit `--env` selector never creates a replacement Store. Compatible updated backends receive complete helpers through runtime provisioning; missing or altered inventoried helper files are repair-required conflicts, not unverified download candidates.

Managed tools update through Host-coordinated `psp self-update` with the backend. Direct Store self-update keeps its existing launcher-only contract: it retains verified helpers and rejects a candidate requiring a backend upgrade, directing the user to Host maintenance. Their status belongs in bootstrap output and normal Store inspection, clearly marked managed; they are not uninstallable via ordinary app commands. An independently installed application named git or 7zip does not satisfy or replace this private dependency. This preserves the previously approved pinning/ownership contract while delivering automatic tool availability.

### 2. Resolve Git from the Store and constrain its process context

Replace Program Files lookup with a closed helper registry. Each entry defines its relative executable path, version, inventory, and supported internal operation. Reuse the existing process runner where its contract fits; pass arguments as values, never through a command string. Keep `clone --no-hardlinks` for local sources.

For managed bucket operations, remove inherited Git repository/config/exec overrides, disable user/system Git configuration and credential helpers, and supply Store-local empty hooks/templates and process-local configuration. Keep helper executable/DLL search rooted in the verified distribution and Windows system dependencies. Do not alter caller environment, user Git settings, or ordinary Scoop. Existing HTTP(S)/explicit-local bucket source rules remain; adding SSH/authentication workflows is not implied.

### 3. Extend a bounded compiler, not a script runtime

Keep the parser and binding resolver inside `internal/compatibility/hooks`; they are domain code, not generic utilities. Introduce typed literal string segments so an escaped dollar sign stays literal through expansion. Initially admit backtick-dollar, backtick-backtick, and backtick-double-quote inside double quotes; reject other escapes and command substitution. Preserve the existing single-quote rules.

Add comma-separated literal lists, multiline `@(...)` string arrays, single-assignment local bindings, and one literal-list `ForEach-Object` block. A loop body may bind local checked paths with positional or named `Join-Path`, read `$_`, contain supported guards and operations, and emit literal/interpolated informational messages. Its variables are scoped to the iteration; predefined roots cannot be reassigned. Resolve paths as root-tagged values rather than concatenated absolute strings. Unsupported conditions, nested loops, dynamic collections, method calls, property access, aliases, and unknown commands remain rejected.

Retain the 64 KiB hook source and depth bounds; initially cap literal collections at 256 elements, expanded actions at 4096, and aggregate literal output at 1 MiB per hook. Bound compilation work before allocation/expansion. Preserve original phase, operation, source line, and iteration identity in diagnostics. Compile all iterations before returning a plan; one rejected construct discards all executable actions. Extend cross-phase read/skip dependency accounting to expanded actions and their tree regions.

Literal line arrays feed the existing default-file action with explicit CRLF/encoding semantics. Guard evaluation remains at execution time; an existing persisted INI file is preserved. Informational statements are recognized as diagnostic-only operations, never a reason to skip an enclosing hook.

### 4. Keep each filesystem operation independent

Add distinct recognizers and typed action contracts for cleanup and recursive copy; extend the literal-default unit for line arrays. Do not recognize a whole PotPlayer script. Translate assignments and loops before the operation registry so unrelated applications use identical operation contracts.

- **Cleanup:** literal path list or a single terminal basename wildcard with a nonempty literal prefix (such as dollar-star or `Uninst*`). No wildcard in ancestors, root-wide star, bracket expression, or variable-derived pattern. Resolve a deterministic match set under the physical version directory, reject declared persistence overlap and reparse points in every selected descendant, then delete only that set. An explicit missing-path-tolerant form permits absent matches but does not suppress permission or validation failures. Recursive deletion is allowed only when requested and fully checked.
- **Tree copy:** source is a checked package-local directory's contents; destination is a package path or exact declared persistence mapping. Prevalidate the entire tree and destination conflicts. Supported `-Force -Recurse` replaces colliding regular files, preserves unrelated destination files, and rejects file/directory conflicts. Write through temporary files and replace entries rather than truncating existing hard links. Do not silently change force-copy into preserve-existing defaults.
- **Copy then cleanup:** keep separate actions in source order. Failure stops the sequence before cleanup. Validation failures cause no action writes; execution failures may leave completed individual file replacements. Do not promise whole-tree transactions or suppress failure with `SilentlyContinue`.

Post-install persistence junctions are verified once against the manifest and resolved to physical persisted locations; operations never follow arbitrary descendant links. Default INI preservation is independent of resource refresh collisions. The Store lock serializes supported operations; this is not protection against a malicious concurrent local filesystem writer.

### 5. Reusable helper contracts and integration

Keep helpers local in an adapter filesystem module loaded by the pinned backend. They accept ordinary roots, relative paths, limits, and options; they must not know app names, manifest fields, or Store layout. Domain-neutral fixtures can model another program refreshing a cache tree.

| Helper | Contract and effects | Reuse and verification |
| --- | --- | --- |
| Bounded tree enumeration | `(root, relative selections, limits) -> ordered entries`; reads metadata, rejects links/type ambiguity/escape, no writes | Reuse .NET filesystem APIs and existing ancestor checks. Test missing matches, literal dollar names, nested links, case collisions, depth/count/size limits. Raw `Get-ChildItem` wildcard traversal does not supply this contract. |
| Checked tree copy | `(sourceRoot, destinationRoot, entries, collisionPolicy) -> completion/error`; prevalidates then copies with per-file atomic replacement; no whole-tree rollback | Extend existing temporary-file replacement mechanics. Test nested resources, collisions, hard links, type conflicts, write failure, and unchanged outside sentinels. Caller authorizes persistence destinations. |
| Checked cleanup | `(root, validated selections) -> completion/error`; revalidates entries before deletion, never follows links; missing policy explicit | Reuse bounded enumeration. Test recursive removal, forbidden target preservation, missing entries, access failures, and no cleanup following failed copy. Caller excludes persisted regions. |

Do not add a generic filesystem framework or duplicate the archive staging writer. The action executor supplies lifecycle identity, approved mappings, and limits; helpers supply filesystem mechanics. Verify each helper before integrating the corresponding recognizer. Use explicit `-LiteralPath` or direct .NET APIs throughout.

### 6. NSIS uses an extraction-only adapter

Keep ZIP/7z/tar/gzip in their existing Go readers. Extend content dispatch for archive-hinted verified payloads to recognize supported NSIS, and force the native handler to NSIS mode; never retry arbitrary formats automatically. Direct EXE downloads retain their existing inert-file behavior.

The NSIS adapter lists members with the pinned full 7-Zip and parses a bounded complete inventory. Reject ambiguous records, case-only path collisions, links, and invalid paths. Exact repeated names with known member sizes use archive-order last-entry-wins semantics: exact-name stdout concatenates the entries, and the reader validates and consumes every declared segment before retaining the last. Reject a repeated-name group with unknown boundaries. A single unknown-size member is permitted with a streaming member-byte cap, aggregate-byte accounting, successful process exit, and no publication until completion. These are default NSIS compatibility semantics, not an unsafe opt-in; the ZIP and other existing readers retain their existing collision rules.

Extract validated names to stdout with wildcard interpretation disabled; Go owns the confined staged file writes and verifies known byte counts. Never give the native process an arbitrary filesystem output destination. Capture diagnostics separately with a byte cap, terminate the process tree on cancellation/timeout, and check exit status for every invocation. Enforce existing archive limits (100,000 entries, 8 GiB total, 4 GiB per member), counting discarded duplicate segments too, plus a bounded inventory/diagnostic size and an overall extraction deadline. Native parsing consumes resources before streaming; constrain memory with a Windows Job Object and treat exceeded limits as failure, not a parser bypass.

This exact-name/stdout inventory contract must be demonstrated against the pinned tool and NSIS fixtures in checkpoint 1. If metadata or selection is ambiguous, reject it. Do not substitute unrestricted `7z x -o...` as a workaround. Stage every payload and publish through the existing validation path only after successful extraction.

The first VM probe found 206 PotPlayer entries, five exact-name duplicate groups, and one unknown-size uninstaller. All six exceptional-name streams completed; known groups matched the sum of listed sizes. This evidence replaces the initial reject-all-duplicates/unknown-size proposal. Broader compatible defaults follow the user's direction while maintaining confinement, integrity, and complete-hook checks. Raising resource limits or a general bypass switch is not part of this change.

Separate filesystem name validation from script operand validation: a literal dollar sign in an archive member is legal filename data, while unresolved script interpolation remains invalid. Carry literal provenance to cleanup operands, and do not globally weaken path validation. Keep traversal, device names, ADS, control characters, and reparse checks.

### 7. Evidence and qualification

Use unrelated synthetic manifests for grammar and operation tests. Include mutations of the PotPlayer fixture (renamed app/config/resources, invalid loop member, trailing command, and malicious paths) to prove generic classification. Use real filesystem fixtures for the helper contracts and both lifecycle layouts. PowerShell fixture changes require uncached Go wrapper runs (`-count=1`).

The disposable Windows VM qualifies blank setup without host Git/7-Zip, automatic Main, adding Extras, unchanged PotPlayer x64 install, shim launch, reinstall/update with custom INI and resource sentinels, relocation, two independent Stores, and complete helper self-update/recovery. Pin manifest revision, payload URL/digest, runtime version, and observed outcome. A header probe or static acceptance count is not installation evidence; do not count a reinstall as proof of a new-version transition.

Primary research: [Scoop installer](https://github.com/ScoopInstaller/Install/blob/master/install.ps1) bootstraps Main with a Git or ZIP path; [Main Git](https://github.com/ScoopInstaller/Main/blob/master/bucket/git.json) and [Main 7-Zip](https://github.com/ScoopInstaller/Main/blob/master/bucket/7zip.json) explain why unchanged package hooks are unsuitable for the trusted seed. [bodgit/sevenzip](https://github.com/bodgit/sevenzip) is a Go 7z reader, not an NSIS implementation. The inspected [NSISExtractor](https://github.com/KokerZhou/NSISExtractor) requires Python and describes an early-stage interface; it is not a suitable direct dependency for this runtime. Obtain full-tool artifacts and notices from [7-Zip upstream](https://www.7-zip.org/download.html) and [Git for Windows](https://github.com/git-for-windows/git/releases).

## Risks / Trade-offs

- Larger release and native dependency maintenance -> pin complete tools, include notices/source obligations, keep updates in the verified runtime inventory.
- Native parser defects or expensive solid extraction -> bounded process, memory/output/time limits, exact fixture qualification, and fail closed on unsupported layouts; no sandbox claim.
- Git configuration can launch external programs -> isolate managed invocation configuration and exercise hostile host configuration fixtures.
- Force-copy can replace user-edited resources -> preserve upstream supported semantics explicitly; test guarded settings separately and never describe all persisted files as immutable.
- More grammar can accidentally admit executable expressions -> typed constants, finite limits, rejection fixtures, and no PowerShell evaluation.
- Bootstrap network failure or partial publication -> readiness only after complete bootstrap, explicit failed-stage reporting, owned staging, and repeat-run recovery tests.

## Migration Plan

Introduce helpers and operation primitives behind internal APIs first; verify them before compiler/backend integration. Bump the managed backend patch revision and all matching descriptor checks together when activating the new contracts; older unmatched backends must reject the plan.

New releases provision full helpers in every new Store. Existing Stores receive helper inventory through Host-managed complete self-update, then use `psp env bootstrap` to add Main if needed; runtime update preserves existing buckets. Bootstrap is not an alternate backend migration command and requires a compatible installed runtime and verified source inventory before provisioning any missing component. Include the new helper namespace in backup, replacement, rollback, drift checks, and relocation. Preserve the prior valid runtime on a failed update. Explicit bootstrap never overwrites altered owned files under the guise of repair.

At meaningful implementation checkpoints, review and validate focused zmem commit messages; record the approved native exception and bootstrap ownership decision without cancelling the remaining portability constraints. Keep active OpenSpec artifacts out of implementation commits. Product docs describe setup and supported behavior; detailed investigation history and lessons belong in zmem. A release/version push remains separately authorized.
