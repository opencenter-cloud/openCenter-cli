package cmd

import (
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/plugins"
	"github.com/spf13/cobra"
)

func NewPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage opencenter plugins",
	}

	cmd.AddCommand(newPluginsListCmd())
	return cmd
}

func newPluginsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List discovered external plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			disc := plugins.DiscoverDetailed()
			opts := getGlobalOptions(cmd)
			if opts.Output == OutputJSON || opts.Output == OutputYAML {
				return writeStructuredOutput(cmd, opts.Output, disc)
			}
			if len(disc) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins found.")
				fmt.Fprintln(cmd.OutOrStdout(), "Discovery order: OPENCENTER_PLUGINS_DIR, <config-dir>/plugins, PATH")
				return nil
			}
			for _, name := range plugins.SortedPluginNames(disc) {
				use := strings.TrimPrefix(name, plugins.BinaryPrefix)
				info := disc[name]
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\n", use, info.Path, info.Status, info.Message)
			}
			return nil
		},
	}
	markReadOnlyCommand(cmd)
	return cmd
}
