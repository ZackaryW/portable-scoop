# Scoop backend compatibility

## Source pin

The inspected upstream revision is
`e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f`. The exact archive URL,
SHA-256, and inspected source-file digests are recorded in
`backend/scoop/upstream.json`. Run `scripts/fetch-scoop.ps1` to fetch and verify
the inspection source under ignored `.work/upstream/`. Fetching does not run
Scoop or invoke its installer.

The archive SHA-256 is
`fa96b15950f3726d2c9a35493e192eefe56d2eddba718e8121c32f7a19257a0a`.
Checksums pin the reviewed input; they are not a signature or an upstream trust
guarantee. Backend releases must additionally record the adapter/patch revision
and digests of the resulting executable source artifacts.

## Integration inventory

All links below refer to the pinned revision, not a moving branch. These are
source observations; dynamic probes and the portable adapter are separate tasks.

| Area | Observed upstream behavior | Required portable integration |
| --- | --- | --- |
| Bootstrap/config | `core.ps1:1347-1372` loads host XDG/profile config unless an existing root config is found in the expected `apps/scoop/current` layout | Provision `scoop/config.json` before loading any backend source; verify config selection and fail instead of host fallback |
| Roots | `core.ps1:1375-1385` gives `SCOOP`, `SCOOP_GLOBAL`, `SCOOP_CACHE` precedence over config/defaults | Set all three explicitly in the child process; global storage remains private and global commands are rejected |
| Environment persistence | `system.ps1:Get-EnvVar`, `Set-EnvVar` open HKCU/HKLM Environment; `Add-Path`/`Remove-Path` call them | Replace the pinned system persistence layer with portable state-backed access and process-only updates; never load its registry writer in the adapted backend |
| Shim creation | `core.ps1:shim` calls `Add-Path` even for an ordinary shim; EXE metadata and CMD wrappers contain absolute paths | Apply portable environment layer and catalog generated shims for exact-root repair; do not expose shims on host PATH |
| Manifest env settings | `install.ps1:env_add_path`, `env_set`, `env_rm_path`, `env_rm` use persistent setters; `env_set` expands PowerShell strings | Apply portable environment layer; preflight supported variables/values and reject executable interpolation or unsupported root/control overrides |
| Shortcuts | `shortcuts.ps1:create_startmenu_shortcuts`, `rm_startmenu_shortcuts`, `shortcut_folder` target host Start Menu and COM shortcuts | Replace create/remove operations with suppressed host integration; do not call `shortcut_folder` in portable mode |
| PowerShell modules | `psmodules.ps1:install_psmodule`, `uninstall_psmodule`, `ensure_in_psmodulepath` create links and update persistent PSModulePath | Suppress host module registration/removal; retain package metadata without calling these host operations |
| Dependencies | `depends.ps1:Get-Dependency` resolves manifest and helper closure; `core.ps1:installed` searches user and global roots; helpers can be found on PATH | Resolve both roots inside the environment, preflight the complete closure, and constrain helper lookup so another Scoop cannot satisfy a package dependency |
| Commands/aliases | `commands.ps1:command_path` falls back to executable `scoop-*` shims when a built-in is absent | Portscoop recognizes a closed command set and invokes only verified built-in paths; no raw-command or shim-alias escape hatch |
| Package hooks/installers | `install.ps1:Invoke-HookScript`, `Invoke-Installer` evaluate scripts and run external installers | Reject unsupported hooks/installers before the dependency closure is installed and before uninstall actions execute |
| Implicit core update | `scoop-install.ps1:61-68` invokes core update when stale; `scoop-update.ps1` uses `is_scoop_outdated` even for app updates | Patch automatic core update paths, reject explicit core-update commands, and verify the immutable pin before execution |
| Current/persist links | `install.ps1:link_current`, `persist_data`, `New-DirectoryJunction` create junctions and hard links | Qualify NTFS behavior, catalog owned links, enforce containment, and rebuild supported links during relocation |
| Installer/bootstrap | The archive has source/assets but is not an installed environment; `bin/scoop.ps1` immediately loads core and discovers commands | Portscoop provisions a known layout, config, private global/cache/temp roots, and verified engine assets itself; do not run the standard host installer |

Pinned references:

