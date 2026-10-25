# Portscoop

**Your Windows apps. Your folders. Independent portable environments.**

Portscoop (`psp`) brings Scoop packages into self-contained **Stores**: each owns its apps, buckets, cache, and persisted data. Keep work tools and personal apps separate, run a Store directly, or use an optional **Host** launcher to find your registered Stores.

- **Start where you are.** Setup defaults to your current folder. A Host can bootstrap its first Store automatically.
- **Keep environments independent.** Each Store has its own identity and package state; ordinary Scoop remains separate.
- **Launch with `pspe`.** A shim name opens the app detached. Add arguments to use it as a normal terminal command.
- **Use familiar manifests.** Portable declarations work directly; reviewed integrations become Store-local actions.

## Get started

Use 64-bit Windows PowerShell 5.1 and an empty folder on local NTFS. Bucket commands require standalone Git for Windows at `%ProgramFiles%\Git\cmd\git.exe`. You do not need Go or an existing Scoop installation.

```powershell
mkdir Portscoop
cd Portscoop
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
Invoke-RestMethod -Uri https://raw.githubusercontent.com/ZackaryW/portable-scoop/main/get.ps1 | Invoke-Expression
```

Choose **Host**, press Enter to use the current folder, and accept creation of the first default Store. Or choose **Store** for an independent environment that needs no Host registration. Adding the Host to PATH is optional.

Install a tool and run it from the folder you just set up:

```powershell
.\psp.exe bucket add main
.\psp.exe install ripgrep
.\pspe.exe rg --version
```

Use `psp` for package management and `pspe` for apps. With Host PATH integration, open a new terminal to use the short command names anywhere.

```powershell
psp env list
pspe rg -n "hello" .          # Terminal input/output and application arguments
pspe spacesniffer            # Detached launch, once installed
pspe --env work rg --help    # Launch from another registered Store
```

[Installation options](docs/internet-install.md) · [Create multiple Stores](docs/setup.md) · [Launch options](docs/application-execution.md)

## Package coverage

**2,517 of 4,148 manifests pass full install preflight (60.7%)**—891 more than the restored baseline. The audit covers pinned Scoop Main and Extras snapshots, Windows x64, and dependency checks. Generic portable operations now cover apps such as Bitwarden and SpaceSniffer, with managed ZIP, 7z and tar/gzip extraction.

Seven real packages received targeted VM checks, including Bitwarden, SpaceSniffer, Rufus and NUnit. The larger figure measures **preflight acceptance**, not thousands of runtime-tested apps. The 70% goal remains unmet; the coverage report explains the remaining 387-entry gap.

This compatibility expansion describes the current source checkout. The internet installer downloads a published release, which may predate these changes. See the [development guide](docs/development.md) for a matching source build.

Portscoop is experimental, and application execution is not sandboxed.
[Coverage report and accepted names](docs/manifest-coverage.md) · [Compatibility rules](docs/compatibility.md)

## Documentation

The [documentation index](docs/README.md) covers installation, multiple Stores, application execution, updates, architecture, compatibility internals, and development. Start there for technical details and qualification evidence.

## Develop Portscoop

The coordinator is written in Go with a pinned PowerShell backend. See the [development guide](docs/development.md) to build, test, and package all three launchers: `portscoop`, `psp`, and `pspe`.
