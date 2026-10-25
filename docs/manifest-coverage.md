# Manifest coverage

**2,517 of 4,148 manifests pass full install preflight (60.7%)** on the pinned
2026-10-25 Main and Extras snapshots. This is **891 more** than the restored
baseline of 1,626 (39.2%).

The aspirational target was 70%, or at least 2,904 entries. **The result is 387
entries short.** Required behavior was not skipped to inflate the count.

| Bucket | Entries | Own manifest passes | Full install preflight passes | Rate |
| --- | ---: | ---: | ---: | ---: |
| Main | 1,700 | 1,310 | 1,296 | 76.2% |
| Extras | 2,448 | 1,233 | 1,221 | 49.9% |
| **Total** | **4,148** | **2,543** | **2,517** | **60.7%** |

Twenty-six entries pass their own fields but fail dependency preflight; the
headline excludes them. Counts represent bucket/name entries, not unique apps.
The [final JSON report](evidence/manifest-coverage-final-2026-10-25.json) includes
all accepted names, per-field blockers, disjoint blocker combinations and closure
failures. These results describe the source implementation, not every published
release. Use a complete matching distribution.

## What was verified

Preflight measures declarations and dependency closure only. It does not download
or run apps, establish URL availability, inspect actual archive content, or prove
application portability. In particular, a generic `.7z` name can hide unsupported
installer content that will fail at runtime without being executed.

Separate disposable-VM qualification covers:

- [Generic hooks](evidence/generic-hooks-2026-10-25.json): Bitwarden 2026.10.1 and
  SpaceSniffer 2.2.0.27 install, reset/persistence and root-scoped detached launch;
  unrelated synthetic mixed-phase and all-skip hooks; measured host sentinels.
- [Archive formats](evidence/archives-2026-10-25.json): `file` 5.45 (7z), `chroma`
  2.27.0 (tar.gz), and `lxc` 6.10.0 (nupkg) installation and CLI launch.
- [Local defaults](evidence/local-operations-2026-10-25.json): Rufus and NUnit
  defaults, packaged-default copy and post-install persistence, customized data
  preservation, and a measured unchanged second Store.
- [Final integrated build](evidence/final-operations-2026-10-25.json): repeated
  mixed operations in fresh Stores after the archive-ancestor validation fix,
  plus full regression, vet and distribution verification.

See [qualification details](generic-qualification.md) for build provenance and
limits. Passing preflight does not mean 2,517 runtime-tested applications.

## Remaining priorities

These are **disjoint own-manifest blocker groups** from the final audit. Each
count is an upper bound on the entries in that group, not a promised gain: a new
handler must still prove portability, and dependencies may remain blocked.

| Exact blocker set | Entries | Next investigation |
| --- | ---: | --- |
| `pre_install` | 396 | Generic literal bindings, bounded arrays and grouped defaults; distinguish required host effects |
| `url` | 201 | Additional container formats and deterministic names; extraction-only MSI needs its own contract |
| `post_install` | 99 | Reusable local transforms and grouped operations, without arbitrary script evaluation |
| `installer` | 97 | Identify extract-only/local operations; executing installers remains unsupported |
| `innosetup` | 92 | Evaluate a managed extraction-only reader separately |
| `bin` | 67 | Classify alias collisions, wrapper arguments and target forms |
| `pre_install` + `url` | 50 | Both payload handling and hook semantics must be solved |
| `pre_install` + `post_install` | 47 | Account for both phases and their data dependencies |
| `shortcuts` | 47 | Portable alias naming, nested names and icon-only declarations |

The three hook-only groups above contain 542 entries in total. Their scripts are
not necessarily portable or representable by a single safe operation. Subsequent
changes should audit operation forms and full dependency closure before claiming
new coverage. Larger MSI/Inno work and new script grammar need reviewed contracts.

## Reproduce the measurement

- Portscoop: `e98d611115c26f8716f856ec99d4f24a781f43a9`.
- Manifest policy: `portable-fields-v1`; backend: `portable-operations-v1.0`.
- Main: `990163c94e827a8a3966f643e86a4861aa13666e`.
- Extras: `6c4f7709ed26a7979bad9e48f73ce549054a45b6`.

The [audit tool](../scripts/manifest-audit/main.go) calls production compilation
and full install preflight for Windows x64. It reads separate clean bucket
checkouts and never uses ordinary Scoop. Unqualified dependencies resolve Main
before Extras; other buckets are unavailable. Rejected-field counts overlap;
blocker-combination counts do not.

```powershell
go run ./scripts/manifest-audit -main C:\audit\Main -extras C:\audit\Extras -output C:\audit\coverage.json
```

Use the pinned bucket revisions and implementation to reproduce these totals.
The tool refuses dirty buckets and uncommitted compiler/backend changes. Later
source revisions can produce different results and are recorded in the output.

## Checkpoint evidence

| Checkpoint | Full preflight passes | Change |
| --- | ---: | ---: |
| [Restored baseline](evidence/manifest-coverage-baseline-2026-10-25.json) | 1,626 | — |
| [Frozen downloads / inert EXEs](evidence/manifest-coverage-downloads-2026-10-25.json) | 2,072 | +446 |
| [Generic ordered hooks](evidence/manifest-coverage-hooks-2026-10-25.json) | 2,150 | +78 |
| [Managed archive readers](evidence/manifest-coverage-archives-2026-10-25.json) | 2,472 | +322 |
| [Local copy/directory/default units](evidence/manifest-coverage-final-2026-10-25.json) | 2,517 | +45 |

The older [1,739-entry prototype report](evidence/manifest-coverage-2026-10-25.json)
and [prototype Bitwarden qualification](bitwarden-qualification.md) belong to
reverted commit `574089a`. They are historical and are not evidence for this
implementation. The generic implementation has the fresh evidence linked above.
