package application

import (
	"portscoop/internal/environment"
	"portscoop/internal/relocation"
	"portscoop/internal/runner"
)

// BuildEnvironment snapshots prepared portable state into a private child block.
func BuildEnvironment(local environment.Context, parent []string) ([]string, error) {
	values, err := relocation.PortableEnvironmentValues(local.Paths)
	if err != nil {
		return nil, err
	}
	return runner.ApplicationEnvironment(local, parent, nil, values)
}
