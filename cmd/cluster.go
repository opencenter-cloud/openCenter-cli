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
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/spf13/cobra"
)

// NewClusterCmd creates the top-level "cluster" command. It has
// several subcommands defined in separate files. Running "opencenter
// cluster" without subcommand prints help.
func NewClusterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cluster",
		Short: "Manage cluster configurations",
		Long: `Manage Kubernetes cluster configurations throughout their lifecycle.

The cluster command provides subcommands for initializing, validating, updating,
and managing cluster configurations. It supports organization-based multi-tenancy
and integrates with GitOps workflows.

Common Workflow:
  1. Initialize a new cluster configuration
  2. Validate the configuration
  3. Run preflight checks
  4. Set up infrastructure and GitOps repository
  5. Bootstrap the cluster with Flux

Configuration files are stored in organization-based directories:
  ~/.config/opencenter/clusters/<organization>/<cluster>/`,
		Example: `  # Initialize a new cluster
  opencenter cluster init my-cluster

  # Initialize with organization
  opencenter cluster init my-cluster --opencenter.meta.organization=myorg

  # Validate configuration
  opencenter cluster validate my-cluster

  # List all clusters
  opencenter cluster list

  # Select active cluster (session-scoped)
  opencenter cluster select my-cluster

  # Select cluster persistently (all terminals)
  opencenter cluster select my-cluster --persistent

  # Export cluster environment
  eval "$(opencenter cluster env)"

  # Select and activate cluster environment
  eval "$(opencenter cluster select my-cluster --activate --export-only)"

  # Deactivate cluster environment
  eval "$(opencenter cluster select --clear --export-only)"

  # Clear persistent cluster selection
  opencenter cluster select --clear-persistent

  # Show current cluster
  opencenter cluster current`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add subcommands
	cmd.AddCommand(newClusterListCmd())
	cmd.AddCommand(newClusterSelectCmd())
	cmd.AddCommand(newClusterCurrentCmd())
	cmd.AddCommand(newClusterEnvCmd())
	cmd.AddCommand(newClusterStatusCmd())
	cmd.AddCommand(newClusterInfoCmd())
	cmd.AddCommand(newClusterInitCmd())
	cmd.AddCommand(newClusterEditCmd())
	cmd.AddCommand(newClusterValidateCmd())
	cmd.AddCommand(newClusterPreflightCmd())

	cmd.AddCommand(newClusterRenderCmd())
	cmd.AddCommand(newClusterBootstrapCmd())
	cmd.AddCommand(newClusterSchemaCmd())
	cmd.AddCommand(newClusterTemplateCmd())
	cmd.AddCommand(newClusterDestroyCmd())
	cmd.AddCommand(newClusterUpdateCmd())
	cmd.AddCommand(newClusterServiceCmd())
	cmd.AddCommand(newClusterCredentialsCmd())
	cmd.AddCommand(newClusterDriftCmd())
	cmd.AddCommand(newClusterBackupCmd())
	cmd.AddCommand(newClusterLockCmd())
	cmd.AddCommand(newClusterUnlockCmd())
	cmd.AddCommand(newClusterConfigCmd())
	cmd.AddCommand(newClusterValidateManifestsCmd())
	cmd.AddCommand(newClusterSyncSecretsCmd())
	cmd.AddCommand(newClusterValidateSecretsCmd())
	cmd.AddCommand(newClusterRotateKeysCmd())
	cmd.AddCommand(newClusterCheckKeysCmd())
	cmd.AddCommand(newClusterAuditLogCmd())
	cmd.AddCommand(newClusterRevokeKeyCmd())
	cmd.AddCommand(newClusterInstallHooksCmd())
	cmd.AddCommand(newClusterKeysCmd())
	return cmd
}

// resolveClusterName resolves the cluster name from command arguments or active cluster.
// It supports both "cluster" and "organization/cluster" formats.
//
// Parameters:
//   - args: Command arguments (first arg should be cluster name if provided)
//   - requireActive: If true and no args provided, returns error if no active cluster
//
// Returns:
//   - clusterName: The resolved cluster name (may include organization prefix)
//   - error: An error if resolution fails
func resolveClusterName(args []string, requireActive bool) (string, error) {
	ctx := context.Background()
	validator := validators.NewClusterNameValidator()

	// If cluster name provided as argument
	if len(args) > 0 {
		clusterName := strings.TrimSpace(args[0])
		if clusterName == "" {
			return "", fmt.Errorf("cluster name cannot be empty")
		}

		// Validate the cluster identifier (handles both "cluster" and "org/cluster" formats)
		parts := strings.Split(clusterName, "/")
		if len(parts) > 2 {
			return "", fmt.Errorf("invalid cluster identifier format: use 'cluster' or 'organization/cluster'")
		}

		// Validate each part
		for _, part := range parts {
			result, err := validator.Validate(ctx, part)
			if err != nil {
				return "", fmt.Errorf("validation error: %w", err)
			}
			if !result.Valid {
				return "", fmt.Errorf("invalid cluster identifier: %s", result.Errors[0].Message)
			}
		}

		return clusterName, nil
	}

	// No argument provided, try to use active cluster
	activeName, err := getActiveCluster()
	if err != nil {
		return "", fmt.Errorf("failed to get active cluster: %w", err)
	}

	if activeName == "" {
		if requireActive {
			return "", fmt.Errorf("no active cluster set. Use 'opencenter cluster select <cluster>' or provide cluster name as argument")
		}
		return "", nil
	}

	return activeName, nil
}

// resolveClusterNameFromFlag resolves the cluster name from a flag value or active cluster.
// This is used by commands that use --cluster flag instead of positional arguments.
//
// Parameters:
//   - flagValue: The value from the --cluster flag (empty string if not provided)
//   - requireActive: If true and no flag provided, returns error if no active cluster
//
// Returns:
//   - clusterName: The resolved cluster name (may include organization prefix)
//   - error: An error if resolution fails
func resolveClusterNameFromFlag(flagValue string, requireActive bool) (string, error) {
	ctx := context.Background()
	validator := validators.NewClusterNameValidator()

	// If cluster flag provided
	if flagValue != "" {
		clusterName := strings.TrimSpace(flagValue)
		if clusterName == "" {
			return "", fmt.Errorf("cluster name cannot be empty")
		}

		// Validate the cluster identifier (handles both "cluster" and "org/cluster" formats)
		parts := strings.Split(clusterName, "/")
		if len(parts) > 2 {
			return "", fmt.Errorf("invalid cluster identifier format: use 'cluster' or 'organization/cluster'")
		}

		// Validate each part
		for _, part := range parts {
			result, err := validator.Validate(ctx, part)
			if err != nil {
				return "", fmt.Errorf("validation error: %w", err)
			}
			if !result.Valid {
				return "", fmt.Errorf("invalid cluster identifier: %s", result.Errors[0].Message)
			}
		}

		return clusterName, nil
	}

	// No flag provided, try to use active cluster
	activeName, err := getActiveCluster()
	if err != nil {
		return "", fmt.Errorf("failed to get active cluster: %w", err)
	}

	if activeName == "" {
		if requireActive {
			return "", fmt.Errorf("no active cluster set. Use 'opencenter cluster select <cluster>' or provide --cluster flag")
		}
		return "", nil
	}

	return activeName, nil
}
