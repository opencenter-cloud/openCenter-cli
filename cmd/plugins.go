package cmd

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/plugins"
	"github.com/spf13/cobra"
)

func newPluginsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugins",
		Short: "Manage openCenter plugins",
	}

	cmd.AddCommand(newPluginsListCmd())
	return cmd
}

func newPluginsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List discovered external plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			disc := plugins.Discover()
			if len(disc) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No plugins found.")
				fmt.Fprintln(cmd.OutOrStdout(), "Discovery order: OPENCENTER_PLUGINS_DIR, <config-dir>/plugins, PATH")
				return nil
			}
			// Sort for stable output
			names := make([]string, 0, len(disc))
			for name := range disc {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				use := strings.TrimPrefix(name, plugins.BinaryPrefix)
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", use, disc[name])
			}
			return nil
		},
	}
}
