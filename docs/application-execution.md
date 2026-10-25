# Running installed applications

`pspe` is the application entry point beside `psp` and `portscoop`. Use `psp`
for package and Store management. Application names are never management
subcommands in `pspe`: `install`, `help`, and `launch` can all name installed shims.

```powershell
pspe editor                         # Detached: no application arguments
pspe rg -n hello .                   # Foreground: application arguments present
pspe --wait python                   # Foreground even without arguments
pspe --detach editor file.txt        # Detached even with arguments
pspe --env work rg --help            # Select work; pass --help to rg
pspe -- tool --version --            # Pass --version and -- to tool
```

Without Host PATH integration, use `& 'X:\my-store\pspe.exe' ...` in PowerShell
or `"X:\my-store\pspe.exe" ...` in CMD. A Store-local executable resolves its
own adjacent identity regardless of your current directory. A Host executable
uses your selected registered alias (`--env`) or the explicitly configured default.

Place launcher options before the shim name. Once a shim name is encountered,
all remaining tokens belong to the application, including `--env`, `--wait`,
`--help`, `--version`, and `--`. Prefix `--` ends launcher option parsing.
`--wait` and `--detach` conflict. `pspe --help` describes the launcher;
`pspe tool --help` asks the tool for help.

Wait mode connects terminal input/output/error, keeps the caller's working
directory, and returns the application's numeric exit status. Launcher failures
return 2 with a `pspe:` diagnostic. Cancellation returns 130 with a diagnostic;
an application returning 2 or 130 itself receives no extra launcher diagnostic.
Detached success is 0 after startup handoff and does not report later app status.

Portscoop preserves the argument values it receives. The calling shell can
transform values before launching a native executable. Windows PowerShell 5.1
can discard empty string arguments; Portscoop cannot reconstruct those lost
values. Prefer a caller with suitable native argument passing when an empty
argument is significant. Quote values according to your calling shell.

## Shim ownership and runtime handlers

Resolution uses the selected Store's recorded shim inventory after relocation
preparation. Equivalent generated representations (such as `.cmd` and shell
wrappers for one batch alias) form one logical name. Name matching ignores case;
paths and Windows device names are rejected. Native aliases require both the
recorded pinned binary shim and its `.shim` target metadata. The validated
original `.exe` or `.com` target is used instead of executing the binary shim.

PowerShell `.ps1` wrapper families describe a Windows PowerShell target; batch
`.cmd`/`.bat` wrapper families describe a batch target. Recognition in the owned
inventory is separate from qualification of an execution handler and argument
form. An unqualified handler or argument form fails explicitly. Native EXE/COM
aliases support bounded literal fixed arguments in their owned `.shim` metadata.
These are parsed using Windows quoting rules and prepended to caller arguments;
they are never shell-evaluated. Dynamic expressions, absolute host paths, and
fixed arguments in script wrappers remain unsupported. Calling an alias without
caller arguments still selects detach mode, even when it has fixed arguments.

Supported manifest shortcuts become Store-owned aliases: for example, `Disk View`
becomes `Disk-View`, usable as `pspe Disk-View`. No Start Menu entry is created.

Python (`.py`), Java (`.jar`), and shell-script wrappers have no qualified
Store-bound runtime handler. They report that missing handler instead of
searching for Python, Java, Bash, or another runtime on host PATH. Their presence
does not enable a host-runtime fallback.

Conflicting wrapper targets, altered/unrecorded artifacts, missing native-family
members, missing target files, and targets escaping their package fail before
launch. Windows file-handle resolution follows junctions when checking the final
target. A missing shim never falls back to ordinary Scoop, another Store, or a
command on host PATH.

## Child environment

Each invocation builds its own environment snapshot. Foreign Scoop activation
paths and stale Portscoop transport settings are filtered, the selected Store's
shims are included on PATH, and validated portable environment values are applied
after relocation. Portable PATH additions prepend that child PATH. Malformed,
duplicate-case, reserved-root, and NUL-bearing state is rejected.

Normal caller Git settings remain available to applications. `pspe` does not
introduce the package runner's private global Git config, hook/template paths,
or prompt restrictions. Neither builder modifies the caller's process environment
or user/machine environment settings. Applications themselves run with the user's
normal permissions; this is not an application runtime sandbox.

## Handler argument scope

Native EXE/COM targets receive the native argument vector. PowerShell scripts use
Windows PowerShell 5.1 with profiles disabled and text output. A private bounded
JSON payload carries values without command-line re-quoting. Declared named
parameters (including aliases and unambiguous abbreviations) bind through an
ordinary parameter splat; undeclared options and positional values remain ordered
array data. A named value can be a separate token or `-Name:value`. Switches accept
bare flags or explicit `true`/`false` (`$true`/`$false`, `1`/`0`) values. Duplicate
or ambiguous named parameters and missing values fail before script execution.
Argument strings are never evaluated as source. Console stdin stays connected in
wait mode, including scripts that read `[Console]::In`.

Batch scripts use fixed Windows CMD with AutoRun and delayed expansion disabled.
The dedicated codec qualifies empty arguments, spaces, Unicode, percent signs,
exclamation marks, carets, shell/redirection metacharacters, named option strings,
and trailing backslashes as batch parameters (`%~1`, etc.). Embedded quotes, NUL,
line breaks, and expanded command lines beyond the qualified 8,000-unit bound
fail explicitly. Batch files forwarding `%*` to another program perform their
own additional argument parsing; that program's quoting rules still apply.

Detached script hosts use a hidden console with NUL streams. They finish consuming
their startup payload before handoff, so no invocation files or launcher terminal
handles are required afterward. Native GUI targets use ordinary visible-window
creation. Successful handoff reports startup acknowledgement; wait mode reports
the application root's eventual status. A foreground root that launches a child
can return while that child continues. Stream draining after root exit is bounded.
