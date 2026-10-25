// Package cli defines Portscoop's public command interface.
package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"portscoop/internal/backend/scoop"
	"portscoop/internal/environment"
	"portscoop/internal/localvol"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
)

type Options struct {
	RegistryPath   string
	Files          environment.Filesystem
	ExecutablePath string
	Volumes        resolver.VolumeSource
	Runner         runner.ProcessRunner
	Backend        scoop.Backend
	Initializer    *scoop.Initializer
}

// New constructs an independent command tree for each invocation.
func New(version string) *cobra.Command {
	cache, _ := os.UserCacheDir()
	path := ""
	if cache != "" {
		path = filepath.Join(cache, "portscoop", "registry.json")
	}
	executable, _ := os.Executable()
	return NewWithOptions(version, Options{RegistryPath: path, ExecutablePath: executable, Volumes: localvol.System{}, Runner: runner.Native{}, Backend: scoop.Managed{}, Initializer: &scoop.Initializer{ReleaseRoot: filepath.Dir(executable)}})
}

// NewWithOptions makes tests select explicit temporary registry/filesystem roots.
func NewWithOptions(version string, options Options) *cobra.Command {
	root := &cobra.Command{
		Use:              "portscoop",
		Aliases:          []string{"psp"},
		Short:            "Manage independent portable Scoop environments",
		Long:             "Portscoop locates registered portable environments or runs directly from a portable root.",
		Version:          version,
		SilenceUsage:     true,
		SilenceErrors:    true,
		TraverseChildren: true,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				return fmt.Errorf("unknown command %q for %s", args[0], cmd.Name())
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.AddCommand(newEnvironmentCommand(options))
	root.AddCommand(releaseVerificationCommand(options))
	addDelegationCommands(root, version, options)
	root.AddCommand(selfUpdateCommand(options))
	return root
}
