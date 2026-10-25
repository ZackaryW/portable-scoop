# shim-execution Specification

## Purpose

Let users execute installed Store shims through a dedicated command with predictable terminal behavior, portable child settings, and independent detached application lifetime.

## Requirements

### Requirement: Keep application invocation on a dedicated entry point
The system SHALL expose application execution through `pspe`. It SHALL keep `portscoop` and `psp` as management entry points, without adding application-name fallback to their package command surface. Shim names SHALL NOT become reserved management subcommands in `pspe`.

#### Scenario: An application name matches a management command
- **WHEN** the selected Store owns a shim named `install` and the user invokes `pspe install --help`
- **THEN** the application receives `--help` and no package installation command is dispatched

#### Scenario: A management launcher receives an application name
- **WHEN** the user invokes `psp editor` and `editor` is not a management command
- **THEN** the management launcher reports an unsupported command instead of launching a shim

### Requirement: Parse launcher options only before the shim name
The system SHALL accept `--env <alias>`, `--wait`, and `--detach` before a required shim name, with launcher help/version options and optional prefix `--`. It SHALL stop launcher-option parsing at the shim name and forward all subsequent arguments in order. Conflicting mode options, unknown prefix options, and missing shim names SHALL fail before application launch.

#### Scenario: Application options resemble launcher options
- **WHEN** the user invokes `pspe --env work tool --env production --wait --output=result.txt`
- **THEN** Store `work` is selected and the application receives `--env`, `production`, `--wait`, and `--output=result.txt` as its arguments

#### Scenario: Help is requested at different boundaries
- **WHEN** the user invokes `pspe --help` or `pspe tool --help`
- **THEN** the first invocation shows launcher help and the second passes `--help` to `tool`

#### Scenario: Conflicting modes
- **WHEN** both `--wait` and `--detach` are supplied before the shim name
- **THEN** the command reports the conflict without starting an application

#### Scenario: Version and argument delimiters retain their boundary
- **WHEN** the user invokes `pspe --version` or `pspe -- tool --version --`
- **THEN** the first invocation prints launcher version information and the second passes `--version` and `--` to the `tool` application

### Requirement: Choose a predictable execution mode
Without an explicit mode, the system SHALL detach when no application arguments follow the shim name and SHALL wait when at least one application argument follows it. Launcher options SHALL NOT count as application arguments. `--wait` and `--detach` SHALL override this default without changing the forwarded application arguments.

#### Scenario: Bare shim launch
- **WHEN** the user invokes `pspe --env work editor`
- **THEN** `editor` is launched detached from Store `work`

#### Scenario: Arguments select terminal execution
- **WHEN** the user invokes `pspe rg -n hello .`
- **THEN** the launcher waits for the selected application with terminal streams connected

#### Scenario: Explicit overrides
- **WHEN** the user invokes `pspe --wait python` or `pspe --detach editor file.txt`
- **THEN** the first application runs in the terminal without arguments and the second launches detached with `file.txt`

### Requirement: Resolve only owned and qualified Store shims
The system SHALL resolve a shim name only against the selected Store's managed shim inventory. It SHALL validate ownership, format, and target containment after relocation preparation. Paths, missing shims, conflicting definitions, altered shims, escaped targets, and unqualified handlers SHALL fail before launch. It SHALL NOT substitute an executable found in ordinary Scoop, another Store, or host PATH.

#### Scenario: The same name exists outside the Store
- **WHEN** a requested shim is absent from the selected Store but exists in ordinary Scoop or another Store
- **THEN** execution fails with a missing-shim diagnostic and no outside executable is launched

#### Scenario: A shim redirects outside its root
- **WHEN** a managed shim or its target has been changed to escape the permitted Store layout
- **THEN** execution reports the offending path and does not start that target

#### Scenario: Unsupported wrapper or unavailable runtime
- **WHEN** a shim requires an unqualified handler or an unavailable required Store runtime
- **THEN** execution reports the format or prerequisite explicitly instead of guessing a shell or searching host PATH

