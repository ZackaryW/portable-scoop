# Tasks

## 1. Field compilation

- [x] 1.1 Introduce the compatibility registry and per-field units, deterministic decisions, selected-architecture compilation, and strict shared validators; verify unit tests for decisions, unsupported fields, branch selection, and existing supported declarations, and document the handler contract.
- [x] 1.2 Implement download hash handling, shortcut-to-bin rewriting, native literal argument validation, and the complete persisted-file hook recognizer; verify positive cases, collisions, malformed digests, script remainders, path escapes, and idempotent recompilation, and document supported rewrites.

## 2. Execution integration

- [x] 2.1 Route preflight and saved-manifest checks through compilation, freeze decisions/actions alongside declarations, and emit sorted compatibility notices; verify closure rejection and frozen-request tests including a changed bucket hook.
- [x] 2.2 Execute only typed initialization actions in the pinned adapter, enforce runtime containment and persistence preservation, reject uncompiled integration, and bump the adapter revision consistently; verify controlled PowerShell filesystem tests and successful backend preparation, and document complete-distribution requirements.
- [x] 2.3 Resolve native fixed arguments from owned shim metadata and prepend them during pspe launch while preserving relocation; verify argument, ownership, and relocation tests and update execution documentation.

## 3. Integration verification

- [x] 3.1 Run the full Go suite, go vet, PowerShell installer bootstrap suite, and strict OpenSpec validation; review the diff for host-isolation boundaries and record results without claiming real-app portability certification.

### Verification evidence

- Full `go test -json ./...` passed; log retained in ignored `.work/compatibility-go-tests.jsonl`. After the final literal-URL parser correction, affected compatibility/shimargs/application/relocation suites passed again.
- `go vet ./...`, Windows PowerShell 5.1 installer bootstrap tests, and strict OpenSpec validation passed.
- Prepared `.work/compatibility-backend-v2` and verified its inventory using `TestPreparedPayloadMatchesRecordedArtifacts` with `PORTSCOOP_TEST_BACKEND_PAYLOAD` set.
- Controlled PowerShell action checks passed, including reparse rejection and preservation. Prepared backend digest checks accepted correct and rejected incorrect MD5/SHA-1/SHA-256/SHA-512 values.
- Native fixed arguments reached a controlled process before caller arguments; owned native metadata survived Store directory relocation.
- Seven existing detach-success tests skipped under the host's restrictive Windows job; one directory-symlink test skipped without the required privilege. The initially unconfigured prepared-backend test was subsequently run explicitly and passed.
- Updated VM fixtures for shortcut alias behavior and module rejection; syntax checked, not executed. Historical VM reports remain unchanged. No real-application VM certification or existing Store migration is claimed.

## Workflow follow-up

- Backward compatibility with older Portscoop releases and Store formats is not required. Breaking releases may require fresh Stores; legacy support and migration are not deferred requirements. Scoop manifest compatibility remains constrained by portability. This clarification needs no runtime change to the existing strict backend revision check.
- Keep active OpenSpec artifacts uncommitted; make incremental local implementation commits using zmem.
- Review before archive. Each push requires a fresh explicit user request.
