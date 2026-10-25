# Updating Portscoop

Run the Host launcher to update its runtime and **all registered Stores**:

```powershell
.\psp.exe self-update --check
.\psp.exe self-update
.\psp.exe self-update --status
```

The Host checks Stores even when its own version is already current. Duplicate
aliases for one UUID are updated once. Offline, invalid, ambiguous or busy Stores
are reported individually; other reachable Stores continue. Registrations and the
default selection are preserved. Reconnect an offline Store and rerun the command.

`portscoop.exe` accepts the same commands. `pspe.exe` launches applications;
`psp update <app>` updates packages. Self-update does not upgrade installed apps.

## Scope and preservation

Host-managed Store updates replace all three launchers and their inventory-owned
backend together. Store UUIDs, apps, shims, buckets, cache, configuration, portable
path state and persisted data stay in place. Edited managed files, redirected
paths, unsupported metadata and new-file collisions stop that Store's update.

Invoking a Store's own `psp.exe` directly updates only that Store's launchers when
the candidate supports its retained backend. For a backend upgrade, register the
Store with a current Host and run that Host's `self-update`. Your working directory
does not select the update target; `--env` is rejected.

When upgrading from a Host version that predates registered Store updates, finish
that Host update first, then rerun `self-update` through the updated Host.

## Versions and outcomes

Use `--version <tag>` to select a published release explicitly; it also works with
`--check`. One invocation pins one release for every target. Newer installations
are never downgraded. Development versions require an explicit target.

`--check` reports availability without downloading payloads or changing files.
It does not establish payload compatibility. `--status` and `--recover` include
registered Stores when invoked through a Host; they cannot be combined with
`--check`, `--version` or each other.

Store results report completed replacement. The Host's **accepted** message means
its worker will finish after the launcher exits; inspect `--status` afterward.
Updates use verified archive checksums and inventories, private staging, backups
and bounded runtime/package locks. Downloaded candidate executables are not run
to perform replacement. Unrelated processes are never forcibly terminated.

| Outcome | Exit code |
| --- | --- |
| Check, current, completed Store update, accepted Host handoff | 0 |
| Status with active pending work and no failures | 3 |
| Failed, offline, invalid, uncertain or recovery-required target | 2 |
| Cancellation | 130 |

A partial result lists each target separately. A failed Store does not undo other
successful Store updates. Inspect the reported location and UUID when resolving
stale registrations; the updater does not remove or repair them automatically.

## Recovery

```powershell
.\psp.exe self-update --status
.\psp.exe self-update --recover
```

Recovery restores each recorded previous runtime, including its backend where
applicable. It does not retry an upgrade or modify apps. A live transaction owner,
damaged backups or changed identity prevents recovery. Reconnect interrupted
media at its recorded location before recovering.

If public launchers are damaged, run the retained `worker.exe` in the reported
transaction folder with `__portscoop_update_recover` and that exact absolute
transaction folder. This helper only accepts its own verified transaction.
Backups and helpers remain available after completion or restoration.
