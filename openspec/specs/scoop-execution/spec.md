# scoop-execution Specification

## Purpose

Execute supported Scoop operations within a selected portable environment while preserving other environments and ordinary Scoop on the same host.

## Requirements

### Requirement: Keep package state independent
The system SHALL keep each environment's engine, configuration, apps, buckets, cache, package persistence, and operation state within that environment's root. Package and dependency lookup SHALL NOT treat installations in another portable root or ordinary Scoop as satisfying the selected environment's dependencies.

#### Scenario: Same package in two environments
- **WHEN** `work` and `personal` independently install different versions of a supported package
- **THEN** each environment reports and uses its own installed version and persisted package data
- **AND** updating or uninstalling one does not modify the other

#### Scenario: Dependency installed only in ordinary Scoop
- **WHEN** a package dependency exists in ordinary Scoop but not in the selected portable environment
- **THEN** the portable backend resolves and provisions that dependency inside the selected environment

### Requirement: Isolate the backend process environment
The system SHALL execute package operations in a separate profile-free process with explicitly resolved portable paths and configuration. Inherited Scoop root/cache/global overrides SHALL NOT redirect the operation. Calling-shell environment variables SHALL remain unchanged when the command succeeds, fails, or is cancelled.

#### Scenario: Caller has ordinary Scoop variables
- **WHEN** portable execution starts from a shell whose Scoop variables point to ordinary Scoop
- **THEN** the backend uses the selected portable paths
- **AND** the calling shell retains its original values after completion

#### Scenario: Caller contains another portable root
- **WHEN** inherited environment settings refer to a different portable environment
- **THEN** they do not redirect package state or configuration lookup away from the selected root

### Requirement: Preserve host integration state during package operations
Supported portable package operations SHALL NOT modify persistent user/machine environment variables, host PATH, Start Menu entries, host PowerShell module registrations, or ordinary Scoop files. Supported package environment settings SHALL be retained as portable state and applied only within managed child processes.

#### Scenario: Supported manifest declares environment settings
- **WHEN** a supported package declares environment values or additional tool paths
- **THEN** these are recorded for the selected portable environment and applied to its managed backend processes
- **AND** persistent host environment settings remain unchanged

#### Scenario: Supported manifest contains shortcut metadata
- **WHEN** a supported package declares shortcuts or module integration metadata
- **THEN** portable execution does not create host Start Menu entries or register modules outside the root

### Requirement: Reject unsupported package effects before execution
The system SHALL preflight manifests and their dependency closure for install, update, and uninstall against its documented portable compatibility policy. Packages requiring unsupported script hooks, external installers, or host integration SHALL fail before those effects execute. It SHALL NOT claim arbitrary-script or application-runtime sandboxing.

#### Scenario: Dependency has an unsupported hook
- **WHEN** an otherwise supported installation depends on a manifest with an unsupported install script
- **THEN** preflight reports the package and unsupported feature before any package installation in that operation begins

#### Scenario: Installed manifest has an unsupported uninstall action
- **WHEN** uninstall would execute a hook not supported by the portable policy
- **THEN** the hook is not run and the command reports the compatibility failure

### Requirement: Expose a controlled package command surface
The system SHALL support install, uninstall, app update, list, search, status, bucket listing/addition/removal, reset, cleanup, and configuration inspection for compatible packages. It SHALL reject unsupported commands, global operations, root-changing configuration writes, and unrestricted engine self-update before backend execution.

#### Scenario: Supported app update
- **WHEN** the user updates a named supported app in a selected environment
- **THEN** that environment's package is updated through the managed backend

#### Scenario: Global operation is requested
- **WHEN** a package command requests global installation or a persistent root override
- **THEN** it fails before package state or host integration is changed

#### Scenario: Unrestricted command would bypass portable policy
- **WHEN** the user requests an unsupported command or unmediated engine update
- **THEN** the system reports that the operation is unsupported rather than forwarding it through a policy bypass

### Requirement: Use only a compatible managed backend
The system SHALL verify the selected environment's pinned backend metadata and executable artifacts before using them. A missing, modified, or incompatible backend SHALL fail visibly. Execution SHALL NOT fall back to a Scoop command discovered on host PATH.

#### Scenario: Portable engine is missing
- **WHEN** the selected environment has no usable managed engine and ordinary Scoop is available
- **THEN** the command reports the missing portable backend without invoking ordinary Scoop

#### Scenario: Backend files no longer match their recorded pin
- **WHEN** managed backend verification detects an incompatible version or altered executable artifact
- **THEN** execution is blocked with a backend repair or compatible-release diagnostic

### Requirement: Validate coexistence and supported runtime behavior
The release SHALL document its supported Windows, filesystem, PowerShell, and manifest compatibility scope. Supported operations SHALL work without a Go toolchain installed. Unsupported runtime/filesystem prerequisites SHALL be reported before initialization publishes readiness or package mutation begins.

#### Scenario: Clean supported machine
- **WHEN** a release is used on a qualified Windows x64 machine with its documented PowerShell prerequisite and no Go installation
- **THEN** both central delegation and direct portable operations function

#### Scenario: Unsupported storage capabilities
- **WHEN** initialization targets storage lacking required locking or managed-link capabilities
- **THEN** it reports the unsupported prerequisite without publishing a ready environment
