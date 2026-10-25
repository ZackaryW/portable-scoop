package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"portscoop/internal/cli"
	"portscoop/internal/runner"
	"portscoop/internal/runtimeinstall"
	"portscoop/internal/selfupdate"
)

var version = "dev"

func main() {
	os.Exit(run())
}

func run() int {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	if len(os.Args) == 2 && os.Args[1] == selfupdate.ProbeOption {
		if err := selfupdate.Probe(); err != nil {
			fmt.Fprintln(os.Stderr, "portscoop:", err)
			return 2
		}
		fmt.Println("independent")
		return 0
	}
	if len(os.Args) > 1 && os.Args[1] == selfupdate.WorkerOption {
		if len(os.Args) != 4 {
			fmt.Fprintln(os.Stderr, "portscoop: invalid private worker invocation")
			return 2
		}
		if err := selfupdate.ServeWorker(ctx, os.Args[2], os.Args[3]); err != nil {
			fmt.Fprintln(os.Stderr, "portscoop:", err)
			return 2
		}
		return 0
	}
	if len(os.Args) > 1 && os.Args[1] == selfupdate.RecoverOption {
		if len(os.Args) != 3 {
			return 2
		}
		result, err := selfupdate.RecoverStaged(ctx, os.Args[2])
		if err != nil {
			fmt.Fprintln(os.Stderr, "portscoop:", err)
			return 2
		}
		fmt.Println(result.Message)
		return result.Code
	}
	command := cli.New(version)
	selected, remaining, findErr := command.Find(os.Args[1:])
	freshConversion := false
	if findErr == nil && selected.Name() == "init" && selected.Parent() != nil && selected.Parent().Name() == "env" && selected.ParseFlags(remaining) == nil && len(selected.Flags().Args()) == 1 {
		exe, _ := os.Executable()
		root := filepath.Dir(exe)
		destination, _ := filepath.Abs(selected.Flags().Args()[0])
		_, hostErr := os.Lstat(filepath.Join(root, runtimeinstall.HostControl))
		_, storeErr := os.Lstat(filepath.Join(root, ".portscoop"))
		freshConversion = strings.EqualFold(destination, root) && os.IsNotExist(hostErr) && os.IsNotExist(storeErr)
	}
	if findErr == nil && !freshConversion && selected.Name() != "self-update" && selected.Name() != "__portscoop_verify_release" {
		exe, err := os.Executable()
		if err != nil {
			return 2
		}
		lease, err := runtimeinstall.Use(ctx, exe)
		if err != nil {
			fmt.Fprintln(os.Stderr, "portscoop:", err)
			return 2
		}
		if lease != nil {
			defer lease.Close()
		}
	}
	if err := command.ExecuteContext(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			fmt.Fprintln(os.Stderr, "portscoop: operation cancelled")
			return 130
		}
		var status *runner.ExitError
		if errors.As(err, &status) {
			return status.Code
		}
		fmt.Fprintln(os.Stderr, "portscoop:", err)
		return 2
	}
	return 0
}
