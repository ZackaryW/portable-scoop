package main

import (
	"context"
	"os"
	"os/signal"

	"portscoop/internal/appcli"
	"portscoop/internal/application"
)

var version = "dev"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	code := (appcli.Frontend{Version: version}).Run(ctx, os.Args[1:], application.Streams{In: os.Stdin, Out: os.Stdout, Err: os.Stderr})
	os.Exit(code)
}
