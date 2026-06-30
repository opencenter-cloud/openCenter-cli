package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func newClusterPoolCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pool",
		Short: "Manage worker pools",
		Long:  "Add, update, remove, scale, and list worker pools in a cluster configuration.",
	}
	cmd.AddCommand(newClusterPoolAddCmd())
	cmd.AddCommand(newClusterPoolUpdateCmd())
	cmd.AddCommand(newClusterPoolScaleCmd())
	cmd.AddCommand(newClusterPoolRemoveCmd())
	cmd.AddCommand(newClusterPoolListCmd())
	return cmd
}

func newClusterPoolAddCmd() *cobra.Command {
	var (
		count          int
		flavor         string
		image          string
		osType         string
		bootVolumeSize int
		bootVolumeType string
		labels         []string
		taints         []string
		cluster        string
	)
	cmd := &cobra.Command{
		Use:   "add <pool-name>",
		Short: "Add a worker pool to the cluster configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Check duplicate
			if poolExists(&cfg, poolName) {
				return fmt.Errorf("pool %q already exists", poolName)
			}

			parsedLabels := parseKeyValues(labels)
			parsedTaints, err := parseTaintFlags(taints)
			if err != nil {
				return err
			}

			if strings.EqualFold(osType, "windows") {
				pool := v2.WindowsWorkerPoolConfig{
					Name:   poolName,
					Count:  count,
					Flavor: flavor,
					Image:  image,
					Labels: parsedLabels,
					Taints: parsedTaints,
				}
				if bootVolumeSize > 0 {
					pool.BootVolume = v2.VolumeConfig{Size: bootVolumeSize, Type: bootVolumeType}
				}
				cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows = append(
					cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows, pool)
			} else {
				pool := v2.WorkerPoolConfig{
					Name:   poolName,
					Count:  count,
					Flavor: flavor,
					Image:  image,
					Labels: parsedLabels,
					Taints: parsedTaints,
				}
				if bootVolumeSize > 0 {
					pool.BootVolume = v2.VolumeConfig{Size: bootVolumeSize, Type: bootVolumeType}
				}
				cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = append(
					cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker, pool)
			}

			if getGlobalOptions(cmd).DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would add %s pool %q (count=%d, flavor=%s)\n", osType, poolName, count, flavor)
				return nil
			}

			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pool %q added. Run `opencenter cluster generate %s` to apply.\n", poolName, clusterName)
			return nil
		},
	}
	cmd.Flags().IntVar(&count, "count", 1, "Number of nodes in the pool")
	cmd.Flags().StringVar(&flavor, "flavor", "", "Instance flavor (required)")
	cmd.Flags().StringVar(&image, "image", "", "OS image override")
	cmd.Flags().StringVar(&osType, "os", "linux", "Pool OS type (linux|windows)")
	cmd.Flags().IntVar(&bootVolumeSize, "boot-volume-size", 0, "Boot volume size in GB")
	cmd.Flags().StringVar(&bootVolumeType, "boot-volume-type", "", "Boot volume type")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "Node label (key=value, repeatable)")
	cmd.Flags().StringSliceVar(&taints, "taint", nil, "Node taint (key=value:effect, repeatable)")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Cluster name")
	_ = cmd.MarkFlagRequired("flavor")
	return cmd
}

