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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/cloud"
	"github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/cloud/vmware"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/importer"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
)

// newClusterDriftCmd creates the parent drift command
func newClusterDriftCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "drift",
		Short: "Detect and reconcile infrastructure drift",
		Long: `Detect and reconcile differences between desired configuration and actual infrastructure state.

Drift detection compares the cluster configuration with the actual state of cloud resources
(VMs, networks, security groups, load balancers) and reports any differences. Drift can be
classified by severity (critical, warning, info) and reconcilability.`,
		Example: `  # Detect drift for a cluster
  opencenter cluster drift detect my-cluster

  # Reconcile detected drift (dry-run)
  opencenter cluster drift reconcile my-cluster --dry-run

  # Reconcile detected drift (apply changes)
  opencenter cluster drift reconcile my-cluster

  # Schedule periodic drift detection
  opencenter cluster drift schedule my-cluster --interval=24h`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newClusterDriftDetectCmd())
	cmd.AddCommand(newClusterDriftReconcileCmd())
	cmd.AddCommand(newClusterDriftScheduleCmd())

	return cmd
}

// newClusterDriftDetectCmd creates the drift detect subcommand
func newClusterDriftDetectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "detect [cluster]",
		Short: "Detect infrastructure drift for a cluster",
		Long: `Detect differences between desired configuration and actual infrastructure state.

This command queries cloud provider APIs to retrieve the actual state of resources
(VMs, networks, security groups, load balancers) and compares them with the desired
configuration. It generates a drift report showing:
  - Resource type and ID
  - Field that has drifted
  - Expected vs actual values
  - Severity (critical, warning, info)
  - Whether the drift is reconcilable

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Detect drift for active cluster
  opencenter cluster drift detect

  # Detect drift for a specific cluster
  opencenter cluster drift detect my-cluster

  # Output drift report as JSON
  opencenter cluster drift detect my-cluster --output=json

  # Show only critical drift
  opencenter cluster drift detect my-cluster --severity=critical`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			clusterName, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Create cloud provider factory
			factory := createCloudProviderFactory(cfg)

			// Get provider for this cluster
			provider, err := factory.GetProvider(canonicalDriftProvider(cfg.OpenCenter.Infrastructure.Provider))
			if err != nil {
				return fmt.Errorf("failed to get cloud provider: %w", err)
			}

			// Get current state from provider
			currentState, err := provider.GetCurrentState(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed to get current infrastructure state: %w", err)
			}

			// Build desired state from configuration
			desiredState := buildDesiredState(cfg)

			// Detect drift
			report, err := provider.DetectDrift(cmd.Context(), desiredState, currentState)
			if err != nil {
				return fmt.Errorf("failed to detect drift: %w", err)
			}

			// Set cluster name and timestamp
			report.ClusterName = clusterName
			report.DetectedAt = time.Now().Format(time.RFC3339)

			// Get output format
			outputFormat, _ := cmd.Flags().GetString("output")
			severityFilter, _ := cmd.Flags().GetString("severity")

			// Filter by severity if specified
			if severityFilter != "" {
				report = filterBySeverity(report, severityFilter)
			}

			configPromotion, _ := buildConfigPromotionPreview(cmd.Context(), clusterName)

			// Output report
			return outputDriftReport(cmd, report, outputFormat, configPromotion)
		},
	}

	cmd.Flags().String("output", "text", "Output format (text, json, yaml)")
	cmd.Flags().String("severity", "", "Filter by severity (critical, warning, info)")

	return cmd
}

