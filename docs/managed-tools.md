# Managed Store tools

Git and the NSIS-capable full 7-Zip distribution are pinned runtime components.
Their provenance and SHA-256 digests are declared in `backend/tools/pins.json`.
Ordinary application manifests do not control these tools or their invocation.

Prepare a new bundle from the repository root:

```powershell
go run ./scripts/prepare-tools -destination .work/prepared-tools
```

Preparation verifies each downloaded or cached artifact, extracts it with Go
readers without running installers, retains required licenses, and writes a
complete `tools.json` inventory before publishing the directory. An existing
destination or a modified cache entry fails explicitly.

MinGit retains its complete upstream license tree and component-version list.
The pin records the matching upstream source-package location for each tool.
Redistribution must preserve those notices and provide corresponding sources
for the distributed components; source links are provenance, not a substitute
for satisfying the applicable source-distribution requirements.

Release packaging includes the complete bundle under `backend/tools`. Store
initialization copies it into `.portscoop/tools`; backend revision
`portable-operations-v1.1` verifies every file against both inventories before
use. Missing, changed, or unlisted helper files require runtime repair.

A Host-coordinated self-update replaces helpers with the backend and restores
the previous files if replacement fails. A direct Store self-update updates
compatible launchers only, retaining its existing backend and helpers. Helpers
are private runtime components, separate from apps installed with `psp install`.

Automatic Main bootstrap and NSIS package extraction require the remaining
backend integration; shipping the helper files alone does not enable them.
