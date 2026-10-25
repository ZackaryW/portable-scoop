# Tasks

## 1. Shared archive verification

- [x] 1.1 Expose isolated build-time verification through the existing updater extractor and complete-distribution checks; add a build command and regression cases for canonical archives, backslashes, duplicate/traversal paths and missing or modified inventory files. Verify focused Go tests and rejection of the published v0.3.1 artifact before checkpointing.

## 2. Canonical packaging

- [x] 2.1 Write explicit canonical ZIP entry names and gate release assets on final archive verification; document the behavior and verify full packaging under Windows PowerShell 5.1 and PowerShell 7, all inventoried files retained, and both outputs accepted by the shared verifier before checkpointing.

## 3. Integrated checks

- [x] 3.1 Run the Go regression suite, vet and strict OpenSpec validation; record the package compatibility evidence and confirm no installed Store, existing release asset or tag was modified.
