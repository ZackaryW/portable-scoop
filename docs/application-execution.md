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