// newClusterDriftReconcileCmd creates the drift reconcile subcommand
func newClusterDriftReconcileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reconcile [cluster]",
		Short: "Reconcile detected infrastructure drift",
		Long: `Reconcile differences between desired configuration and actual infrastructure state.

This command first detects drift, then applies changes to bring the actual infrastructure
state back in line with the desired configuration. Only reconcilable drift can be fixed
automatically. Non-reconcilable drift (e.g., deleted resources, manual resource creation)
requires manual intervention.

Use --dry-run to see what changes would be made without applying them.

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Reconcile drift for active cluster
  opencenter cluster drift reconcile

  # Show what would be reconciled (dry-run)
  opencenter cluster drift reconcile my-cluster --dry-run

  # Apply reconciliation
  opencenter cluster drift reconcile my-cluster

  # Reconcile with confirmation prompt
  opencenter cluster drift reconcile my-cluster --confirm`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			clusterName, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			confirm, _ := cmd.Flags().GetBool("confirm")
			toConfig, _ := cmd.Flags().GetBool("to-config")

			if toConfig {
				return reconcileDriftToConfig(cmd, clusterName, dryRun)
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Create cloud provider factory
			factory := createCloudProviderFactory(cfg)

			// Get provider for this cluster
			provider, err := factory.GetProvider(canonicalDriftProvider(cfg.OpenCenter.Infrastructure.Provider))
			if err != nil {
				return fmt.Errorf("failed to get cloud provider: %w", err)
			}

			// Get current state from provider
			currentState, err := provider.GetCurrentState(cmd.Context(), cfg)
			if err != nil {
				return fmt.Errorf("failed to get current infrastructure state: %w", err)
			}

			// Build desired state from configuration
			desiredState := buildDesiredState(cfg)

			// Detect drift
			report, err := provider.DetectDrift(cmd.Context(), desiredState, currentState)
			if err != nil {
				return fmt.Errorf("failed to detect drift: %w", err)
			}

			// Set cluster name and timestamp
			report.ClusterName = clusterName
			report.DetectedAt = time.Now().Format(time.RFC3339)

			if len(report.Drifts) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No drift detected")
				return nil
			}

			// Show what would be reconciled
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Drift reconciliation plan for cluster %s:\n\n", clusterName)
				for _, drift := range report.Drifts {
					if drift.Reconcilable {
						fmt.Fprintf(cmd.OutOrStdout(), "  - %s %s.%s: %v -> %v\n",
							drift.ResourceType, drift.ResourceName, drift.Field,
							drift.Actual, drift.Expected)
					}
				}
				return nil
			}

			// Confirm before applying
			if confirm {
				fmt.Fprintf(cmd.OutOrStdout(), "About to reconcile %d drift items. Continue? [y/N]: ", len(report.Drifts))
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" {
					return fmt.Errorf("reconciliation cancelled")
				}
			}

			// Apply reconciliation
			if err := provider.ReconcileDrift(cmd.Context(), report); err != nil {
				return fmt.Errorf("failed to reconcile drift: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Successfully reconciled %d drift items\n", report.Summary.ReconcilableCount)
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be changed without applying")
	cmd.Flags().Bool("confirm", false, "Prompt for confirmation before applying changes")
	cmd.Flags().Bool("to-config", false, "Promote approved live cluster state back into config")
	cmd.Flags().Bool("to-cluster", false, "Reconcile infrastructure to match the current config (default)")
	cmd.MarkFlagsMutuallyExclusive("to-config", "to-cluster")

	return cmd
}

