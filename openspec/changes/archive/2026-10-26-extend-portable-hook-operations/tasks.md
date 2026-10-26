# Tasks

Each numbered section is a coherent checkpoint. Complete its implementation, focused checks, and relevant contract documentation before starting dependent work. Review the diff and validate focused zmem commit messages at each checkpoint; stage explicit implementation paths, excluding this active OpenSpec change. Do not claim a checkpoint complete when its verification is unavailable. No push is authorized by this plan.

## 1. Qualify and prepare pinned bootstrap tools

- [x] 1.1 Pin upstream portable Git and full NSIS-capable 7-Zip sources/artifacts and build a reproducible normalized tool preparation recipe that never runs an installer; verify SHA-256 failures are rejected, required files are inventoried, and notices/source obligations are included.
- [x] 1.2 In the disposable VM, verify the pinned 7-Zip supports bounded NSIS listing and exact-name stdout extraction with separate diagnostics; exercise literal dollar names, known-boundary repeated names, bounded unknown-size singletons, ambiguous duplicate boundaries, corrupted payloads, timeout/cancellation, and a digest-verified PotPlayer payload. Record actual outcomes and block dependent NSIS integration if the contract fails.
- [x] 1.3 Verify the prepared Git works without host Git and establish isolated clone configuration using hostile host-config fixtures; document the managed tool acquisition/version contract and verify no user/system configuration or ordinary Scoop sentinels changed.

## 2. Own complete tools through provisioning and self-update

- [x] 2.1 Add the closed release-to-Store helper namespace mapping, pin inventory, complete-file verification, and missing/tampered/extra-file rejection; verify isolated provisioning tests cover executables, DLLs, plugins, notices, and namespace escape attempts.
- [x] 2.2 Extend Store self-update preparation, ownership checks, backup, replacement, rollback, and relocation to the complete helper inventory; verify fault-injection tests preserve apps, buckets, persistence, identity, and the prior complete runtime on failure.
- [x] 2.3 Update backend patch revision and matching release/update descriptors consistently, package the helper bundle, and document runtime-owned tool status/update behavior; verify a locally packaged release and initialization/update fixtures accept matching inventories and reject older mismatched backends.

## 3. Bootstrap Main and use private Git

- [x] 3.1 Replace Program Files Git lookup with verified Store helper resolution and constrained process-local Git configuration; verify real bucket clone operations with hostile PATH/config/environment fixtures cannot invoke host tools, credentials, hooks, or templates.
- [x] 3.2 Integrate automatic Main acquisition at the recorded revision into staged direct Store and Host-default-Store initialization; verify success without host tools, network failure before readiness, Host-only behavior, and unchanged preexisting files.
- [x] 3.3 Add `psp env bootstrap` with Store selection, locking, version/status output, owned staging/receipt, repeat-run handling, and conflicts; verify existing-Store bootstrap, partial failure/retry, official Main preservation, conflicting Main rejection, and two-Store isolation.
- [x] 3.4 Update setup/CLI help and concise bootstrap documentation, remove the standalone Git prerequisite, and verify the documented blank-start and existing-Store commands in isolated fixtures.

## 4. Compile bounded literal collections

- [x] 4.1 Extend lexical tokens and syntax for explicit literal escapes, comma lists, multiline literal arrays, and immutable local bindings; verify positive and rejection tests preserve source positions and distinguish literal dollar characters from variable expansion.
- [x] 4.2 Add root-tagged checked path joins and finite literal-list `ForEach-Object` expansion with iteration scope and documented limits; verify order, boundary limits, root reassignment rejection, unsupported nested/dynamic constructs, and complete-plan rejection on a bad iteration.
- [x] 4.3 Extend literal default content for line arrays and recognize bounded informational statements without evaluation; verify byte-exact encodings/CRLF, repeated initialization preserving settings, and unchanged optional-AppData-import dependency checks. Update the supported hook grammar documentation in this checkpoint.

## 5. Verify reusable filesystem mechanics

- [x] 5.1 Add bounded literal tree enumeration and terminal-prefix wildcard selection using explicit roots/limits; verify domain-neutral real-filesystem fixtures for deterministic matches, literal dollar names, nested reparse points, case/type conflicts, escape attempts, and count/depth/size limits.
- [x] 5.2 Add checked recursive copy with prevalidation and per-file atomic replacement; verify new/nested files, overwrite semantics, preserved unrelated entries, hard-link behavior, type conflicts, injected write failures, and unchanged outside sentinels.
- [x] 5.3 Add checked cleanup with explicit missing-match behavior and no link traversal; verify complete-selection validation before deletion, recursive removal, access errors, and partial-execution reporting. Document helper contracts beside their implementations; do not claim whole-tree rollback.

## 6. Integrate generic hook operations in both lifecycle layouts

- [x] 6.1 Add independent cleanup and tree-copy recognizers and typed actions with expanded read/write effects; verify unrelated manifest fixtures, source-order copy/cleanup behavior, persistence overlap rejection, skipped-import consumers, and no executable plan for unsupported remainder.
- [x] 6.2 Integrate helper calls into the frozen backend action executor, validating physical version/current/persistence identity and stopping after failure; verify pre-install and post-install filesystem fixtures, tampered action/phase rejection, resource collision semantics, and preserved guarded INI files.
- [x] 6.3 Add the unchanged pinned PotPlayer manifest plus renamed/mutated generic variants and existing Bitwarden/SpaceSniffer regressions; run focused Go and uncached PowerShell-wrapper tests, and update the operation contract documentation without adding app-specific production dispatch.

## 7. Integrate bounded NSIS extraction

- [x] 7.1 Add the managed NSIS process adapter using the qualified listing/exact-name/stdout contract, process-tree cancellation, Job Object memory bounds, and explicit time/output limits; verify valid member streaming and failure cases using pinned native fixtures in the VM and parser/process tests locally.
- [x] 7.2 Integrate NSIS content detection into the verified frozen-download extraction path and reuse Go staging/publication; verify archive-order exact-name replacement, bounded unknown-size singletons, ambiguous-boundary rejection, wrong-format rejection, digest mismatch, literal dollar paths, traversal/ADS/reparse/case collisions, quota failures, and zero publication on extraction failure.
- [x] 7.3 Verify existing ZIP/7z/tar/gzip/direct-EXE paths retain their behavior and never silently fall back to the native helper; document the exact NSIS support boundary and resource limits with tested fixture evidence.

## 8. Qualify the complete portable workflow

- [x] 8.1 Run blank-machine VM setup with no host Git/7-Zip, verify Main and tool versions, add Extras, install unchanged PotPlayer, and launch its shim; record manifest revision, payload digest, release identity, and unchanged ordinary Scoop/persistent environment sentinels.
- [x] 8.2 Verify PotPlayer reinstall and a real supported version transition with custom settings and resource sentinels, then relocate the Store and exercise a second independent Store; verify guarded INI preservation, documented resource collision semantics, and no writes to the prior root. Report unavailable transition evidence explicitly rather than treating reinstall as update proof.
- [x] 8.3 Qualify Host-coordinated backend/helper update across the new revision, including interruption/recovery, Main preservation, and subsequent package operations; verify direct Store self-update retains helpers for a compatible launcher update and rejects a required backend migration with the Host-maintenance diagnostic.
- [x] 8.4 Run the full Go test suite and vet after integration, perform release packaging validation, and verify the finished change against all four spec deltas. Update any compatibility audit only with pinned inputs and clearly separated static/operational evidence; keep development test narratives out of the product README.