func newClusterPoolUpdateCmd() *cobra.Command {
	var (
		count          int
		flavor         string
		image          string
		bootVolumeSize int
		bootVolumeType string
		cluster        string
	)
	cmd := &cobra.Command{
		Use:   "update <pool-name>",
		Short: "Update an existing worker pool",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			updated := false
			for i := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
				if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i].Name == poolName {
					p := &cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i]
					if cmd.Flags().Changed("count") {
						p.Count = count
					}
					if cmd.Flags().Changed("flavor") {
						p.Flavor = flavor
					}
					if cmd.Flags().Changed("image") {
						p.Image = image
					}
					if cmd.Flags().Changed("boot-volume-size") {
						p.BootVolume.Size = bootVolumeSize
					}
					if cmd.Flags().Changed("boot-volume-type") {
						p.BootVolume.Type = bootVolumeType
					}
					updated = true
					break
				}
			}
			if !updated {
				for i := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
					if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[i].Name == poolName {
						p := &cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[i]
						if cmd.Flags().Changed("count") {
							p.Count = count
						}
						if cmd.Flags().Changed("flavor") {
							p.Flavor = flavor
						}
						if cmd.Flags().Changed("image") {
							p.Image = image
						}
						if cmd.Flags().Changed("boot-volume-size") {
							p.BootVolume.Size = bootVolumeSize
						}
						if cmd.Flags().Changed("boot-volume-type") {
							p.BootVolume.Type = bootVolumeType
						}
						updated = true
						break
					}
				}
			}
			if !updated {
				return fmt.Errorf("pool %q not found", poolName)
			}

			if getGlobalOptions(cmd).DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would update pool %q\n", poolName)
				return nil
			}

			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pool %q updated. Run `opencenter cluster generate %s` to apply.\n", poolName, clusterName)
			return nil
		},
	}
	cmd.Flags().IntVar(&count, "count", 0, "Number of nodes")
	cmd.Flags().StringVar(&flavor, "flavor", "", "Instance flavor")
	cmd.Flags().StringVar(&image, "image", "", "OS image override")
	cmd.Flags().IntVar(&bootVolumeSize, "boot-volume-size", 0, "Boot volume size in GB")
	cmd.Flags().StringVar(&bootVolumeType, "boot-volume-type", "", "Boot volume type")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Cluster name")
	return cmd
}

func newClusterPoolScaleCmd() *cobra.Command {
	var (
		count   int
		cluster string
	)
	cmd := &cobra.Command{
		Use:   "scale <pool-name>",
		Short: "Scale a worker pool to a specific count",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			scaled := false
			for i := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
				if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i].Name == poolName {
					cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i].Count = count
					scaled = true
					break
				}
			}
			if !scaled {
				for i := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
					if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[i].Name == poolName {
						cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[i].Count = count
						scaled = true
						break
					}
				}
			}
			if !scaled {
				return fmt.Errorf("pool %q not found", poolName)
			}

			if getGlobalOptions(cmd).DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would scale pool %q to %d\n", poolName, count)
				return nil
			}

			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pool %q scaled to %d. Run `opencenter cluster generate %s` to apply.\n", poolName, count, clusterName)
			return nil
		},
	}
	cmd.Flags().IntVar(&count, "count", 0, "Target node count")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Cluster name")
	_ = cmd.MarkFlagRequired("count")
	return cmd
}

