package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/flux"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitops"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var stateDir string
	var configDir string

	cmd := &cobra.Command{
		Use:           "opencenter-local",
		Short:         "Local Kind + Gitea workflow plugin for openCenter",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			if strings.TrimSpace(configDir) != "" {
				_ = os.Setenv("OPENCENTER_CONFIG_DIR", configDir)
			}
		},
	}

	cmd.PersistentFlags().StringVar(&stateDir, "state-dir", "", "plugin state directory (defaults to $OPENCENTER_CONFIG_DIR/local)")
	cmd.PersistentFlags().StringVar(&configDir, "config-dir", "", "override openCenter config directory")

	cmd.AddCommand(newGiteaCmd(&stateDir))
	cmd.AddCommand(newGitOpsCmd(&stateDir))
	cmd.AddCommand(newFluxCmd(&stateDir))

	return cmd
}

func newGiteaCmd(stateDir *string) *cobra.Command {
	var runtime string

	cmd := &cobra.Command{
		Use:   "gitea",
		Short: "Manage the disposable local Gitea instance",
	}
	cmd.PersistentFlags().StringVar(&runtime, "runtime", "", "container runtime to use (docker or podman)")

	cmd.AddCommand(&cobra.Command{
		Use:   "up",
		Short: "Start local Gitea and provision tokens plus the test repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			service, err := gitea.NewService(localdev.NewExecutor(), *stateDir, gitea.DefaultSettings(runtime))
			if err != nil {
				return err
			}
			status, err := service.Up(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Gitea is ready at %s\n", status.BaseURL)
			fmt.Fprintf(cmd.OutOrStdout(), "Repository URL: %s\n", status.LocalRepoURL)
			fmt.Fprintf(cmd.OutOrStdout(), "CA certificate: %s\n", status.CAPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Admin token: %s\n", status.AdminTokenPath)
			fmt.Fprintf(cmd.OutOrStdout(), "User token: %s\n", status.UserTokenPath)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show the current local Gitea state",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			service, err := gitea.NewService(localdev.NewExecutor(), *stateDir, gitea.DefaultSettings(runtime))
			if err != nil {
				return err
			}
			status, err := service.Status(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Runtime: %s\n", status.Metadata.Runtime)
			fmt.Fprintf(cmd.OutOrStdout(), "Running: %t\n", status.Running)
			fmt.Fprintf(cmd.OutOrStdout(), "Base URL: %s\n", status.BaseURL)
			fmt.Fprintf(cmd.OutOrStdout(), "Repository URL: %s\n", status.LocalRepoURL)
			fmt.Fprintf(cmd.OutOrStdout(), "Networks: %s\n", strings.Join(status.AttachedNetworks, ", "))
			fmt.Fprintf(cmd.OutOrStdout(), "Kind Attached: %t\n", status.KindAttached)
			if status.KindIP != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "Kind IP: %s\n", status.KindIP)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "CA certificate: %s\n", status.CAPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Admin token present: %t (%s)\n", status.AdminTokenExists, status.AdminTokenPath)
			fmt.Fprintf(cmd.OutOrStdout(), "User token present: %t (%s)\n", status.UserTokenExists, status.UserTokenPath)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "destroy",
		Short: "Stop local Gitea and remove plugin state",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := gitea.NewService(localdev.NewExecutor(), *stateDir, gitea.DefaultSettings(runtime))
			if err != nil {
				return err
			}
			if err := service.Destroy(cmd.Context()); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Local Gitea state removed.")
			return nil
		},
	})

	var cluster string
	attachCmd := &cobra.Command{
		Use:   "attach-kind",
		Short: "Connect Gitea to the Kind network and reissue the TLS cert",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			resolver, err := localdev.NewClusterResolver()
			if err != nil {
				return err
			}
			clusterCtx, err := resolver.Resolve(ctx, cluster)
			if err != nil {
				return err
			}
			if !strings.EqualFold(clusterCtx.Config.OpenCenter.Infrastructure.Provider, "kind") {
				return fmt.Errorf("cluster %q is not a kind cluster", cluster)
			}

			service, err := gitea.NewService(localdev.NewExecutor(), *stateDir, gitea.DefaultSettings(runtime))
			if err != nil {
				return err
			}
			result, err := service.AttachKind(ctx)
			if err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Kind network attached.\n")
			fmt.Fprintf(cmd.OutOrStdout(), "Kind IP: %s\n", result.KindIP)
			fmt.Fprintf(cmd.OutOrStdout(), "In-cluster repo URL: %s\n", result.InClusterRepoURL)
			fmt.Fprintf(cmd.OutOrStdout(), "CA certificate: %s\n", result.CAPath)
			return nil
		},
	}
	attachCmd.Flags().StringVar(&cluster, "cluster", "", "kind cluster name")
	_ = attachCmd.MarkFlagRequired("cluster")
	cmd.AddCommand(attachCmd)

	return cmd
}

func newGitOpsCmd(stateDir *string) *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "gitops",
		Short: "Operate on local GitOps repositories",
	}

	pushCmd := &cobra.Command{
		Use:   "push",
		Short: "Push the cluster GitOps repo to local Gitea",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := gitops.NewService(localdev.NewExecutor(), *stateDir)
			if err != nil {
				return err
			}
			result, err := service.Push(cmd.Context(), cluster)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Pushed %s on branch %s to %s (%s)\n", result.GitDir, result.Branch, result.RemoteName, result.RemoteURL)
			return nil
		},
	}
	pushCmd.Flags().StringVar(&cluster, "cluster", "", "cluster name or organization/cluster")
	_ = pushCmd.MarkFlagRequired("cluster")
	cmd.AddCommand(pushCmd)

	return cmd
}

func newFluxCmd(stateDir *string) *cobra.Command {
	var cluster string

	cmd := &cobra.Command{
		Use:   "flux",
		Short: "Run local Flux bootstrap helpers",
	}

	bootstrapCmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Bootstrap Flux against the attached local Gitea repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			service, err := flux.NewService(localdev.NewExecutor(), *stateDir)
			if err != nil {
				return err
			}
			result, err := service.Bootstrap(cmd.Context(), cluster)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Flux bootstrapped from %s\n", result.RepoURL)
			fmt.Fprintf(cmd.OutOrStdout(), "Branch: %s\n", result.Branch)
			fmt.Fprintf(cmd.OutOrStdout(), "Kubeconfig: %s\n", result.KubeconfigPath)
			return nil
		},
	}
	bootstrapCmd.Flags().StringVar(&cluster, "cluster", "", "cluster name or organization/cluster")
	_ = bootstrapCmd.MarkFlagRequired("cluster")
	cmd.AddCommand(bootstrapCmd)

	return cmd
}
