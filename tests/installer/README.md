# Installer verification

Run the bootstrap contract checks in Windows PowerShell 5.1:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File tests/installer/bootstrap-tests.ps1
```

These checks replace network calls inside the test process and supply a minimal
fixture ZIP whose setup script writes only a temporary receipt. They exercise
the real entry and download/extraction scripts, including `Invoke-Expression`,
version selection, option forwarding, checksum failure, existing destinations,
and ZIP traversal rejection. Host and Store wizard defaults, as well as scripted
omission of a destination, must install directly in the current folder. No real
Host PATH writes or Scoop operations run. The fixture setup refuses destinations
outside its temporary test root.

`qualify-release.ps1` runs only in the disposable VMware Windows guest. Build a
real release with `scripts/package.ps1`, then create a bundle ZIP containing:

```text
get.ps1
scripts/install.ps1
host-state.ps1                  # from tests/backend/
portscoop-windows-amd64.zip     # from dist/release-assets/<version>/
portscoop-windows-amd64.zip.sha256
```

Transfer that bundle and the qualifier to the guest and run:

```powershell
powershell.exe -NoProfile -NonInteractive -ExecutionPolicy Bypass -File C:\Tests\qualify-release.ps1 -Bundle C:\Tests\installer-bundle.zip -Results C:\Tests\installer-results.json
```

The qualifier creates a new `C:\Users\Public\PortscoopInstaller-<UUID>` folder,
redirects download calls to the supplied actual release, and performs real Host
and Store setup. It checks bare `psp env list`, two identities, direct portable
execution, central delegation, optional registration/default, and refusal to
overwrite an existing store. Fresh empty stores return the pinned Scoop backend's
empty-app warning and exit code 1 for `list`; the test checks that response.

Host registration uses a private LOCALAPPDATA profile. The test temporarily adds
the new Host folder to guest user PATH and restores the exact previous value in
`finally`. The read-only Scoop sentinel and user/machine environment hashes must
match after restoration. Installations and a transcript remain for inspection.
After the test, use the full Host executable path and set LOCALAPPDATA to that
test's `host-profile` to inspect its private registrations.

This qualifies the release payload and installer behavior without a compiler in
the guest. It does **not** qualify the live GitHub endpoint: publication and a
separate live download smoke test are still required. See
[Windows evidence](evidence/windows-release.json) for the qualified artifact.

After publication, `qualify-live.ps1` tests the actual GitHub endpoints in the
disposable guest. Pass the guest path to `tests/backend/host-state.ps1` as
`-Sentinel`, an output JSON path as `-Results`, and the published version as
`-ReleaseVersion`. It automates console answers for the Host wizard while keeping
all network cmdlets real. It accepts the current-folder defaults for both Host
and Store, also installs a Store through the matching tag, and checks central
delegation. It restores the guest user PATH and retains installed
files and a transcript. See [live release evidence](evidence/live-0.1.0.json).
