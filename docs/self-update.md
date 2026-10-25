# Updating Portscoop

Use the management launcher in the installation you want to update:

```powershell
.\psp.exe self-update --check
.\psp.exe self-update
.\psp.exe self-update --status
# Select a published version explicitly:
.\psp.exe self-update --version v0.2.2
```

The version above illustrates the syntax; it is not a statement that the release
has been published. `portscoop.exe` supports the same commands. `pspe.exe` runs
applications; `psp update <app>` updates packages.

Self-update selects the folder containing the invoked management executable.
Your working directory, registered default, and other Stores do not change that
target. `--env` is rejected, including an explicitly empty value. To update a
different Store, invoke its own `psp.exe` by full path.

`--check` may be combined with `--version`. It reads release metadata without
downloading candidate code, creating update ownership, or changing your files.
Availability does not establish compatibility with a Store's retained backend.
`--status` and `--recover` cannot be combined with each other, `--check`, or
`--version`. Known downgrades are rejected. Automatic selection treats an equal
or newer installed semantic version as current; an uncomparable development
build requires an explicit published target.

## Ownership and replacement scope

The first release implementing this command requires a complete deployment into
an empty folder. An older executable cannot run a command it does not contain.
The internet installer still refuses non-empty destinations; it is not an
in-place updater. Keep older Stores separately when their data is needed.

The distribution contains an inventoried `runtime-update.json` descriptor. Hosts
retain the complete `release.json` source inventory and use `.portscoop-host/`
for their identity and update controls. That namespace does not identify a Store.
Store initialization writes `.portscoop/state/runtime.json`, binding its UUID,
version, and all three launcher hashes. Unknown or edited ownership must be
inspected; self-update does not adopt arbitrary executables or reconstruct an
inventory by guessing.

A Host update replaces its owned distribution, including the source backend and
release inventory used to create future Stores. It preserves the `store` child,
registrations, default selection, persistent environment, and unrelated files.
New-file collisions or changes to owned files stop preparation. Source copying
for Store creation excludes Host update controls and unrelated files.

A Store update replaces only `portscoop.exe`, `psp.exe`, `pspe.exe`, and its runtime
receipt. Its UUID, backend, packages, shims, buckets, cache, configuration, path
state, and persisted data remain in place. The candidate must explicitly support
the retained backend contract. Otherwise initialize a fresh Store from the new
complete release. Self-update does not migrate Store formats or promise backward
compatibility. Updating a Host does not update its default Store.

## Downloads and completion

Discovery uses the official `ZackaryW/portable-scoop` GitHub release API. Downloads
are pinned to one resolved tag and the Windows amd64 ZIP/checksum asset names.
HTTPS redirects are restricted to GitHub release infrastructure, with response,
time, entry-count, and expanded-size bounds. Verification covers the archive
SHA-256, safe extraction paths, complete inventory, descriptor, identical
management binaries, and all three launchers. Checksums share the publisher's
HTTPS trust model; they are not independent release signatures. Semantic version
ordering uses [Go's semver package](https://pkg.go.dev/golang.org/x/mod/semver).

Staging and backups live in a protected current-user-only transaction directory
on the installation's volume. A verified copy of the current management runtime
acts as the worker. Downloaded candidate executables are not run to perform the
update. The worker uses NUL streams and must escape external job restrictions
without changing those restrictions. A restrictive terminal or automation job
can reject this handoff; retry from an ordinary Windows terminal after inspecting
the reported status.

The invoking launcher reports **accepted**, then exits. This does not mean the
update finished. The worker waits up to 30 seconds for that exact process to exit,
then obtains its own update ownership, exclusive runtime lease, and (for a Store)
operation lock. Coordination waits are bounded at five seconds; startup handshake
waits are bounded at 15 seconds. Existing commands may finish; unrelated processes
are never force-terminated. Application preparation releases the Store mutation
lock after process creation. A foreground `pspe` runtime retains its runtime-use
lease until it exits, so it can delay launcher replacement without blocking
ordinary package operations.

New commands encountering pending state report the transaction and recovery
instructions before using mutable runtime files. `--status` and `--recover`
remain reachable. An uncertain acknowledgement retains the transaction; it never
automatically starts another worker.

| Outcome | Exit code |
| --- | --- |
| Check, already current, accepted handoff, absent/completed status | 0 |
| Pending status | 3 |
| Failure, uncertain handoff, interrupted/recovery-required status | 2 |
| Cancellation before accepted handoff | 130 |

## Recovery

```powershell
& 'F:\portable\work\psp.exe' self-update --status
& 'F:\portable\work\psp.exe' self-update --recover
```

Recovery restores the recorded previous runtime and reports that restoration as
a failed update, not as a successful upgrade. It validates the same identity,
relative operation set, staged source, and backups. It refuses damaged metadata,
unknown paths, redirected locations, or changed user files. It does not select
another installation if the recorded volume is absent. Reconnect interrupted
media at the recorded location before recovery; moving an unfinished transaction
is unsupported.

If the public launcher files are unavailable, run the retained `worker.exe`
inside the reported transaction folder with `__portscoop_update_recover` followed
by that exact absolute transaction folder. This private entry point only accepts
its own recorded transaction; it cannot target an arbitrary directory. Inspect
`journal.json` and the final `result.json` for the outcome. Backups and staged
helpers remain for inspection after completion or restoration; the updater does
not recursively delete installed directories. File-use failures trigger rollback
where possible and retain recovery-required state otherwise.

This implementation is not runtime-qualified. Automated tests, test authoring,
and disposable-VM qualification were deferred for this change. Compilation,
artifact inspection, and source review do not establish replacement or recovery
behavior on a live installation.
