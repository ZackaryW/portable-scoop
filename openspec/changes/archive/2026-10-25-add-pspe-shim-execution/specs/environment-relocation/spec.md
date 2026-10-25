# Spec Delta

## MODIFIED Requirements

### Requirement: Coordinate operations within each environment
The system SHALL serialize package mutations, bootstrap changes, relocation repair, and application launch preparation within each canonical environment root. Commands needing stable state SHALL coordinate with active mutations or report the environment busy. Application process creation SHALL occur inside launch coordination, but waiting for or running an application SHALL NOT retain the Store's mutation lock. Separate environments SHALL NOT share an exclusive operation lock.

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

### Requirement: Prepare relocated roots before backend execution
The system SHALL detect a changed canonical root and prepare managed paths before starting any backend command or application that relies on them. Preparation and dependent execution or application process creation SHALL remain coordinated so another mutation cannot intervene. Fresh bootstrap SHALL record a prepared root only after successful setup.

#### Scenario: Direct invocation after a directory move
- **WHEN** a ready root is moved to another supported directory and its portable launcher is invoked directly
- **THEN** required managed paths are prepared for the new root before the backend starts
- **AND** no host registration is required for that direct invocation

#### Scenario: Central invocation after drive rediscovery
- **WHEN** central discovery locates the same identity under a new drive letter
- **THEN** the portable launcher performs relocation preparation before executing the delegated package command

#### Scenario: Direct application launch after a move
- **WHEN** a moved Store's `pspe.exe` is invoked directly from an unrelated working directory
- **THEN** owned shims and portable path state are prepared for the new root before application creation
- **AND** the application keeps the caller's working directory

#### Scenario: Application preparation fails
- **WHEN** relocation preparation detects corrupt state, an unknown shim, or an escaped managed target
- **THEN** application execution is blocked with the preparation diagnostic and no alternative target is launched
