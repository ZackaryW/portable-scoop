# Spec Delta

## MODIFIED Requirements

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
