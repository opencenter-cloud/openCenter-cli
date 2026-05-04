package cluster

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	talosdeploy "github.com/opencenter-cloud/opencenter-cli/internal/deployment/talos"
)

type talosBootstrapRuntime interface {
	ReadInventory(ctx context.Context) error
	GenerateSecrets(ctx context.Context) error
	ApplyMachineConfigs(ctx context.Context) error
	BootstrapControlPlane(ctx context.Context) error
	ExportTalosConfig(ctx context.Context) error
	ExportKubeconfig(ctx context.Context) error
	WaitReady(ctx context.Context) error
}

var newTalosBootstrapRuntime = func(cfg *v2.Config, clusterPaths *paths.ClusterPaths) (talosBootstrapRuntime, error) {
	return talosdeploy.NewRuntime(cfg, clusterPaths)
}

type talosBootstrapProvider struct {
	runner lifecycleCommandRunner
}

func newTalosBootstrapProvider(runner lifecycleCommandRunner) lifecycleBootstrapProvider {
	return &talosBootstrapProvider{runner: runner}
}

func (p *talosBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for Talos OpenStack bootstrap")
	}
	if cfg.Deployment.Talos == nil {
		return nil, fmt.Errorf("deployment.talos must be configured for Talos bootstrap")
	}

	openTofuPath := strings.TrimSpace(cfg.OpenTofu.Path)
	if openTofuPath == "" {
		openTofuPath = "opentofu"
	}
	planEnv := openStackPlanEnv(opts.KubeconfigPath)
	artifactPaths := talosdeploy.ResolveArtifactPaths(clusterPaths, cfg.ClusterName())
	var runtime talosBootstrapRuntime
	getRuntime := func() (talosBootstrapRuntime, error) {
		if runtime != nil {
			return runtime, nil
		}
		runtime, err = newTalosBootstrapRuntime(cfg, clusterPaths)
		if err != nil {
			return nil, err
		}
		return runtime, nil
	}

	return []bootstrapStep{
		{
			ID:          "talos-preflight",
			Description: "Validate OpenStack credentials and Talos bootstrap prerequisites",
			Plan: BootstrapPlanStep{
				ID:         "talos-preflight",
				Action:     "Validate OpenStack credentials and Talos bootstrap prerequisites",
				WorkingDir: clusterDir,
				Reads:      []string{clusterDir},
				Notes: []string{
					"Plan only; OpenStack credentials, infrastructure directory, and Talos inventory were not checked.",
					"Talos OpenStack requires deployment.talos.network.management_cidrs and Kubernetes API port 443.",
				},
			},
			Run: func(ctx context.Context) error {
				if err := validateTalosBootstrapManagementCIDRs(cfg); err != nil {
					return err
				}
				if _, err := os.Stat(clusterDir); err != nil {
					return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
				}
				creds, err := extractOpenStackBootstrapCredentials(cfg)
				if err != nil {
					return err
				}
				if err := validateOpenStackBootstrap(creds); err != nil {
					return err
				}
				if _, err := getRuntime(); err != nil {
					return err
				}
				return nil
			},
		},
		{
			ID:          "opentofu-init",
			Description: "Initialize OpenTofu",
			Plan: BootstrapPlanStep{
				ID:          "opentofu-init",
				Action:      "Initialize OpenTofu",
				WorkingDir:  clusterDir,
				Commands:    []BootstrapPlanCommand{commandPlan(openTofuPath, "init")},
				Environment: planEnv,
				Reads:       []string{clusterDir},
				Writes:      []string{filepath.Join(clusterDir, ".terraform")},
				Notes:       []string{"Plan only; OpenTofu binary, backend access, and provider initialization were not checked."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildOpenStackBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				_, runErr := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
				return runErr
			},
		},
		{
			ID:          "opentofu-apply",
			Description: "Apply OpenTofu infrastructure",
			Plan: BootstrapPlanStep{
				ID:          "opentofu-apply",
				Action:      "Apply OpenTofu infrastructure",
				WorkingDir:  clusterDir,
				Commands:    []BootstrapPlanCommand{commandPlan(openTofuPath, "apply", "-auto-approve")},
				Environment: planEnv,
				Reads:       []string{clusterDir},
				Writes:      []string{"OpenStack infrastructure resources", filepath.Join(clusterDir, "terraform.tfstate"), artifactPaths.InventoryPath},
				Notes: []string{
					"Plan only; OpenStack API access and infrastructure changes were not simulated.",
					"OpenTofu will expose Kubernetes API on 443 and create per-node Talos management access for deployment.talos.network.management_cidrs.",
				},
			},
			Run: func(ctx context.Context) error {
				env, err := buildOpenStackBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				_, runErr := p.runner.Run(ctx, clusterDir, env, openTofuPath, "apply", "-auto-approve")
				return runErr
			},
		},
		talosRuntimeStep("talos-read-inventory", "Read Talos inventory contract", clusterDir, []string{artifactPaths.InventoryPath}, nil, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.ReadInventory(ctx)
		}),
		talosRuntimeStep("talos-generate-secrets", "Generate or load Talos machine secrets", clusterDir, []string{artifactPaths.InventoryPath}, []string{artifactPaths.MachineSecretsPath}, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.GenerateSecrets(ctx)
		}),
		talosRuntimeStep("talos-apply-machine-configs", "Apply Talos machine configs through the Talos API", clusterDir, []string{artifactPaths.InventoryPath, artifactPaths.MachineSecretsPath, artifactPaths.PatchesDir}, []string{"Talos machine configuration applied to nodes"}, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.ApplyMachineConfigs(ctx)
		}),
		talosRuntimeStep("talos-bootstrap-controlplane", "Bootstrap Talos control plane", clusterDir, []string{artifactPaths.InventoryPath, artifactPaths.MachineSecretsPath}, []string{"Talos control plane bootstrapped"}, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.BootstrapControlPlane(ctx)
		}),
		talosRuntimeStep("talos-export-talosconfig", "Export Talos client configuration", clusterDir, []string{artifactPaths.MachineSecretsPath}, []string{artifactPaths.TalosConfigPath}, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.ExportTalosConfig(ctx)
		}),
		talosRuntimeStep("talos-export-kubeconfig", "Export Kubernetes kubeconfig from Talos", clusterDir, []string{artifactPaths.TalosConfigPath}, []string{artifactPaths.KubeconfigPath}, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.ExportKubeconfig(ctx)
		}),
		talosRuntimeStep("talos-wait-ready", "Wait for Talos and Kubernetes readiness", clusterDir, []string{artifactPaths.TalosConfigPath, artifactPaths.KubeconfigPath}, nil, getRuntime, func(ctx context.Context, runtime talosBootstrapRuntime) error {
			return runtime.WaitReady(ctx)
		}),
	}, nil
}

