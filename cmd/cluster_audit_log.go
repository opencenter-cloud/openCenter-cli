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
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/spf13/cobra"
)

// newClusterAuditLogCmd creates the command for viewing audit logs.
func newClusterAuditLogCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "audit-log [cluster]",
		Short: "View audit log for key operations",
		Long: `View the audit log for key access and modification events.

This command displays audit events for secrets operations including:
  • Key access and decryption
  • Key generation and rotation
  • Key revocation
  • Secrets synchronization
  • Drift detection

The audit log provides a tamper-evident record of all key operations
for security compliance and incident investigation.

Event types:
  • secrets.sync - Secrets synchronized from config
  • secrets.drift_detected - Drift detected between config and manifests
  • secrets.validated - Secrets validation completed
  • key.generated - New key generated
  • key.rotated - Key rotation completed
  • key.revoked - Key revoked
  • key.accessed - Key accessed for encryption/decryption

If no cluster name is provided, uses the currently active cluster.`,
		Example: `  # View recent audit events for active cluster
  opencenter cluster audit-log

  # View audit events for specific cluster
  opencenter cluster audit-log my-cluster

  # View events from the last 7 days
  opencenter cluster audit-log my-cluster --since 7d

  # Filter by event type
  opencenter cluster audit-log my-cluster --event-type key.rotated

  # Export audit log to JSON file
  opencenter cluster audit-log my-cluster --export audit-report.json

  # Verify audit log integrity
  opencenter cluster audit-log my-cluster --verify`,
		Args: cobra.MaximumNArgs(1),
		RunE: runClusterAuditLog,
	}

	cmd.Flags().String("since", "30d", "Show events since duration (e.g., 7d, 24h, 1w)")
	cmd.Flags().String("event-type", "", "Filter by event type (secrets.sync, key.rotated, key.revoked, etc.)")
	cmd.Flags().String("export", "", "Export audit log to JSON file")
	cmd.Flags().Bool("verify", false, "Verify audit log integrity")

	return cmd
}

func runClusterAuditLog(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Get flags
	sinceStr, _ := cmd.Flags().GetString("since")
	eventType, _ := cmd.Flags().GetString("event-type")
	exportPath, _ := cmd.Flags().GetString("export")
	verify, _ := cmd.Flags().GetBool("verify")

	// Resolve cluster name
	clusterName, err := resolveClusterNameForCommand(cmd, args, true)
	if err != nil {
		return err
	}

	// Parse since duration
	since, err := parseDuration(sinceStr)
	if err != nil {
		return fmt.Errorf("invalid --since duration: %w", err)
	}

	auditLogPath := security.GetDefaultAuditLogPath()

	// Check if audit log exists
	if _, err := os.Stat(auditLogPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.OutOrStdout(), "No audit log found for cluster %s\n", clusterName)
		fmt.Fprintln(cmd.OutOrStdout(), "\nAudit logging is enabled when secrets and key workflows run through the CLI.")
		fmt.Fprintln(cmd.OutOrStdout(), "Audit events are stored at:", auditLogPath)
		return nil
	}

	logger, err := security.NewDefaultAuditLogger()
	if err != nil {
		return fmt.Errorf("failed to initialize audit logger: %w", err)
	}
	defer logger.Close()

	// Verify integrity if requested
	if verify {
		if err := logger.VerifyIntegrity(); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Audit log integrity verification failed: %v\n", err)
			return err
		}
		fmt.Fprintln(cmd.OutOrStdout(), "✓ Audit log integrity verified")
		if exportPath == "" && eventType == "" {
			// If only verifying, exit here
			return nil
		}
	}

	// Read and filter audit events
	events, err := readAuditLog(ctx, logger, clusterName, since, eventType)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	// Export if requested
	if exportPath != "" {
		if err := logger.ExportEventsToJSON(events, exportPath); err != nil {
			return fmt.Errorf("failed to export audit log: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Audit log exported to: %s\n", exportPath)
		return nil
	}

	// Display events
	displayAuditEvents(cmd, clusterName, events, since)

	return nil
}

// parseDuration parses a duration string like "7d", "24h", "1w"
func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 30 * 24 * time.Hour, nil // Default 30 days
	}
	return security.ParseDuration(s)
}

// readAuditLog reads and filters audit events from the log file
func readAuditLog(ctx context.Context, logger *security.AuditLogger, cluster string, since time.Duration, eventType string) ([]security.AuditEvent, error) {
	return logger.QueryEvents(ctx, security.EventFilter{
		StartTime: time.Now().Add(-since),
		EventType: eventType,
		Resource:  cluster,
	})
}

// displayAuditEvents formats and displays audit events
func displayAuditEvents(cmd *cobra.Command, cluster string, events []security.AuditEvent, since time.Duration) {
	if len(events) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No audit events found for cluster %s in the last %s\n", cluster, since)
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Audit Log for cluster %s (last %s)\n\n", cluster, since)

	// Group events by type
	eventsByType := make(map[string][]security.AuditEvent)
	for _, event := range events {
		eventsByType[event.EventType] = append(eventsByType[event.EventType], event)
	}

	// Display events by type
	for eventType, typeEvents := range eventsByType {
		fmt.Fprintf(cmd.OutOrStdout(), "%s (%d events):\n", eventType, len(typeEvents))
		for _, event := range typeEvents {
			fmt.Fprintf(cmd.OutOrStdout(), "  [%s] %s\n", event.Timestamp.Format("2006-01-02 15:04:05"), event.Actor)
			if event.Resource != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Resource: %s\n", event.Resource)
			}
			if fingerprint, ok := event.Details["key_fingerprint"]; ok {
				fmt.Fprintf(cmd.OutOrStdout(), "    Fingerprint: %v\n", fingerprint)
			}
			if len(event.Details) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "    Details: %v\n", event.Details)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout())
	}

	// Display summary
	fmt.Fprintf(cmd.OutOrStdout(), "Summary:\n")
	fmt.Fprintf(cmd.OutOrStdout(), "  Total events: %d\n", len(events))
	fmt.Fprintf(cmd.OutOrStdout(), "  Event types: %d\n", len(eventsByType))
	fmt.Fprintf(cmd.OutOrStdout(), "  Time range: %s\n", since)
}
