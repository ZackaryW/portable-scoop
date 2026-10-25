# Process execution

Portscoop starts an explicit managed executable with the caller's working
directory and standard input, output, and error streams. It does not reconstruct
a command string through CMD. The PowerShell adapter receives a private JSON
request file, preserving empty strings, Unicode, quotes, and shell metacharacters
as data. PowerShell runs without profiles. Request files are removed on normal
completion; an abruptly terminated invocation may leave a request in its private
`scoop/tmp` directory.

Successful commands return 0. A backend failure retains its numeric exit status
and its standard error output; Portscoop does not add a second generic failure
message. Launcher validation or process-start failures return 2 with a
`portscoop:` diagnostic explaining the failed check. Ctrl+C cancellation returns
130 with `portscoop: operation cancelled`. A backend may itself use 2 or 130;
the diagnostic identifies launcher errors, since exit numbers alone cannot
reserve values in a third-party backend's status space.

On supported Windows, each launched process starts suspended, enters a Job
Object configured with `KILL_ON_JOB_CLOSE`, and then resumes. Descendants remain
in that job, including a delegated portable launcher and its backend. Cancellation
kills the root process and closes the job. Abrupt launcher termination closes the
non-inherited job handle in the kernel. When a managed root exits normally,
remaining descendants are also stopped: package operations must not launch
background services. OS file locks held by terminated processes are released;
the lock filename can remain and must not be deleted to break a lock.

Fixture tests observe input/output/error streams, argument boundaries, original
working directory, nonzero backend status, missing executable diagnostics, child
and grandchild cancellation, abrupt owner termination, and subsequent lock
acquisition. These tests launch only temporary fixture processes.

Process environment isolation keeps root overrides and activation paths out of
the calling shell. It does not prevent PowerShell code from writing persistent
environment variables, shortcuts, or other host files. The pinned Scoop adapter
and manifest policy must separately suppress or reject those effects before real
package execution is enabled. Portscoop does not sandbox arbitrary application
runtime behavior.
# Application execution

`pspe` uses application lifecycle handling independent of the package runner.
See [application execution](application-execution.md) for supported handlers and
arguments. Package handshake/protocol 1 and package containment remain unchanged.

Host execution uses a separate versioned handshake over a current-user-only
local file mailbox. The directory is atomically created with a protected DACL;
messages are strict bounded JSON (1 MiB), and startup negotiation is bounded to
15 seconds. No network listener is used. Only an expected Store UUID, logical
shim, mode, arguments, and caller directory cross this boundary. There is no raw
executable, command string, environment override, or secondary Store selector.
Transient Windows sharing/byte-range file locks retry only the control I/O within
that deadline. Invalid messages and other I/O failures remain immediate errors;
these retries never repeat application creation.

The Store rechecks its identity under its operation lock, verifies the compatible
managed backend, prepares relocation, validates its owned shim target, snapshots
the child environment, and creates the application. The lock ends after startup;
the application session does not retain it and does not capture app edits into
the ownership inventory. Other package operations may encounter normal Windows
file-in-use errors while an application is open.

Redirected output pipes are forwarded through helper-owned pipes with bounded
draining after root exit. This prevents a surviving application child from
retaining the caller's pipeline handle. Real console handles stay connected as
console handles so interactive applications can use console APIs.

Detached startup remains owned until a commit request completes. The Store then
acknowledges committed handoff. Before commit, control failure/cancellation cleans
up the pending application. After commit, the app survives helper completion and
later Host cancellation. If the acknowledgement is lost after commit may have
occurred, the diagnostic states that the app may be running; no automatic retry
is attempted. A missing `pspe.exe` or incompatible execution handshake requests a
compatible Store update without changing package-management availability.

Detached creation first runs a bounded metadata-only helper with the same Windows
breakaway policy. It inspects the actual remaining external job before Portscoop
adds any startup job. Residual membership is rejected: even a job without an
immediate kill-on-close flag can conceal an owning ancestor, and independence
cannot be verified. The helper accepts no app command. Creation then rechecks
membership while the app is still suspended, before assigning its temporary
startup job. Portscoop never weakens an external job's limits. This handles the
Windows rule that breakaway may stop at an ancestor which forbids further breakaway.

