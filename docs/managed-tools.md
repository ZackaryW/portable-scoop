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

New Stores acquire Main at revision `990163c94e827a8a3966f643e86a4861aa13666e`
using their private Git before becoming ready. `psp env bootstrap` prepares an
absent Main in an existing compatible Store and reports tool versions. It keeps
an existing official Main checkout and reports conflicting sources.

Archive-hinted NSIS payloads use the private 7-Zip reader; installers are never
run. ZIP, 7z, tar and gzip retain their Go readers. An ordinary EXE download
remains an inert file unless the manifest identifies it as an archive.

NSIS extraction validates the complete member list before streaming exact names
into Go-owned staging files. Exact repeated names keep the last entry when all
segment sizes are known. A single entry with an unknown size is streamed under
the byte limits. Ambiguous duplicates, case collisions, links and unsafe paths
fail before publication. Literal dollar signs are ordinary filename data.

Limits are 100,000 entries, 4 GiB per member and 8 GiB total, including discarded
duplicate entries; 16 MiB of inventory and 64 KiB of diagnostics per process;
2 GiB of committed process-tree memory; three minutes per invocation and thirty
minutes per archive. Cancellation terminates the managed process tree. These
rules have no bypass switch and do not enable arbitrary native archive formats.
