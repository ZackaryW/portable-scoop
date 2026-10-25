# Install from the internet

Requires a published Windows x64 release in `ZackaryW/portable-scoop` and
64-bit Windows PowerShell 5.1 FullLanguage. Go and an existing Scoop installation
are unnecessary. Store initialization requires local NTFS with junction support.
Git is needed later for Git-backed buckets; see [setup](setup.md).

Open PowerShell:

```powershell
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/get.ps1 | Invoke-Expression
```

The wizard asks for **Host** or **Store** and a destination folder. Host is the
central launcher: it finds registered stores and delegates commands to them.
Store is an independent portable installation containing its own packages and
state. Install Host once and repeat Store setup for each independent environment.
Choose empty folders; existing files are never overwritten by this installer.

Host setup asks whether to add its folder to user PATH (default yes). With that
choice, `psp` and `portscoop` work immediately in this console and in newly opened
consoles. Otherwise use the executable's full path. Store setup leaves PATH
alone; invoke its `psp.exe` directly or register it with a Host.

For explicit options, run the downloaded script in a child scope:

```powershell
$get = Invoke-RestMethod https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/get.ps1
& ([scriptblock]::Create($get)) -Mode Host -Destination 'C:\Tools\portscoop' -AddToPath -NonInteractive
& ([scriptblock]::Create($get)) -Mode Store -Destination 'F:\Portable\Work' -Name Work -NonInteractive
& ([scriptblock]::Create($get)) -Mode Store -Destination 'G:\Portable\Personal' -Name Personal -NonInteractive

psp env register 'F:\Portable\Work' --name work
psp env register 'G:\Portable\Personal' --name personal
psp env default work
psp env list
psp --env personal list
& 'F:\Portable\Work\psp.exe' list
```

In scripted Host setup, PATH changes require `-AddToPath`. `-NoPath` also skips
the interactive PATH question. Store setup can explicitly register itself with
`-RegisterAs work -SetDefault`; registration is never implicit. `-SetDefault`
requires `-RegisterAs`. `-NonInteractive` requires both mode and destination.

To select a release, add `-Version 0.1.0` (substitute a published version).
This downloads the worker script from tag `v0.1.0` and that release's assets.
Without a version, the worker comes from `main` and the archive comes from
GitHub's latest stable release. Prereleases require an explicit version.
To pin the entry script too, fetch `get.ps1` from the same version tag.

The entry script downloads `scripts/install.ps1`. The worker downloads
`portscoop-windows-amd64.zip.sha256` and `portscoop-windows-amd64.zip` over HTTPS,
checks SHA-256 before extraction, rejects unsafe ZIP paths, then runs the
release's `setup.ps1` in Windows PowerShell 5.1. Setup checks the release's
artifact inventory before copying and initializing it. The script trusts this
repository and its release publisher; the checksum is not a digital signature.
Downloaded temporary files are removed after setup. The installer never changes
execution policy itself or invokes the ordinary Scoop installer.

If no matching release exists, setup reports the download failure without
creating a destination. Installation does not upgrade or replace an existing
Host or Store. See [setup](setup.md) for manual extraction, removal, and rollback.

## Build and publish

```powershell
.\scripts\package.ps1 -Version 0.1.0
```

This creates the versioned archive in `dist/`, plus upload-ready fixed-name ZIP
and checksum assets in `dist/release-assets/0.1.0/`. The ZIP includes both
executables, the pinned Scoop payload, setup scripts, and documentation. Upload
both fixed-name files to the matching GitHub release; do not upload only the
GitHub-generated source ZIP.

The Windows workflow tests each main-branch push and pull request. Pushing a
`vMAJOR.MINOR.PATCH` tag also builds and publishes its assets after validation;
tags with a prerelease suffix create prereleases. Publish source on `main` before
advertising the unversioned one-liner. The release must contain the matching
tagged installer scripts for versioned installation.
