# Historical Bitwarden prototype qualification

**Historical evidence only:** implementation 574089a was reverted by e561421. These results do not establish current Bitwarden support or qualify the generic replacement.

On 2026-10-25, Bitwarden **2026.10.1** was installed and launched in the disposable Windows VMware guest using a fresh Store at `C:\Users\Public\Portscoop-Bitwarden-20261025\store`.

The local development distribution was `0.3.1-bitwarden-dev1`, with backend `portable-fields-v1.1` and the implementation committed as `574089afb898f619bf1ec9dcd51820f6a1ca9d7a`. This is development evidence, not a published release announcement.

The manifest was copied unchanged from Extras revision `6c4f7709ed26a7979bad9e48f73ce549054a45b6` into the fresh Store's local Extras bucket. The executable was downloaded from the manifest's upstream GitHub release URL. This avoids bucket drift while exercising real download, hash verification, installation, reset, persistence, and shim execution.

## Observed results

The [machine-readable evidence summary](evidence/bitwarden-2026-10-25.json) records the measured checks and scope.

- Downloaded and installed EXE SHA-256: `db04fc8ae05b71b5ccd3292cc95c5f47957a19fd0aa74447944333be7a4b7d8c`.
- Preflight reported the local rename and explicit skip of the optional host AppData import.
- Installation succeeded, produced `Bitwarden.exe`, and created Store-owned launch aliases.
- `psp reset bitwarden` preserved a sentinel in `scoop\persist\bitwarden\bitwarden-appdata`.
- `pspe Bitwarden` returned successfully; a Bitwarden process ran from the new Store. Electron subprocesses ran from its private `scoop\tmp` directory and used the package's `bitwarden-appdata` path.
- Portable profile/cache files appeared under Store persistence.
- The guest's `%APPDATA%\Bitwarden` file fingerprint, ordinary `%USERPROFILE%\scoop` file fingerprint, and user/machine PATH values matched their pre-run values.

No account login, real vault import, synchronization, auto-update, relocation, or exhaustive filesystem/registry monitoring was performed. These observations establish installation and initial portable-profile startup for this version, not complete runtime sandboxing.

## Current status

The generic operation implementation must be qualified again in a fresh VM Store. The prototype distribution and its logs remain local investigation artifacts; they are not a supported release.
