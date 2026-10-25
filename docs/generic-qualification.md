# Generic compatibility qualification

The generic implementation uses operation syntax and checked paths, with no
application-specific production handler. Actual app manifests are qualification
inputs. All real installation checks ran in the disposable VMware Windows guest;
ordinary physical-host Scoop was not used for application operations.

| Slice | Development package | Evidence |
| --- | --- | --- |
| Ordered hooks | `0.3.1-operations-dev2` | [Bitwarden, SpaceSniffer, synthetic hooks](evidence/generic-hooks-2026-10-25.json) |
| Managed extraction | `0.3.1-operations-dev4` | [7z, tar.gz, nupkg apps](evidence/archives-2026-10-25.json) |
| Local defaults | `0.3.1-operations-dev5` | [Rufus, NUnit, mixed defaults and two Stores](evidence/local-operations-2026-10-25.json) |
| Final integrated build | `0.3.1-operations-dev6` | [Fresh Store, mixed operations and regression checks](evidence/final-operations-2026-10-25.json) |

Each report records its package SHA-256 and implementation provenance. The local
operations package preceded a subsequent archive-ancestor validation fix; that
report explicitly records the difference. These are development builds, not
published release claims.

The final build includes the ancestor validation fix and repeats the local-default
qualification in fresh Stores: Rufus and NUnit, synthetic package-local copy,
directory and pre/post persisted writes, reset preservation and second-Store
isolation. Full Go regression and vet passed; the final package passed distribution
verification and prepared-backend download/action checks. The earlier format and
GUI checks remain attributed to their specific builds above.

Reproduction scripts are under `tests/application/`: `qualify-generic-hooks.ps1`,
`qualify-archives.ps1`, and `qualify-local-operations.ps1`. Each requires a fresh
root, a matching `release.zip`, and the named manifests from the pinned buckets.
Scripts using synthetic payloads also need `probe-server.exe`, built from
`tests/backend/server`. They refuse to run outside VMware and leave Stores and
JSON/transcripts for inspection. None edits persistent PATH.

The checks establish the observed installation, file contents, reset behavior,
launch/CLI results and named sentinels. They do not establish full UI behavior,
account sync, every application feature, or every possible runtime write. Rufus
was installed and its defaults inspected; no device-writing operation was run.
NUnit help was launched without executing third-party tests.

Bitwarden's required package rename was executed; its guarded optional import
from host AppData was reported skipped. The compiler never reads the host profile
to classify that operation. Synthetic tests cover other names, changed guards,
unknown tails and dependencies on skipped data.
