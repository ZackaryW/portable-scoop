package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"portscoop/internal/appcli"
	"portscoop/internal/application"
	"portscoop/internal/backend/scoop"
	"portscoop/internal/environment"
	"portscoop/internal/localvol"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	streams := application.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}
	if len(os.Args) == 2 && os.Args[1] == application.JobProbeOption {
		os.Exit(appcli.Report(0, application.WriteJobProbe(os.Stdout), streams))
	}
	executable, err := os.Executable()
	if err != nil {
		os.Exit(appcli.Report(0, err, streams))
	}
	local := application.Local{Verify: func(paths environment.Paths) error { _, err := scoop.Verify(paths); return err }}
	if len(os.Args) > 1 && os.Args[1] == application.ControlOption {
		if len(os.Args) != 3 {
			os.Exit(appcli.Report(0, fmt.Errorf("invalid execution control invocation"), streams))
		}
		code, err := application.ServeControl(ctx, os.Args[2], executable, version, local, streams)
		os.Exit(appcli.Report(code, err, streams))
	}
	cache, _ := os.UserCacheDir()
	router := application.Router{Executable: executable, RegistryPath: filepath.Join(cache, "portscoop", "registry.json"), Volumes: localvol.System{}, Local: local}
	code := (appcli.Frontend{Version: version, Executor: router}).Run(ctx, os.Args[1:], streams)
	os.Exit(code)
}
