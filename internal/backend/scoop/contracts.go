package scoop

import (
	"context"
	"portscoop/internal/environment"
	"portscoop/internal/runner"
)

type Backend interface {
	Execute(context.Context, environment.Context, []string, runner.Streams) (int, error)
}
