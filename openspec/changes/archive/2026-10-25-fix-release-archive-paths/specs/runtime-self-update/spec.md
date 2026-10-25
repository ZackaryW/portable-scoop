## ADDED Requirements

### Requirement: Produce updater-compatible release archives
Release packaging SHALL encode ZIP entry names with canonical forward-slash relative paths under Windows PowerShell 5.1 and PowerShell 7. It SHALL verify the finished archive using the updater's extraction and complete-distribution checks before exposing publishable ZIP and checksum assets. Invalid archives SHALL stop packaging without relaxing updater validation.

#### Scenario: Package a nested distribution
- **WHEN** a release includes nested backend and documentation files
- **THEN** the finished ZIP uses forward-slash entry names and passes updater extraction and inventory verification
- **AND** packaging works under either supported PowerShell edition

#### Scenario: Final artifact is invalid
- **WHEN** the finished ZIP contains backslashes, duplicate paths, traversal, missing required files or changed inventory content
- **THEN** verification fails before publishable assets are created
- **AND** verification does not launch an update worker or alter an installed Host or Store
