# Design

## Context

See proposal.md. The published v0.3.1 ZIP has 200 backslash entries and no duplicates; its checksum matches. The local PowerShell 7 package has no backslash entries. CI uses Windows PowerShell 5.1. `Disk.Stage` rejects backslashes in its private ZIP extraction path, while packaging currently verifies only the pre-compression folder.

## Goals / Non-Goals

**Goals:** Make packaging independent of shell-specific ZIP naming and check the artifact consumed by self-update. Preserve runtime extraction checks and isolate verification in temporary storage.

**Non-Goals:** Change Store migration, update handoff/recovery, existing published assets, release versions or ordinary Scoop.

## Decisions

- Use explicit .NET ZipArchive entries with normalized relative names in the existing PowerShell packaging script. Preserve all inventoried files including hidden files. Do not rely on changing CI to PowerShell 7, which would leave Windows PowerShell packaging broken.
- Factor the existing extractor to accept an archive path. Add a build-time verifier that calls this same extraction implementation and distribution verification in owned temporary storage. No additional public management command or downloaded executable execution is required.
- Gate creation of the release-assets directory and checksum on completed ZIP verification. Regression tests cover the shared extractor and a complete synthetic release; full package checks run under both shells. Existing runtime rejection policy remains unchanged.

## Risks / Trade-offs

- Extra extraction work during packaging increases build time and disk use; temporary output is cleaned on success and failure.
- A valid archive does not prove full update handoff or backend compatibility; the existing updater checks still apply and this fix makes no broader qualification claim.

## Migration Plan

Commit the corrected packager and regression checks locally. A separately authorized release can ship canonical archives that existing launchers already accept. Preserve v0.3.1 and existing Stores; do not rewrite tags or published assets.
