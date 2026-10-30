package cmd

import (
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
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
  opencenter.gitops.repository.url=https://github.com/acme/platform.git

base_domain provenance: an explicit opencenter.cluster.base_domain assignment
pins the field (the cluster config owns it; cluster_defaults.base_domain from
settings never rewrites it). Pass --follow-settings to instead tag the field
managed.base_domain=default so settings changes propagate at every config load
(generate/validate/deploy). A cluster that has reached the bootstrap stage is
live: its DNS identity pins automatically and --follow-settings is rejected
there, because re-pointing a deployed cluster is a destructive rename.`,
		Example: `  # Freeze the cluster's own domain (pins against settings changes)
  opencenter cluster set my-cluster opencenter.cluster.base_domain=k8s.example.com

  # Seed a domain and keep following cluster_defaults.base_domain from settings
  opencenter cluster set my-cluster opencenter.cluster.base_domain=k8s.example.com --follow-settings`,
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

			configAssignments := make([]configAssignment, 0, len(assignments))
			for _, assignment := range assignments {
				key, value, ok := strings.Cut(assignment, "=")
				if !ok || strings.TrimSpace(key) == "" {
					return fmt.Errorf("invalid assignment %q: expected path=value", assignment)
				}
				configAssignments = append(configAssignments, configAssignment{Path: key, Value: value})
			}
			if err := applyConfigAssignments(cfg, configAssignments); err != nil {
				return fmt.Errorf("error setting config field: %w", err)
			}

			// Provenance: an explicit cluster-side edit of base_domain hands
			// ownership to the cluster. Pinning here keeps the settings file
			// from re-propagating over the user's choice at the next load,
			// and the derived cluster_fqdn is recomputed so the identity
			// fields cannot disagree on disk. --follow-settings is the
			// explicit opt-in to the opposite: tag the field
			// managed.base_domain=default so every load re-applies
			// cluster_defaults.base_domain from settings. A cluster that has
			// reached bootstrap is live; its DNS identity stays pinned.
			followSettings, _ := cmd.Flags().GetBool("follow-settings")
			for _, assignment := range configAssignments {
				if assignment.Path != "opencenter.cluster.base_domain" {
					continue
				}
				if followSettings && cfg.OpenCenter.Meta.Stage == v2.StageBootstrap {
					return fmt.Errorf("--follow-settings cannot apply to cluster %s: it is in the bootstrap stage, where base_domain is pinned (re-pointing a live cluster's DNS identity is a destructive rename)", name)
				}
				if followSettings {
					cfg.OpenCenter.Cluster.Managed.DefaultBaseDomain()
				} else {
					cfg.OpenCenter.Cluster.Managed.PinBaseDomain()
				}
				if derived := cfg.OpenCenter.Cluster.DeriveClusterFQDN(cfg.OpenCenter.Meta.Region); derived != "" {
					cfg.OpenCenter.Cluster.ClusterFQDN = derived
				}
				break
			}

			strict, _ := cmd.Flags().GetBool("strict")
			if strict {
				app, err := GetApp(cmd.Context())
				if err != nil {
					return fmt.Errorf("resolving application context: %w", err)
				}
				result, err := app.ValidateService.ValidateConfig(cmd.Context(), cfg)
				if err != nil {
					return fmt.Errorf("validation error: %w", err)
				}
				if !result.Valid {
					output := app.ValidateService.FormatResultGrouped(result, result.Provider)
					fmt.Fprint(cmd.ErrOrStderr(), output)
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
			if followSettings {
				fmt.Fprintf(cmd.OutOrStdout(), "base_domain now follows cluster_defaults.base_domain from settings (managed: default)\n")
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Next: opencenter cluster validate %s\n", name)
			return nil
		},
	}
	cmd.Flags().Bool("strict", false, "fail if the resulting configuration is not valid")
	cmd.Flags().Bool("follow-settings", false, "with an opencenter.cluster.base_domain assignment, tag the field managed.base_domain=default so cluster_defaults.base_domain from settings keeps propagating at every config load (default: pin the field to the cluster)")
	return cmd
}
