package application

import (
	"context"

	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
)

type Router struct {
	Executable, RegistryPath string
	Files                    environment.Filesystem
	Volumes                  resolver.VolumeSource
	Local                    Local
	Client                   Client
}

var _ Executor = Router{}

func (r Router) Execute(ctx context.Context, in Invocation, streams Streams) (int, error) {
	if err := ValidateInvocation(in); err != nil {
		return 0, err
	}
	store := registry.Store{Path: r.RegistryPath}
	selection, err := resolver.Select(r.Files, r.Executable, store.Read, in.Selector)
	if err != nil {
		return 0, err
	}
	if selection.Local {
		return r.Local.Execute(ctx, environment.Context{Identity: selection.Identity, Paths: selection.Paths}, in, streams)
	}
	target, err := (resolver.Discovery{Files: r.Files, Volumes: r.Volumes}).Resolve(ctx, selection)
	if err != nil {
		return 0, err
	}
	if err := resolver.Refresh(ctx, store, target); err != nil {
		return 0, err
	}
	return r.Client.Forward(ctx, target, in, streams)
}
