// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

// newClusterStatusCmd creates the "cluster status" command.
func newClusterStatusCmd() *cobra.Command {
	var showPaths bool
	var quiet bool
	var syncStatus bool
	var syncTimeout time.Duration

	cmd := &cobra.Command{
		Use:   "status [name]",
		Short: "Show cluster status information",
		Long: `Show cluster status information.

This command displays:
- The requested cluster, or the currently active cluster when no name is passed
- Basic cluster metadata (environment, region, organization)
- Cluster lifecycle state (stage and status)
- Key file paths (with --paths flag)

If no cluster is requested and no cluster is active, it will show available clusters
and suggest using 'opencenter cluster use' to set one.`,
		Example: `  # Show active cluster status
  opencenter cluster status

  # Show a specific cluster
  opencenter cluster status my-cluster

  # Show active cluster with file paths
  opencenter cluster status --paths

  # Sync service status from the live cluster into configuration
  opencenter cluster status my-cluster --sync

  # Quiet output (just the cluster name)
  opencenter cluster status --quiet`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if syncStatus {
				return runClusterStatusSync(cmd, args, getGlobalOptions(cmd).DryRun, getGlobalOptions(cmd).Output, syncTimeout)
			}

			ctx := cmd.Context()
			requestedCluster := strings.TrimSpace("")
			if len(args) > 0 {
				requestedCluster = strings.TrimSpace(args[0])
			}

			activeCluster := ""
			if requestedCluster == "" {
				var err error
				activeCluster, err = getActiveCluster()
				if err != nil {
					return fmt.Errorf("failed to get active cluster: %w", err)
				}
			}

			clusterName := requestedCluster
			if clusterName == "" {
				clusterName = activeCluster
			}

			if clusterName == "" {
				if quiet {
					return nil
				}

				fmt.Fprintf(cmd.OutOrStdout(), "No active cluster set\n\n")

				clusters, listErr := listClusters(ctx)
				if listErr == nil && len(clusters) > 0 {
					fmt.Fprintf(cmd.OutOrStdout(), "Available clusters:\n")
					for _, cluster := range clusters {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", cluster)
					}
					fmt.Fprintf(cmd.OutOrStdout(), "\nUse 'opencenter cluster use <name>' to set an active cluster\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "No clusters found. Use 'opencenter cluster init <name>' to create one.\n")
				}
				return nil
			}

			if quiet {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\n", clusterName)
				return nil
			}

			cfg, err := loadConfig(ctx, clusterName)
			if err != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", clusterName)
				fmt.Fprintf(cmd.OutOrStdout(), "Status: Configuration not found or invalid\n")
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Cluster: %s\n", clusterName)
			if activeCluster != "" && activeCluster == clusterName {
				fmt.Fprintf(cmd.OutOrStdout(), "  Active:       yes\n")
			} else if activeCluster != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  Active:       no (active cluster: %s)\n", activeCluster)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "  Name:         %s\n", cfg.OpenCenter.Meta.Name)
			fmt.Fprintf(cmd.OutOrStdout(), "  Environment:  %s\n", cfg.OpenCenter.Meta.Env)
			fmt.Fprintf(cmd.OutOrStdout(), "  Region:       %s\n", cfg.OpenCenter.Meta.Region)
			fmt.Fprintf(cmd.OutOrStdout(), "  Stage:        %s\n", displayLifecycleValue(cfg.OpenCenter.Meta.Stage))
			fmt.Fprintf(cmd.OutOrStdout(), "  Status:       %s\n", cfg.OpenCenter.Meta.Status)
			fmt.Fprintf(cmd.OutOrStdout(), "  Organization: %s\n", cfg.OpenCenter.Meta.Organization)
			fmt.Fprintf(cmd.OutOrStdout(), "  Provider:     %s\n", cfg.OpenCenter.Infrastructure.Provider)

			pathResolver := paths.NewPathResolver(config.ResolveClustersDir())
			resolvedClusterPaths, _ := pathResolver.Resolve(ctx, cfg.ClusterName(), cfg.OpenCenter.Meta.Organization)

			if showPaths && resolvedClusterPaths != nil {
				fmt.Fprintf(cmd.OutOrStdout(), "\nCluster Paths:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config Directory:  %s\n", resolvedClusterPaths.ClusterDir)
				fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key:          %s\n", resolvedClusterPaths.SOPSKeyPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Directory:  %s\n", resolvedClusterPaths.GitOpsDir)

				if _, err := os.Stat(resolvedClusterPaths.SOPSKeyPath); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✓ Present\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  SOPS Key Status:   ✗ Missing\n")
				}

				if _, err := os.Stat(resolvedClusterPaths.GitOpsDir); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✓ Initialized\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Status:     ✗ Not initialized\n")
				}

				if _, err := os.Stat(resolvedClusterPaths.KubeconfigPath); err == nil {
					fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✓ Present\n")
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        ✗ Missing\n")
				}
			}

			if strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "kind") && resolvedClusterPaths != nil {
				renderedKindConfigPath := filepath.Join(resolvedClusterPaths.ClusterDir, "kind-config.yaml")
				gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
				kindConfigReady := pathExists(renderedKindConfigPath)
				kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
				clusterExists, apiReady, endpoint, providerError := kindClusterStatus(ctx, &cfg, resolvedClusterPaths.KubeconfigPath)
				defaultCNIStatus := "Enabled"
				if cfg.OpenCenter.Infrastructure.Kind != nil && cfg.OpenCenter.Infrastructure.Kind.DisableDefaultCNI {
					defaultCNIStatus = "Disabled"
				}

				fmt.Fprintf(cmd.OutOrStdout(), "\nKind Status:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config:            ✓ Present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Default CNI:       %s\n", defaultCNIStatus)
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Setup:      %s\n", statusLabel(gitOpsReady, "Ready", "Not ready"))
				fmt.Fprintf(cmd.OutOrStdout(), "  kind-config.yaml:  %s\n", statusLabel(kindConfigReady, "Present", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        %s\n", statusLabel(kubeconfigReady, "Present", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Cluster Exists:    %s\n", statusLabel(clusterExists, "Present", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         %s\n", statusLabel(apiReady, "Ready", "Not ready"))
				if endpoint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Endpoint:      %s\n", endpoint)
				}
				if providerError != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Provider Check:    %s\n", providerError)
				}
			}
			if strings.EqualFold(cfg.OpenCenter.Infrastructure.Provider, "openstack") && resolvedClusterPaths != nil {
				infraDir := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "infrastructure", "clusters", cfg.ClusterName())
				gitOpsReady := pathExists(cfg.OpenCenter.GitOps.Repository.LocalDir)
				infraReady := pathExists(infraDir)
				kubeconfigReady := pathExists(resolvedClusterPaths.KubeconfigPath)
				apiReady, endpoint, providerError := cloudClusterStatus(ctx, resolvedClusterPaths.KubeconfigPath)

				fmt.Fprintf(cmd.OutOrStdout(), "\nOpenStack Status:\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  Config:            ✓ Present\n")
				fmt.Fprintf(cmd.OutOrStdout(), "  GitOps Repo:       %s\n", statusLabel(gitOpsReady, "Ready", "Not ready"))
				fmt.Fprintf(cmd.OutOrStdout(), "  Infrastructure:    %s\n", statusLabel(infraReady, "Rendered", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  OpenTofu State:    %s\n", openTofuStateStatus(&cfg, infraDir))
				fmt.Fprintf(cmd.OutOrStdout(), "  Kubeconfig:        %s\n", statusLabel(kubeconfigReady, "Present", "Missing"))
				fmt.Fprintf(cmd.OutOrStdout(), "  API Ready:         %s\n", statusLabel(apiReady, "Ready", "Not ready"))
				if endpoint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  API Endpoint:      %s\n", endpoint)
				}
				if providerError != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "  Provider Check:    %s\n", providerError)
				}
			}

			stage := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Stage))
			status := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Status))
			nextSteps := nextStepsForCluster(clusterName, stage, status)
			fmt.Fprintf(cmd.OutOrStdout(), "\nNext Steps:\n")
			for _, step := range nextSteps {
				fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", step)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&showPaths, "paths", false, "show cluster file paths and their status")
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "quiet output (just the cluster name)")
	cmd.Flags().BoolVar(&syncStatus, "sync", false, "sync service status from the live cluster into configuration")
	cmd.Flags().DurationVar(&syncTimeout, "sync-timeout", 30*time.Second, "timeout for live cluster status sync")

	return cmd
}

func displayLifecycleValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "unknown"
	}

	return value
}

func nextStepsForCluster(clusterName, stage, status string) []string {
	switch {
	case stage == "" || stage == config.StageInit:
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Review the cluster config and rerun 'opencenter cluster init %s --force' if needed", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'opencenter cluster generate %s' to generate the GitOps repository", clusterName),
			fmt.Sprintf("Run 'opencenter cluster validate %s' to validate configuration", clusterName),
		}
	case stage == config.StageSetup:
		if status == config.StatusRunning {
			return []string{
				fmt.Sprintf("Wait for 'opencenter cluster generate %s' to finish", clusterName),
			}
		}
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Fix the generate error and rerun 'opencenter cluster generate %s'", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'opencenter cluster deploy %s' to provision the cluster", clusterName),
		}
	case stage == config.StageBootstrap:
		if status == config.StatusRunning {
			return []string{
				fmt.Sprintf("Wait for 'opencenter cluster deploy %s' to finish", clusterName),
			}
		}
		if status == config.StatusFailed {
			return []string{
				fmt.Sprintf("Fix the deploy error and rerun 'opencenter cluster deploy %s'", clusterName),
			}
		}
		return []string{
			fmt.Sprintf("Run 'eval $(opencenter cluster env %s)' or set KUBECONFIG to the cluster-owned kubeconfig", clusterName),
			"Use 'kubectl' to interact with the cluster",
		}
	default:
		if status == config.StatusFailed {
			return []string{"Check cluster logs and rerun the last failed lifecycle command"}
		}
		return []string{"Check cluster documentation for next steps"}
	}
}

