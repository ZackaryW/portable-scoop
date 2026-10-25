package application

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"portscoop/internal/metadata"
)

// JobProbeOption is a metadata-only helper, not an execution endpoint. It accepts
// no executable, invocation, environment, or Store selector.
const JobProbeOption = "--portscoop-probe-job"

type JobMembership struct {
	InJob      bool   `json:"in_job"`
	LimitFlags uint32 `json:"limit_flags"`
}

type ExternalJobError struct{ Flags uint32 }

func (e *ExternalJobError) Error() string {
	return fmt.Sprintf("external job prevents verified independent detached lifetime (limits=0x%x)", e.Flags)
}

// WriteJobProbe runs before assigning the helper to any Portscoop startup job.
// Querying NULL here sees the actual external job left after Windows breakaway,
// which the creating process cannot query by an unnamed child's job handle.
func WriteJobProbe(out io.Writer) error {
	info, err := currentJobMembership()
	if err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(info)
}

func currentJobMembership() (JobMembership, error) {
	inside, err := processHasJob(windows.CurrentProcess())
	if err != nil {
		return JobMembership{}, err
	}
	info := JobMembership{InJob: inside}
	if inside {
		var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
		if err := windows.QueryInformationJobObject(0, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil); err != nil {
			return info, fmt.Errorf("inspect external job limits: %w", err)
		}
		info.LimitFlags = limits.BasicLimitInformation.LimitFlags
	}
	return info, nil
}

type boundedProbe struct{ data []byte }

func (b *boundedProbe) Write(data []byte) (int, error) {
	if len(b.data)+len(data) > 1024 {
		return 0, fmt.Errorf("external job probe exceeded size bound")
	}
	b.data = append(b.data, data...)
	return len(data), nil
}

func probeExternalJob(ctx context.Context) (JobMembership, error) {
	if err := ctx.Err(); err != nil {
		return JobMembership{}, err
	}
	current, err := currentJobMembership()
	if err != nil {
		return current, err
	}
	// Windows refuses CREATE_BREAKAWAY_FROM_JOB when the immediate job permits
	// neither explicit nor silent breakaway. Classify that known restriction
	// before CreateProcess reports an otherwise ambiguous access-denied error.
	if current.InJob && current.LimitFlags&(windows.JOB_OBJECT_LIMIT_BREAKAWAY_OK|windows.JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK) == 0 {
		return current, &ExternalJobError{Flags: current.LimitFlags}
	}
	self, err := os.Executable()
	if err != nil {
		return JobMembership{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, self, JobProbeOption)
	// No Portscoop job is assigned to this metadata-only helper: doing so would
	// hide the remaining external ancestor from QueryInformationJobObject(NULL).
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_BREAKAWAY_FROM_JOB, HideWindow: true}
	cmd.WaitDelay = 500 * time.Millisecond
	var output, diagnostic boundedProbe
	cmd.Stdout, cmd.Stderr = &output, &diagnostic
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return JobMembership{}, ctx.Err()
		}
		return JobMembership{}, fmt.Errorf("external job lifetime probe failed: %w", err)
	}
	var info JobMembership
	if err := metadata.Decode(output.data, &info); err != nil {
		return info, fmt.Errorf("invalid external job lifetime probe: %w", err)
	}
	if info.InJob {
		return info, &ExternalJobError{Flags: info.LimitFlags}
	}
	return info, nil
}
