# Portable compatibility policy

`portable-fields-v1` is the manifest contract implemented in
`internal/compatibility/`; command classification remains in
`internal/backend/scoop/policy.go`. It is a preflight layer, not a replacement
package engine. The pinned adapter enforces it before package execution.

## Commands

The recognized surface is `install`, `uninstall`, named or `*` app `update`,
`list`, `search`, `status`, `bucket list/add/rm` (`remove` maps to `rm`), `reset`,
`cleanup`, and configuration inspection. Package-changing commands require
explicit package targets (`*` is supported for update/cleanup). The only optional
package flags in this first contract are `--no-cache` for install/update and
`--force` for app update. Other upstream flags fail explicitly.

`config` accepts zero arguments or one setting name and never writes. Global
operations, root overrides, skipped hashes, custom executable command aliases,
raw forwarding, engine installation/update, and generated `@version` manifests
are rejected. A package reference is a simple name or `bucket/name`; it is not a
filesystem path or manifest URL. Use the environment's chosen bucket revision
for a fixed package version. Downloaded engine updates are delivered only through
a compatible Portscoop release, with a recorded upstream/patch revision and
verified artifact hashes.

## Manifest fields

See [Manifest compatibility](compatibility.md) for the per-field use/rewrite/skip/reject
contract and supported replacements. HTTP(S) ZIP payloads support declared SHA-256,
SHA-1, SHA-512, and MD5 digests; release integrity still requires SHA-256. Shortcut
entries become Store-owned launch aliases. Native shims support literal fixed
arguments. One complete persisted-file initialization hook pattern has a typed
replacement; unknown scripts and required unsupported integration fail explicitly.
Module discovery is unsupported, rather than silently ignored.

Before install/update/uninstall, the entire dependency closure is loaded,
validated, deduplicated, and cycle-checked with a dependency-first execution
plan. Any unsupported dependency rejects the operation before package
mutation starts. Uninstall and reset must additionally validate the saved
installed manifest, not just a possibly changed bucket manifest. The patched
engine consumes a frozen snapshot and never uses `Get-Command` fallback for a
missing package bin. Relocation checks constrain managed artifacts and links.

Passing declaration validation does not sandbox application execution, prove
downloaded software is trustworthy, or make every application store its runtime
data portably. Supported applications are qualified explicitly in integration
tests. Mutable cache, bucket state, installed apps, and persistence are private to
each environment in this release.

## Verification

`go test ./internal/backend/scoop` covers accepted commands/manifests, global and
raw-command bypasses, selected architecture effects, path traversal,
environment interpolation/overrides, dependency rejection, and cycles. The
isolated upstream ZIP fixture is recorded separately in `tests/backend/evidence`;
the adapted-backend and release qualification evidence is stored there too.
