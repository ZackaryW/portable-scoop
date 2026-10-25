# Upstream backend probe

## Per-field compatibility checks

`go test ./internal/compatibility ./internal/backend/scoop` includes the pure
compiler tests and controlled Windows PowerShell action tests. The latter runs
only owned temporary filesystem fixtures, with no application installation or
host registration. To also exercise the prepared backend's checksum functions:

```powershell
.\scripts\prepare-backend.ps1 -Destination .\.work\field-backend
.\tests\backend\compatibility-tests.ps1 -Root .\.work\field-action-fixture -Backend .\.work\field-backend
$env:PORTSCOOP_TEST_BACKEND_PAYLOAD = (Resolve-Path .\.work\field-backend).Path
go test ./internal/backend/scoop -run TestPreparedPayloadMatchesRecordedArtifacts
Remove-Item Env:\PORTSCOOP_TEST_BACKEND_PAYLOAD
```

Use fresh fixture directories. These checks are not a real-application VM
qualification. Historical reports below describe their recorded backend
revision. The current VM fixture checks shortcut aliases and rejects unsupported
module integration; it has not been rerun for this compatibility change.

## Unmodified upstream effects

This probe intentionally measures the unmodified engine's persistent user
environment, Start Menu, and PowerShell module integration. Run it only in a
disposable Windows user/VM. A temporary working folder or child process is not
sufficient isolation. Do not pass the confirmation switch on your normal account.

Prepare the verified source and a standalone fixture HTTP server:

```powershell
.\scripts\fetch-scoop.ps1
go build -o .\.work\probe-server.exe ./tests/backend/server
```

Copy the repository, its prepared `.work/upstream/Scoop-<revision>` source, and
`.work/probe-server.exe` into the disposable machine. It needs Windows x64,
Windows PowerShell 5.1 in FullLanguage mode, and NTFS temporary storage. The
prepared probe needs no Go installation or external network access there. Its
HTTP server binds only `127.0.0.1` and serves the disposable ZIP fixture.

Inside that machine, run:

```powershell
.\tests\backend\probe-upstream.ps1 `
  -UpstreamSource '.\.work\upstream\Scoop-e6aa3b366bdee8ed138c1e0f7b85192ebdd35d0f' `
  -ServerExecutable '.\.work\probe-server.exe' `
  -ConfirmDisposableMachine
```

The probe copies the verified source into its own expected Scoop layout and
creates a local bucket/ZIP fixture. It checks root-local config, plain-archive
install, shims, directory persistence, listing, and uninstall. The emitted path
points to an `observations.json` report; adjacent logs retain command evidence.
Host-effect observations are booleans, not dumps of your environment values.
Keep those artifacts for the compatibility review, then discard the test user/VM.

Observed results were collected on 2026-10-24 in the dedicated VMware testing VM
(Windows 11 Pro 10.0.26200, Windows PowerShell 5.1.26100.6584, NTFS, FullLanguage).
`evidence/upstream-windows.json` records a passing config/install/persist/list/
uninstall probe and the upstream user environment, module-path, and shortcut
writes. The guest had no pre-existing Scoop installation. The user explicitly
authorized this disposable VM run without a snapshot after encrypted-VM snapshot
authentication failed. No host folders were shared: the bundle and result ZIP
were copied through VMware Tools.

`evidence/host-comparison.json` records 223 unchanged host Scoop files and
unchanged user/machine environment hashes before and after the guest run. This
is evidence for the upstream compatibility probe, not qualification of an
implemented portable backend. Full logs remain in ignored `.work/vmware-testing/`.

The subsequent adapted-backend fixture also passed; see
`evidence/adapter-windows.json`. `probe-adapter.ps1` consumes a prepared backend
and fixture server in a fresh guest folder, suppresses host integration, and
checks install/list/update/uninstall with directory persistence.

For whole-release qualification, build `scripts/package.ps1` on the host. Stage
the resulting archive as `release.zip` alongside `qualify-release.ps1`,
`host-state.ps1`, and the compiled `probe-server.exe` in a fresh disposable guest
folder. Optionally include standalone Windows Go test binaries built with
`go test -c` for registry, relocation, runner, and resolver packages. Those
binaries run without a Go installation. Run `qualify-release.ps1` with Windows
PowerShell 5.1 and collect `results.zip` from that folder through VMware Tools.

The release suite creates its own host-profile registry, two portable roots,
ordinary-Scoop sentinels, a loopback ZIP repository, and a small guest-only NTFS
VHD for actual drive-letter changes. It temporarily tests the explicit central
user-PATH option and restores the prior value. Its `finally` block detaches only
that named VHD. It does not require shared host folders, host Scoop execution,
or a compiler inside the guest. Results include command logs and a machine-readable
observation report. Run it only in the authorized disposable VMware guest.

Bucket qualification requires standalone Git in the guest. The fixture can
consume `git-runtime.zip` staged from an existing standalone Git installation's
`cmd` and runtime directories, extracting it only under the disposable guest's
`ProgramFiles\Git` when no Git is present. No Git or other software is installed
on the host by this suite.

`evidence/release-windows.json` records the passing `0.1.0-dev.7` matrix on
Windows 11 Pro / PowerShell 5.1 with no Go toolchain. It includes standalone
MinGit 2.56.0.windows.3, two environments, ordinary-Scoop coexistence, both CLI
names from CMD/PowerShell, all supported command families, policy rejections,
actual NTFS VHD drive-letter reassignment, and the five compiled Windows suites.
`evidence/qualification.json` records the exact release SHA-256 and Git artifact
provenance. `evidence/host-final-comparison.json` confirms all 223 host Scoop
sentinels and both persistent-environment hashes remained unchanged through the
whole implementation and qualification run. Raw command logs are retained in
ignored `.work/vmware-testing/cli-results-final/` and the guest staging directory.
