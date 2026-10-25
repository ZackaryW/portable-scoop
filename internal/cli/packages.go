package cli

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"portscoop/internal/backend/scoop"
	"portscoop/internal/delegation"
	"portscoop/internal/environment"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
)

func addDelegationCommands(root *cobra.Command, version string, options Options) {
	var selector string
	root.PersistentFlags().StringVar(&selector, "env", "", "Registered environment alias (place before package command)")
	execute := func(cmd *cobra.Command, local environment.Context, args []string) error {
		if _, err := scoop.Classify(args); err != nil {
			return err
		}
		if options.Backend == nil {
			return fmt.Errorf("portable package execution is not available in this build")
		}
		directory, err := os.Getwd()
		if err != nil {
			return err
		}
		local.WorkingDirectory = directory
		code, err := options.Backend.Execute(cmd.Context(), local, args, runner.Streams{In: cmd.InOrStdin(), Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()})
		if err != nil {
			return err
		}
		if code != 0 {
			return &runner.ExitError{Code: code}
		}
		return nil
	}
	handshake := &cobra.Command{Use: delegation.HandshakeCommand, Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		local, err := delegation.Local(options.Files, options.ExecutablePath, "", environment.ProtocolMajor)
		if err != nil {
			return err
		}
		data, err := delegation.EncodeHello(local, version)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(cmd.OutOrStdout(), string(data))
		return err
	}}
	internalExecute := &cobra.Command{Use: delegation.ExecuteCommand, Hidden: true, DisableFlagParsing: true, RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) < 4 || args[2] != "--" {
			return fmt.Errorf("invalid delegated invocation envelope")
		}
		major, err := strconv.Atoi(args[1])
		if err != nil {
			return err
		}
		local, err := delegation.Local(options.Files, options.ExecutablePath, args[0], major)
		if err != nil {
			return err
		}
		return execute(cmd, local, args[3:])
	}}
	root.AddCommand(handshake, internalExecute)
	for _, name := range []string{"install", "uninstall", "update", "list", "search", "status", "bucket", "reset", "cleanup", "config"} {
		command := &cobra.Command{Use: name + " [arguments]", Short: "Run " + name + " in the selected portable environment", DisableFlagParsing: true, RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") {
				return cmd.Help()
			}
			backendArgs := append([]string{cmd.Name()}, args...)
			if _, err := scoop.Classify(backendArgs); err != nil {
				return err
			}
			store := registry.Store{Path: options.RegistryPath}
			selection, err := resolver.Select(options.Files, options.ExecutablePath, store.Read, selector)
			if err != nil {
				return err
			}
			if selection.Local {
				return execute(cmd, environment.Context{Identity: selection.Identity, Paths: selection.Paths}, backendArgs)
			}
			target, err := (resolver.Discovery{Files: options.Files, Volumes: options.Volumes}).Resolve(cmd.Context(), selection)
			if err != nil {
				return err
			}
			if err := resolver.Refresh(cmd.Context(), store, target); err != nil {
				return err
			}
			directory, err := os.Getwd()
			if err != nil {
				return err
			}
			code, err := (delegation.Client{Runner: options.Runner}).Forward(cmd.Context(), target, backendArgs, directory, runner.Streams{In: cmd.InOrStdin(), Out: cmd.OutOrStdout(), Err: cmd.ErrOrStderr()})
			if err != nil {
				return err
			}
			if code != 0 {
				return &runner.ExitError{Code: code}
			}
			return nil
		}}
		root.AddCommand(command)
	}
}
