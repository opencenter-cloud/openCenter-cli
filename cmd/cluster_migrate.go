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
	"fmt"
	"os"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

func newClusterMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [cluster-name]",
		Short: "Migrate cluster configurations from legacy to organization-based structure",
		Long: `Migrate cluster configurations from the legacy flat directory structure 
to the new organization-based directory structure. This command can migrate 
individual clusters or all legacy clusters at once.

The migration process:
1. Creates a backup of the existing cluster configuration
2. Creates the organization-based directory structure
3. Moves cluster files to the new structure
4. Updates configuration with organization metadata
5. Validates the migration was successful

If migration fails, you can use the --rollback flag to restore from backup.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Initialize configuration manager and path resolver
			configManager, err := config.NewConfigManager("")
			if err != nil {
				return fmt.Errorf("failed to initialize configuration manager: %w", err)
			}

			pathResolver := config.NewPathResolver(configManager)
			migrationManager := config.NewMigrationManager(pathResolver, configManager)

			// Get flags
			organization, _ := cmd.Flags().GetString("organization")
			if organization == "" {
				organization = "opencenter"
			}

			backup, _ := cmd.Flags().GetBool("backup")
			rollback, _ := cmd.Flags().GetString("rollback")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			force, _ := cmd.Flags().GetBool("force")

			// Handle rollback operation
			if rollback != "" {
				if len(args) == 0 {
					return fmt.Errorf("cluster name is required for rollback operation")
				}
				clusterName := args[0]

				fmt.Fprintf(cmd.OutOrStdout(), "Rolling back cluster '%s' from backup '%s'...\n", clusterName, rollback)
				if err := migrationManager.RestoreCluster(clusterName, rollback); err != nil {
					return fmt.Errorf("rollback failed: %w", err)
				}

				fmt.Fprintf(cmd.OutOrStdout(), "Successfully rolled back cluster '%s'\n", clusterName)
				return nil
			}

			// Detect legacy clusters
			legacyClusters, err := migrationManager.DetectLegacyStructure()
			if err != nil {
				return fmt.Errorf("failed to detect legacy clusters: %w", err)
			}

			if len(legacyClusters) == 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "No legacy clusters found to migrate.\n")
				return nil
			}

			// Determine which clusters to migrate
			var clustersToMigrate []string
			if len(args) == 1 {
				// Migrate specific cluster
				clusterName := args[0]
				found := false
				for _, legacy := range legacyClusters {
					if legacy == clusterName {
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("cluster '%s' is not a legacy cluster or does not exist", clusterName)
				}
				clustersToMigrate = []string{clusterName}
			} else {
				// Migrate all legacy clusters
				clustersToMigrate = legacyClusters

				if !force {
					fmt.Fprintf(cmd.OutOrStdout(), "Found %d legacy clusters to migrate: %v\n", len(legacyClusters), legacyClusters)
					fmt.Fprintf(cmd.OutOrStdout(), "This will migrate all clusters to organization '%s'. Use --force to proceed without confirmation.\n", organization)
					return fmt.Errorf("migration cancelled - use --force to proceed")
				}
			}

			// Dry run mode
			if dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "DRY RUN: Would migrate the following clusters to organization '%s':\n", organization)
				for _, clusterName := range clustersToMigrate {
					fmt.Fprintf(cmd.OutOrStdout(), "  - %s\n", clusterName)
				}
				return nil
			}

			// Perform migrations
			var backupPaths []string
			var migrated []string
			var failed []string

			for _, clusterName := range clustersToMigrate {
				fmt.Fprintf(cmd.OutOrStdout(), "Migrating cluster '%s' to organization '%s'...\n", clusterName, organization)

				// Create backup if requested
				var backupPath string
				if backup {
					fmt.Fprintf(cmd.OutOrStdout(), "  Creating backup...\n")
					backupPath, err = migrationManager.BackupCluster(clusterName)
					if err != nil {
						fmt.Fprintf(cmd.ErrOrStderr(), "  Failed to create backup for cluster '%s': %v\n", clusterName, err)
						failed = append(failed, clusterName)
						continue
					}
					backupPaths = append(backupPaths, backupPath)
					fmt.Fprintf(cmd.OutOrStdout(), "  Backup created at: %s\n", backupPath)
				}

				// Perform migration
				fmt.Fprintf(cmd.OutOrStdout(), "  Migrating files and directories...\n")
				if err := migrationManager.MigrateClusterToOrganization(clusterName, organization); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  Migration failed for cluster '%s': %v\n", clusterName, err)
					failed = append(failed, clusterName)

					// Attempt rollback if backup exists
					if backup && backupPath != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "  Attempting rollback...\n")
						if rollbackErr := migrationManager.RestoreCluster(clusterName, backupPath); rollbackErr != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "  Rollback also failed: %v\n", rollbackErr)
						} else {
							fmt.Fprintf(cmd.OutOrStdout(), "  Rollback successful\n")
						}
					}
					continue
				}

				// Validate migration
				fmt.Fprintf(cmd.OutOrStdout(), "  Validating migration...\n")
				if err := migrationManager.ValidatePostMigration(clusterName, organization); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  Migration validation failed for cluster '%s': %v\n", clusterName, err)
					failed = append(failed, clusterName)
					continue
				}

				migrated = append(migrated, clusterName)
				fmt.Fprintf(cmd.OutOrStdout(), "  Successfully migrated cluster '%s'\n", clusterName)
			}

			// Summary
			fmt.Fprintf(cmd.OutOrStdout(), "\nMigration Summary:\n")
			fmt.Fprintf(cmd.OutOrStdout(), "  Successfully migrated: %d clusters\n", len(migrated))
			if len(migrated) > 0 {
				for _, cluster := range migrated {
					fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", cluster)
				}
			}

			if len(failed) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "  Failed to migrate: %d clusters\n", len(failed))
				for _, cluster := range failed {
					fmt.Fprintf(cmd.OutOrStdout(), "    - %s\n", cluster)
				}
			}

			if backup && len(backupPaths) > 0 {
				fmt.Fprintf(cmd.OutOrStdout(), "\nBackups created:\n")
				for _, backupPath := range backupPaths {
					fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", backupPath)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "\nTo rollback a cluster, use: openCenter cluster migrate --rollback <backup-path> <cluster-name>\n")
			}

			if len(failed) > 0 {
				os.Exit(1)
			}

			return nil
		},
	}

	cmd.Flags().String("organization", "opencenter", "Target organization for migration")
	cmd.Flags().Bool("backup", true, "Create backup before migration")
	cmd.Flags().String("rollback", "", "Rollback cluster from specified backup path")
	cmd.Flags().Bool("dry-run", false, "Show what would be migrated without performing the migration")
	cmd.Flags().Bool("force", false, "Force migration of all legacy clusters without confirmation")

	return cmd
}
