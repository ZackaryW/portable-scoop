# environment-resolution Specification

## Purpose

Resolve the intended portable environment deterministically and delegate central commands without confusing drive locations, environments, or ordinary Scoop.

## Requirements

### Requirement: Select exactly one intended environment
The system SHALL select an explicit registered alias, otherwise its adjacent portable environment, otherwise an explicitly configured central default. A central invocation without a selector or default SHALL fail. A portable invocation SHALL reject an explicit selector for another identity. The working directory SHALL NOT implicitly select a target.

#### Scenario: Explicit central selection
- **WHEN** the central launcher runs `psp --env work list` while another identity is the default
- **THEN** only the identity registered as `work` is selected

#### Scenario: Portable invocation from an unrelated directory
- **WHEN** a portable launcher is invoked by absolute path from another working directory without a selector
- **THEN** it selects the environment adjacent to its executable without consulting a host default

#### Scenario: No implicit single-environment default
- **WHEN** the central launcher has one registration but no configured default and no selector
- **THEN** it reports that selection is required before invoking a backend

#### Scenario: Portable launcher rejects cross-environment selection
- **WHEN** a portable launcher is given a selector resolving to another installation identity
- **THEN** it reports the conflict and launches neither environment's backend

### Requirement: Verify identity at every candidate location
The system SHALL verify the requested installation identity and supported ready metadata before accepting a location. A saved path or drive letter SHALL NOT be sufficient. Invalid adjacent portable metadata SHALL cause an error rather than central fallback.

#### Scenario: A different drive reuses the saved letter
- **WHEN** the saved location exists but contains a different environment identity
- **THEN** that location is rejected for the requested environment

#### Scenario: Corrupt local marker
- **WHEN** the invoked portable launcher has malformed or unsupported adjacent metadata
- **THEN** it reports the metadata error without invoking the default or system Scoop

### Requirement: Rediscover changed drive letters with bounded discovery
The system SHALL search verified registration hints and their drive-relative paths on available supported local volumes, with bounded handling of unavailable volumes. It SHALL refresh the saved location only after finding one unique matching root. Discovery SHALL NOT recursively scan entire drives.

#### Scenario: Same environment on a new drive letter
- **WHEN** the registered environment moves from `F:\tools\work` to `G:\tools\work` with its identity unchanged
- **THEN** central discovery selects the verified root at `G:\tools\work`
- **AND** records its verified location for later invocations

#### Scenario: Containing directory changes
- **WHEN** a root is moved to a directory outside all recorded candidate paths
- **THEN** discovery reports it unavailable and explains how to refresh registration with its new path

#### Scenario: Requested environment is absent
- **WHEN** no candidate contains the requested ready identity
- **THEN** the command fails without invoking another portable environment or ordinary Scoop

### Requirement: Reject ambiguous identities
The system SHALL consider all bounded candidates and fail if distinct canonical roots advertise the requested identity. Equivalent paths to the same canonical root SHALL count as one candidate. An earlier cached match SHALL NOT conceal a discovered duplicate.

#### Scenario: Copied environment is also connected
- **WHEN** the saved root and a second candidate root contain the same installation identity
- **THEN** selection fails with both conflicting locations and no backend is invoked

#### Scenario: Equivalent root paths
- **WHEN** two candidate paths resolve to the same canonical root
- **THEN** they are treated as one location rather than an identity conflict

### Requirement: Delegate central operations through a compatible portable launcher
The central launcher SHALL verify the portable launcher's identity and delegation protocol before forwarding a package command. The portable launcher SHALL recheck identity and execute locally without rediscovery. Incompatible protocols and mismatched identities SHALL fail before backend execution.

#### Scenario: Compatible delegation
- **WHEN** a central command resolves a compatible portable launcher with the expected identity
- **THEN** it delegates once to that launcher and the portable launcher executes against its own root

#### Scenario: Version mismatch
- **WHEN** the portable launcher's delegation protocol is incompatible with the central launcher
- **THEN** an actionable compatibility error is returned without invoking Scoop

#### Scenario: Target changes between discovery and execution
- **WHEN** the adjacent identity no longer matches the expected identity when delegated execution starts
- **THEN** execution fails without selecting a replacement root

### Requirement: Preserve the delegated command contract
Delegation SHALL preserve backend argument values, caller working directory, standard input/output/error behavior, and backend exit status. Cancellation SHALL terminate the managed invocation tree and release its operation locks. Portscoop selection arguments SHALL NOT be forwarded as Scoop arguments.

#### Scenario: Argument values survive forwarding
- **WHEN** a supported command includes empty values, spaces, quotes, Unicode, or shell metacharacters
- **THEN** the backend receives the intended argument values without evaluating them as injected script source

#### Scenario: Streams and failure status
- **WHEN** a backend reads standard input, writes to both output streams, and exits with a nonzero status
- **THEN** central and direct invocation expose equivalent streams and the same backend exit status

#### Scenario: Cancel a delegated operation
- **WHEN** the user cancels a central package operation
- **THEN** its delegated launcher and managed backend processes terminate
- **AND** another operation can subsequently acquire the environment lock
