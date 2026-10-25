# Proposal

## Why

Updating Portscoop currently requires replacing a Host release or copying compatible Store launchers manually; the bootstrap installer requires an empty destination. Users need a dedicated **Portscoop self-update** command that preserves portable installations and clearly differs from updating installed apps.

## What Changes

- Add equivalent `psp self-update` and `portscoop self-update` management commands. Keep `psp update <app>` as package management and `pspe` as application execution.
- Target the installation containing the invoked management executable, independent of CWD and the Host's default Store. Do not update every registered Store or accept `--env` retargeting in this initial scope.
- Support checking the latest stable release, selecting a published version, reading update status, and explicitly recovering an interrupted update.
- Download a pinned GitHub release archive and checksum, validate its inventory and update descriptor, then stage an owned transaction inside the installation.
- Update a managed Host's complete release payload while preserving its `store` child and registrations. Update only the three launchers in a ready Store with a compatible existing backend; preserve its identity, engine, apps, shims, persistence, configuration, and path state.
- Use a private Windows helper to finish replacement after the invoking launcher exits. Distinguish accepted handoff from completed update and retain a status/rollback journal.
- Coordinate updates with other runtime users, package operations, and application startup. Reject incompatible releases, unknown ownership, and unrecoverable replacement conditions without overwriting user data.
- Defer automated tests and VM qualification at the user's request; do not describe this change as runtime-qualified.

## Capabilities

### New Capabilities

- `runtime-self-update`: Portscoop release discovery, installation-local update selection, verified staging, Windows handoff, owned replacement, status, and recovery.

### Modified Capabilities

- `environment-resolution`: Scope Store selection to Store operations and make self-update select the invoked installation rather than the registered default.
- `environment-relocation`: Coordinate runtime replacement with Store operations and launch preparation, including pending-update diagnostics and recovery boundaries.

## Impact

- Add a separate Go self-update service and Cobra command; reuse fixed layout, metadata validation, file locking, release verification, and private transport primitives without routing updates through Scoop.
- Extend packaging with an inventoried update descriptor while preserving the current strict `release.json` envelope. Add owned Host control storage outside `.portscoop`, which currently identifies an adjacent Store.
- Add installation-use coordination, a validated helper entry point, update transaction metadata, and documentation. No installer overwrite mode, Store reinitialization, backend migration, automatic fleet update, PATH mutation, or new end-user compiler is included.
- Build on the implemented three-launcher layout from `add-pspe-shim-execution`; its remaining qualification is paused, not declared complete. Keep active OpenSpec artifacts uncommitted. Publication and every Git push remain separately authorized actions.