func statusLabel(ok bool, okLabel, missingLabel string) string {
	if ok {
		return "✓ " + okLabel
	}

	return "✗ " + missingLabel
}

func pathExists(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}

	_, err := os.Stat(path)
	return err == nil
}

func openTofuStateStatus(cfg *v2.Config, infraDir string) string {
	if cfg == nil || !cfg.OpenTofu.Enabled {
		return "✗ Disabled"
	}

	backendType := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
	switch backendType {
	case "", "local":
		statePath := ""
		if cfg.OpenTofu.Backend.Local != nil {
			statePath = strings.TrimSpace(cfg.OpenTofu.Backend.Local.Path)
		}
		if statePath == "" {
			statePath = fmt.Sprintf(".opentofu-local-%s/terraform.tfstate", cfg.ClusterName())
		}
		if !filepath.IsAbs(statePath) {
			statePath = filepath.Join(infraDir, statePath)
		}
		return statusLabel(pathExists(statePath), "Present", "Missing")
	case "s3", "aws":
		if cfg.OpenTofu.Backend.S3 != nil &&
			strings.TrimSpace(cfg.OpenTofu.Backend.S3.Bucket) != "" &&
			strings.TrimSpace(cfg.OpenTofu.Backend.S3.Key) != "" {
			return "✓ Remote backend configured"
		}
		return "✗ Remote backend unconfigured"
	default:
		return fmt.Sprintf("✗ Unsupported backend (%s)", backendType)
	}
}

func cloudClusterStatus(ctx context.Context, kubeconfigPath string) (bool, string, string) {
	if !pathExists(kubeconfigPath) {
		return false, "", ""
	}

	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	runner := security.GetDefaultCommandRunner()

	clusterInfoCmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "cluster-info")
	if err != nil {
		return false, "", err.Error()
	}
	if err := clusterInfoCmd.Run(); err != nil {
		return false, "", err.Error()
	}

	endpointCmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		return false, "", err.Error()
	}
	output, err := endpointCmd.Output()
	if err != nil {
		return false, "", err.Error()
	}

	return true, strings.TrimSpace(string(output)), ""
}

func kindClusterStatus(ctx context.Context, cfg *v2.Config, kubeconfigPath string) (bool, bool, string, string) {
	env := kindprovider.BuildEnvironment("")

	provider := kindprovider.NewProvider()
	clusterName := cfg.ClusterName()

	exists, err := provider.ClusterExists(ctx, clusterName, env)
	if err != nil {
		return false, false, "", err.Error()
	}
	if !exists || !pathExists(kubeconfigPath) {
		return exists, false, "", ""
	}

	statusCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	ready, endpoint, err := provider.APIReady(statusCtx, kubeconfigPath)
	if err != nil {
		return true, false, "", err.Error()
	}

	return true, ready, endpoint, ""
}
