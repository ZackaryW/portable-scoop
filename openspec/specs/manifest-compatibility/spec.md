# manifest-compatibility Specification

## Purpose

Translate Scoop manifest declarations into checked portable behavior, with a separate compatibility decision for each field before any package mutation.

## Requirements

### Requirement: Report field compatibility deterministically
The system SHALL classify each effective manifest field as use, rewrite, skip, or reject. It SHALL reject unknown fields and report rejected fields in deterministic order. A rejected manifest SHALL produce no executable plan.

#### Scenario: Multiple unsupported fields
- **WHEN** a manifest contains an unknown key and an unsupported installer
- **THEN** both field failures are reported in stable order and execution is blocked

#### Scenario: Optional integration is replaced
- **WHEN** a shortcut can become a portable launch alias
- **THEN** the report identifies the rewrite and explains that host Start Menu integration is omitted

### Requirement: Inspect only selected architecture effects
The system SHALL structurally validate architecture declarations, select the Windows x64 branch, and inspect its merged effects. Unselected branch scripts SHALL NOT execute or block a valid selected branch merely because their effects are unsupported.

#### Scenario: ARM branch contains a script
- **WHEN** a valid x64 manifest also has an ARM-only script
- **THEN** the x64 operation uses only the effective x64 fields

### Requirement: Freeze portable replacements
The system SHALL freeze approved declarations and ordered typed replacement actions for the full dependency closure before mutation. Replacement actions SHALL NOT evaluate original script text. Saved manifests SHALL be rechecked before operations that consume them. Only actions approved for the invoked lifecycle phase SHALL execute.

#### Scenario: Bucket changes after preflight
- **WHEN** a bucket script changes after preflight
- **THEN** execution uses the frozen approved action without rereading that script

#### Scenario: Script has an unrecognized remainder
- **WHEN** an otherwise recognized initialization hook contains additional unrecognized statements
- **THEN** preflight rejects the whole hook

#### Scenario: Multiple approved operations
- **WHEN** a hook contains multiple supported required operations and an optional operation approved for skipping
- **THEN** required actions execute in source order, and only the approved optional operation is omitted

#### Scenario: Phase mismatch
- **WHEN** an action compiled for pre-install is presented for post-install execution
- **THEN** execution rejects the mismatch rather than executing or silently omitting the action

### Requirement: Initialize persisted configuration portably
The system SHALL recognize the supported operation that creates an empty package configuration file when its matching persisted file is absent, alone or composed with other supported operations. It SHALL create only the checked package-relative file, preserve existing persisted content, and reject paths escaping the Store or package through reparse points. Pre-install and post-install handling SHALL respect their different persistence layouts.

#### Scenario: SpaceSniffer first installation
- **WHEN** its recognized hook is present and no persisted configuration exists
- **THEN** the replacement creates the package configuration file before persistence is linked

#### Scenario: Reinstallation with saved configuration
- **WHEN** a persisted configuration file already exists
- **THEN** its contents remain unchanged

#### Scenario: Post-install initialization
- **WHEN** a supported configuration initialization occurs after Store persistence is linked
- **THEN** the replacement preserves the existing persisted file and accepts only the verified Store-owned persistence mapping

### Requirement: Support declared download digests
The system SHALL validate SHA-1, SHA-256, SHA-512, and MD5 manifest digests with exact algorithm-specific lengths and verify downloads through the managed backend. Release and backend artifact integrity SHALL remain SHA-256-only. Supported payloads SHALL include checked HTTP(S) ZIP-family, 7z, tar/gzip archives and direct EXE files. Downloads SHALL be verified before extraction or retention; installers SHALL NOT execute.

#### Scenario: SHA-1 ZIP download
- **WHEN** a manifest declares a valid SHA-1 digest for a ZIP
- **THEN** preflight accepts that digest and a content mismatch fails installation

#### Scenario: Direct executable payload
- **WHEN** a manifest declares a direct EXE URL with a safe literal filename and no extraction directives or installer
- **THEN** the backend verifies the digest and retains the file without executing it during installation

#### Scenario: Ambiguous executable URL
- **WHEN** a URL has a query, fragment, or encoded path that makes its downloaded filename ambiguous under the supported rules
- **THEN** preflight rejects it