### Requirement: Preserve application argument data and working directory
The system SHALL preserve the ordered argument values received from its calling shell and the caller's working directory through direct and delegated execution. Empty values, Unicode, quotes, and metacharacters SHALL remain argument data in qualified handlers. Launcher transport SHALL NOT evaluate argument text as script source or promise to recover values already transformed by the calling shell.

#### Scenario: Argument boundaries survive delegation
- **WHEN** the received argument vector contains empty values, spaces, quotes, Unicode, `--key=value`, or shell metacharacters
- **THEN** the qualified target receives the same values in the same order and the original caller directory

### Requirement: Apply Store settings only to application children
The system SHALL give application children the selected Store's supported portable environment settings and shim path while filtering conflicting inherited Store and Scoop overrides. It SHALL keep the calling shell and persistent host environment unchanged. Package-operation transport settings and Git restrictions SHALL NOT be imposed on ordinary application CLI behavior.

#### Scenario: Store environment and caller environment differ
- **WHEN** a Store declares portable environment values and the caller contains Scoop settings for another installation
- **THEN** the application receives the selected Store's settings and the caller retains its original environment

#### Scenario: A Git application uses its normal configuration
- **WHEN** a supported Store Git shim is executed through `pspe`
- **THEN** application execution does not force the backend bucket runner's empty global configuration, disabled credential helpers, or hook/template overrides

### Requirement: Preserve foreground CLI behavior
In wait mode, the system SHALL connect the application's input, output, and error streams, wait for its root process, and return its numeric exit status. Interactive input and shell redirection SHALL work within the qualified CMD and PowerShell scope. Cancellation while waiting SHALL stop the managed foreground invocation and report cancellation without changing package-operation containment behavior.

#### Scenario: Pipeline and nonzero status
- **WHEN** input is piped into a wait-mode application that writes both output streams and exits with a nonzero status
- **THEN** the input, output, diagnostics, and application exit status are preserved through direct and central execution

#### Scenario: Interactive application without arguments
- **WHEN** `pspe --wait` starts an interactive fixture without application arguments
- **THEN** it can receive console input and remain usable until it exits or the user cancels

#### Scenario: Foreground cancellation
- **WHEN** the user cancels a foreground invocation before it has completed
- **THEN** the managed foreground process tree stops, the launcher reports cancellation, and Store preparation locks are released

#### Scenario: An application launcher completes normally
- **WHEN** a foreground application launcher exits normally after starting its application child
- **THEN** `pspe` returns the launcher process's status without applying package-operation teardown solely because a launch helper completed

### Requirement: Acknowledge detached launch and release application ownership
In detached mode, success SHALL mean that application creation and the bounded launch handoff succeeded, not that the application completed. After acknowledgement, the application SHALL survive normal exit of Host and portable launch helpers. It SHALL NOT inherit their terminal streams or require launcher-owned transport files after handoff. Startup failures SHALL return a launcher diagnostic.

#### Scenario: Detached application survives delegation
- **WHEN** a detached application launch is acknowledged and both central and portable launchers exit
- **THEN** the application remains running without retaining the launchers' terminal handles or Store preparation lock

#### Scenario: Detached GUI application opens normally
- **WHEN** an installed GUI application is launched detached
- **THEN** its normal application window can open while internal launch helpers do not display additional consoles

#### Scenario: Launch fails before acknowledgement
- **WHEN** validation, application creation, or the launch handoff fails before acknowledgement
- **THEN** the launcher reports failure and cleans up uncommitted launch helpers and children

#### Scenario: Detached application exits later
- **WHEN** an acknowledged detached application subsequently exits with a nonzero status
- **THEN** the already completed launcher does not claim to return that later application status

### Requirement: Keep launch diagnostics explicit
Launcher validation and process-start failures SHALL return status 2 with a `pspe:` diagnostic. Successful detached handoff SHALL return 0. Foreground application statuses SHALL pass through unchanged, and foreground cancellation SHALL return 130 with a cancellation diagnostic. Diagnostics SHALL distinguish launcher failures from application statuses that use the same numeric values.

#### Scenario: Missing shim and application failure are distinguishable
- **WHEN** a shim cannot be resolved or a launched foreground application itself exits with status 2
- **THEN** only the launcher failure receives a launcher-generated missing-shim diagnostic
