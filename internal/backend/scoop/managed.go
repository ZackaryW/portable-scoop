package scoop

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"portscoop/internal/environment"
	"portscoop/internal/relocation"
	"portscoop/internal/runner"
)

type Managed struct{ Runner runner.ProcessRunner }

func (b Managed) Execute(ctx context.Context, local environment.Context, args []string, streams runner.Streams) (int, error) {
	op, err := Classify(args)
	if err != nil {
		return 0, err
	}
	preparation := relocation.Manager{}
	return (relocation.Coordinator{Prepare: preparation.Prepare, Capture: preparation.Capture, Diagnostic: streams.Err, Operation: op.Name}).Do(ctx, local, func() (int, error) { return b.executeLocked(ctx, local, op, streams) })
}

func (b Managed) executeLocked(ctx context.Context, local environment.Context, op Operation, streams runner.Streams) (int, error) {
	var err error
	local.Backend, err = Verify(local.Paths)
	if err != nil {
		return 0, err
	}
	manifests, err := Plan(local.Paths, op)
	if err != nil {
		return 0, fmt.Errorf("portable preflight: %w", err)
	}
	reportCompatibility(streams.Err, manifests, op.Verbose)
	if err = CheckRuntime(ctx, local.Paths, b.Runner); err != nil {
		return 0, err
	}
	frozen, err := json.Marshal(manifests)
	if err != nil {
		return 0, err
	}
	command, cleanup, err := runner.PrepareRequest(local, runner.Request{Arguments: op.Arguments, Manifests: frozen}, os.Environ(), streams)
	if err != nil {
		return 0, err
	}
	defer cleanup()
	process := b.Runner
	if process == nil {
		process = runner.Native{}
	}
	return process.Run(ctx, command)
}