### Requirement: Provide Store launch aliases
The system SHALL rewrite supported shortcut entries into Store-owned shims without host shortcuts. It SHALL deduplicate identical aliases, reject conflicting targets or arguments, and prepend literal fixed native-shim arguments to caller arguments. Unsupported shortcut options or script-wrapper arguments SHALL fail explicitly.

#### Scenario: Shortcut-only package
- **WHEN** a package declares a supported shortcut and no bin
- **THEN** a deterministic logical alias is installed and can be resolved by pspe

#### Scenario: Native arguments and relocation
- **WHEN** a native alias with fixed literal arguments is launched after Store relocation
- **THEN** the relocated target receives fixed arguments followed by caller arguments

### Requirement: Reject required effects without an implementation
The system SHALL reject unsupported installers, unrecognized lifecycle hooks, and module integration rather than silently omit required behavior. Metadata that is not used during installation SHALL be marked skipped without executing its contents.

#### Scenario: Module metadata is declared
- **WHEN** a manifest requires PowerShell module integration
- **THEN** preflight reports that portable module discovery is not yet implemented

### Requirement: Resolve download names deterministically
The system SHALL freeze the network URL, safe local filename, payload handling, and expected digest for each download before mutation. Supported Scoop filename fragments SHALL designate a checked local filename rather than a host path. Queries SHALL NOT independently override that filename. Conflicting destinations, unsafe decoded names, and unsupported content SHALL fail explicitly.

#### Scenario: Explicit filename fragment
- **WHEN** an HTTP(S) URL ends in a supported fragment such as #/tool.exe or #/tool.zip
- **THEN** the fragment is excluded from the network request and its checked basename determines the local filename and declared handling

#### Scenario: Traversal or duplicate destination
- **WHEN** a filename fragment escapes the package directory or two downloads ambiguously target the same file
- **THEN** preflight rejects the download plan

### Requirement: Extract supported archives within the Store
The system SHALL extract supported ZIP-family, 7z, tar/gzip, and NSIS payloads through managed extraction into Store-owned staging. It SHALL reject traversal, absolute paths, alternate data streams, reserved names, ambiguous case collisions, and unapproved links before publishing extracted files. Declared extraction paths SHALL remain checked and ordered with downloads. Extraction SHALL NOT run an installer or use ordinary Scoop helpers.

#### Scenario: Portable archive formats
- **WHEN** a verified supported archive, including a ZIP-format nupkg or a tar.gz payload, has safe entries and matching extraction declarations
- **THEN** its files are placed within the expected package layout

#### Scenario: Filename does not establish format
- **WHEN** a URL names an unsupported installer or another unsupported payload as dl.7z
- **THEN** unsupported content fails explicitly rather than being executed or falsely reported installed

#### Scenario: Malicious member
- **WHEN** an archive contains a member that could write outside staging or redirect a later member
- **THEN** extraction fails without publishing that archive's files or writing outside the Store

#### Scenario: Supported NSIS behind an archive hint
- **WHEN** a digest-verified payload declared with a supported archive hint is identified as supported NSIS content
- **THEN** only its validated file payload is extracted with the managed NSIS capability
- **AND** installer scripts, callbacks, and executable payloads are never run during extraction

### Requirement: Perform generic local copy and default initialization
The system SHALL recognize bounded package-local copy, directory creation, and literal default-file initialization operations independently of app identity. Sources and destinations SHALL be checked against package and declared persistence ownership. Existing persisted defaults SHALL be preserved, and runtime-dependent scripts or arbitrary command output SHALL NOT become literal content.

#### Scenario: Copy a packaged default
- **WHEN** a recognized operation copies a checked packaged default to a declared configuration location only if persisted configuration is absent
- **THEN** the default is created locally without overwriting existing persisted content

#### Scenario: Dynamic content
- **WHEN** a purported default uses command substitution or reads an unapproved host path
- **THEN** the operation is rejected before mutation

