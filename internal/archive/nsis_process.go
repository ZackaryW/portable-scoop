package archive

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"portscoop/internal/runner"
)

const nsisInventoryLimit = 16 << 20
const nsisDiagnosticLimit = 64 << 10
const nsisMemoryLimit = 2 << 30
const nsisProcessTimeout = 3 * time.Minute
const nsisExtractionTimeout = 30 * time.Minute

// NSISReader never receives an output directory. The caller owns every write.
type NSISReader interface {
	List(context.Context, string) ([]byte, error)
	Stream(context.Context, string, string, io.Writer, int64) error
}

// NativeNSIS requires a fixed executable from the verified Store inventory.
// Runner is injectable for process-contract tests; nil uses the contained runner.
type NativeNSIS struct {
	Executable  string
	Environment []string
	Runner      runner.ProcessRunner
}

type cappedOutput struct {
	w      io.Writer
	left   int64
	cancel context.CancelFunc
	err    error
}

func (w *cappedOutput) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if int64(len(p)) > w.left {
		w.err = fmt.Errorf("NSIS output limit exceeded")
		w.cancel()
		return 0, w.err
	}
	n, err := w.w.Write(p)
	w.left -= int64(n)
	if err == nil && n != len(p) {
		err = io.ErrShortWrite
	}
	if err != nil {
		w.err = err
		w.cancel()
	}
	return n, err
}

func (n NativeNSIS) run(ctx context.Context, args []string, output io.Writer, limit int64) error {
	if !filepath.IsAbs(n.Executable) || limit < 0 {
		return fmt.Errorf("invalid managed NSIS process request")
	}
	ctx, cancel := context.WithTimeout(ctx, nsisProcessTimeout)
	defer cancel()
	var diagnostics bytes.Buffer
	out := &cappedOutput{w: output, left: limit, cancel: cancel}
	errout := &cappedOutput{w: &diagnostics, left: nsisDiagnosticLimit, cancel: cancel}
	r := n.Runner
	if r == nil {
		r = runner.Native{}
	}
	code, err := r.Run(ctx, runner.Command{Executable: n.Executable, Arguments: args,
		Directory: filepath.Dir(n.Executable), Environment: n.Environment,
		MemoryLimit: nsisMemoryLimit, Streams: runner.Streams{Out: out, Err: errout}})
	if out.err != nil {
		return out.err
	}
	if errout.err != nil {
		return errout.err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("NSIS reader: %w", err)
	}
	if code != 0 {
		return fmt.Errorf("NSIS reader exited %d: %s", code, diagnostics.String())
	}
	return nil
}

func (n NativeNSIS) List(ctx context.Context, source string) ([]byte, error) {
	var b bytes.Buffer
	err := n.run(ctx, []string{"l", "-slt", "-ba", "-tNsis", "-sccUTF-8", "--", source}, &b, nsisInventoryLimit)
	return b.Bytes(), err
}

func (n NativeNSIS) Stream(ctx context.Context, source, name string, output io.Writer, limit int64) error {
	return n.run(ctx, []string{"x", "-so", "-spd", "-ssc", "-tNsis", "-bso0", "-bsp0", "-bse2", "--", source, name}, output, limit)
}
