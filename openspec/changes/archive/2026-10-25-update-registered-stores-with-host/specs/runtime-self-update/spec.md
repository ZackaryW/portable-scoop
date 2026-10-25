# Spec Delta

## MODIFIED Requirements

### Requirement: Update only the invoked managed installation
Self-update SHALL target the supported managed Host or ready Store adjacent to the invoked management executable. It SHALL preserve that location, reject invalid adjacent state and `--env` retargeting, and ignore CWD and the Host default for update selection. A Host SHALL also include all registered Store identities; direct Store invocation SHALL remain local to that Store.

#### Scenario: Host has a default Store
- **WHEN** Host `psp self-update` runs with a registered default and other connected Stores
- **THEN** the Host and all registered Store identities are selected
- **AND** default selection does not restrict the update scope

#### Scenario: Store invocation from another directory
- **WHEN** a ready Store's management executable runs self-update from an unrelated CWD
- **THEN** that same Store location and identity are selected without Host registration

#### Scenario: Unsupported or ambiguous installation state
- **WHEN** adjacent Store metadata is invalid, a Host lacks a supported ownership inventory, or `--env` is supplied
- **THEN** self-update fails with a diagnostic before runtime replacement
- **AND** it does not fall back to another installation

### Requirement: Discover and pin a published release
The system SHALL resolve the latest stable release or an explicit `--version <tag>` from the configured official Portscoop repository. It SHALL pin the resolved release for all subsequent downloads, identify the supported platform, and reject known downgrades and unsupported versions. An uncomparable development version SHALL require an explicit target.

#### Scenario: Select the latest release
- **WHEN** self-update runs without an explicit version
- **THEN** a stable published release is resolved once and subsequent artifacts belong to that resolved tag
- **AND** a later change to the repository's latest release does not mix update artifacts

#### Scenario: Already current or newer
- **WHEN** automatic latest-release selection finds that the installed comparable version is equal to or newer than the candidate
- **THEN** that installation is reported current or newer without replacing its runtime files
- **AND** a Host continues checking registered Stores even when its own version is current

#### Scenario: Unsupported explicit target
- **WHEN** an explicit version is unpublished, a known downgrade, or unsupported for the current platform/update contract
- **THEN** the command reports that condition without replacing the installation

### Requirement: Preserve Host portability and registered Stores
A managed Host update SHALL replace only its owned distribution payload, including the matching source inventory, at the same installation location. It SHALL preserve its default Store child, unrelated user files, registrations, default selection, and persistent environment. Host control storage SHALL NOT make the Host appear to be a Store.

#### Scenario: Host owns a default Store
- **WHEN** a Host with a `store` child updates its distribution
- **THEN** the Host's runtime and release inventory advance together
- **AND** the Store's identity, packages, and data remain unchanged
- **AND** its managed runtime can advance through its own Store transaction

#### Scenario: Host maintains future Store creation
- **WHEN** an updated Host initializes an additional Store
- **THEN** initialization uses its coherent updated source payload
- **AND** update control files are not copied into the new Store

### Requirement: Preserve Store data during compatible launcher updates
A directly invoked Store self-update SHALL replace only `portscoop.exe`, `psp.exe`, and `pspe.exe`, plus owned update receipts. It SHALL retain the Store UUID, backend, apps, shims, configuration, portable path state, caches, buckets, and persistence. Candidate compatibility with the retained backend SHALL be established before replacement; backend replacement belongs to the Host-managed Store update contract.

#### Scenario: Compatible Store launcher update
- **WHEN** the candidate supports the Store's retained backend contract
- **THEN** all three launchers advance as one owned transaction
- **AND** Store identity and package data remain unchanged

#### Scenario: Candidate requires a backend migration
- **WHEN** the candidate cannot run with the Store's existing backend contract
- **THEN** self-update reports that a migration is required before changing installed launchers
- **AND** it does not reinitialize or replace the Store backend
- **AND** it directs the user to a current Host for managed backend replacement

### Requirement: Separate Windows handoff from update completion
Replacement of the invoking installation SHALL use an owned Windows worker that can finish after the launcher exits. Host-managed Store transactions SHALL complete under independent Store locks before Host handoff. Before handoff, failures or cancellation SHALL clean up uncommitted startup. An accepted handoff SHALL identify its transaction and report pending completion, not completed success. The worker SHALL NOT require the launcher's terminal handles or silently retry uncertain handoffs.

