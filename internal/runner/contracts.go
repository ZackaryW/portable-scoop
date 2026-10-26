package runner

import (
	"context"
	"fmt"
	"io"
)

type Streams struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

type Command struct {
	Executable  string
	Arguments   []string
	Environment []string
	Directory   string
	Streams     Streams
	// MemoryLimit caps committed memory of the entire Windows process job.
	// Zero preserves the ordinary runner policy without a memory cap.
	MemoryLimit uint64
}

// ProcessRunner must distinguish a backend exit code from a launch error.
type ProcessRunner interface {
	Run(context.Context, Command) (int, error)
}

type ExitError struct{ Code int }

func (e *ExitError) Error() string { return fmt.Sprintf("backend exited with status %d", e.Code) }
