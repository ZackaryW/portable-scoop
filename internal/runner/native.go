package runner

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Native starts an explicit executable without invoking a command shell.
type Native struct{}

func (Native) Run(ctx context.Context, command Command) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	process := exec.CommandContext(ctx, command.Executable, command.Arguments...)
	process.Env, process.Dir = command.Environment, command.Directory
	process.Stdin, process.Stdout, process.Stderr = command.Streams.In, command.Streams.Out, command.Streams.Err
	cleanup, err := startContained(process, command.MemoryLimit)
	if err != nil {
		if ctx.Err() != nil {
			return 0, ctx.Err()
		}
		return 0, fmt.Errorf("launch %s: %w", command.Executable, err)
	}
	defer cleanup()
	err = process.Wait()
	if ctx.Err() != nil {
		return 0, ctx.Err()
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode(), nil
	}
	if err != nil {
		return 0, fmt.Errorf("launch %s: %w", command.Executable, err)
	}
	return 0, nil
}
