package application

import (
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"os"
	"unicode/utf16"

	"portscoop/internal/environment"
	"portscoop/internal/localipc"
	"portscoop/internal/runner"
)

//go:embed powershell-reader.ps1
var powerShellReader string

type powerShellRequest struct {
	SchemaVersion int      `json:"schema_version"`
	ExpectedID    string   `json:"expected_id"`
	Target        string   `json:"target"`
	Arguments     []string `json:"arguments"`
}
type startupMessage struct {
	State string `json:"state"`
	Error string `json:"error"`
}

type powerShellProcess struct {
	Process
	endpoint *localipc.Endpoint
}

func encodePowerShell(source string) string {
	units := utf16.Encode([]rune(source))
	data := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(data[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(data)
}

func startPowerShell(ctx context.Context, starter Starter, local environment.Context, target string, spec Command) (Process, error) {
	executable, err := runner.PowerShell(spec.Environment, nil)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(local.Paths.Temp, 0700); err != nil {
		return nil, err
	}
	control, err := localipc.New(local.Paths.Temp)
	if err != nil {
		return nil, err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = control.Close()
		}
	}()
	startup, cancel := context.WithTimeout(ctx, localipc.StartupTimeout)
	defer cancel()
	if err := control.Write(startup, powerShellRequest{1, local.Identity.ID, target, append([]string{}, spec.Args...)}); err != nil {
		return nil, err
	}
	spec.Executable = executable
	spec.Environment = append(spec.Environment, "PORTSCOOP_PS_CONTROL="+control.Directory())
	spec.Args = []string{"-NoLogo", "-NoProfile", "-OutputFormat", "Text", "-ExecutionPolicy", "Bypass"}
	if spec.Mode == Detach {
		spec.HiddenConsole = true
		spec.Args = append(spec.Args, "-NonInteractive")
	}
	spec.Args = append(spec.Args, "-EncodedCommand", encodePowerShell(powerShellReader))
	p, err := starter.Start(startup, spec)
	if err != nil {
		return nil, err
	}
	var message startupMessage
	if err := control.Read(startup, &message); err != nil {
		_ = p.Abort()
		return nil, fmt.Errorf("PowerShell startup: %w", err)
	}
	if message.State != "ready" {
		_ = p.Abort()
		return nil, fmt.Errorf("PowerShell startup rejected invocation: %s", message.Error)
	}
	wrapped := &powerShellProcess{Process: p, endpoint: control}
	if spec.Mode == Wait {
		if err := wrapped.consume(startup); err != nil {
			_ = p.Abort()
			return nil, err
		}
		return p, nil
	}
	cleanup = false
	return wrapped, nil
}

func (p *powerShellProcess) consume(ctx context.Context) error {
	if err := p.endpoint.Write(ctx, startupMessage{State: "start"}); err != nil {
		return err
	}
	var message startupMessage
	if err := p.endpoint.Read(ctx, &message); err != nil {
		return err
	}
	if message.State != "consumed" {
		return fmt.Errorf("PowerShell did not consume its invocation")
	}
	return nil
}
func (p *powerShellProcess) Commit(ctx context.Context) error {
	defer p.endpoint.Close()
	startup, cancel := context.WithTimeout(ctx, localipc.StartupTimeout)
	defer cancel()
	if err := p.consume(startup); err != nil {
		_ = p.Process.Abort()
		return err
	}
	if err := p.Process.Commit(startup); err != nil {
		_ = p.Process.Abort()
		return err
	}
	return nil
}
func (p *powerShellProcess) Abort() error { defer p.endpoint.Close(); return p.Process.Abort() }
