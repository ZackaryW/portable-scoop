# Spec Delta

## MODIFIED Requirements

### Requirement: Coordinate operations within each environment
The system SHALL serialize package mutations, bootstrap changes, relocation repair, runtime replacement, and application launch preparation within each canonical environment root. Commands needing stable state SHALL coordinate with active mutations or report the environment busy. Application process creation SHALL occur inside launch coordination, but waiting for or running an application SHALL NOT retain the Store's mutation lock. Runtime replacement SHALL also exclude active runtime users and prevent new operations from using a partially replaced installation. Separate environments SHALL NOT share an exclusive operation lock.

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

#### Scenario: Launch competes with a package mutation
- **WHEN** an application launch is requested while its Store has an active package mutation
- **THEN** launch preparation waits within the Store's bound or reports busy before starting the application
- **AND** target validation and process creation occur without an intervening coordinated mutation

#### Scenario: A long-running application remains open
- **WHEN** a foreground or detached application continues running after successful process creation
- **THEN** another Store operation is not blocked solely by the application's retained preparation lock
- **AND** operating-system file-use restrictions can still report individual files busy

#### Scenario: Runtime replacement competes with Store use
- **WHEN** a Store self-update reaches replacement while package operations or launcher processes are active
- **THEN** the worker obtains the installation's exclusive runtime coordination and Store operation lock or reports busy within the bound
- **AND** no package operation or application startup observes a mixed runtime

#### Scenario: Caller exits after update handoff
- **WHEN** the invoking self-update process exits and its worker obtains coordination
- **THEN** the worker reloads identity, pending-transaction state, and runtime ownership before making replacements
- **AND** it does not rely on an unchecked lock handoff from the exited caller

#### Scenario: Interrupted runtime replacement
- **WHEN** runtime replacement is incomplete and no worker owns the operation lock
- **THEN** new runtime-dependent operations report recovery-required state before proceeding
- **AND** status and explicit recovery are not blocked merely by a leftover lock file
