# Spec Delta

## Purpose

Keep supported portable environments usable after relocation and prevent concurrent package operations from corrupting their managed state.

## ADDED Requirements

### Requirement: Coordinate operations within each environment
The system SHALL serialize package mutations, bootstrap changes, and relocation repair within each canonical environment root. Commands needing stable state SHALL coordinate with active mutations or report the environment busy. Separate environments SHALL NOT share an exclusive operation lock.

#### Scenario: Concurrent mutations in one root
- **WHEN** two invocations attempt to mutate the same environment
- **THEN** at most one backend mutation or preparation runs at a time
- **AND** the other invocation waits within a documented bound or reports that the environment is busy

#### Scenario: Different environments remain independent
- **WHEN** package operations run against `work` and `personal` concurrently
- **THEN** neither operation is blocked solely by the other environment's mutation lock

#### Scenario: Read while updating
- **WHEN** a package listing requiring stable state is requested during an update
- **THEN** it obtains a consistent view after coordination or reports the environment busy

### Requirement: Release locks after process termination
Operation locks SHALL cease blocking after the owning invocation terminates. A leftover lock artifact SHALL NOT by itself keep an environment permanently busy. Interrupted package mutations SHALL expose a recoverable diagnostic state where further inspection is required.

#### Scenario: Cancel the lock owner
- **WHEN** a lock-holding invocation and its managed children terminate after cancellation
- **THEN** a later invocation can acquire the lock without manually deleting a lock file

### Requirement: Prepare relocated roots before backend execution
The system SHALL detect a changed canonical root and prepare managed paths before starting any backend command that relies on them. Preparation and execution SHALL remain coordinated so another mutation cannot intervene between them. Fresh bootstrap SHALL record a prepared root only after successful setup.

#### Scenario: Direct invocation after a directory move
- **WHEN** a ready root is moved to another supported directory and its portable launcher is invoked directly
- **THEN** required managed paths are prepared for the new root before the backend starts
- **AND** no host registration is required for that direct invocation

#### Scenario: Central invocation after drive rediscovery
- **WHEN** central discovery locates the same identity under a new drive letter
- **THEN** the portable launcher performs relocation preparation before executing the delegated package command

### Requirement: Limit repair to owned managed artifacts
Relocation repair SHALL update or rebuild only supported Portscoop/Scoop-generated shims, managed links, and adapter-controlled path state. Replacements SHALL match the exact previous root. Repair SHALL NOT rewrite unrelated drive references, downloaded manifests, application binaries, arbitrary application configuration, or user files.

#### Scenario: Root move without a drive-letter change
- **WHEN** an environment moves from `F:\tools\work` to `F:\portable\work`
- **THEN** supported managed references to the previous root are rebuilt or updated to the new root

#### Scenario: Unrelated absolute path in state
- **WHEN** managed state includes an unrelated path on the same drive as the previous root
- **THEN** repair leaves that unrelated path unchanged

### Requirement: Constrain repair and links to the environment
The system SHALL verify that repair targets and expected managed link destinations remain within the canonical environment root. Unknown artifact formats, unsupported package relocation, or links escaping the permitted root SHALL block dependent execution with diagnostics rather than trigger broad rewriting.

#### Scenario: Managed link points outside the root
- **WHEN** repair encounters a managed link whose destination escapes the environment
- **THEN** repair reports the offending artifact, does not modify the external destination, and does not launch the dependent backend command

#### Scenario: Unsupported artifact format
- **WHEN** relocation encounters an artifact it cannot safely interpret or reconstruct
- **THEN** it reports that artifact and blocks backend execution until the condition is resolved

### Requirement: Recover interrupted relocation preparation
The system SHALL retain enough repair state to resume interrupted preparation idempotently. The new prepared root SHALL be committed only after all required managed repairs succeed. A failed repair SHALL preserve diagnostics and block backend execution.

#### Scenario: Interruption after partial repair
- **WHEN** relocation stops after some artifacts have changed but before completion
- **THEN** the next invocation reconciles or completes the recorded repair before starting Scoop
- **AND** already repaired artifacts are handled safely

#### Scenario: Repair fails before completion
- **WHEN** a required artifact cannot be repaired
- **THEN** the last successfully prepared root is not replaced with a false success state
- **AND** an actionable error identifies the repair failure
