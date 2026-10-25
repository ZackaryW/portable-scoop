# runtime-self-update Specification

## Purpose

Let users update the invoked Portscoop installation from a verified published release while preserving portable environments and retaining explicit Windows completion and recovery state.

## Requirements

### Requirement: Keep self-update separate from package updates
The system SHALL expose equivalent `psp self-update` and `portscoop self-update` commands for Portscoop maintenance. `update <app>` SHALL remain package management, and `pspe` SHALL remain application execution. Self-update SHALL NOT dispatch a Scoop update command.

#### Scenario: Update Portscoop through either management name
- **WHEN** a user invokes `psp self-update` or `portscoop self-update`
- **THEN** the Portscoop runtime update workflow runs with equivalent selection, output, and status behavior
- **AND** no installed package is updated as part of that workflow

#### Scenario: Update an application
- **WHEN** a user invokes `psp update <app>`
- **THEN** the existing selected-Store package update behavior applies
- **AND** the command does not update Portscoop itself

### Requirement: Update only the invoked managed installation
Self-update SHALL target the supported managed Host or ready Store adjacent to the invoked management executable. It SHALL preserve that location, reject invalid adjacent state and `--env` retargeting, and ignore CWD and the Host default for update selection. Other installations SHALL NOT be updated implicitly.

#### Scenario: Host has a default Store
- **WHEN** Host `psp self-update` runs with a registered default and other connected Stores
- **THEN** only that Host installation is selected
- **AND** the default Store and other registered Stores are not updated

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
- **THEN** the command reports that no upgrade is needed and does not replace runtime files

#### Scenario: Unsupported explicit target
- **WHEN** an explicit version is unpublished, a known downgrade, or unsupported for the current platform/update contract
- **THEN** the command reports that condition without replacing the installation

### Requirement: Check availability without updating
`self-update --check` SHALL report the installed version, installation kind, and available candidate without changing runtime files, identity, registry, PATH, or update ownership state. It SHALL distinguish an available candidate from a completed compatibility check and SHALL NOT execute downloaded code.

#### Scenario: A newer release is available
- **WHEN** `psp self-update --check` resolves a newer release
- **THEN** the report identifies the current and candidate versions
- **AND** the installation remains unchanged

#### Scenario: Availability does not establish Store compatibility
- **WHEN** release metadata identifies a candidate but its Store update compatibility has not been verified
- **THEN** the report does not claim that the Store has been updated or that compatibility is confirmed

### Requirement: Verify and stage the complete candidate
The system SHALL verify the pinned archive checksum, extraction paths, release inventory, and supported update descriptor before accepting a replacement transaction. All three launchers SHALL be present and hashed, and the two management names SHALL remain identical. Staging SHALL use private owned storage and SHALL NOT overwrite unrelated files.

#### Scenario: Candidate is complete
- **WHEN** the archive and every required update artifact pass verification
- **THEN** an owned staged candidate is available for compatibility and replacement planning
- **AND** the installed runtime has not yet been replaced

#### Scenario: Candidate is incomplete or altered
- **WHEN** a checksum, required launcher, inventory digest, descriptor, or extraction path is invalid
- **THEN** self-update fails before launching a replacement worker or changing installed runtime files

#### Scenario: Candidate collides with user data
- **WHEN** a proposed destination is unowned, redirected, or conflicts with an unrelated existing file
- **THEN** the candidate is rejected and the pre-existing file is preserved

### Requirement: Preserve Host portability and registered Stores
A managed Host update SHALL replace only its owned distribution payload, including the matching source inventory, at the same installation location. It SHALL preserve its default Store child, unrelated user files, registrations, default selection, and persistent environment. Host control storage SHALL NOT make the Host appear to be a Store.

#### Scenario: Host owns a default Store
- **WHEN** a Host with a `store` child updates its distribution
- **THEN** the Host's runtime and release inventory advance together
- **AND** the Store's identity, runtime, packages, and data remain unchanged

#### Scenario: Host maintains future Store creation
- **WHEN** an updated Host initializes an additional Store
- **THEN** initialization uses its coherent updated source payload
- **AND** update control files are not copied into the new Store