#### Scenario: Worker accepts an update
- **WHEN** staged content and the private worker handshake are accepted
- **THEN** the invoking command reports accepted handoff and how to inspect its status
- **AND** the worker waits for the invoking process to exit before replacing its loaded runtime files

#### Scenario: Independent worker lifetime is unavailable
- **WHEN** an inherited job or another startup condition prevents the worker from outliving the launcher
- **THEN** the command reports the condition before accepting replacement ownership
- **AND** it does not weaken external job policies

#### Scenario: Handoff acknowledgement is lost
- **WHEN** the launcher cannot determine whether the worker accepted ownership
- **THEN** it reports the uncertain transaction and does not start a duplicate worker automatically

## ADDED Requirements

### Requirement: Update registered Stores independently
Host maintenance SHALL resolve registered identities using bounded portable discovery, update each unambiguous reachable identity once, and report offline, invalid, ambiguous, busy, newer and failed entries. A per-Store failure SHALL NOT stop other Stores or the Host. Registrations and defaults SHALL remain unchanged. All updates in one invocation SHALL use one pinned release.

#### Scenario: Duplicate aliases and stale registrations
- **WHEN** aliases share a valid Store UUID and another registration points at a different UUID
- **THEN** the valid identity is updated once and the mismatched entry is reported without adoption or registry cleanup

#### Scenario: Disconnected or busy Store
- **WHEN** one registered Store is unavailable or cannot obtain its locks within the existing bounds
- **THEN** the report identifies that Store and continues with other targets
- **AND** the command returns an incomplete outcome rather than claiming every Store was updated

#### Scenario: Moved drive or duplicate identity
- **WHEN** registered hints resolve to a moved Store or multiple copies of one identity
- **THEN** bounded discovery selects the unique identity or reports ambiguity without mutating either copy

### Requirement: Replace only managed Store backend artifacts
A Host-managed Store update SHALL replace launchers, runtime receipt, backend metadata and inventory-owned adapter/engine files in one recoverable transaction. It SHALL validate current ownership and candidate hashes, reject unsupported metadata or files outside the managed backend namespace, and preserve Store identity and all package/user state. It SHALL NOT run backend migration scripts.

#### Scenario: Upgrade an older managed backend
- **WHEN** a registered Store has an older supported metadata schema and backend revision
- **THEN** the Host upgrades its managed backend and three launchers together
- **AND** UUID, apps, shims, buckets, cache, persistence, config and portable path state remain unchanged

#### Scenario: Ownership drift or collision
- **WHEN** an owned backend artifact is changed, redirected, outside the allowed namespace, or a new artifact collides with user data
- **THEN** the Store update is rejected without replacing its runtime or user data

#### Scenario: Failure after partial replacement
- **WHEN** a Store transaction fails or is interrupted after replacement starts
- **THEN** verified backups restore the previous launchers and backend together or retain explicit recovery-required state
- **AND** no downloaded candidate executable is used to perform the update

### Requirement: Report and recover Host maintenance per installation
Host check, status and recovery SHALL include registered Stores and identify each target by alias, identity and location. Checks SHALL remain read-only. Status SHALL report each independent transaction; recovery SHALL act only on recorded pending transactions. Partial outcomes SHALL return 2, active-only pending status SHALL return 3, and complete/no-op outcomes SHALL return 0. Cancellation SHALL stop further targets.

#### Scenario: Host is already current
- **WHEN** a current Host checks or updates with an older registered Store
- **THEN** the Store's current and candidate versions are reported and a normal update upgrades that Store

#### Scenario: Recover a Store without a pending Host update
- **WHEN** Host recovery encounters a pending Store transaction and no pending Host transaction
- **THEN** it restores the Store's recorded previous runtime and reports the Host as having no pending transaction

#### Scenario: Inspect mixed outcomes
- **WHEN** some Store updates completed, one entry was offline and Host replacement was accepted
- **THEN** output distinguishes completed Stores, the unavailable entry and pending Host completion
