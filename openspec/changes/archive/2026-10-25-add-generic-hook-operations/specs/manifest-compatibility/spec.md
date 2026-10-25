# Spec Delta

## MODIFIED Requirements

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

## ADDED Requirements

### Requirement: Resolve download names deterministically
The system SHALL freeze the network URL, safe local filename, payload handling, and expected digest for each download before mutation. Supported Scoop filename fragments SHALL designate a checked local filename rather than a host path. Queries SHALL NOT independently override that filename. Conflicting destinations, unsafe decoded names, and unsupported content SHALL fail explicitly.

#### Scenario: Explicit filename fragment
- **WHEN** an HTTP(S) URL ends in a supported fragment such as #/tool.exe or #/tool.zip
- **THEN** the fragment is excluded from the network request and its checked basename determines the local filename and declared handling

#### Scenario: Traversal or duplicate destination
- **WHEN** a filename fragment escapes the package directory or two downloads ambiguously target the same file
- **THEN** preflight rejects the download plan

### Requirement: Extract supported archives within the Store
The system SHALL extract supported ZIP-family, 7z, and tar/gzip payloads through managed readers into Store-owned staging. It SHALL reject traversal, absolute paths, alternate data streams, reserved names, ambiguous case collisions, and unapproved links before publishing extracted files. Declared extraction paths SHALL remain checked and ordered with downloads. Extraction SHALL NOT run an installer or use ordinary Scoop helpers.

#### Scenario: Portable archive formats
- **WHEN** a verified supported archive, including a ZIP-format nupkg or a tar.gz payload, has safe entries and matching extraction declarations
- **THEN** its files are placed within the expected package layout

#### Scenario: Filename does not establish format
- **WHEN** a URL names an installer or another unsupported payload as dl.7z
- **THEN** unsupported content fails explicitly rather than being executed or falsely reported installed

#### Scenario: Malicious member
- **WHEN** an archive contains a member that could write outside staging or redirect a later member
- **THEN** extraction fails without publishing that archive's files or writing outside the Store

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
