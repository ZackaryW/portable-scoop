# Proposal

## Why

The published v0.3.1 ZIP contains Windows backslash entry names, which the existing self-updater correctly rejects as noncanonical. Packaging verified the source folder before compression and missed the incompatible archive produced by Windows PowerShell 5.1.

## What Changes

- Write explicit forward-slash archive entry names, independently of PowerShell version.
- Validate the completed ZIP with the updater's extraction and distribution checks before creating publishable assets.
- Cover canonical round trips and rejection paths with regression tests and reproduce packaging under Windows PowerShell 5.1.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `runtime-self-update`: Require release packaging to produce updater-compatible archives and validate the final artifact before publication.

## Impact

Release packaging, shared self-update extraction verification, a build-time verification command, tests and developer documentation. No relaxed runtime path checks, installed-file mutation, release replacement, version bump or push is included.