func reconcileDriftToConfig(cmd *cobra.Command, clusterIdentifier string, dryRun bool) error {
	cfg, _, _, clusterPaths, err := loadNativeV2ConfigWithIdentifier(cmd.Context(), clusterIdentifier)
	if err != nil {
		return fmt.Errorf("failed to load cluster configuration: %w", err)
	}

	liveResult, err := importer.BuildLiveImportResult(cmd.Context(), cfg, clusterPaths.KubeconfigPath)
	if err != nil {
		return fmt.Errorf("failed to inspect live cluster state: %w", err)
	}

	plan, err := importer.PrepareClusterWritePlan(cmd.Context(), liveResult)
	if err != nil {
		return fmt.Errorf("failed to prepare config promotion plan: %w", err)
	}

	if strings.TrimSpace(plan.Diff) == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "No promotable live-to-config drift detected")
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Live-to-config patch for cluster %s:\n\n", plan.ClusterName)
	fmt.Fprintln(cmd.OutOrStdout(), plan.Diff)

	if dryRun {
		return nil
	}

	testMode := os.Getenv("OPENCENTER_TEST_MODE") == "1"
	prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)
	confirmed, err := prompter.Confirm(cmd.Context(), fmt.Sprintf("Apply live-to-config patch for %s?", plan.ClusterName))
	if err != nil {
		return fmt.Errorf("confirmation prompt failed: %w", err)
	}
	if !confirmed {
		return fmt.Errorf("live-to-config reconciliation cancelled")
	}

	if err := importer.ApplyClusterWritePlan(plan); err != nil {
		return fmt.Errorf("failed to apply config patch: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Updated config for %s at %s\n", plan.ClusterName, plan.ConfigPath)
	return nil
}

type configPromotionPreview struct {
	ClusterName string                          `json:"cluster_name" yaml:"cluster_name"`
	FieldCount  int                             `json:"field_count" yaml:"field_count"`
	Fields      []importer.FieldInferenceResult `json:"fields,omitempty" yaml:"fields,omitempty"`
	Skipped     []importer.SkippedField         `json:"skipped,omitempty" yaml:"skipped,omitempty"`
	Diff        string                          `json:"diff,omitempty" yaml:"diff,omitempty"`
}

func buildConfigPromotionPreview(ctx context.Context, clusterIdentifier string) (*configPromotionPreview, error) {
	cfg, _, _, clusterPaths, err := loadNativeV2ConfigWithIdentifier(ctx, clusterIdentifier)
	if err != nil {
		return nil, err
	}

	liveResult, err := importer.BuildLiveImportResult(ctx, cfg, clusterPaths.KubeconfigPath)
	if err != nil {
		return nil, err
	}

	approved, skipped := importer.SelectApprovedFields(liveResult)
	plan, err := importer.PrepareClusterWritePlan(ctx, liveResult)
	if err != nil {
		return nil, err
	}

	if len(approved) == 0 && strings.TrimSpace(plan.Diff) == "" {
		return nil, nil
	}

	return &configPromotionPreview{
		ClusterName: liveResult.ClusterName,
		FieldCount:  len(approved),
		Fields:      approved,
		Skipped:     skipped,
		Diff:        plan.Diff,
	}, nil
}

// newClusterDriftScheduleCmd creates the drift schedule subcommand
func newClusterDriftScheduleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "schedule [cluster]",
		Short: "Schedule periodic drift detection",
		Long: `Schedule periodic drift detection for a cluster.

This command sets up a background process that periodically checks for drift and
reports results. Drift reports can be sent to a callback URL or logged locally.

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # Schedule drift detection for active cluster every 24 hours
  opencenter cluster drift schedule --interval=24h

  # Schedule for specific cluster every 24 hours
  opencenter cluster drift schedule my-cluster --interval=24h

  # Schedule with custom callback
  opencenter cluster drift schedule my-cluster --interval=12h --callback=https://example.com/drift`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			clusterName, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			intervalStr, _ := cmd.Flags().GetString("interval")
			callbackURL, _ := cmd.Flags().GetString("callback")

			// Parse interval
			interval, err := time.ParseDuration(intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			// Load configuration
			cfg, err := loadCanonicalConfig(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Create cloud provider factory
			factory := createCloudProviderFactory(cfg)

			// Get provider for this cluster
			provider, err := factory.GetProvider(canonicalDriftProvider(cfg.OpenCenter.Infrastructure.Provider))
			if err != nil {
				return fmt.Errorf("failed to get cloud provider: %w", err)
			}

			// Schedule periodic drift detection
			fmt.Fprintf(cmd.OutOrStdout(), "Scheduling drift detection for cluster %s every %s\n", clusterName, interval)
			if callbackURL != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Drift reports will be sent to: %s\n", callbackURL)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Drift detection scheduled. Press Ctrl+C to stop.")

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-cmd.Context().Done():
					return nil
				case <-ticker.C:
					currentState, err := provider.GetCurrentState(cmd.Context(), cfg)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Error getting current state: %v\n", err)
						continue
					}

					desiredState := buildDesiredState(cfg)
					report, err := provider.DetectDrift(cmd.Context(), desiredState, currentState)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "Error detecting drift: %v\n", err)
						continue
					}

					report.ClusterName = clusterName
					report.DetectedAt = time.Now().Format(time.RFC3339)

					if callbackURL != "" {
						if err := sendDriftCallback(cmd.Context(), callbackURL, report); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "Error sending drift callback: %v\n", err)
							continue
						}
						fmt.Fprintf(cmd.OutOrStdout(), "Drift detected: %d items (sent to %s)\n", len(report.Drifts), callbackURL)
						continue
					}

					fmt.Fprintf(cmd.OutOrStdout(), "Drift detected: %d items\n", len(report.Drifts))
				}
			}
		},
	}

	cmd.Flags().String("interval", "24h", "Interval between drift checks (e.g., 1h, 24h, 7d)")
	cmd.Flags().String("callback", "", "Callback URL for drift reports")

	return cmd
}

