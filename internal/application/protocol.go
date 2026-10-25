package application

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/localipc"
	"portscoop/internal/resolver"
)

const ExecutionProtocol = 1
const ControlOption = "--portscoop-execution-control"

type HelloRequest struct {
	Protocol   int    `json:"execution_protocol"`
	ExpectedID string `json:"expected_id"`
}
type ExecutionHello struct {
	Protocol int    `json:"execution_protocol"`
	ID       string `json:"id"`
	Version  string `json:"launcher_version"`
	Modes    []Mode `json:"modes"`
	Error    string `json:"error,omitempty"`
}
type Envelope struct {
	Protocol   int        `json:"execution_protocol"`
	ExpectedID string     `json:"expected_id"`
	Invocation Invocation `json:"invocation"`
}

func (e Envelope) Validate(id string) error {
	if e.Protocol != ExecutionProtocol {
		return fmt.Errorf("incompatible execution protocol; update Store execution support")
	}
	if !environment.ValidID(e.ExpectedID) || e.ExpectedID != id {
		return fmt.Errorf("execution identity mismatch")
	}
	if e.Invocation.Selector != "" {
		return fmt.Errorf("delegated Store invocation cannot rediscover a selector")
	}
	return ValidateInvocation(e.Invocation)
}

// ServeControl is a Store-only endpoint. It accepts a bounded logical invocation,
// never an executable path, environment override, or shell command string.
func ServeControl(ctx context.Context, directory, executable, version string, local Local, streams Streams) (int, error) {
	control, err := localipc.Open(directory)
	if err != nil {
		return 0, err
	}
	defer control.Close()
	startup, cancel := context.WithTimeout(ctx, localipc.StartupTimeout)
	defer cancel()
	var request HelloRequest
	if err := control.Read(startup, &request); err != nil {
		return 0, err
	}
	id, paths, err := environment.Load(environment.OSFilesystem{}, filepath.Dir(executable))
	if err != nil {
		return 0, err
	}
	hello := ExecutionHello{Protocol: ExecutionProtocol, ID: id.ID, Version: version, Modes: []Mode{Wait, Detach}}
	if request.Protocol != ExecutionProtocol || request.ExpectedID != id.ID {
		hello.Error = "incompatible execution protocol or Store identity"
	}
	if err := control.Write(startup, hello); err != nil {
		return 0, err
	}
	if hello.Error != "" {
		return 0, nil
	}
	var envelope Envelope
	if err := control.Read(startup, &envelope); err != nil {
		return 0, err
	}
	fail := func(err error) (int, error) {
		if writeErr := control.Write(startup, startupMessage{State: "failed", Error: err.Error()}); writeErr != nil {
			return 0, writeErr
		}
		return 0, nil
	}
	if err := envelope.Validate(id.ID); err != nil {
		return fail(err)
	}
	p, err := local.Start(startup, environment.Context{Identity: id, Paths: paths}, envelope.Invocation, streams)
	if err != nil {
		return fail(err)
	}
	defer p.Abort()
	if err := control.Write(startup, startupMessage{State: "ready"}); err != nil {
		return 0, err
	}
	if envelope.Invocation.Mode == Wait {
		return p.Wait(ctx)
	}
	var commit startupMessage
	if err := control.Read(startup, &commit); err != nil {
		return 0, err
	}
	if commit.State != "commit" {
		return fail(fmt.Errorf("invalid detached commit request"))
	}
	if err := p.Commit(startup); err != nil {
		return fail(err)
	}
	if err := control.Write(startup, startupMessage{State: "committed"}); err != nil {
		return 0, fmt.Errorf("detached acknowledgement lost after commit; application may be running; not retried: %w", err)
	}
	return 0, nil
}

type Client struct {
	Starter       Starter
	TempDirectory string
}

func (c Client) Forward(ctx context.Context, target resolver.Target, in Invocation, streams Streams) (int, error) {
	in.Selector = ""
	envelope := Envelope{ExecutionProtocol, target.Identity.ID, in}
	if err := envelope.Validate(target.Identity.ID); err != nil {
		return 0, err
	}
	executable := filepath.Join(target.Paths.Root, "pspe.exe")
	info, err := os.Lstat(executable)
	if err != nil || !info.Mode().IsRegular() {
		return 0, fmt.Errorf("Store needs compatible execution support: restore pspe.exe from a verified release")
	}
	parent := c.TempDirectory
	if parent == "" {
		parent = os.TempDir()
	}
	control, err := localipc.New(parent)
	if err != nil {
		return 0, err
	}
	defer control.Close()
	startup, cancel := context.WithTimeout(ctx, localipc.StartupTimeout)
	defer cancel()
	if err := control.Write(startup, HelloRequest{ExecutionProtocol, target.Identity.ID}); err != nil {
		return 0, err
	}
	starter := c.Starter
	if starter == nil {
		starter = Native{}
	}
	p, err := starter.Start(startup, Command{Executable: executable, Args: []string{ControlOption, control.Directory()}, Environment: os.Environ(), Directory: in.Directory, Mode: Wait, Streams: streams, HiddenConsole: true})
	if err != nil {
		return 0, fmt.Errorf("Store needs compatible execution support: %w", err)
	}
	defer p.Abort()
	var hello ExecutionHello
	if err := control.Read(startup, &hello); err != nil {
		return 0, fmt.Errorf("execution handshake failed; Store needs compatible execution support: %w", err)
	}
	if hello.Error != "" || hello.ID != target.Identity.ID || hello.Protocol != ExecutionProtocol || len(hello.Modes) != 2 || hello.Modes[0] != Wait || hello.Modes[1] != Detach {
		return 0, fmt.Errorf("Store execution handshake is incompatible or has another identity; update execution support")
	}
	if err := control.Write(startup, envelope); err != nil {
		return 0, err
	}
	var status startupMessage
	if err := control.Read(startup, &status); err != nil {
		return 0, err
	}
	if status.State == "failed" {
		return 0, fmt.Errorf("Store application startup: %s", status.Error)
	}
	if status.State != "ready" {
		return 0, fmt.Errorf("invalid Store startup acknowledgement")
	}
	if in.Mode == Wait {
		_ = control.Close()
		return p.Wait(ctx)
	}
	if err := control.Write(startup, startupMessage{State: "commit"}); err != nil {
		return 0, err
	}
	if err := control.Read(startup, &status); err != nil {
		return 0, fmt.Errorf("detached acknowledgement lost; application may be running; not retried: %w", err)
	}
	if status.State == "failed" {
		return 0, fmt.Errorf("Store detached handoff: %s", status.Error)
	}
	if status.State != "committed" {
		return 0, fmt.Errorf("invalid detached commit acknowledgement; not retried")
	}
	// Handoff, rather than helper completion, is the public success boundary.
	finish, finishCancel := context.WithTimeout(context.Background(), time.Second)
	defer finishCancel()
	_, _ = p.Wait(finish)
	return 0, nil
}
