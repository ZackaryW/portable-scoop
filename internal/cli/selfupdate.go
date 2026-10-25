package cli

import (
	"fmt"
	"github.com/spf13/cobra"
	"portscoop/internal/runner"
	"portscoop/internal/selfupdate"
)

func selfUpdateCommand(options Options) *cobra.Command {
	var request selfupdate.Options
	command := &cobra.Command{Use: "self-update", Short: "Update this Host or Store's Portscoop runtime", Long: "Update only the installation containing the invoked executable. Host defaults and the current directory do not select the target. Accepted handoff is pending completion; inspect --status afterward.", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		if cmd.Flags().Changed("env") || cmd.Root().PersistentFlags().Changed("env") {
			return fmt.Errorf("self-update targets the invoked installation and does not accept --env")
		}
		result, err := selfupdate.Default().Run(cmd.Context(), options.ExecutablePath, request)
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