// Helper functions

// createCloudProviderFactory creates and configures a cloud provider factory with all available providers.
func createCloudProviderFactory(cfg v2.Config) *cloud.CloudProviderFactory {
	factory := cloud.NewCloudProviderFactory()

	openstackConfig := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstackConfig != nil {
		openstackProvider := openstack.NewProvider(gophercloud.AuthOptions{
			IdentityEndpoint:            openstackConfig.AuthURL,
			ApplicationCredentialID:     openstackConfig.ApplicationCredentialID,
			ApplicationCredentialSecret: openstackConfig.ApplicationCredentialSecret,
			DomainName:                  openstackConfig.Domain,
			TenantName:                  openstackConfig.TenantName,
		}, openstackConfig.Region)
		factory.RegisterProvider("openstack", openstackProvider)
	}

	factory.RegisterProvider("vmware", vmware.NewProvider())

	return factory
}

// buildDesiredState constructs the desired infrastructure state from configuration.
func buildDesiredState(cfg v2.Config) *cloud.InfrastructureState {
	state := &cloud.InfrastructureState{
		Servers:        []cloud.Server{},
		Networks:       []cloud.Network{},
		SecurityGroups: []cloud.SecurityGroup{},
		LoadBalancers:  []cloud.LoadBalancer{},
		Volumes:        []cloud.Volume{},
		FloatingIPs:    []cloud.FloatingIP{},
	}

	provider := canonicalDriftProvider(cfg.Provider())
	clusterName := cfg.ClusterName()
	masterPrefix := "master"
	workerPrefix := "worker"
	serverImage := ""
	if provider == "openstack" && cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		serverImage = cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID
	}

	if provider == "vmware" {
		networkName := ""
		datastoreName := ""
		if vmwareCfg := cfg.OpenCenter.Infrastructure.Cloud.VMware; vmwareCfg != nil {
			networkName = strings.TrimSpace(vmwareCfg.Network)
			datastoreName = strings.TrimSpace(vmwareCfg.Datastore)
		}

		for i := 0; i < cfg.OpenCenter.Infrastructure.Compute.MasterCount; i++ {
			nodeName := fmt.Sprintf("%s-%s-%d", clusterName, masterPrefix, i+1)
			server := cloud.Server{
				Name:   nodeName,
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": clusterName,
					"role":    "control-plane",
				},
			}
			if networkName != "" {
				server.Networks = []string{networkName}
			}
			state.Servers = append(state.Servers, server)

			if datastoreName != "" {
				state.Volumes = append(state.Volumes, cloud.Volume{
					Name:   fmt.Sprintf("%s@%s", nodeName, datastoreName),
					Status: "in-use",
				})
			}
		}

		for i := 0; i < cfg.OpenCenter.Infrastructure.Compute.WorkerCount; i++ {
			nodeName := fmt.Sprintf("%s-%s-%d", clusterName, workerPrefix, i+1)
			role := "worker"
			server := cloud.Server{
				Name:   nodeName,
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": clusterName,
					"role":    role,
				},
			}
			if networkName != "" {
				server.Networks = []string{networkName}
			}
			state.Servers = append(state.Servers, server)

			if datastoreName != "" {
				state.Volumes = append(state.Volumes, cloud.Volume{
					Name:   fmt.Sprintf("%s@%s", nodeName, datastoreName),
					Status: "in-use",
				})
			}
		}
	} else {
		for i := 0; i < cfg.OpenCenter.Infrastructure.Compute.MasterCount; i++ {
			name := fmt.Sprintf("%s-%s-%d", clusterName, masterPrefix, i+1)
			state.Servers = append(state.Servers, cloud.Server{
				Name:   name,
				Flavor: cfg.OpenCenter.Infrastructure.Compute.FlavorMaster,
				Image:  serverImage,
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": clusterName,
					"role":    "control-plane",
				},
			})

			if size := cfg.OpenCenter.Infrastructure.Storage.MasterVolumeSize; size > 0 {
				state.Volumes = append(state.Volumes, cloud.Volume{
					Name:       fmt.Sprintf("%s-boot", name),
					Size:       size,
					Status:     "in-use",
					AttachedTo: name,
				})
			}
		}

		for i := 0; i < cfg.OpenCenter.Infrastructure.Compute.WorkerCount; i++ {
			name := fmt.Sprintf("%s-%s-%d", clusterName, workerPrefix, i+1)
			state.Servers = append(state.Servers, cloud.Server{
				Name:   name,
				Flavor: cfg.OpenCenter.Infrastructure.Compute.FlavorWorker,
				Image:  serverImage,
				Status: "ACTIVE",
				Tags: map[string]string{
					"cluster": clusterName,
					"role":    "worker",
				},
			})

			if size := cfg.OpenCenter.Infrastructure.Storage.WorkerVolumeSize; size > 0 {
				state.Volumes = append(state.Volumes, cloud.Volume{
					Name:       fmt.Sprintf("%s-boot", name),
					Size:       size,
					Status:     "in-use",
					AttachedTo: name,
				})
			}
		}
	}

	switch provider {
	case "openstack":
		networkName := fmt.Sprintf("%s-network", clusterName)
		networkID := ""
		subnetID := ""
		floatingIPPool := ""
		apiACL := []string(nil)
		useOctavia := cfg.OpenCenter.Infrastructure.Networking.UseOctavia
		loadBalancerProvider := cfg.OpenCenter.Infrastructure.Networking.LoadbalancerProvider
		subnetNodes := cfg.OpenCenter.Infrastructure.Networking.SubnetNodes
		if openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack; openstackCfg != nil {
			networkID = strings.TrimSpace(openstackCfg.NetworkID)
			subnetID = strings.TrimSpace(openstackCfg.SubnetID)
			floatingIPPool = strings.TrimSpace(openstackCfg.FloatingIPPool)
			useOctavia = useOctavia || openstackCfg.UseOctavia
			if openstackCfg.Networking != nil {
				if networkID == "" {
					networkID = strings.TrimSpace(openstackCfg.Networking.NetworkID)
				}
				if subnetID == "" {
					subnetID = strings.TrimSpace(openstackCfg.Networking.SubnetID)
				}
				if floatingIPPool == "" {
					floatingIPPool = strings.TrimSpace(openstackCfg.Networking.FloatingIPPool)
				}
				apiACL = append(apiACL, openstackCfg.Networking.K8sAPIPortACL...)
			}
		}
		if networkID != "" {
			networkName = networkID
		}

		state.Networks = append(state.Networks, cloud.Network{
			ID:   networkID,
			Name: networkName,
			Subnets: []cloud.Subnet{
				{
					ID:   subnetID,
					Name: fmt.Sprintf("%s-subnet-nodes", clusterName),
					CIDR: subnetNodes,
				},
			},
		})

		apiRules := make([]cloud.SecurityRule, 0, len(apiACL))
		for _, cidr := range apiACL {
			apiRules = append(apiRules, cloud.SecurityRule{
				Direction: "ingress",
				Protocol:  "tcp",
				PortRange: "6443",
				RemoteIP:  cidr,
			})
		}

		state.SecurityGroups = append(state.SecurityGroups,
			cloud.SecurityGroup{
				Name:  fmt.Sprintf("%s-control-plane-sg", clusterName),
				Rules: apiRules,
			},
			cloud.SecurityGroup{
				Name: fmt.Sprintf("%s-worker-sg", clusterName),
			},
		)

		if loadBalancerProvider == "ovn" || loadBalancerProvider == "octavia" || useOctavia {
			state.LoadBalancers = append(state.LoadBalancers, cloud.LoadBalancer{
				Name:     fmt.Sprintf("%s-api", clusterName),
				Protocol: "TCP",
				Port:     6443,
			})
		}

		if !useOctavia && loadBalancerProvider != "octavia" && floatingIPPool != "" {
			state.FloatingIPs = append(state.FloatingIPs, cloud.FloatingIP{
				ID:     "cluster-api",
				Status: "ACTIVE",
			})
		}
	case "vmware":
		if cfg.OpenCenter.Infrastructure.Cloud.VMware != nil && strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.VMware.Network) != "" {
			networkName := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.VMware.Network)
			state.Networks = append(state.Networks, cloud.Network{
				Name: networkName,
			})
		}
	default:
		state.Networks = append(state.Networks, cloud.Network{
			Name: fmt.Sprintf("%s-network", clusterName),
			Subnets: []cloud.Subnet{
				{
					Name: fmt.Sprintf("%s-subnet-nodes", clusterName),
					CIDR: cfg.OpenCenter.Infrastructure.Networking.SubnetNodes,
				},
			},
		})
	}

	return state
}

func canonicalDriftProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "vsphere":
		return "vmware"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

// outputDriftReport outputs a drift report in the specified format
func outputDriftReport(cmd *cobra.Command, report *cloud.DriftReport, format string, promotion *configPromotionPreview) error {
	switch format {
	case "json":
		payload := struct {
			*cloud.DriftReport
			ConfigPromotion *configPromotionPreview `json:"config_promotion,omitempty"`
		}{
			DriftReport:     report,
			ConfigPromotion: promotion,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report to JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))

	case "yaml":
		payload := struct {
			*cloud.DriftReport
			ConfigPromotion *configPromotionPreview `yaml:"config_promotion,omitempty"`
		}{
			DriftReport:     report,
			ConfigPromotion: promotion,
		}
		data, err := yaml.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal report to YAML: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(data))

	case "text":
		fallthrough
	default:
		// Human-readable text format
		fmt.Fprintf(cmd.OutOrStdout(), "Drift Report for Cluster: %s\n", report.ClusterName)
		fmt.Fprintf(cmd.OutOrStdout(), "Detected At: %s\n", report.DetectedAt)
		fmt.Fprintf(cmd.OutOrStdout(), "Overall Severity: %s\n", report.OverallSeverity)
		fmt.Fprintf(cmd.OutOrStdout(), "Reconcilable: %v\n\n", report.Reconcilable)

		fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
		fmt.Fprintf(cmd.OutOrStdout(), "  Total Drifts: %d\n", report.Summary.TotalDrifts)
		fmt.Fprintf(cmd.OutOrStdout(), "  Critical: %d\n", report.Summary.CriticalCount)
		fmt.Fprintf(cmd.OutOrStdout(), "  Warning: %d\n", report.Summary.WarningCount)
		fmt.Fprintf(cmd.OutOrStdout(), "  Info: %d\n", report.Summary.InfoCount)
		fmt.Fprintf(cmd.OutOrStdout(), "  Reconcilable: %d\n\n", report.Summary.ReconcilableCount)

		if len(report.Drifts) > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Drifts:\n")
			for i, drift := range report.Drifts {
				fmt.Fprintf(cmd.OutOrStdout(), "  %d. %s %s (%s)\n",
					i+1, drift.ResourceType, drift.ResourceName, drift.ResourceID)
				fmt.Fprintf(cmd.OutOrStdout(), "     Field: %s\n", drift.Field)
				fmt.Fprintf(cmd.OutOrStdout(), "     Expected: %v\n", drift.Expected)
				fmt.Fprintf(cmd.OutOrStdout(), "     Actual: %v\n", drift.Actual)
				fmt.Fprintf(cmd.OutOrStdout(), "     Severity: %s\n", drift.Severity)
				fmt.Fprintf(cmd.OutOrStdout(), "     Reconcilable: %v\n", drift.Reconcilable)
				if drift.Message != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "     Message: %s\n", drift.Message)
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}
		}

		if promotion != nil && promotion.FieldCount > 0 {
			fmt.Fprintf(cmd.OutOrStdout(), "Config Promotion Candidates:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Fields: %d\n", promotion.FieldCount)
			if strings.TrimSpace(promotion.Diff) != "" {
				fmt.Fprintln(cmd.OutOrStdout())
				fmt.Fprintln(cmd.OutOrStdout(), promotion.Diff)
			}
		}
	}

	return nil
}

