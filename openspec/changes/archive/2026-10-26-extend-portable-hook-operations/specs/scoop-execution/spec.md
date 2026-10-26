## ADDED Requirements

### Requirement: Resolve managed helpers within the selected Store
The system SHALL resolve internal Git and NSIS extraction helpers from verified, versioned artifacts owned by the selected Store. It SHALL NOT discover replacements from host PATH, Program Files, another Store, or a user-installed package. Missing or altered helper artifacts SHALL block dependent operations with an actionable diagnostic. Helper availability SHALL NOT bypass manifest compatibility checks.

#### Scenario: Host has conflicting tools
- **WHEN** host PATH and another Store contain Git or 7-Zip executables
- **THEN** the operation uses only the selected Store's verified managed helper

#### Scenario: Helper was altered
- **WHEN** a managed executable, library, or other inventoried helper artifact differs from its recorded content
- **THEN** the dependent operation fails before invoking that helper and identifies the repair path

#### Scenario: Unsupported manifest with available tools
- **WHEN** Git and the extractor are present but a manifest requires an unsupported hook or installer action
- **THEN** preflight still rejects the unsupported effect

### Requirement: Isolate managed helper invocations
The system SHALL invoke managed helpers through fixed operation-specific entry points with validated arguments, working directories, and process-local configuration. Git bucket operations SHALL NOT consume host Git configuration, credential helpers, hooks, or templates. NSIS extraction SHALL NOT execute the payload, evaluate command text, or load unowned extractor extensions. Persistent host state SHALL remain unchanged.

#### Scenario: Host Git configuration injects behavior
- **WHEN** the calling user has Git configuration or environment overrides for hooks, credentials, templates, executable search, or repository paths
- **THEN** managed bucket operations use the Store's isolated configuration and approved paths instead
- **AND** the caller's configuration and environment remain unchanged

#### Scenario: Manifest attempts to supply extractor arguments
- **WHEN** manifest content could be interpreted as switches, wildcards, response files, or commands
- **THEN** it cannot change the managed extractor invocation contract

### Requirement: Maintain helpers as complete runtime components
Git and the NSIS extractor SHALL be provisioned, verified, and updated as complete managed runtime components, including required libraries and notices. Runtime updates SHALL preserve user packages, buckets, and persistence and SHALL recover helper components together with their matching backend. Tool versions and ownership SHALL be inspectable; ordinary package commands SHALL NOT mutate managed helper artifacts.

#### Scenario: Complete Store runtime update
- **WHEN** a Host-managed Store update installs a compatible runtime containing new helper versions
- **THEN** executable artifacts and matching metadata are updated together
- **AND** user package installations, buckets, and persisted data remain unchanged

#### Scenario: Interrupted helper replacement
- **WHEN** an update fails while replacing a managed helper
- **THEN** recovery restores a verified complete runtime or reports recovery failure without claiming readiness

#### Scenario: Ordinary Git package is installed
- **WHEN** the Store separately contains an ordinary Git or 7-Zip application package
- **THEN** that package cannot replace or satisfy the managed helper's integrity contract