### Requirement: Preserve Store data during compatible launcher updates
A Store self-update SHALL replace only `portscoop.exe`, `psp.exe`, and `pspe.exe`, plus owned update receipts. It SHALL retain the Store UUID, backend, apps, shims, configuration, portable path state, caches, buckets, and persistence. Candidate compatibility with the retained backend SHALL be established before replacement; backend migrations are outside this update contract.

#### Scenario: Compatible Store launcher update
- **WHEN** the candidate supports the Store's retained backend contract
- **THEN** all three launchers advance as one owned transaction
- **AND** Store identity and package data remain unchanged

#### Scenario: Candidate requires a backend migration
- **WHEN** the candidate cannot run with the Store's existing backend contract
- **THEN** self-update reports that a migration is required before changing installed launchers
- **AND** it does not reinitialize or replace the Store backend

### Requirement: Coordinate runtime replacement
The system SHALL exclude conflicting runtime users during replacement and coordinate Store commits with package operations and application startup. It SHALL bound waits, reject concurrent update transactions for one installation, and prevent new commands from using a partially replaced runtime. Independent installations SHALL retain independent coordination.

#### Scenario: Another runtime user is active
- **WHEN** an existing command or open runtime file prevents replacement
- **THEN** the updater waits within its documented bound or reports busy/failure
- **AND** it does not force termination of unrelated sessions

#### Scenario: Update is pending
- **WHEN** a new command encounters an accepted or interrupted update for its installation
- **THEN** it waits within a documented bound or reports the update/recovery state before using the runtime
- **AND** status and recovery remain available through supported entry points

#### Scenario: Update another installation
- **WHEN** one Store is updating and another independent Store is used
- **THEN** no exclusive update lock is shared solely because they are registered on the same Host

### Requirement: Separate Windows handoff from update completion
The Windows updater SHALL use an owned worker that can finish after the invoking launcher exits. Before handoff, failures or cancellation SHALL clean up uncommitted startup. An accepted handoff SHALL identify its transaction and report pending completion, not completed success. The worker SHALL NOT require the launcher's terminal handles or silently retry uncertain handoffs.

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

### Requirement: Retain update status and restore interrupted transactions
The system SHALL journal the owned replacement set and verified backups before changing runtime files. `self-update --status` SHALL distinguish pending, completed, failed, and recovery-required attempts. `--recover` SHALL restore or finish only the recorded transaction, preserve user data, and refuse unsupported or mismatched recovery metadata.

#### Scenario: Update completes
- **WHEN** all intended runtime files and inventories match the candidate
- **THEN** a completed receipt identifies the installed version and transaction result
- **AND** ordinary startup can use the coherent runtime

#### Scenario: Replacement fails
- **WHEN** a replacement step fails after commit begins
- **THEN** the worker restores the verified previous owned runtime where possible
- **AND** it records failure or recovery-required state rather than reporting completion

#### Scenario: Media is interrupted
- **WHEN** storage disappears or the worker terminates during replacement
- **THEN** retained journal/backups support explicit recovery after the installation is available again
- **AND** no other root or ordinary Scoop installation is adopted as a replacement target

#### Scenario: Recovery state cannot be trusted
- **WHEN** transaction identity, schema, paths, or backup digests are invalid
- **THEN** recovery stops with an inspection diagnostic and leaves unrelated files unchanged

### Requirement: Report self-update outcomes explicitly
Successful checks, no-op updates, and accepted handoffs SHALL return 0 with distinct messages. Pre-handoff failures SHALL return 2 with a Portscoop diagnostic, and cancellation before handoff SHALL return 130. Status queries SHALL return 3 for pending work, 2 for failed/recovery-required work, and 0 for completed or absent work. Handoff status SHALL NOT claim a later worker result.

#### Scenario: Accepted update later fails
- **WHEN** a worker fails after the invoking command returned accepted handoff
- **THEN** `--status` exposes that failure and returns 2
- **AND** the original handoff message is not described as proof of completion

#### Scenario: Update still running
- **WHEN** `--status` observes an active accepted transaction
- **THEN** it reports pending work and returns 3
