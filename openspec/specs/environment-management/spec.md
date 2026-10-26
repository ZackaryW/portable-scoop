# environment-management Specification

## Purpose

Allow users to create independent portable Scoop environments and register them on a host without coupling their package data to that host.

## Requirements

### Requirement: Initialize a self-contained environment
The system SHALL initialize a portable environment at a chosen supported destination with a unique installation identity, versioned metadata, a ready backend, and private package storage. Initialization SHALL NOT require central installation or host registration.

#### Scenario: Initialize on a fresh portable destination
- **WHEN** the user runs `psp env init <path> --name work` for a supported empty destination
- **THEN** the destination contains a ready environment with a new identity, a working local launcher, and independent engine, config, apps, buckets, cache, and persistence storage
- **AND** no host registration or PATH change is made

#### Scenario: Initialization fails before readiness
- **WHEN** provisioning or platform checks fail
- **THEN** the destination is not advertised as a ready environment
- **AND** files predating the initialization attempt remain unchanged

#### Scenario: Refuse an existing or unrelated destination
- **WHEN** initialization targets an existing environment or an unrelated non-empty directory
- **THEN** it fails without overwriting identity or user data

### Requirement: Register portable environments by identity
The system SHALL register an existing ready environment under a case-insensitive host alias and record its identity and verified location hints. Re-registering the same alias and identity SHALL refresh hints. An alias SHALL NOT silently change to a different identity.

#### Scenario: Register two environments
- **WHEN** the user registers distinct roots as `work` and `personal`
- **THEN** both aliases are listed with their respective identities and recorded locations
- **AND** each root retains its own package state

#### Scenario: Refresh a moved registration
- **WHEN** an existing alias is registered against a new path with its original identity
- **THEN** its location hints are refreshed without changing its identity

#### Scenario: Alias collision
- **WHEN** `WORK` is already registered and a different identity is registered as `work`
- **THEN** registration fails without changing the existing mapping

### Requirement: Persist host metadata reliably
The system SHALL version and serialize host registry changes, preserve complete registry contents across failed writes, and reject corrupt or unsupported registry data without silently replacing it with an empty registry.

#### Scenario: Concurrent registration updates
- **WHEN** two processes register different environments concurrently
- **THEN** both completed registrations are preserved without a lost update

#### Scenario: Interrupted registry replacement
- **WHEN** saving a registry update is interrupted before completion
- **THEN** the next invocation can read a complete previously committed or newly committed registry
- **AND** it does not treat a partial document as valid

#### Scenario: Unsupported registry version
- **WHEN** a command encounters an unsupported registry schema
- **THEN** it reports the incompatibility and leaves the registry unchanged

### Requirement: Manage an explicit host default
The system SHALL allow a registered environment to be selected as the host default. The default SHALL refer to the environment identity and SHALL NOT switch automatically when a different environment becomes available.

#### Scenario: Select the default
- **WHEN** the user runs `psp env default work`
- **THEN** subsequent unqualified central package commands select the identity registered as `work`

#### Scenario: Default drive is disconnected
- **WHEN** the default environment is unavailable and another registered environment is connected
- **THEN** an unqualified central package command reports the default unavailable without selecting the other environment

### Requirement: Unregistration preserves portable data
The system SHALL remove only the requested host alias on unregistration. If the removed alias was the last registration of the default identity, the default SHALL be cleared. Portable files SHALL remain unchanged.

#### Scenario: Unregister the last alias for the default
- **WHEN** the user unregisters the last alias for the default environment
- **THEN** its registry entry and default selection are removed
- **AND** the portable environment remains usable through its own launcher

### Requirement: Provide central and portable deployment through both CLI names
The system SHALL provide `portscoop` and `psp` with equivalent management behavior in CMD and PowerShell, and `pspe` as a separate application execution entry point. Setup SHALL support central deployment and independent portable deployment with all three executables. Central user-PATH integration SHALL occur only when explicitly requested during setup.

#### Scenario: Equivalent entry points
- **WHEN** the same operation is invoked through `portscoop` and `psp` from CMD or PowerShell
- **THEN** environment selection, arguments, output, and exit status have equivalent behavior

#### Scenario: Central setup opts into PATH
- **WHEN** the user explicitly enables PATH integration while deploying the central launcher
- **THEN** setup adds only the central launcher directory to user PATH
- **AND** it does not add any environment's package shims

