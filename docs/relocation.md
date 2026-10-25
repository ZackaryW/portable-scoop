# Moving and recovering an environment

Move the entire portable root while no Portscoop operation is running. Invoke
`portscoop.exe` or `psp.exe` from its new directory. Direct invocation needs no
host registration. After a same-drive directory move, register the new location
again under its existing alias. Drive-letter-only changes are rediscovered from
the saved relative path and identity; disconnected roots stay registered.

Preparation holds the same per-root OS lock as the backend operation. All package
commands, including stable reads, wait up to five seconds for that root and then
report busy. Other roots use independent locks. Never delete a lock file to break
a lock; Windows releases the lock when its owner dies.

The prepared-root record owns the exact generated shim content, supported current
and persist junctions, persisted-file hardlinks, and portable environment state.
Repair substitutes the complete previous root prefix, including a directory move
without a drive-letter change. It preserves unrelated paths on the same drive,
downloaded manifests, executable payloads, user files, and application config.
It does not promise to repair application-internal absolute paths.

The supported shim implementation is the pinned engine's kiennq EXE shim plus
its qualified EXE metadata, CMD, PowerShell, and shell wrapper formats. Modified
or unknown shims block preparation. Managed junction targets must remain inside
their expected app version or persist directory. Persisted files retain their
NTFS hardlinks; a copy that splits a hardlink can be rejoined only when both
copies have identical content. Differing copies require explicit data
reconciliation before retrying.

`.portscoop/state/repair.json` journals the old root, new root, and owned artifacts
before repairs begin. The prepared root is committed only after the complete
inventory validates. After an interruption, rerun the same command at the same
new location; already repaired files and links are recognized, and missing links
from an interrupted replacement are recreated. Preserve the journal and prior
prepared-root record. Moving the root again during an unfinished repair is
unsupported; return it to the journal's destination and finish repair first.

An external link, unexpected shim edit, corrupt state, or media failure blocks
backend execution and identifies the artifact. Inspect it and restore its known
managed form or a compatible state backup before retrying. Do not replace a root
prefix throughout the whole filesystem or edit the journal to suppress errors.

`.portscoop/state/operation.json` records running, failed, complete, or
inspection-required package operations. Abruptly terminated operations retain
the running diagnostic; the next invocation warns about interruption. Scoop's
failed-install checks and package metadata may require manual inspection. Repair
recovery is idempotent; arbitrary interrupted package installs are not rolled
back automatically. Cancellation stops all managed descendants and releases
locks, but downloaded files or partial package state can remain for inspection.
