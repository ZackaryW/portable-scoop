# Manifest coverage

## Download checkpoint (2026-10-25)

Frozen naming and inert EXE retention raise full install-preflight acceptance to
**2,072 / 4,148 (50.0%)**, up 446 from the restored baseline: Main 1,105 / 1,700;
Extras 967 / 2,448. This is static acceptance, not an installation success rate.
[Evidence](evidence/manifest-coverage-downloads-2026-10-25.json) pins the exact
implementation and the same bucket snapshots. Hook and extraction expansion
remain pending at this checkpoint; the 70% aspiration is unchanged.

The restored baseline passes **1,626 of 4,148 full install preflights (39.2%)**, measured on 2026-10-25. The goal for the generic compatibility change is **70%** of combined Main and Extras: at least 2,904 entries in this snapshot. It is a goal, not a mandatory gate or permission to skip required behavior.

| Bucket | Entries | Own manifest passes | Full install preflight passes |
| --- | ---: | ---: | ---: |
| Main | 1,700 | 867 | 854 |
| Extras | 2,448 | 780 | 772 |
| **Total** | **4,148** | **1,647** | **1,626** |

Twenty-one entries pass their own declaration checks but fail dependency preflight. The headline excludes them. Entries are counted as bucket/name, not unique apps. Examples passing this baseline include ripgrep, fd, fzf, zoxide, and SpaceSniffer. Bitwarden is not supported in this baseline.

## Reproduce the baseline

- Portscoop: `e561421d85eaee0718f89263305d93e0aba64763`; policy `portable-fields-v1`, backend `portable-fields-v1.0`.
- Main: `990163c94e827a8a3966f643e86a4861aa13666e`.
- Extras: `6c4f7709ed26a7979bad9e48f73ce549054a45b6`.

The [baseline report](evidence/manifest-coverage-baseline-2026-10-25.json) records accepted names, overlapping rejected-field counts, disjoint blocker combinations, and dependency failures. The [audit tool](../scripts/manifest-audit/main.go) calls production compilation and full install preflight. It reads separate clean bucket checkouts and never accesses an ordinary Scoop installation. Unqualified dependencies resolve Main before Extras; other buckets are unavailable.

Check out the recorded bucket revisions in independent directories, then run from the repository root with the audit tool supplied alongside this report:

```powershell
go run ./scripts/manifest-audit -main C:\audit\Main -extras C:\audit\Extras -output C:\audit\coverage.json
```

The tool refuses dirty bucket checkouts and uncommitted compiler/backend changes. Later implementation revisions can produce different results and are recorded in the output.

## Expansion priorities

The baseline has 971 entries blocked only by URL handling, 317 by URL plus pre-install hooks, 253 by pre-install hooks alone, and 42 by post-install hooks alone. These are disjoint own-manifest blocker groups, not guaranteed additional full-preflight passes.

Priorities are deterministic download naming, safe archive formats, then reusable local hook operations. Report measured gains after each verified checkpoint. Extraction-only MSI/Inno support and alias conflicts are further candidates requiring reviewed behavior; none may be counted supported merely by bypassing their required effects.

## Interpretation and historical evidence

Preflight acceptance does not establish URL availability, archive contents, qualified shim handlers, app correctness, or runtime data portability. No apps are downloaded or run by the audit. Actual installs and launches require separate qualification evidence.

The [1,739-entry prototype report](evidence/manifest-coverage-2026-10-25.json) and [Bitwarden VM report](bitwarden-qualification.md) belong to reverted commit `574089a`. They are explicitly historical and must not be presented as current support. The generic replacement requires a fresh audit and qualification run.
