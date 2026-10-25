# Spec Delta

## Purpose

Translate Scoop manifest declarations into checked portable behavior, with a separate compatibility decision for each field before any package mutation.

## ADDED Requirements

### Requirement: Report field compatibility deterministically
The system SHALL classify each effective manifest field as use, rewrite, skip, or reject. It SHALL reject unknown fields and report rejected fields in deterministic order. A rejected manifest SHALL produce no executable plan.

#### Scenario: Multiple unsupported fields
- **WHEN** a manifest contains an unknown key and an unsupported installer
- **THEN** both field failures are reported in stable order and execution is blocked

#### Scenario: Optional integration is replaced
- **WHEN** a shortcut can become a portable launch alias
- **THEN** the report identifies the rewrite and explains that host Start Menu integration is omitted

### Requirement: Inspect only selected architecture effects
The system SHALL structurally validate architecture declarations, select the Windows x64 branch, and inspect its merged effects. Unselected branch scripts SHALL NOT execute or block a valid selected branch merely because their effects are unsupported.

#### Scenario: ARM branch contains a script
- **WHEN** a valid x64 manifest also has an ARM-only script
- **THEN** the x64 operation uses only the effective x64 fields

### Requirement: Freeze portable replacements
The system SHALL freeze approved declarations and typed replacement actions for the full dependency closure before mutation. Replacement actions SHALL NOT evaluate original script text. Saved manifests SHALL be rechecked before operations that consume them.

#### Scenario: Bucket changes after preflight
- **WHEN** a bucket script changes after preflight
- **THEN** execution uses the frozen approved action without rereading that script

#### Scenario: Script has an unrecognized remainder
- **WHEN** an otherwise recognized initialization hook contains additional statements
- **THEN** preflight rejects the whole hook

### Requirement: Initialize persisted configuration portably
The system SHALL recognize the complete supported pre-install pattern that creates an empty package configuration file when its matching persisted file is absent. It SHALL create only the checked package-relative file, preserve existing persisted content, and reject paths escaping the Store or package through reparse points.

#### Scenario: SpaceSniffer first installation
- **WHEN** its recognized hook is present and no persisted configuration exists
- **THEN** the replacement creates the package configuration file before persistence is linked

#### Scenario: Reinstallation with saved configuration
- **WHEN** a persisted configuration file already exists
- **THEN** its contents remain unchanged

### Requirement: Support declared download digests
The system SHALL validate SHA-1, SHA-256, SHA-512, and MD5 manifest digests with exact algorithm-specific lengths and verify downloads through the managed backend. Release and backend artifact integrity SHALL remain SHA-256-only. Archive support SHALL remain restricted to checked HTTP(S) ZIP downloads in this change.

#### Scenario: SHA-1 ZIP download
- **WHEN** a manifest declares a valid SHA-1 digest for a ZIP
- **THEN** preflight accepts that digest and a content mismatch fails installation

### Requirement: Provide Store launch aliases
The system SHALL rewrite supported shortcut entries into Store-owned shims without host shortcuts. It SHALL deduplicate identical aliases, reject conflicting targets or arguments, and prepend literal fixed native-shim arguments to caller arguments. Unsupported shortcut options or script-wrapper arguments SHALL fail explicitly.

#### Scenario: Shortcut-only package
- **WHEN** a package declares a supported shortcut and no bin
- **THEN** a deterministic logical alias is installed and can be resolved by pspe

#### Scenario: Native arguments and relocation
- **WHEN** a native alias with fixed literal arguments is launched after Store relocation
- **THEN** the relocated target receives fixed arguments followed by caller arguments

### Requirement: Reject required effects without an implementation
The system SHALL reject unsupported installers, unrecognized lifecycle hooks, and module integration rather than silently omit required behavior. Metadata that is not used during installation SHALL be marked skipped without executing its contents.

#### Scenario: Module metadata is declared
- **WHEN** a manifest requires PowerShell module integration
- **THEN** preflight reports that portable module discovery is not yet implemented
