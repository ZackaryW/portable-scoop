# Environment registrations

The host registry is `%LOCALAPPDATA%\portscoop\registry.json`. It stores
case-insensitive aliases, durable environment identities, verified location
hints, and an explicitly selected default. Registration requires a ready
environment marker. Initialize roots from a verified release as described in
`setup.md`; package operations use the constrained policy in `portable-policy.md`.

Once two portable environments are initialized:

```powershell
psp env register 'E:\Portscoop\work' --name work
psp env register 'F:\Portscoop\personal' --name personal
psp env list
psp env default work
```

`portscoop` and `psp` use the same command interface. Aliases use 1-64 ASCII
letters, digits, underscores, or hyphens and start with a letter or digit.
`WORK` and `work` refer to the same alias. An existing alias cannot silently
change to another installation identity. Defaults refer to identities, so a
second alias for the same environment retains the same default selection.

`env list` prints alias, identity, default marker, availability diagnostics, and
saved location. Availability checks at saved paths are bounded to two seconds
with at most four filesystem probes in flight. A wrong identity at a saved path
is distinguished from an unavailable location. Listing does not discard entries
or select a different default when media is disconnected. Drive rediscovery for
central command routing is implemented separately by the resolver.

If an environment's containing directory moves, refresh the registration:

```powershell
psp env register 'E:\Portable\work' --name work
```

Re-registration of the same alias and identity refreshes its location hints.
Registry updates use an OS-owned lock and atomic replacement of a fully written
JSON file. Corrupt/unsupported registries produce an error and remain available
for inspection; the CLI does not silently reset them. Update lock waits are
bounded to five seconds for these management commands.

Remove a host registration with:

```powershell
psp env unregister work
```

This removes the alias and, when it was the last alias for the default identity,
clears that default. Every portable file remains in place. Register/default/list/
unregister do not run Scoop, edit package state, or add portable shims to PATH.

An explicit `env bootstrap` request (including first-Store setup) keeps a reachable
default with the registered identity. If all saved locations for that default are
missing, it creates and selects the Host's `store` child instead, retaining the old
registrations for reconnection. If necessary, the new alias is `default-2`,
`default-3`, and so on. Invalid metadata, a mismatched identity, or another inspection
error fails setup rather than reporting success or replacing that selection.

Implementation tests inject temporary registry paths through the CLI's options
contract. Tests never select the developer's real host registry or Scoop root.

## Routing and delegation

Package routing delegates to the selected portable executable after identity and
protocol verification. Its adapted backend verifies and preflights each operation:

```powershell
psp --env work list
psp --env personal install tool
& 'E:\Portscoop\work\psp.exe' list
```

Put `--env` before the package command. Selection uses an explicit registered
alias first, otherwise the portable identity adjacent to the executable,
otherwise the explicitly configured host default. A direct portable invocation
without a selector does not read the host registry and works from any working
directory. A portable launcher rejects a selector for a different identity.
Missing/corrupt adjacent portable state fails instead of invoking a host default.

A central launcher requires a selector or configured default even with only one
registration. It never infers the target from the current directory or chooses a
different connected environment when the intended one is unavailable.

Central discovery checks saved paths and recorded drive-relative suffixes on
available local fixed/removable volumes. Volume GUIDs prioritize candidate
locations; the marker's installation UUID establishes identity. Enumeration has
a one-second budget, and candidate file probes have a two-second budget with at
most four probes in flight. No full-disk recursive search is performed. A move
to a different containing directory requires `env register` to refresh the hint.

All bounded candidates are checked, including those after a valid cache hit.
Equivalent canonical roots count once; distinct roots advertising one ID produce
an ambiguity error listing the conflicts. Raw folder copies retain their UUID.
Disconnect an unwanted duplicate or initialize a genuinely independent
environment with a new identity before using both copies separately.

After unique resolution, the central launcher verifies the portable launcher's
read-only handshake and forwards to a local-only endpoint. The endpoint rechecks
the expected UUID and protocol major before invoking its backend. Unsupported
metadata/protocol versions, changed identities, missing launchers, and unavailable
roots produce errors without falling back to another Scoop installation. Tests
cover protocol mismatch, identity replacement, selector removal, bounded
handshake output, and rejection of recursive central execution.
