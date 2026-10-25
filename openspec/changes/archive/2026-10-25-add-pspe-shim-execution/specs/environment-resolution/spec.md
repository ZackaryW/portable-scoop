# Spec Delta

## MODIFIED Requirements

### Requirement: Delegate central operations through a compatible portable launcher
The central launcher SHALL verify the portable launcher's identity and the protocol for the requested package or application operation before forwarding it. The portable launcher SHALL recheck identity and execute locally without rediscovery. Missing execution support, incompatible protocols, and mismatched identities SHALL fail before the requested operation starts. Existing package delegation SHALL remain compatible.

#### Scenario: Compatible delegation
- **WHEN** a central command resolves a compatible portable launcher with the expected identity
- **THEN** it delegates once to that launcher and the portable launcher executes against its own root

#### Scenario: Version mismatch
- **WHEN** the portable launcher's delegation protocol is incompatible with the central launcher
- **THEN** an actionable compatibility error is returned without invoking Scoop

#### Scenario: Target changes between discovery and execution
- **WHEN** the adjacent identity no longer matches the expected identity when delegated execution starts
- **THEN** execution fails without selecting a replacement root

#### Scenario: Compatible application execution
- **WHEN** Host `pspe` resolves a Store advertising compatible application execution support
- **THEN** the selected Store verifies its expected identity and launches the requested local shim once
- **AND** application arguments and mode are not interpreted as package-management commands

#### Scenario: An older Store lacks execution support
- **WHEN** Host `pspe` selects a Store without the required execution entry point or compatible execution protocol
- **THEN** it reports that the Store needs compatible execution support and does not substitute another Store or a host command
- **AND** the Store's existing compatible package-management commands remain usable

### Requirement: Preserve the delegated command contract
Delegation SHALL preserve argument values and caller working directory. Package operations and foreground applications SHALL preserve their terminal streams and exit status, and cancellation SHALL stop their managed invocation trees and release locks. Acknowledged detached applications SHALL outlive completed launch delegation. Store-selection options SHALL NOT become application or Scoop arguments.

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

#### Scenario: Detached application handoff completes
- **WHEN** a selected Store acknowledges successful detached application creation
- **THEN** Host and portable launch helpers can exit without stopping the launched application
- **AND** the Host returns launch success rather than a later application exit status

#### Scenario: Application handoff does not complete
- **WHEN** a launch helper fails or execution capability negotiation fails before detached handoff
- **THEN** the Host reports the failure without automatically retrying the application in another location
- **AND** uncommitted launch helpers and children are cleaned up
