package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/importer"
	"github.com/opencenter-cloud/opencenter-cli/internal/ui"
	"github.com/spf13/cobra"
)

func newClusterImportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import running clusters into openCenter config",
		Long: `Discover cluster metadata from GitOps sources with kubeconfig fallback,
persist the discovery artifact, and create or patch openCenter cluster configs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(newClusterImportScanCmd())
	cmd.AddCommand(newClusterImportReportCmd())
	cmd.AddCommand(newClusterImportApplyCmd())
	return cmd
}

func newClusterImportScanCmd() *cobra.Command {
	var repoPath string
	var serviceNamespaces []string

	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Scan a customer GitOps repo and persist a cluster import artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := filepath.Abs(strings.TrimSpace(repoPath))
			if err != nil {
				return fmt.Errorf("resolve repo path: %w", err)
			}

			scanner := importer.NewScanner()
			if err := scanner.ApplyNamespaceOverrides(serviceNamespaces); err != nil {
				return err
			}

			result, err := scanner.ScanRepo(cmd.Context(), repoPath)
			if err != nil {
				return err
			}

			store, err := importer.NewArtifactStore()
			if err != nil {
				return err
			}

			artifact, err := store.Save(repoPath, result, time.Now().UTC())
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Discovered %d clusters from %s\n", result.Summary.ClustersDiscovered, repoPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Saved scan artifact: %s\n", artifact.Path)
			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo", "", "Path to the customer GitOps repository")
	cmd.Flags().StringArrayVar(&serviceNamespaces, "service-namespace", nil, "Override service namespace ownership (svc=ns1,ns2)")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}

func newClusterImportReportCmd() *cobra.Command {
	var repoPath string
	var outputFormat string

	cmd := &cobra.Command{
		Use:   "report",
		Short: "Render the latest cluster import artifact for a repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := filepath.Abs(strings.TrimSpace(repoPath))
			if err != nil {
				return fmt.Errorf("resolve repo path: %w", err)
			}

			store, err := importer.NewArtifactStore()
			if err != nil {
				return err
			}

			artifact, err := store.LoadLatest(repoPath)
			if err != nil {
				return err
			}

			rendered, err := importer.RenderScanResult(artifact.Result, outputFormat)
			if err != nil {
				return err
			}

			fmt.Fprint(cmd.OutOrStdout(), string(rendered))
			if outputFormat == "json" || outputFormat == "yaml" {
				fmt.Fprintln(cmd.OutOrStdout())
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo", "", "Path to the customer GitOps repository")
	cmd.Flags().StringVar(&outputFormat, "output", "text", "Output format (text, json, yaml)")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}

func newClusterImportApplyCmd() *cobra.Command {
	var repoPath string

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Create or patch openCenter configs from the latest import artifact",
		RunE: func(cmd *cobra.Command, args []string) error {
			repoPath, err := filepath.Abs(strings.TrimSpace(repoPath))
			if err != nil {
				return fmt.Errorf("resolve repo path: %w", err)
			}

			store, err := importer.NewArtifactStore()
			if err != nil {
				return err
			}

			artifact, err := store.LoadLatest(repoPath)
			if err != nil {
				return err
			}

			testMode := os.Getenv("OPENCENTER_TEST_MODE") == "1"
			prompter := ui.GetPrompter(os.Stdin, cmd.OutOrStdout(), testMode)

			plans := make([]*importer.ClusterWritePlan, 0, len(artifact.Result.Clusters))
			createCount := 0
			for _, cluster := range artifact.Result.Clusters {
				plan, err := importer.PrepareClusterWritePlan(context.Background(), cluster)
				if err != nil {
					return fmt.Errorf("prepare write plan for %s: %w", cluster.ClusterName, err)
				}
				if plan.Create {
					createCount++
				}
				plans = append(plans, plan)
			}

			createApproved := true
			if createCount > 0 {
				createApproved, err = prompter.Confirm(cmd.Context(), fmt.Sprintf("Create %d new cluster config(s)?", createCount))
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}
			}

			for _, plan := range plans {
				if plan.Create {
					if !createApproved {
						fmt.Fprintf(cmd.OutOrStdout(), "Skipped new config for %s\n", plan.ClusterName)
						continue
					}
					if err := importer.ApplyClusterWritePlan(plan); err != nil {
						return err
					}
					fmt.Fprintf(cmd.OutOrStdout(), "Created config for %s at %s\n", plan.ClusterName, plan.ConfigPath)
					continue
				}

				if strings.TrimSpace(plan.Diff) == "" {
					fmt.Fprintf(cmd.OutOrStdout(), "No approved changes for %s\n", plan.ClusterName)
					continue
				}

				fmt.Fprintln(cmd.OutOrStdout(), plan.Diff)
				confirmed, err := prompter.Confirm(cmd.Context(), fmt.Sprintf("Apply config patch for %s?", plan.ClusterName))
				if err != nil {
					return fmt.Errorf("confirmation prompt failed: %w", err)
				}
				if !confirmed {
					fmt.Fprintf(cmd.OutOrStdout(), "Skipped config patch for %s\n", plan.ClusterName)
					continue
				}

				if err := importer.ApplyClusterWritePlan(plan); err != nil {
					return err
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Updated config for %s at %s\n", plan.ClusterName, plan.ConfigPath)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&repoPath, "repo", "", "Path to the customer GitOps repository")
	_ = cmd.MarkFlagRequired("repo")
	return cmd
}
