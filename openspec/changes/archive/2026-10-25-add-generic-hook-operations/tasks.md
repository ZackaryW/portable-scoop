# Tasks

Each numbered group is a coherent implementation checkpoint. Complete its implementation, relevant tests, and documentation before proceeding to dependent groups. Review the diff and use zmem-author-commits for an authorized focused commit of explicit implementation paths; exclude active OpenSpec artifacts and preserve unrelated work. Do not run a full regression suite for every small edit. The final coverage target is aspirational; completing a task requires its stated behavior and evidence, not a fabricated 70% result.

## 1. Reproducible coverage baseline

- [x] 1.1 Finalize the audit tool with full dependency-preflight totals, per-field blockers, and blocker combinations; verify deterministic fixture cases and reconcile accepted-name counts against totals on the pinned Main/Extras snapshots.
- [x] 1.2 Record the restored implementation baseline and snapshot revisions, label reverted-prototype evidence as historical, and document the 70% target, denominator, limitations, and ranked blockers; verify README/docs no longer present reverted results as current support.

## 2. Generic operation compiler contract

- [x] 2.1 Implement the non-evaluating lexer, complete-construct accounting, ordered operation registry, and phase-aware diagnostics as an independently testable unit; verify quote/comment/block boundaries, trailing statements, unsupported expansions, and deterministic output without changing live hook dispatch yet.
- [x] 2.2 Add app-independent rename, existing empty-config initialization, and guarded optional-import recognizers with read/write dependency checks; verify unrelated app/profile names, mixed operations, all-skip hooks, preserved ordering, unsafe imported-data dependencies, and rejection of altered guards or extra effects. Document the supported grammar and extension contract.

## 3. Deterministic download names and inert executable payloads

- [x] 3.1 Compile and freeze safe download names, URL fragments, query handling, paired digests/extraction declarations, and direct EXE retention; verify fragment traversal, encoded separators, duplicate filenames, multiple downloads, and idempotent saved-manifest compilation.
- [x] 3.2 Integrate frozen naming into the managed backend, update its matching contract, and document the behavior; verify controlled HTTP download/hash tests, no installer execution, ZIP regressions, release verification, and the coverage change before checkpointing.

## 4. Ordered hook execution in both installation phases

- [x] 4.1 Connect pre_install and post_install fields to the operation compiler and execute ordered frozen actions with phase and layout validation; verify rename chains, zero-action skipped hooks, current-alias checks, persistence preservation, missing files, overwrite rejection, and reparse-path rejection in Windows tests.
- [x] 4.2 Qualify Bitwarden and SpaceSniffer plus unrelated synthetic manifests through the generic pipeline in fresh disposable VM Stores; verify download, installation, launch, reset/persistence, selective import omission, and unchanged measured host sentinels. Update field/backend docs and record actual evidence and coverage; ensure no app-specific production dispatch remains.

## 5. Managed archive extraction

- [x] 5.1 Implement a Store-confined extraction unit using standard ZIP/tar/gzip readers and an exact reviewed pure-Go 7z dependency pin with license attribution; verify real fixtures for ZIP/nupkg, 7z, tar, and tar.gz plus traversal, case collision, links, malformed content, and resource-limit failures before integrating it.
- [x] 5.2 Integrate extraction with the frozen request and backend, preserving checked extract_dir/extract_to and paired multi-download behavior; verify supported container contents, misleading EXE-to-7z names, cancellation/failure containment, and absence of external helper fallback. Document format limits and verify a matching package build.
- [x] 5.3 Run representative real package installs for newly supported archive families in the disposable VM and rerun the pinned coverage audit; record installation evidence separately from static acceptance and checkpoint the completed extraction slice.

## 6. Additional reusable local operations

- [x] 6.1 Implement bounded package-local copy, directory creation, and literal default-file initialization in separate operation units; verify packaged defaults, missing/existing persisted data, cross-operation dependencies, dynamic-content rejection, and pre/post lifecycle layouts using unrelated fixtures.
- [x] 6.2 Integrate the new operations through frozen plans, document supported operand/guard forms, and verify representative manifest installations and Store isolation in the VM; rerun the audit and record measured gains before checkpointing.

## 7. Integrated qualification and product documentation

- [x] 7.1 Run the full Go regression suite, vet, and complete distribution verification after integration; verify a fresh matching VM Store using representative mixed payload/hook cases and report remaining runtime qualification limits.
- [x] 7.2 Publish the final combined and per-bucket full-preflight results against the same pinned snapshots, compare with the 70% target, and provide ranked remaining blockers with non-overlapping estimates where possible; verify totals and accepted-name lists and explicitly report any shortfall without treating the target as a mandatory pass gate.
- [x] 7.3 Finish the concise product README and docs/ index, placing architecture, development, compatibility mechanics, audit methodology, expansion priorities, and qualification evidence under docs/; verify examples, links, source revisions, and the distinction between current support, prototype history, and published releases.

## Workflow follow-up

- Review or verify the completed change before requesting archive; do not archive automatically.
- If later authorized and archive succeeds, run `pspec state clear --change add-generic-hook-operations` in this repository and report cleanup separately. Never clear state on failed/cancelled archive.
- Every push still needs fresh explicit authorization.