// filterBySeverity filters a drift report by severity level
func filterBySeverity(report *cloud.DriftReport, severityStr string) *cloud.DriftReport {
	var targetSeverity cloud.Severity
	switch severityStr {
	case "critical":
		targetSeverity = cloud.SeverityCritical
	case "warning":
		targetSeverity = cloud.SeverityWarning
	case "info":
		targetSeverity = cloud.SeverityInfo
	default:
		return report // Invalid severity, return unfiltered
	}

	filtered := &cloud.DriftReport{
		ClusterName: report.ClusterName,
		DetectedAt:  report.DetectedAt,
		Drifts:      []cloud.DriftItem{},
	}

	for _, drift := range report.Drifts {
		if drift.Severity == targetSeverity {
			filtered.Drifts = append(filtered.Drifts, drift)
		}
	}

	// Recalculate summary manually
	filtered.Summary.TotalDrifts = len(filtered.Drifts)
	filtered.Reconcilable = true
	filtered.OverallSeverity = cloud.SeverityInfo

	for _, drift := range filtered.Drifts {
		switch drift.Severity {
		case cloud.SeverityCritical:
			filtered.Summary.CriticalCount++
			filtered.OverallSeverity = cloud.SeverityCritical
		case cloud.SeverityWarning:
			filtered.Summary.WarningCount++
			if filtered.OverallSeverity < cloud.SeverityWarning {
				filtered.OverallSeverity = cloud.SeverityWarning
			}
		case cloud.SeverityInfo:
			filtered.Summary.InfoCount++
		}

		if drift.Reconcilable {
			filtered.Summary.ReconcilableCount++
		} else {
			filtered.Reconcilable = false
		}
	}

	return filtered
}

func sendDriftCallback(ctx context.Context, callbackURL string, report *cloud.DriftReport) error {
	return sendDriftCallbackWithDoer(ctx, callbackURL, report, http.DefaultClient)
}

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

func sendDriftCallbackWithDoer(ctx context.Context, callbackURL string, report *cloud.DriftReport, doer httpDoer) error {
	body, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal drift report: %w", err)
	}

	requestCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, callbackURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build callback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := doer.Do(req)
	if err != nil {
		return fmt.Errorf("callback POST failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("callback returned HTTP %d", resp.StatusCode)
	}

	return nil
}
