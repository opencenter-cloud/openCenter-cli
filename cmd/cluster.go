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
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	"github.com/opencenter-cloud/opencenter-cli/internal/resilience"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
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
  1. Create a cluster config
     opencenter cluster init prod --org acme
  2. Complete provider-specific settings
     opencenter cluster configure acme/prod
  3. Validate config and prerequisites
     opencenter cluster validate acme/prod
     opencenter cluster doctor acme/prod
  4. Generate GitOps assets
     opencenter cluster generate acme/prod
  5. Deploy the cluster
     opencenter cluster deploy acme/prod

Configuration files are stored in organization-based directories:
  ~/.config/opencenter/clusters/<organization>/<cluster>/`,
		Example: `  # Create a cluster config
  opencenter cluster init prod --org acme

  # Set active cluster
  opencenter cluster use acme/prod

  # Show active cluster
  opencenter cluster active

  # Generate GitOps assets
  opencenter cluster generate acme/prod

  # Deploy the cluster
  opencenter cluster deploy acme/prod

  # Describe configuration and state
  opencenter cluster describe acme/prod`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	// Add subcommands
	cmd.AddCommand(newClusterListCmd())
	cmd.AddCommand(newClusterUseCmd())
	cmd.AddCommand(newClusterActiveCmd())
	cmd.AddCommand(newClusterEnvCmd())
	cmd.AddCommand(newClusterStatusCmd())
	cmd.AddCommand(newClusterDescribeCmd())
	cmd.AddCommand(newClusterInitCmd())
	cmd.AddCommand(newClusterConfigureCmd())
	cmd.AddCommand(newClusterEditCmd())
	cmd.AddCommand(newClusterSetCmd())
	cmd.AddCommand(newClusterNormalizeCmd())
	cmd.AddCommand(newClusterExportCmd())
	cmd.AddCommand(newClusterValidateCmd())
	cmd.AddCommand(newClusterDoctorCmd())
	cmd.AddCommand(newClusterGenerateCmd())
	cmd.AddCommand(newClusterDeployCmd())
	cmd.AddCommand(newClusterSchemaCmd())
	cmd.AddCommand(newClusterTemplateCmd())
	cmd.AddCommand(newClusterDestroyCmd())
	cmd.AddCommand(newClusterServiceCmd())
	cmd.AddCommand(newClusterDriftCmd())
	cmd.AddCommand(newClusterBackupCmd())
	cmd.AddCommand(newClusterLockCmd())
	cmd.AddCommand(newClusterUnlockCmd())
	cmd.AddCommand(newClusterImportCmd())
	return cmd
}

func missingActiveClusterError(command string) error {
	return fmt.Errorf("no active cluster is set\n\nFix:\n  opencenter cluster list\n  opencenter cluster use <org/name>\n\nOr pass a cluster explicitly:\n  %s <org/name>", command)
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
			return "", missingActiveClusterError("opencenter cluster validate")
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
			return "", missingActiveClusterError("opencenter cluster validate")
		}
		return "", nil
	}

	return activeName, nil
}

// LockAcquisitionResult contains the result of attempting to acquire a lock
type LockAcquisitionResult struct {
	Lock          resilience.Lock
	LockManager   resilience.LockManager
	ExistingLock  *resilience.LockState
	WasBroken     bool
	UserConfirmed bool
}

// AcquireLockWithPrompt attempts to acquire a lock on a cluster resource.
// If a lock already exists, it checks the --break-lock flag or prompts the user
// for confirmation before breaking the lock.
//
// Parameters:
//   - ctx: Context for the operation
//   - cmd: The cobra command (for flags and I/O)
//   - resource: The resource name to lock (typically cluster name)
//   - operation: Description of the operation (e.g., "destroy", "bootstrap")
//   - ttl: Time-to-live for the lock
//   - metadata: Additional metadata to store with the lock
//
// Returns:
//   - *LockAcquisitionResult: Contains the acquired lock and related info
//   - error: An error if lock acquisition fails
func AcquireLockWithPrompt(ctx context.Context, cmd *cobra.Command, resource string, operation string, ttl time.Duration, metadata map[string]string) (*LockAcquisitionResult, error) {
	lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock manager: %w", err)
	}

	result := &LockAcquisitionResult{
		LockManager: lockMgr,
	}

	// Check for existing lock first
	existingLock, err := lockMgr.GetLockInfo(resource)
	if err != nil {
		return nil, fmt.Errorf("failed to check existing lock: %w", err)
	}

	if existingLock != nil {
		result.ExistingLock = existingLock

		// Check if --break-lock flag is set
		breakLock, _ := cmd.Flags().GetBool("break-lock")

		if breakLock {
			// Force break the lock
			if err := lockMgr.ForceBreak(resource); err != nil {
				return nil, fmt.Errorf("failed to break existing lock: %w", err)
			}
			result.WasBroken = true
			fmt.Fprintf(cmd.OutOrStdout(), "Broke existing lock held by %s (operation: %s)\n",
				existingLock.Owner, existingLock.Metadata["operation"])
		} else {
			// Prompt user for confirmation
			testMode := os.Getenv("OPENCENTER_TEST_MODE") != ""
			prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)

			// Build informative message about the existing lock
			lockAge := time.Since(existingLock.AcquiredAt).Round(time.Second)
			expiresIn := time.Until(existingLock.ExpiresAt).Round(time.Second)

			message := fmt.Sprintf(
				"An existing lock was found:\n"+
					"  Owner: %s\n"+
					"  Operation: %s\n"+
					"  Acquired: %s ago\n"+
					"  Expires in: %s\n\n"+
					"Do you want to break this lock and proceed with %s?",
				existingLock.Owner,
				existingLock.Metadata["operation"],
				lockAge,
				expiresIn,
				operation,
			)

			confirmed, err := prompter.Confirm(ctx, message)
			if err != nil {
				return nil, fmt.Errorf("confirmation prompt failed: %w", err)
			}
			if !confirmed {
				return nil, fmt.Errorf("operation cancelled: existing lock not broken")
			}

			result.UserConfirmed = true

			// Break the lock
			if err := lockMgr.ForceBreak(resource); err != nil {
				return nil, fmt.Errorf("failed to break existing lock: %w", err)
			}
			result.WasBroken = true
			fmt.Fprintf(cmd.OutOrStdout(), "Broke existing lock held by %s\n", existingLock.Owner)
		}
	}

	// Now acquire the lock
	lock, err := lockMgr.AcquireWithMetadata(ctx, resource, ttl, metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire lock for cluster %q: %w\nAnother operation may be in progress. Wait for it to complete or use 'opencenter cluster describe %s' to check lock status", resource, err, resource)
	}

	result.Lock = lock
	return result, nil
}
