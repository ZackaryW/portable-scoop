# Spec Delta

## MODIFIED Requirements

### Requirement: Reject unsupported package effects before execution
The system SHALL preflight manifests and their dependency closure for install, update, and uninstall against its documented portable compatibility policy. It SHALL execute approved portable replacements for supported fields, visibly omit optional nonportable integration, and reject required unsupported effects before mutation. It SHALL NOT claim arbitrary-script or application-runtime sandboxing.

#### Scenario: Dependency has an unsupported hook
- **WHEN** an otherwise supported installation depends on a manifest with an unsupported install script
- **THEN** preflight reports the package and unsupported feature before any package installation in that operation begins

#### Scenario: Installed manifest has an unsupported uninstall action
- **WHEN** uninstall would execute a hook not supported by the portable policy
- **THEN** the hook is not run and the command reports the compatibility failure

#### Scenario: Supported hook has a portable replacement
- **WHEN** a hook fully matches a supported portable operation
- **THEN** the approved replacement runs at its lifecycle phase without evaluating the hook's script text

#### Scenario: Old backend cannot execute the plan
- **WHEN** the selected Store has an incompatible backend revision
- **THEN** the launcher reports the need for a matching complete backend before package mutation
- **AND** execution is blocked without automatic migration or a legacy backend fallback; support for older Portscoop releases and Store formats is not required
