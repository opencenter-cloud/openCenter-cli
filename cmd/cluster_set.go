package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func parseClusterSetArgs(args []string) (clusterArg string, assignments []string, err error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("at least one path=value assignment is required")
	}
	if strings.Contains(args[0], "=") {
		return "", args, nil
	}
	if len(args) == 1 {
		return "", nil, fmt.Errorf("at least one path=value assignment is required after cluster name")
	}
	return args[0], args[1:], nil
}

func newClusterSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set [cluster] <path=value>...",
		Short: "Set fields in an existing cluster configuration",
		Long: `Set one or more fields in an existing cluster configuration.

Fields use native v2 dot notation, for example:
  opencenter.meta.env=prod
  opencenter.gitops.repository.url=https://github.com/acme/platform.git`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterArg, assignments, err := parseClusterSetArgs(args)
			if err != nil {
				return err
			}

			nameArgs := []string{}
			if clusterArg != "" {
				nameArgs = []string{clusterArg}
			}
			name, err := resolveClusterNameForCommand(cmd, nameArgs, true)
			if err != nil {
				return err
			}

			cfg, _, _, _, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), name)
			if err != nil {
				return fmt.Errorf("failed to load cluster %s: %w", name, err)
			}

			for _, assignment := range assignments {
				key, value, ok := strings.Cut(assignment, "=")
				if !ok || strings.TrimSpace(key) == "" {
					return fmt.Errorf("invalid assignment %q: expected path=value", assignment)
				}
				if err := setField(cfg, key, value); err != nil {
					return fmt.Errorf("error setting config field %q: %w", key, err)
				}
			}

			strict, _ := cmd.Flags().GetBool("strict")
			if strict {
				if err := validateNativeV2Config(cfg); err != nil {
					fmt.Fprintln(cmd.ErrOrStderr(), err)
					return fmt.Errorf("validation failed")
				}
			}

			if getGlobalOptions(cmd).DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would update cluster configuration %s\n", name)
				for _, assignment := range assignments {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", assignment)
				}
				return nil
			}

			if err := saveNativeV2Config(cmd.Context(), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Updated cluster configuration %s\n", name)
			fmt.Fprintf(cmd.OutOrStdout(), "Next: opencenter cluster validate %s\n", name)
			return nil
		},
	}
	cmd.Flags().Bool("strict", false, "fail if the resulting configuration is not valid")
	return cmd
}
