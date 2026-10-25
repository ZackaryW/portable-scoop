package appcli

import (
	"context"
	"errors"
	"fmt"
	"os"

	"portscoop/internal/application"
)

const Usage = `Usage: pspe [--env <alias>] [--wait | --detach] <shim> [arguments...]

Launch an installed Store shim. No application arguments means detached launch;
one or more arguments means foreground execution. Override with --wait/--detach.
Launcher options stop at the shim name. Every later token belongs to the app.

  pspe editor                         Launch detached
  pspe rg -n hello .                   Wait with terminal streams and app status
  pspe --wait python                   Wait without app arguments
  pspe --detach editor file.txt        Detach with app arguments
  pspe --env work rg --help            Select work; forward --help to rg
  pspe -- tool --version --            Forward --version and -- to tool

  --env <alias>   Select a registered Store (before the shim only)
  --wait         Wait for the application and return its status
  --detach       Return 0 after successful startup handoff
  --help, -h     Show launcher help
  --version      Show launcher version

Arguments are preserved as received. Your calling shell can transform them first;
Windows PowerShell 5.1 native argument passing can lose empty string arguments.
`

type Frontend struct {
	Version   string
	Executor  application.Executor
	Directory func() (string, error)
}

// Run separates launcher failures (diagnostic/status 2), cancellation (130), and
// application statuses (returned unchanged without a second diagnostic).
func (f Frontend) Run(ctx context.Context, args []string, streams application.Streams) int {
	code, err := f.execute(ctx, args, streams)
	return Report(code, err, streams)
}

func Report(code int, err error, streams application.Streams) int {
	if err == nil {
		return code
	}
	if errors.Is(err, context.Canceled) {
		fmt.Fprintln(streams.Err, "pspe: application execution cancelled")
		return 130
	}
	fmt.Fprintln(streams.Err, "pspe:", err)
	return 2
}

func (f Frontend) execute(ctx context.Context, args []string, streams application.Streams) (int, error) {
	in, action, err := Parse(args)
	if err != nil {
		return 0, err
	}
	switch action {
	case Help:
		_, err = fmt.Fprint(streams.Out, Usage)
		return 0, err
	case Version:
		_, err = fmt.Fprintln(streams.Out, "pspe version", f.Version)
		return 0, err
	}
	wd := f.Directory
	if wd == nil {
		wd = os.Getwd
	}
	in.Directory, err = wd()
	if err != nil {
		return 0, err
	}
	if f.Executor == nil {
		return 0, fmt.Errorf("application execution service is unavailable")
	}
	return f.Executor.Execute(ctx, in, streams)
}
