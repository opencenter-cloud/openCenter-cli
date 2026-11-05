package cmd

import "github.com/spf13/cobra"

func newClusterProviderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "provider",
		Short: "Manage provider configuration",
		Long: `Configure how a cluster connects to its cloud provider.

Each subcommand works on an existing cluster whose infrastructure.provider
matches the subcommand's provider, and changes only that cluster's
configuration file -- it does not deploy or modify resources in the cloud.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error { return cmd.Help() },
	}
	cmd.AddCommand(newClusterProviderOpenStackCmd())
	return cmd
}
