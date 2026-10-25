package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"portscoop/internal/backend/scoop"
	"portscoop/internal/registry"
)

func newEnvironmentCommand(options Options) *cobra.Command {
	manager := registry.Manager{Store: registry.Store{Path: options.RegistryPath}, Files: options.Files}
	command := &cobra.Command{Use: "env", Short: "Manage host registrations for portable environments"}
	var displayName string
	initialize := &cobra.Command{Use: "init <path> --name <display-name>", Short: "Initialize an independent portable root from this verified release", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		if options.Initializer == nil {
			return fmt.Errorf("portable initialization requires a verified release payload")
		}
		id, err := options.Initializer.Init(cmd.Context(), args[0], displayName)
		if err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Initialized %s (%s) at %s\n", id.Name, id.ID, args[0])
		return nil
	}}
	initialize.Flags().StringVar(&displayName, "name", "", "Portable environment display name")
	initialize.MarkFlagRequired("name")
	var name string
	register := &cobra.Command{Use: "register <path> --name <alias>", Short: "Register a ready portable environment without modifying its package data", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if err := manager.Register(ctx, args[0], name); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Registered", name)
		return nil
	}}
	register.Flags().StringVar(&name, "name", "", "Case-insensitive host alias")
	register.MarkFlagRequired("name")
	defaultCommand := &cobra.Command{Use: "default <alias>", Short: "Select the explicit host default environment", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if err := manager.SetDefault(ctx, args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Default environment:", args[0])
		return nil
	}}
	unregister := &cobra.Command{Use: "unregister <alias>", Short: "Remove a host registration while preserving every portable file", Args: cobra.ExactArgs(1), RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		if err := manager.Unregister(ctx, args[0]); err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Unregistered", args[0])
		return nil
	}}
	list := &cobra.Command{Use: "list", Short: "List registrations and bounded availability diagnostics", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(cmd.Context(), 2*time.Second)
		defer cancel()
		rows, err := manager.List(ctx)
		if err != nil {
			return err
		}
		out := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 4, 2, ' ', 0)
		fmt.Fprintln(out, "ALIAS\tIDENTITY\tDEFAULT\tSTATUS\tLOCATION")
		for _, row := range rows {
			marker := ""
			if row.Default {
				marker = "*"
			}
			fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\n", row.Alias, row.Entry.ID, marker, row.Status, row.Entry.Path)
		}
		return out.Flush()
	}}
	command.AddCommand(initialize, register, defaultCommand, unregister, list)
	return command
}

func releaseVerificationCommand(options Options) *cobra.Command {
	return &cobra.Command{Use: "__portscoop_verify_release", Hidden: true, Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, args []string) error {
		_, err := scoop.VerifyRelease(filepath.Dir(options.ExecutablePath))
		if err != nil {
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "Portscoop release payload verified")
		return nil
	}}
}
