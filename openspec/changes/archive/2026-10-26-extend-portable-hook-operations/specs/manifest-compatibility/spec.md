## MODIFIED Requirements

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

## ADDED Requirements

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