### Requirement: Classify hook operations independently of application identity
The system SHALL classify supported pre-install and post-install operations by their syntax, paths, conditions, and effects. The same operation SHALL receive the same decision across unrelated packages. Matching SHALL NOT depend on application names, vendor names, or an entire application's hook text. Every executable construct SHALL be accounted for before any hook plan is accepted.

#### Scenario: Unrelated packages use the same operation
- **WHEN** two unrelated manifests use equivalent checked renames or optional-import constructs with different filenames and profile names
- **THEN** both receive equivalent operation decisions without adding application-specific production handlers

#### Scenario: Supported operations in either phase
- **WHEN** a pre-install or post-install hook contains only operations supported in that phase and layout
- **THEN** the compiler produces an ordered phase-specific plan rather than rejecting or skipping the lifecycle category wholesale

#### Scenario: Unsupported syntax remains
- **WHEN** a hook includes an unrecognized command, substitution, branch, or trailing executable statement
- **THEN** the entire manifest has no executable plan, including previously recognized actions

### Requirement: Skip only proven optional host-data imports
The system SHALL omit a host-AppData import only when a recognized guarded construct makes the import optional, confines its destination to declared Store persistence, and has no required operation dependent on the imported data. Classification SHALL NOT inspect the actual host profile. Required local work in the same hook SHALL remain intact. An AppData reference alone SHALL NOT authorize omission.

#### Scenario: Optional directory import
- **WHEN** a recognized import copies from a checked host profile directory only if that directory exists and the declared portable profile is empty
- **THEN** the import is reported skipped without reading or copying host data, and independent required local operations remain in the plan

#### Scenario: Required consumer of skipped data
- **WHEN** a required operation would consume files supplied by an otherwise optional host import
- **THEN** preflight rejects the unsafe omission before any package mutation

#### Scenario: Unconditional import or host mutation
- **WHEN** an import is unconditional, has an unsupported destination, deletes host data, or includes unrecognized side effects
- **THEN** it is rejected rather than silently filtered out

### Requirement: Explain individual hook decisions
The system SHALL identify the lifecycle field, operation position, disposition, and reason for each accepted rewrite or skip and each rejected operation. A mixed hook's field-level outcome SHALL NOT conceal which operation was omitted. An entirely skipped supported hook SHALL require no executable action.

#### Scenario: Rename and optional import
- **WHEN** a hook contains a checked rename followed by an optional import
- **THEN** diagnostics distinguish the rename rewrite from the import skip

#### Scenario: Only optional imports
- **WHEN** all executable constructs in a hook are recognized optional imports safe to omit
- **THEN** diagnostics explain each skip and the backend does not evaluate the hook or require a dummy action

### Requirement: Confine package rename actions
The system SHALL rename only checked regular package files within the expected physical Store version directory. It SHALL reject missing sources, existing destinations, invalid path components, and unapproved reparse paths. Only the verified lifecycle current-directory alias MAY be resolved to that physical directory; actions SHALL NOT follow arbitrary links or modify declared persisted paths. Existing destination bytes SHALL never be overwritten.

#### Scenario: Ordered rename chain
- **WHEN** a supported hook renames one package file and then renames that result
- **THEN** both actions run in order with unchanged file content

#### Scenario: Unsafe path or existing destination
- **WHEN** either rename path escapes the expected package location or the destination already exists
- **THEN** execution rejects the rename without overwriting the destination

### Requirement: Compile bounded literal hook collections
The system SHALL accept supported literal arrays, immutable local bindings, checked path joins, and finite literal-list iteration in hooks without script evaluation. Expansion SHALL preserve source order and literal character provenance, enforce explicit size limits, and resolve every construct before producing executable actions. Unknown syntax or an invalid iteration SHALL reject the entire plan.

#### Scenario: Defaults for several configuration files
- **WHEN** a literal list iterates over filenames and initializes each missing persisted configuration from a literal line array
- **THEN** the plan contains the corresponding ordered guarded writes with the declared encoding and line endings
- **AND** existing persisted configurations remain unchanged

#### Scenario: Escaped dollar is literal
- **WHEN** a supported quoted path contains an escaped dollar sign followed by a permitted wildcard
- **THEN** the dollar sign remains filename data and is never interpreted as a variable or host path

