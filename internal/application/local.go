package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
)

// Local starts applications after coordinated Store preparation. Verify is
// injected at the composition root; this service has no package dispatch API.
type Local struct {
	Starter     Starter
	Verify      func(environment.Paths) error
	BusyTimeout time.Duration
}

func ValidateInvocation(in Invocation) error {
	if err := relocation.ValidateShimName(in.Shim); err != nil {
		return err
	}
	if in.Mode != Wait && in.Mode != Detach {
		return fmt.Errorf("unsupported application mode")
	}
	if !filepath.IsAbs(in.Directory) || strings.ContainsRune(in.Directory, 0) {
		return fmt.Errorf("application working directory must be absolute")
	}
	if len(in.Args) > 4096 {
		return fmt.Errorf("application argument count exceeds bound")
	}
	size := len(in.Directory) + len(in.Shim)
	for _, arg := range in.Args {
		if strings.ContainsRune(arg, 0) || len(arg) > 65536 {
			return fmt.Errorf("invalid or oversized application argument")
		}
		size += len(arg)
	}
	if size > 512*1024 {
		return fmt.Errorf("application invocation exceeds size bound")
	}
	return nil
}

func (l Local) Start(ctx context.Context, local environment.Context, in Invocation, streams Streams) (Process, error) {
	if err := ValidateInvocation(in); err != nil {
		return nil, err
	}
	starter := l.Starter
	if starter == nil {
		starter = Native{}
	}
	var process Process
	err := (relocation.Coordinator{BusyTimeout: l.BusyTimeout, Prepare: (relocation.Manager{}).Prepare}).PrepareLaunch(ctx, local, func(paths environment.Paths) error {
		if l.Verify == nil {
			return fmt.Errorf("Store runtime verification is unavailable")
		}
		if err := l.Verify(paths); err != nil {
			return err
		}
		shim, err := (relocation.Manager{}).ResolveOwnedShim(paths, in.Shim)
		if err != nil {
			return err
		}
		local.Paths = paths
		child, err := BuildEnvironment(local, os.Environ())
		if err != nil {
			return err
		}
		spec := Command{Executable: shim.Target, Args: append([]string{}, in.Args...), Environment: child, Directory: in.Directory, Mode: in.Mode, Streams: streams}
		switch shim.Handler {
		case "native":
			process, err = starter.Start(ctx, spec)
		case "powershell":
			process, err = startPowerShell(ctx, starter, local, shim.Target, spec)
		case "batch":
			process, err = startBatch(ctx, starter, shim.Target, spec)
		default:
			err = fmt.Errorf("shim has no qualified execution handler")
		}
		return err
	})
	if err != nil {
		if process != nil {
			_ = process.Abort()
		}
		return nil, err
	}
	return process, nil
}

func (l Local) Execute(ctx context.Context, local environment.Context, in Invocation, streams Streams) (int, error) {
	p, err := l.Start(ctx, local, in, streams)
	if err != nil {
		return 0, err
	}
	defer p.Abort()
	if in.Mode == Detach {
		if err := p.Commit(ctx); err != nil {
			return 0, fmt.Errorf("detached startup handoff: %w", err)
		}
		return 0, nil
	}
	return p.Wait(ctx)
}
