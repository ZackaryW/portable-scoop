## ADDED Requirements

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