- [Core](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/core.ps1)
- [System environment](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/system.ps1)
- [Package installation and links](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/install.ps1)
- [Dependencies](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/depends.ps1)
- [Shortcuts](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/shortcuts.ps1)
- [Modules](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/lib/psmodules.ps1)
- [Install command](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/libexec/scoop-install.ps1)
- [Update command](https://github.com/ScoopInstaller/Scoop/blob/e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f/libexec/scoop-update.ps1)

## Verification boundary

The source inventory confirms that process scoping and Scoop's
`use_isolated_path` alone do not meet the portable host-integration contract.
The backend must be adapted before real package operations are enabled.
Dynamic probes must run with isolated user/VM fixtures for any unadapted host
effects; temporary folders alone do not isolate HKCU or Start Menu state.

The isolated VMware probe passed on 2026-10-24 with Windows 11 Pro 10.0.26200 and
Windows PowerShell 5.1.26100.6584 on NTFS. It observed successful root-local config,
ZIP install/persistence/list/uninstall and confirmed changes to guest user PATH,
a manifest environment variable, user PSModulePath, and a Start Menu shortcut.
Guest machine PATH did not change. Host comparison found all 223 recorded Scoop
files and both persistent-environment hashes unchanged. See
`tests/backend/evidence/` for the observed results.

The adapted backend also passed the VMware install/list/update/uninstall probe
on 2026-10-24. `tests/backend/evidence/adapter-windows.json` records portable
environment persistence, suppressed shortcuts/modules, preserved package data,
and unchanged guest host-state and ordinary-Scoop sentinels. This is the adapter
fixture; full CLI, two-environment, and release qualification remain in group 9.

## Adapter revision portable-fields-v1.1

`scripts/prepare-backend.ps1` verifies the upstream inputs, uses PowerShell's AST
to replace named functions at recorded patch sites, and emits every resulting
file's digest. It refuses changed inputs or missing patch sites. The versioned
sources are in `backend/scoop/adapter/`; the original Scoop license remains in
the engine payload.

The replacement system layer records values in
`.portscoop/state/environment.json`, atomically replaces that file, and applies
them only to managed child processes. It never reads/writes persistent host
environment variables or broadcasts environment changes. Shortcuts are compiled
into Store-owned bin aliases; module declarations are rejected until portable
discovery exists. Missing shim targets fail instead of using
an executable from PATH. ZIP extraction always uses the Windows PowerShell
archive implementation. Git-backed bucket operations require standalone Git at
`%ProgramFiles%\Git\cmd\git.exe`; another Scoop's Git is not used as a helper.
Managed Git calls discard inherited `GIT_*` controls, disable system/global
config and terminal credential prompts, and use empty private hook/template
directories. Manifests cannot set Git control variables. This initial Git bucket
surface supports public HTTP(S) and explicit local repositories; user Git config,
credential helpers, templates, and hooks are not imported into it.

Engine and implicit bucket self-update functions reject execution. Package
manifests are resolved inside the selected root, preflighted with their closure,
flattened to the supported 64bit declaration, and frozen in the private request.
The patched manifest reader and installed-manifest writer consume this snapshot,
preventing a changed bucket from replacing a checked declaration before install.
Saved manifests from every affected installed version are checked before
uninstall/update/reset/cleanup. The recognized persisted-file hook runs only its
frozen typed action, with execution-time containment checks. Other hooks and
installers fail inside the patched engine if reached unexpectedly.

Verification checks the pin/patch/protocol, all recorded engine and adapter
artifacts, required libraries, root-local config, and unrecorded added files.
Missing or altered files produce a compatible-release repair diagnostic; there
is no system-Scoop fallback. The initialization prerequisite probe checks local
NTFS, OS file locking, junction creation, x64, and PowerShell 5.1 FullLanguage
before publishing an atomic ready marker.

Provisioning accepts an empty destination or this verified release payload.
It stages under the destination, preserves existing files on failure, and never
registers the root automatically. A failure before publication cleans its own
staging. A media failure during publication can leave unready partial state;
preserve that directory for inspection and initialize a new empty destination.

## Runtime and compatibility matrix

| Prerequisite or behavior | Qualified evidence | Scope |
| --- | --- | --- |
| Windows x64 | Whole release on Windows 11 Pro 10.0.26200, VMware20,1 | Windows 10 is a design target, not a tested release claim |
| Windows PowerShell 5.1 | Probe ran with 5.1.26100.6584 | PowerShell 7 is not advertised until explicitly tested |
| FullLanguage | Initialization runtime check and rejected constrained-language fixture | Requires PowerShell 5.1 FullLanguage |
| Local NTFS | Init, locking, junction/hardlink, directory move, interrupted repair, and actual VHD drive-letter change passed | FAT/exFAT/UNC are not qualified |
| No Go on target | Guest reports no Go; both CLI names work from CMD and PowerShell | Go is a build dependency only |
| Root-local config and plain ZIP workflow | Adapter and full CLI install/list/update/reset/cleanup/uninstall fixtures passed | Fixed-version ZIP subset only |
| Manifest subset | Go tests and guest rejection of dependency hooks, saved uninstall hooks, altered backend, and command bypasses | Arbitrary scripts/applications are not sandboxed |
| Host environment and shortcut isolation | Two-root matrix preserved the other root, ordinary-Scoop sentinels, persistent environment, and host integration | Supported package operations only |
| Git-backed buckets | Fixed standalone Git prerequisite with isolated config/hooks/templates | Public HTTP(S) or explicit local repositories; qualification report records the tested Git version |

The table above records qualification of the earlier ZIP adapter, not new
real-application certification. The current per-field policy, supported digest
algorithms and portable replacements are documented in `docs/compatibility.md`.
Controlled tests cover the new executor, frozen actions, native fixed arguments,
and relocation; no arbitrary script or application sandbox is claimed.
