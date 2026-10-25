package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"

	"portscoop/internal/cli"
	"portscoop/internal/runner"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	command := cli.New(version)
	if err := command.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "portscoop: operation cancelled")
			os.Exit(130)
		}
		var status *runner.ExitError
		if errors.As(err, &status) {
			os.Exit(status.Code)
		}
		fmt.Fprintln(os.Stderr, "portscoop:", err)
		os.Exit(2)
	}
}
