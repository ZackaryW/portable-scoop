# Managed archive extraction

The independently verified `internal/archive` unit stages extraction under a
private directory inside the Store. It uses Go's ZIP, tar and gzip readers and
`github.com/bodgit/sevenzip v1.6.5` (upstream commit
`dcfc72a0ee9f527c55521f44ffdf1c31b732e256`, BSD-3-Clause). The 7z reader and its
compiled dependencies are pinned by `go.mod`/`go.sum`; attribution is under
[licenses](licenses/modules.txt). The small LZMA2 test fixture comes from that
sevenzip version's `testdata/lzma2.7z` under the same license.

ZIP-format `.nupkg` is a container only. Its scripts are never evaluated.
Supported readers cover ZIP/nupkg, 7z, tar, tar.gz/tgz and single-file gzip.
Opening a 7z stream does not establish support for NSIS, Inno, MSI, or arbitrary
EXEs renamed to `.7z`. Encrypted or multi-volume archives are not supported;
the reader is given one checked file and cannot discover neighboring volumes.

Every member is checked before publication: relative paths, no alternate streams
or device names, no links/reparse points/special files, no duplicate files or
case-colliding paths. Ancestors must remain physical Store-owned paths. ZIP and
gzip readers check their checksums; nonzero recorded 7z member CRCs are checked
explicitly. The download digest remains the outer payload integrity check.

Default bounds are 100,000 members, 4 GiB per member, 8 GiB total expanded data,
8 GiB input, and 1,024 characters per member path. These bound the accepted
payload, not every internal decoder allocation. Cancellation is checked during
reads and publication. Private staging is removed after success or failure.
Extraction selectors are checked directory paths; files outside `extract_dir`
are validated but not published. Existing files cannot be overwritten.

The adapted backend calls a hidden operation on the Store's verified runtime.
It validates the request location/identity, recompiles the frozen manifest,
checks download plans for equality and verifies payload digests again. The full
download set is extracted into private staging before package publication.
`extract_dir` and `extract_to` pair by download index, including empty entries
for retained EXEs. Explicit package dependencies remain; implicit extraction
helper dependencies are disabled. No external extractor or installer is invoked.

A `.7z` filename hint can also contain a ZIP (Scoop commonly uses `#/dl.7z` for
ZIP payloads). Only supported container content is accepted; a misleading NSIS
or other installer EXE still fails without execution.
