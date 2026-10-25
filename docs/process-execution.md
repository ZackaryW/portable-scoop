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
