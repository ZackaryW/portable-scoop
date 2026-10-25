package delegation

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"portscoop/internal/environment"
	"portscoop/internal/metadata"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
)

const HandshakeCommand = "__portscoop_handshake"
const ExecuteCommand = "__portscoop_execute"

type Hello struct {
	ID              string `json:"id"`
	ProtocolMajor   int    `json:"protocol_major"`
	LauncherVersion string `json:"launcher_version"`
}

func Local(files environment.Filesystem, executable, expectedID string, major int) (environment.Context, error) {
	if major != environment.ProtocolMajor {
		return environment.Context{}, fmt.Errorf("incompatible delegation protocol %d", major)
	}
	if files == nil {
		files = environment.OSFilesystem{}
	}
	if !filepath.IsAbs(executable) {
		return environment.Context{}, fmt.Errorf("portable launcher path must be absolute")
	}
	id, paths, err := environment.Load(files, filepath.Dir(executable))
	if err != nil {
		return environment.Context{}, fmt.Errorf("delegated invocation requires adjacent portable identity: %w", err)
	}
	if expectedID != "" && id.ID != expectedID {
		return environment.Context{}, fmt.Errorf("portable identity changed before delegated execution")
	}
	return environment.Context{Identity: id, Paths: paths}, nil
}

type Client struct{ Runner runner.ProcessRunner }

type handshakeOutput struct{ buffer bytes.Buffer }

func (b *handshakeOutput) Len() int      { return b.buffer.Len() }
func (b *handshakeOutput) Bytes() []byte { return b.buffer.Bytes() }

func (b *handshakeOutput) Write(data []byte) (int, error) {
	if b.Len()+len(data) > 64*1024 {
		return 0, fmt.Errorf("portable handshake exceeded protocol size limit")
	}
	return b.buffer.Write(data)
}

func (c Client) Verify(ctx context.Context, target resolver.Target, workingDirectory string) error {
	if c.Runner == nil {
		return fmt.Errorf("launcher process execution is unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	var output handshakeOutput
	code, err := c.Runner.Run(ctx, runner.Command{Executable: filepath.Join(target.Paths.Root, "portscoop.exe"), Arguments: []string{HandshakeCommand}, Directory: workingDirectory, Environment: os.Environ(), Streams: runner.Streams{Out: &output}})
	if err != nil {
		return err
	}
	if code != 0 {
		return fmt.Errorf("portable handshake failed with status %d", code)
	}
	if output.Len() > 64*1024 {
		return fmt.Errorf("portable handshake exceeded protocol size limit")
	}
	var hello Hello
	if err := metadata.Decode(output.Bytes(), &hello); err != nil {
		return fmt.Errorf("invalid portable handshake: %w", err)
	}
	if hello.ID != target.Identity.ID {
		return fmt.Errorf("portable handshake identity mismatch")
	}
	if hello.ProtocolMajor != environment.ProtocolMajor {
		return fmt.Errorf("incompatible portable protocol %d", hello.ProtocolMajor)
	}
	return nil
}

func (c Client) Forward(ctx context.Context, target resolver.Target, args []string, workingDirectory string, streams runner.Streams) (int, error) {
	if err := c.Verify(ctx, target, workingDirectory); err != nil {
		return 0, err
	}
	forwarded := []string{ExecuteCommand, target.Identity.ID, strconv.Itoa(environment.ProtocolMajor), "--"}
	forwarded = append(forwarded, args...)
	return c.Runner.Run(ctx, runner.Command{Executable: filepath.Join(target.Paths.Root, "portscoop.exe"), Arguments: forwarded, Directory: workingDirectory, Environment: os.Environ(), Streams: streams})
}

func EncodeHello(local environment.Context, version string) ([]byte, error) {
	return json.Marshal(Hello{ID: local.Identity.ID, ProtocolMajor: environment.ProtocolMajor, LauncherVersion: version})
}
