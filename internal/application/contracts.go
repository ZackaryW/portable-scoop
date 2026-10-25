// Package application defines Store application execution independently of
// package command dispatch and the package runner's kill-on-close policy.
package application

import (
	"context"
	"io"
)

type Mode string

const (
	Wait   Mode = "wait"
	Detach Mode = "detach"
)

// Invocation contains argument data already received from the caller's shell.
// Mode is resolved before dispatch; Args never contains launcher options.
type Invocation struct {
	Selector  string   `json:"selector,omitempty"`
	Shim      string   `json:"shim"`
	Args      []string `json:"args"`
	Mode      Mode     `json:"mode"`
	Directory string   `json:"directory"`
}

type Streams struct {
	In       io.Reader
	Out, Err io.Writer
}

// Executor returns application status separately from launcher failures.
type Executor interface {
	Execute(context.Context, Invocation, Streams) (int, error)
}

type ExecuteFunc func(context.Context, Invocation, Streams) (int, error)

func (f ExecuteFunc) Execute(ctx context.Context, in Invocation, streams Streams) (int, error) {
	return f(ctx, in, streams)
}

// Command is a fully validated application process, never a shell command line.
type Command struct {
	Executable        string
	Args, Environment []string
	Directory         string
	Mode              Mode
	Streams           Streams
	// HiddenConsole is for console-host handlers during detached startup.
	// Native GUI targets never request it.
	HiddenConsole bool
	// WindowsCommandLine is produced only by the explicit batch codec. It is
	// never accepted from a public invocation or delegation envelope.
	WindowsCommandLine string
}

// Starter owns startup until Commit transfers a detached process's lifetime.
// Abort must clean up an uncommitted process; Wait must preserve normal children.
type Starter interface {
	Start(context.Context, Command) (Process, error)
}
type Process interface {
	Wait(context.Context) (int, error)
	Commit(context.Context) error
	Abort() error
}
