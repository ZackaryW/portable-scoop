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

Portscoop does not guarantee backward compatibility with older releases or Store
formats. A breaking release may require a fresh Store initialized from its complete
verified distribution. Legacy support, migration, and in-place downgrade are not
release requirements. This release requires schema/protocol 1 and backend patch
`portable-fields-v1.0`; mismatched launchers and backends fail verification.

Retain the old Store separately if its packages or persisted data are still needed.
Returning to an earlier release means using its matching Store, not replacing
binaries inside a newer Store. Do not initialize over an existing Store or assume
that copying its internal state into a fresh Store is supported.

The disposable VMware qualification fixture `tests/backend/qualify-release.ps1`
creates a compatible local `fixture/tool` ZIP package and dependency, then installs,
lists, updates, resets, cleans, relocates, and uninstalls it across two roots.
The fixture server, generated manifests, and logs remain in its guest test folder
for inspection; it is not a script to run against a normal user's environment.
# Deploying management and execution launchers

See [Updating Portscoop](self-update.md) for installation-local `psp self-update`,
ownership requirements, compatible launcher updates, and recovery. Older releases
without the command need a fresh complete deployment first. The updater is not
runtime-qualified yet.

Releases contain three launchers: identical management executables
`portscoop.exe`/`psp.exe`, and the separate `pspe.exe` execution executable. New
Host, standalone Store, and Host-created default Store deployments receive all
three. Only an explicitly requested Host directory is added to user PATH;
individual Store shim directories are not added.

Deploy a complete verified release into an empty folder. For a breaking release,
initialize a fresh Store and register/select it with that release's Host. Reinstall
supported applications there. Keep the prior Store separate for any data you need
to recover; there is no automatic migration of its internal state.

Missing execution support or a backend revision mismatch is a deployment error.
Do not copy only the new `pspe.exe` or management executables into an old Store as
an upgrade procedure. Mixing Host binaries with a different `release.json` also
invalidates its source payload for Store initialization.