func newClusterPoolRemoveCmd() *cobra.Command {
	var (
		force   bool
		cluster string
	)
	cmd := &cobra.Command{
		Use:   "remove <pool-name>",
		Short: "Remove a worker pool from the cluster configuration",
		Long: `Remove a worker pool definition. The pool must have count=0 (scaled to zero)
before removal to prevent orphaned infrastructure. Use --force to bypass.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			poolName := args[0]

			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			removed := false
			for i, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
				if pool.Name == poolName {
					if pool.Count > 0 && !force {
						return poolRemoveBlockedError(poolName, pool.Count)
					}
					cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = append(
						cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[:i],
						cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i+1:]...)
					removed = true
					break
				}
			}
			if !removed {
				for i, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
					if pool.Name == poolName {
						if pool.Count > 0 && !force {
							return poolRemoveBlockedError(poolName, pool.Count)
						}
						cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows = append(
							cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[:i],
							cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows[i+1:]...)
						removed = true
						break
					}
				}
			}
			if !removed {
				return fmt.Errorf("pool %q not found", poolName)
			}

			if getGlobalOptions(cmd).DryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Would remove pool %q\n", poolName)
				return nil
			}

			if err := saveConfig(cmd.Context(), cfg); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pool %q removed from cluster %q.\n", poolName, clusterName)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "Bypass scale-to-zero check")
	cmd.Flags().StringVar(&cluster, "cluster", "", "Cluster name")
	return cmd
}

func newClusterPoolListCmd() *cobra.Command {
	var cluster string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all worker pools",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clusterName, err := resolveClusterNameFromFlagForCommand(cmd, cluster, true)
			if err != nil {
				return err
			}
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			format := getGlobalOptions(cmd).Output
			if format == OutputJSON || format == OutputYAML {
				return outputPoolsStructured(cmd, &cfg, format)
			}

			// Default worker pool
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-7s %-20s %-10s %s\n", "NAME", "TYPE", "COUNT", "FLAVOR", "VOLUME", "STATUS")
			wvSize := cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize
			fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-7d %-20s %-10s %s\n",
				"default", "linux",
				cfg.OpenCenter.Infrastructure.Compute.WorkerCount,
				cfg.OpenCenter.Infrastructure.Compute.FlavorWorker,
				formatVolume(wvSize), poolStatus(cfg.OpenCenter.Infrastructure.Compute.WorkerCount))

			// Default Windows pool
			if cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-7d %-20s %-10s %s\n",
					"default-windows", "windows",
					cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows,
					cfg.OpenCenter.Infrastructure.Compute.FlavorWorkerWindows,
					"-", poolStatus(cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows))
			}

			// Additional Linux pools
			for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-7d %-20s %-10s %s\n",
					pool.Name, "linux", pool.Count, pool.Flavor,
					formatVolume(pool.BootVolume.Size), poolStatus(pool.Count))
			}

			// Additional Windows pools
			for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
				fmt.Fprintf(cmd.OutOrStdout(), "%-20s %-10s %-7d %-20s %-10s %s\n",
					pool.Name, "windows", pool.Count, pool.Flavor,
					formatVolume(pool.BootVolume.Size), poolStatus(pool.Count))
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&cluster, "cluster", "", "Cluster name")
	return cmd
}

// Helpers

func poolExists(cfg *v2.Config, name string) bool {
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
		if p.Name == name {
			return true
		}
	}
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if p.Name == name {
			return true
		}
	}
	return false
}

func poolRemoveBlockedError(name string, count int) error {
	return fmt.Errorf(`pool %q has count=%d. Scale to zero first to decommission nodes:
  opencenter cluster pool scale %s --count=0
  opencenter cluster generate
  opencenter cluster deploy
Then re-run: opencenter cluster pool remove %s`, name, count, name, name)
}

func poolStatus(count int) string {
	if count == 0 {
		return "draining"
	}
	return "active"
}

func formatVolume(size int) string {
	if size <= 0 {
		return "-"
	}
	return fmt.Sprintf("%dGB", size)
}

func parseKeyValues(items []string) map[string]string {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]string, len(items))
	for _, item := range items {
		k, v, _ := strings.Cut(item, "=")
		m[k] = v
	}
	return m
}

func parseTaintFlags(items []string) ([]v2.TaintConfig, error) {
	if len(items) == 0 {
		return nil, nil
	}
	var taints []v2.TaintConfig
	for _, item := range items {
		// Format: key=value:effect
		kvPart, effect, ok := strings.Cut(item, ":")
		if !ok {
			return nil, fmt.Errorf("invalid taint format %q (expected key=value:effect)", item)
		}
		key, value, _ := strings.Cut(kvPart, "=")
		taints = append(taints, v2.TaintConfig{Key: key, Value: value, Effect: effect})
	}
	return taints, nil
}

func outputPoolsStructured(cmd *cobra.Command, cfg *v2.Config, format OutputFormat) error {
	type poolEntry struct {
		Name   string `json:"name" yaml:"name"`
		Type   string `json:"type" yaml:"type"`
		Count  int    `json:"count" yaml:"count"`
		Flavor string `json:"flavor" yaml:"flavor"`
		Status string `json:"status" yaml:"status"`
	}
	var pools []poolEntry
	pools = append(pools, poolEntry{
		Name: "default", Type: "linux",
		Count: cfg.OpenCenter.Infrastructure.Compute.WorkerCount,
		Flavor: cfg.OpenCenter.Infrastructure.Compute.FlavorWorker,
		Status: poolStatus(cfg.OpenCenter.Infrastructure.Compute.WorkerCount),
	})
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
		pools = append(pools, poolEntry{Name: p.Name, Type: "linux", Count: p.Count, Flavor: p.Flavor, Status: poolStatus(p.Count)})
	}
	for _, p := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		pools = append(pools, poolEntry{Name: p.Name, Type: "windows", Count: p.Count, Flavor: p.Flavor, Status: poolStatus(p.Count)})
	}
	return writeStructuredOutput(cmd, format, pools)
}
