package cmd

import "github.com/spf13/cobra"

// newSecretsCmd creates the top-level "secrets" command.
func newSecretsCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "secrets",
        Short: "Manage secrets helpers (SOPS, etc.)",
        RunE: func(cmd *cobra.Command, args []string) error {
            return cmd.Help()
        },
    }
    cmd.AddCommand(newSecretsSopsKeygenCmd())
    return cmd
}

