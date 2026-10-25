# Setup and two-environment use

For a Scoop-style PowerShell download command with Host/Store selection and a
custom destination, see [internet installation](internet-install.md).

Extract a verified Windows x64 release archive. No Go compiler is needed on the
target. The initial supported runtime is Windows PowerShell 5.1 FullLanguage on
local NTFS. Initialization tests x64, locking, and junction support before making
a root ready. PowerShell 7, UNC, FAT/exFAT, and arbitrary manifest scripts are not
qualified. Git-backed bucket operations additionally require standalone Git at
`%ProgramFiles%\Git\cmd\git.exe`.
Managed bucket operations use private Git integration settings; public HTTP(S)
and explicit local repositories are supported. User Git credential helpers,
global config, hooks, and templates are not imported.

From the extracted release folder, choose an empty central destination:

```powershell
.\setup.ps1 -Mode Central -Destination C:\Tools\portscoop
# Explicit optional user-PATH integration:
.\setup.ps1 -Mode Central -Destination C:\Tools\portscoop -AddToPath
```

These are alternatives for a fresh destination. Setup refuses unrelated
non-empty directories. `-AddToPath` changes only the requested central directory's
user-PATH entry; open a new terminal afterward. Without that option, invoke the
central executable by its full path.

To create the first default Store automatically during central setup, add
`-CreateDefaultStore`. This initializes the Host's `store` subfolder and selects
its `default` registration. After Host-only setup, `psp env bootstrap` performs
the same operation. Existing defaults are preserved. Package catalogs can then
be added with `psp bucket add`.

Create two portable environments without changing host PATH or registrations:

```powershell
C:\Tools\portscoop\psp.exe env init 'F:\portable\work' --name Work
C:\Tools\portscoop\psp.exe env init 'G:\portable\personal' --name Personal
```

Alternatively use `setup.ps1 -Mode Portable -Destination 'F:\portable\work'
-Name Work` from an extracted release. Portable setup refuses `-AddToPath`.
Each initialization generates its own identity and private package state.

Opt into central discovery explicitly:

```powershell
C:\Tools\portscoop\psp.exe env register 'F:\portable\work' --name work
C:\Tools\portscoop\psp.exe env register 'G:\portable\personal' --name personal
C:\Tools\portscoop\psp.exe env default work
C:\Tools\portscoop\psp.exe --env personal list
& 'F:\portable\work\psp.exe' list
```

The central launcher requires a selector or explicit default even when only one
environment is registered. A portable launcher rejects a selector for another
identity. A disconnected default reports unavailable; it never silently chooses
another available environment. A copied root retains its identity and is rejected
when central discovery finds both copies. Initialize a fresh root for an
independent installation. Directory moves require re-registration; see
`relocation.md` for preparation and interrupted-operation recovery.

Ordinary `scoop` keeps its existing commands, paths, and package state. Portscoop
uses `portscoop`/`psp`, fixed managed backend paths, and a private child environment.
It neither substitutes ordinary Scoop dependencies nor falls back to its command.
Supported package operations do not register Start Menu shortcuts or host modules
or change persistent host environment variables. Applications themselves can
still write runtime data outside the root. Read `portable-policy.md` before
choosing packages; most script-heavy Scoop manifests are intentionally rejected.

Remove registrations without deleting portable data:

```powershell
C:\Tools\portscoop\psp.exe env unregister work
C:\Tools\portscoop\psp.exe env unregister personal
```

For central removal, remove exactly its directory from user PATH if you explicitly
added it, then remove the central release deployment. Preserve the portable roots
and any remaining host aliases. Never recursively remove a portable root as part
of central unsetup. No ordinary-Scoop uninstaller is involved.

For launcher rollback, extract the previous compatible release to a new central
directory, verify it, and update only the explicit central PATH entry. Keep the
old deployment until the replacement works. Portable rollback must preserve
`environment.json`, apps, persistence, registry IDs, and operation state. Replace
only the two launchers and managed engine/adapter payload from a compatible
release. This first state contract requires schema/protocol 1 and backend patch
`portable-fields-v1.0`; different patches/pins fail verification rather than attempting
an unsupported state downgrade. Keep an offline root backup before a release
change. There is no arbitrary package-data rollback guarantee.

The disposable VMware qualification fixture `tests/backend/qualify-release.ps1`
creates a compatible local `fixture/tool` ZIP package and dependency, then installs,
lists, updates, resets, cleans, relocates, and uninstalls it across two roots.
The fixture server, generated manifests, and logs remain in its guest test folder
for inspection; it is not a script to run against a normal user's environment.
# Adding execution support to an existing Store

New releases contain three launchers: identical management executables
`portscoop.exe`/`psp.exe`, and the separate `pspe.exe` execution executable. New
Host, standalone Store, and Host-created default Store deployments receive all
three. Only an explicitly requested Host directory is added to user PATH;
individual Store shim directories are not added.

An older Store lacking `pspe.exe` continues to support its compatible management
protocol. New Host `pspe` reports the missing execution support and does not
silently update it or fall back to another application location.

For a compatible Store launcher update:

1. Extract a new verified release into a separate empty folder and run its
   `portscoop.exe __portscoop_verify_release` command. Compare the Store's
   `.portscoop/backend/backend.json` revisions with that release's
   `backend/backend.json`. This release requires matching upstream and patch
   revisions; it does not migrate a different engine patch.
2. Close Store application sessions and wait for package operations to finish.
   Back up the installed management executables and any existing `pspe.exe`
   outside the Store's shim/engine directories.
3. Copy the verified release's three executable files beside the Store's existing
   launchers. Keep `.portscoop/environment.json`, `.portscoop/state`, backend
   metadata, `scoop`, packages, persistence, and Host registry entries intact.
   Never copy another Store's identity or initialize over an existing Store.
4. Check the three launcher versions, run `psp list`, and exercise a qualified
   installed shim with `pspe`. Registry/default selection and the UUID stay intact.

To roll back, restore the backed-up compatible management executables and prior
execution executable. If none existed, remove only the newly added `pspe.exe`.
Do not delete portable data. A new Host then reports missing execution support
while compatible management operations continue to work.

For a Host update, deploy the complete verified release to another empty folder
and select its launcher path. Host registry data remains separate; retain any
existing `store` directory and its registration. Replacing only Host binaries
without matching `release.json` would invalidate its source payload for future
Store initialization.

