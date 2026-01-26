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
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/operations"
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
			_, err = config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			// Drift detection requires cloud provider implementation
			return fmt.Errorf("drift detection requires cloud provider implementation (not yet available)")

			// TODO: Implement cloud provider factory and drift detection
			// Once cloud provider is implemented:
			// 1. Create configuration manager
			// 2. Create cloud provider based on config
			// 3. Create drift detector
			// 4. Detect drift and output report
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

			// Load configuration
			_, err = config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			_ = dryRun // Use dryRun to avoid unused variable error

			// Drift reconciliation requires cloud provider implementation
			return fmt.Errorf("drift reconciliation requires cloud provider implementation (not yet available)")

			// TODO: Implement cloud provider factory and drift reconciliation
			// Once cloud provider is implemented:
			// 1. Create configuration manager
			// 2. Create cloud provider based on config
			// 3. Create drift detector
			// 4. Reconcile drift with dry-run support
		},
	}

	cmd.Flags().Bool("dry-run", false, "Show what would be changed without applying")
	cmd.Flags().Bool("confirm", false, "Prompt for confirmation before applying changes")

	return cmd
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

			// Parse interval
			interval, err := time.ParseDuration(intervalStr)
			if err != nil {
				return fmt.Errorf("invalid interval: %w", err)
			}

			// Load configuration
			_, err = config.Load(clusterName)
			if err != nil {
				return fmt.Errorf("failed to load cluster configuration: %w", err)
			}

			_ = interval // Use interval to avoid unused variable error

			// Drift scheduling requires cloud provider implementation
			return fmt.Errorf("drift scheduling requires cloud provider implementation (not yet available)")

			// TODO: Implement cloud provider factory and scheduling
			// Once cloud provider is implemented:
			// 1. Create configuration manager
			// 2. Create cloud provider based on config
			// 3. Create drift detector
			// 4. Schedule periodic drift checks with callback
		},
	}

	cmd.Flags().String("interval", "24h", "Interval between drift checks (e.g., 1h, 24h, 7d)")
	cmd.Flags().String("callback", "", "Callback URL for drift reports")

	return cmd
}

// Helper functions

// outputDriftReport outputs a drift report in the specified format
func outputDriftReport(cmd *cobra.Command, report *operations.DriftReport, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal report to JSON: %w", err)
		}
		fmt.Fprintln(cmd.OutOrStdout(), string(data))

	case "yaml":
		data, err := yaml.Marshal(report)
		if err != nil {
			return fmt.Errorf("failed to marshal report to YAML: %w", err)
		}
		fmt.Fprint(cmd.OutOrStdout(), string(data))

	case "text":
		fallthrough
	default:
		// Human-readable text format
		fmt.Fprintf(cmd.OutOrStdout(), "Drift Report for Cluster: %s\n", report.Cluster)
		fmt.Fprintf(cmd.OutOrStdout(), "Detected At: %s\n", report.DetectedAt.Format(time.RFC3339))
		fmt.Fprintf(cmd.OutOrStdout(), "Overall Severity: %s\n", report.Severity)
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
				fmt.Fprintf(cmd.OutOrStdout(), "     Reconcilable: %v\n\n", drift.Reconcilable)
			}
		}
	}

	return nil
}

// filterBySeverity filters a drift report by severity level
func filterBySeverity(report *operations.DriftReport, severityStr string) *operations.DriftReport {
	var targetSeverity operations.Severity
	switch severityStr {
	case "critical":
		targetSeverity = operations.SeverityCritical
	case "warning":
		targetSeverity = operations.SeverityWarning
	case "info":
		targetSeverity = operations.SeverityInfo
	default:
		return report // Invalid severity, return unfiltered
	}

	filtered := &operations.DriftReport{
		ID:         report.ID,
		Cluster:    report.Cluster,
		DetectedAt: report.DetectedAt,
		Drifts:     []operations.ResourceDrift{},
	}

	for _, drift := range report.Drifts {
		if drift.Severity == targetSeverity {
			filtered.Drifts = append(filtered.Drifts, drift)
		}
	}

	// Recalculate summary manually
	filtered.Summary.TotalDrifts = len(filtered.Drifts)
	filtered.Reconcilable = true
	filtered.Severity = operations.SeverityInfo

	for _, drift := range filtered.Drifts {
		switch drift.Severity {
		case operations.SeverityCritical:
			filtered.Summary.CriticalCount++
			filtered.Severity = operations.SeverityCritical
		case operations.SeverityWarning:
			filtered.Summary.WarningCount++
			if filtered.Severity < operations.SeverityWarning {
				filtered.Severity = operations.SeverityWarning
			}
		case operations.SeverityInfo:
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
