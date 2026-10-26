## MODIFIED Requirements

### Requirement: Replace only managed Store backend artifacts
A Host-managed Store update SHALL replace launchers, runtime receipt, backend metadata and inventory-owned adapter, engine, and helper files in one recoverable transaction. It SHALL validate current ownership and candidate hashes, reject unsupported metadata or files outside the managed backend namespaces, and preserve Store identity and all package/user state. It SHALL NOT run backend migration scripts.

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

#### Scenario: Introduce or update managed tools
- **WHEN** a verified Host candidate includes a complete pinned Git and NSIS extraction helper inventory
- **THEN** its owned tool files and matching backend metadata are installed together with the Store launchers
- **AND** existing Main and other bucket checkouts are not reset or overwritten

#### Scenario: Direct Store update requires new helpers
- **WHEN** a directly invoked Store self-update requires a different backend or managed helper contract
- **THEN** it retains the installed backend and helpers and directs the user to Host-managed maintenance
- **AND** it does not silently perform backend migration through bootstrap
