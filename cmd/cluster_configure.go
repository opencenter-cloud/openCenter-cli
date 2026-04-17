package cmd

import (
	"fmt"
	"os"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterConfigureCmd() *cobra.Command {
	var (
		organization string
		provider     string
		guided       bool
	)

	cmd := &cobra.Command{
		Use:   "configure [name]",
		Short: "Guided cluster configuration for supported providers",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !guided {
				return fmt.Errorf("--guided is required; non-guided cluster configure is not implemented")
			}

			name, err := resolveClusterName(args, false)
			if err != nil {
				return err
			}
			if name == "" {
				return fmt.Errorf("cluster name is required")
			}

			app, err := GetApp(cmd.Context())
			if err != nil {
				return err
			}

			runner := ui.GetGuidedPromptRunner(cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr(), os.Getenv("OPENCENTER_TEST_MODE") != "")
			result, err := app.ConfigureService.Configure(cmd.Context(), cluster.ConfigureOptions{
				Identifier:   name,
				Organization: organization,
				Provider:     provider,
			}, runner)
			if err != nil {
				return err
			}

			action := "Updated"
			if result.Created {
				action = "Created"
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%s guided cluster configuration at %s\n", action, result.ConfigPath)
			return nil
		},
	}

	cmd.Flags().BoolVar(&guided, "guided", false, "run the interactive guided configure workflow")
	cmd.Flags().StringVar(&organization, "org", "", "organization for new cluster configurations")
	cmd.Flags().StringVar(&provider, "type", "openstack", "infrastructure provider for new cluster configurations")

	return cmd
}