#### Scenario: Unsupported operation in an iteration
- **WHEN** any iteration includes an unsupported command, substitution, binding, or destination
- **THEN** no executable plan is returned, including actions from earlier iterations

#### Scenario: Expansion limit
- **WHEN** a collection or expanded action sequence exceeds the documented compilation limit
- **THEN** preflight rejects it with a limit diagnostic before package mutation

### Requirement: Confine package cleanup operations
The system SHALL support checked literal path lists and bounded basename wildcard cleanup inside the physical package directory. Cleanup SHALL reject persistence overlap, reparse traversal, and paths outside that directory. It SHALL validate the complete selected tree before deleting entries. An approved missing-path policy SHALL NOT suppress containment, access, or execution errors.

#### Scenario: Remove installer debris
- **WHEN** a supported cleanup selects package-local dollar-prefixed entries and uninstaller-prefixed entries
- **THEN** only matching confined entries and their checked descendants are removed
- **AND** unrelated package files, persisted data, and host files remain unchanged

#### Scenario: Cleanup reaches a junction or persisted path
- **WHEN** a selected cleanup entry overlaps declared persistence or contains an unapproved reparse point
- **THEN** cleanup fails before deleting any selected entry

#### Scenario: No matches
- **WHEN** an approved missing-path-tolerant cleanup has no matches
- **THEN** it succeeds without deleting anything

### Requirement: Copy packaged resource trees with explicit collision semantics
The system SHALL support checked recursive copies from package-local resource trees into package paths or verified declared persistence mappings. Supported force-copy operations SHALL replace colliding regular files, preserve unrelated destination entries, and reject type conflicts or unapproved links. Source cleanup SHALL occur only after successful copying. Guarded default initialization SHALL retain its separate preserve-existing behavior.

#### Scenario: Merge packaged resources after persistence linking
- **WHEN** a supported post-install operation force-copies a packaged resource tree into a verified persisted directory
- **THEN** source files replace matching destination files and add missing files while unrelated destination files remain
- **AND** the source tree is removed only after the copy succeeds

#### Scenario: Copy fails
- **WHEN** a resource copy fails validation or execution
- **THEN** it reports failure and does not execute subsequent source cleanup or report installation success
- **AND** any completed writes are reported without claiming whole-tree rollback

#### Scenario: Guarded settings remain preserved
- **WHEN** a manifest both initializes missing settings and refreshes resource directories
- **THEN** existing guarded settings remain unchanged even when resource file collisions are overwritten

### Requirement: Bound native extraction before publication
Native NSIS extraction SHALL validate a complete unambiguous member inventory and enforce file-count, byte, execution-time, and output limits. Exact repeated names with known boundaries SHALL retain the last archive entry; single unknown-size entries SHALL use streaming byte limits. Case-only collisions, ambiguous boundaries, corruption, or exceeded limits SHALL fail without publication. Other installer formats SHALL NOT become implicitly supported.

#### Scenario: Excessive or ambiguous payload
- **WHEN** NSIS content exceeds a limit or contains case-only collisions or ambiguously selectable members
- **THEN** extraction fails without publishing files into the application directory

#### Scenario: Literal NSIS directory name
- **WHEN** a supported NSIS member uses a safe relative directory containing a literal dollar sign
- **THEN** it remains a literal confined path and cannot resolve to an environment variable

#### Scenario: Repeated exact-name resources
- **WHEN** the inventory contains multiple entries with an identical literal name and known byte boundaries
- **THEN** extraction validates every segment and retains the last entry in archive order without an opt-in bypass
- **AND** all streamed bytes, including discarded earlier segments, count against resource limits

#### Scenario: Unknown-size singleton
- **WHEN** an otherwise valid unique member has no listed size
- **THEN** extraction measures and limits its actual byte stream and publishes it only after successful completion

#### Scenario: Unknown-size duplicate
- **WHEN** a repeated-name group includes an unknown-size segment whose boundary cannot be determined
- **THEN** extraction rejects the ambiguity rather than guessing the winning bytes

#### Scenario: Other executable container
- **WHEN** a payload is an unsupported executable container despite an archive filename hint
- **THEN** it is rejected rather than sent to an unrestricted native extraction fallback
