# Spec Delta

## MODIFIED Requirements

### Requirement: Select exactly one intended environment
For Store-targeted package and application operations, the system SHALL select an explicit registered alias, otherwise its adjacent portable environment, otherwise an explicitly configured central default. A central Store operation without a selector or default SHALL fail. A portable invocation SHALL reject an explicit selector for another identity. The working directory SHALL NOT implicitly select a target. Self-update SHALL select only the invoked managed installation and SHALL NOT use the Host default or accept Store-selection retargeting.

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

#### Scenario: Self-update a Host without selecting a Store
- **WHEN** a supported Host runs `psp self-update` without a registered default
- **THEN** self-update selects that Host installation
- **AND** it does not require, initialize, or select a Store

#### Scenario: Self-update ignores a different default and caller directory
- **WHEN** a Host has a default Store and its management launcher runs self-update from another Store's directory
- **THEN** only the Host containing that invoked launcher is selected for update

#### Scenario: Reject retargeting self-update
- **WHEN** self-update receives `--env work`
- **THEN** it reports that self-update targets the invoked installation
- **AND** it does not update the Host or any Store
