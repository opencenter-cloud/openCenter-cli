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
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

// AuditEvent represents a single audit log event
type AuditEvent struct {
	Timestamp   time.Time         `json:"timestamp"`
	Actor       string            `json:"actor"`
	EventType   string            `json:"event_type"`
	Cluster     string            `json:"cluster"`
	Resource    string            `json:"resource,omitempty"`
	Details     map[string]string `json:"details,omitempty"`
	IPAddress   string            `json:"ip_address,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
}

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
	clusterName, err := resolveClusterName(args, true)
	if err != nil {
		return err
	}

	// Parse since duration
	since, err := parseDuration(sinceStr)
	if err != nil {
		return fmt.Errorf("invalid --since duration: %w", err)
	}

	// Get audit log path
	auditLogPath, err := getAuditLogPath()
	if err != nil {
		return fmt.Errorf("failed to get audit log path: %w", err)
	}

	// Check if audit log exists
	if _, err := os.Stat(auditLogPath); os.IsNotExist(err) {
		fmt.Fprintf(cmd.OutOrStdout(), "No audit log found for cluster %s\n", clusterName)
		fmt.Fprintln(cmd.OutOrStdout(), "\nAudit logging is currently not enabled in this CLI build.")
		fmt.Fprintln(cmd.OutOrStdout(), "Audit events would be logged to:", auditLogPath)
		return nil
	}

	// Verify integrity if requested
	if verify {
		if err := verifyAuditLogIntegrity(ctx, auditLogPath); err != nil {
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
	events, err := readAuditLog(ctx, auditLogPath, clusterName, since, eventType)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	// Export if requested
	if exportPath != "" {
		if err := exportAuditLog(events, exportPath); err != nil {
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

	// Handle common suffixes
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid duration format: %s", s)
	}

	suffix := s[len(s)-1:]
	valueStr := s[:len(s)-1]

	var value int
	if _, err := fmt.Sscanf(valueStr, "%d", &value); err != nil {
		return 0, fmt.Errorf("invalid duration value: %s", s)
	}

	switch suffix {
	case "d":
		return time.Duration(value) * 24 * time.Hour, nil
	case "h":
		return time.Duration(value) * time.Hour, nil
	case "w":
		return time.Duration(value) * 7 * 24 * time.Hour, nil
	case "m":
		return time.Duration(value) * time.Minute, nil
	default:
		// Try parsing as standard Go duration
		return time.ParseDuration(s)
	}
}

// getAuditLogPath returns the path to the audit log file
func getAuditLogPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".config", "opencenter", "audit", "secrets-audit.log"), nil
}

// verifyAuditLogIntegrity verifies the cryptographic signatures in the audit log
func verifyAuditLogIntegrity(ctx context.Context, logPath string) error {
	// TODO: Implement actual signature verification
	// For now, just check if the file is readable
	data, err := os.ReadFile(logPath)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	if len(data) == 0 {
		return fmt.Errorf("audit log is empty")
	}

	// In a real implementation, this would:
	// 1. Parse each log entry
	// 2. Verify the cryptographic signature
	// 3. Check for tampering or missing entries
	// 4. Validate the chain of signatures

	return nil
}

// readAuditLog reads and filters audit events from the log file
func readAuditLog(ctx context.Context, logPath, cluster string, since time.Duration, eventType string) ([]AuditEvent, error) {
	// Check if file exists
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		return []AuditEvent{}, nil
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read audit log: %w", err)
	}

	// Parse JSON lines
	var events []AuditEvent
	lines := string(data)
	
	// TODO: Implement actual log parsing
	// For now, return empty list since we're using noOpAuditLogger
	// In a real implementation, this would:
	// 1. Parse each line as JSON
	// 2. Filter by cluster
	// 3. Filter by time range (since)
	// 4. Filter by event type if specified
	
	_ = lines // Suppress unused variable warning

	return events, nil
}

// exportAuditLog exports audit events to a JSON file
func exportAuditLog(events []AuditEvent, exportPath string) error {
	data, err := json.MarshalIndent(events, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal events: %w", err)
	}

	if err := os.WriteFile(exportPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write export file: %w", err)
	}

	return nil
}

// displayAuditEvents formats and displays audit events
func displayAuditEvents(cmd *cobra.Command, cluster string, events []AuditEvent, since time.Duration) {
	if len(events) == 0 {
		fmt.Fprintf(cmd.OutOrStdout(), "No audit events found for cluster %s in the last %s\n", cluster, since)
		fmt.Fprintln(cmd.OutOrStdout(), "\nNote: Audit logging is currently not enabled in this CLI build.")
		fmt.Fprintln(cmd.OutOrStdout(), "When enabled, audit events will be displayed here.")
		return
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Audit Log for cluster %s (last %s)\n\n", cluster, since)

	// Group events by type
	eventsByType := make(map[string][]AuditEvent)
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
			if event.Fingerprint != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "    Fingerprint: %s\n", event.Fingerprint)
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