func validateTalosBootstrapManagementCIDRs(cfg *v2.Config) error {
	if cfg == nil || cfg.Deployment.Talos == nil {
		return fmt.Errorf("deployment.talos must be configured for Talos bootstrap")
	}
	cidrs := cfg.Deployment.Talos.Network.ManagementCIDRs
	if len(cidrs) == 0 {
		return fmt.Errorf("deployment.talos.network.management_cidrs must contain at least one IPv4 CIDR before Talos OpenStack bootstrap")
	}
	for _, cidr := range cidrs {
		ip, _, err := net.ParseCIDR(strings.TrimSpace(cidr))
		if err != nil || ip == nil || ip.To4() == nil {
			return fmt.Errorf("deployment.talos.network.management_cidrs contains invalid IPv4 CIDR %q", cidr)
		}
	}
	return nil
}

func talosRuntimeStep(
	id string,
	action string,
	workingDir string,
	reads []string,
	writes []string,
	getRuntime func() (talosBootstrapRuntime, error),
	run func(context.Context, talosBootstrapRuntime) error,
) bootstrapStep {
	return bootstrapStep{
		ID:          id,
		Description: action,
		Plan: BootstrapPlanStep{
			ID:         id,
			Action:     action,
			WorkingDir: workingDir,
			Reads:      reads,
			Writes:     writes,
		},
		Run: func(ctx context.Context) error {
			runtime, err := getRuntime()
			if err != nil {
				return err
			}
			return run(ctx, runtime)
		},
	}
}
