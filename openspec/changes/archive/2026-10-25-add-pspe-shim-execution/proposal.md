# Proposal

## Why

Portscoop can install applications into independent Stores, but users have no dedicated command to execute their managed shims with the Store's environment. A separate `pspe` entry point can provide normal terminal behavior and detached app launching without mixing application names into the package-management interface.

## What Changes

- Ship `pspe.exe` alongside `portscoop.exe` and `psp.exe` in Host and Store deployments.
- Add `pspe [--env <alias>] [--wait | --detach] <shim> [application arguments...]`. With no application arguments, launch detached; with arguments, wait in the terminal. Explicit mode flags override this default.
- Stop launcher-option parsing at the shim name and preserve the remaining argument vector, working directory, and foreground streams and exit status.
- Resolve only qualified managed shims in the selected Store, after identity, layout, ownership, and relocation checks. Apply the Store's portable environment only to child processes.
- Add a distinct execution handshake and dispatch path so application lifetime does not inherit package-operation teardown. Keep the existing package protocol and command allowlist intact.
- Coordinate launch preparation with package mutations, then release preparation locks before waiting for a running application.
- Report missing execution support, unsupported shim handlers, invalid targets, and launch failures explicitly; preserve existing package commands for older Stores.

## Capabilities

### New Capabilities

- `shim-execution`: Dedicated application invocation, mode selection, argument forwarding, managed-shim resolution, child environment, process lifetime, and execution diagnostics through `pspe`.

### Modified Capabilities

- `environment-management`: Deliver and verify the execution entry point in both deployment modes while keeping the two management names equivalent.
- `environment-resolution`: Extend compatible delegation to application execution and distinguish foreground application cancellation from acknowledged detached execution.
- `environment-relocation`: Prepare and start application launches under the Store's coordination boundary without retaining its mutation lock for the application's lifetime.

## Impact

- Add a Go execution entry point and frontend, reusing the current identity, registry, resolver, and volume-discovery components.
- Add application execution services, shim launch descriptors, child-environment preparation, and separate Windows application lifecycle handling. The existing package runner intentionally kills descendants on root exit and cannot be reused unchanged.
- Extend delegation with an execution-specific handshake and acknowledgement contract. Do not add fields to the strict existing package handshake or reinterpret application names as Scoop commands.
- Update `scripts/build.ps1`, `scripts/package.ps1`, release verification, Store initialization, setup qualification, and documentation for the third executable.
- Add parser, ownership, argument, stream, cancellation, detached-survival, concurrency, relocation, compatibility, and disposable-VM qualification coverage. Keep the Windows x64, local NTFS, and qualified PowerShell baseline.
- No new compiler/runtime requirement for end users, persistent shell activation, global application aliases, arbitrary executable paths, or broader package-manifest permissions. Application execution is not a runtime sandbox.
