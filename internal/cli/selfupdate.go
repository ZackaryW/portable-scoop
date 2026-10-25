package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"portscoop/internal/registry"
	"portscoop/internal/resolver"
	"portscoop/internal/runner"
	"portscoop/internal/selfupdate"
)

func selfUpdateCommand(options Options) *cobra.Command {
	var request selfupdate.Options
	command := &cobra.Command{Use: "self-update", Short: "Update this Host and registered Stores, or this Store", Long: "A Host updates all reachable registered Stores and its own runtime. A direct Store invocation updates only that Store. Store apps and data are preserved. Offline or invalid entries are reported separately. Accepted Host handoff is pending completion; inspect --status afterward.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("env") || cmd.Root().PersistentFlags().Changed("env") {
			return fmt.Errorf("self-update targets the invoked installation and does not accept --env")
		}
		maintenance := selfupdate.HostMaintenance{Service: selfupdate.Default(), ReadRegistry: (registry.Store{Path: options.RegistryPath}).Read, Resolve: (resolver.Discovery{Files: options.Files, Volumes: options.Volumes}).Resolve}
		result, err := maintenance.Run(cmd.Context(), options.ExecutablePath, request)
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), result.Message)
		if result.Code != 0 {
			return &runner.ExitError{Code: result.Code}
		}
		return nil
	}}
	command.Flags().BoolVar(&request.Check, "check", false, "Check availability without downloading or changing files")
	command.Flags().StringVar(&request.Version, "version", "", "Published release tag (known downgrades are rejected)")
	command.Flags().BoolVar(&request.Status, "status", false, "Inspect the pending or most recent update")
	command.Flags().BoolVar(&request.Recover, "recover", false, "Restore the recorded previous runtime after an interrupted update")
	return command
}