#### Scenario: Portable deployment without host setup
- **WHEN** a portable release is deployed and initialized without central setup
- **THEN** both local CLI names work without adding directories to host PATH

#### Scenario: Execution entry point is deployed in both modes
- **WHEN** a compatible release is deployed as a Host or initialized as a Store
- **THEN** `pspe.exe` is present beside the two management executables and is covered by release verification
- **AND** direct Store execution needs no Host installation or registration

#### Scenario: Execution artifact is missing or altered
- **WHEN** a release claiming execution support lacks its recorded `pspe.exe` artifact or its content does not match the release inventory
- **THEN** deployment reports the incomplete or altered release instead of publishing execution readiness

### Requirement: List unavailable registrations without losing them
The system SHALL list all registered environments with their aliases, identities, default status, and availability diagnostics. Unavailable storage SHALL NOT cause a registration to be discarded or a listing to wait indefinitely.

#### Scenario: List with one disconnected drive
- **WHEN** `psp env list` runs while one registered environment is disconnected
- **THEN** connected and disconnected registrations are both listed with distinguishable availability
- **AND** the disconnected registration remains stored

### Requirement: Bootstrap a usable Store automatically
New Store initialization SHALL provision the official Main bucket and verified portable Git and NSIS extraction tools inside that Store before publishing readiness. This SHALL apply to direct initialization and a Host's requested default Store. Bootstrap SHALL require neither an existing Git or archiver installation nor host PATH changes. A Host installation without a Store SHALL NOT create a bucket or application environment.

#### Scenario: Blank Windows environment
- **WHEN** a user initializes a Store on a supported machine without Git, 7-Zip, or ordinary Scoop
- **THEN** the ready Store can search Main and perform supported package and bucket operations using its own tools
- **AND** bootstrap reports the installed helper versions and Main location

#### Scenario: First default Store
- **WHEN** Host setup creates the user's first default Store
- **THEN** it completes the same bootstrap before registering or selecting that Store as ready

#### Scenario: Failure during bootstrap
- **WHEN** acquisition, integrity checking, tool provisioning, or Main initialization fails
- **THEN** initialization reports the failed stage without publishing Store readiness
- **AND** preexisting files and host integration state remain unchanged

### Requirement: Bootstrap existing Stores explicitly and safely
The system SHALL provide idempotent `psp env bootstrap` for an existing Store. It SHALL verify and report runtime-provisioned helpers, provision an absent official Main bucket, and preserve packages, persistence, identity, and existing bucket contents. Conflicting Main sources or missing or altered inventoried tools SHALL fail without silent replacement. Backend/helper migration SHALL remain Host-managed runtime maintenance.

#### Scenario: Older Store lacks bootstrap components
- **WHEN** Host-managed maintenance has supplied an older Store's compatible backend and complete helpers, and that Store runs bootstrap
- **THEN** the helpers are verified and an absent Main bucket is provisioned without reinstalling user applications

#### Scenario: Already bootstrapped
- **WHEN** bootstrap runs against a Store with verified helpers and an existing official Main bucket
- **THEN** it reports readiness and versions without resetting the bucket checkout or downloading tools again

#### Scenario: Owned helper is missing
- **WHEN** bootstrap finds a missing or altered inventoried helper
- **THEN** it reports a runtime repair conflict without downloading an unowned replacement or claiming readiness

#### Scenario: Conflicting Main bucket
- **WHEN** a bucket named main is already registered to another source
- **THEN** bootstrap reports that conflict and preserves the user's bucket

#### Scenario: Failure and retry in an existing Store
- **WHEN** explicit bootstrap fails after staging a missing component
- **THEN** it preserves the previously usable components and reports incomplete bootstrap
- **AND** retry uses only verified owned state and never adopts a partial component as ready

### Requirement: Preserve bootstrap independence after relocation
Store helper discovery and bucket operations SHALL resolve from the Store's current location. Moving a Store SHALL NOT require reinstalling Git, the extractor, or Main, and SHALL NOT retain executable paths pointing into another Store or the original drive.

#### Scenario: Store changes drive letter
- **WHEN** a bootstrapped Store is moved to a supported different path or drive
- **THEN** managed Git, extraction, and Main discovery work from the relocated Store
- **AND** no operation writes to its previous location
