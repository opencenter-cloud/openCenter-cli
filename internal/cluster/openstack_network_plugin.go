package cluster

import (
	"context"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

const (
	openStackNetworkPluginStepID = "openstack-install-network-plugin"

	openStackNetworkPluginMethodHelm          = "helm"
	openStackNetworkPluginMethodKustomizeHelm = "kustomize-helm"

	defaultCiliumChartVersion  = "1.19.3"
	defaultKubeOVNChartVersion = "v1.17.0"

	defaultCalicoChartVersion = "v3.32.0"
	calicoHelmRepo            = "https://docs.tigera.io/calico/charts"
	calicoHelmRepoName        = "projectcalico"
	calicoHelmChart           = "projectcalico/tigera-operator"
)

type openStackNetworkPluginSelection struct {
	Name          string
	InstallMethod string
	Version       string
	Namespace     string
	ReleaseName   string
	Chart         string
	Repo          string
	ChartName     string
}

func (p *openstackBootstrapProvider) buildNetworkPluginInstallStep(cfg *v2.Config, clusterDir string, planEnv []BootstrapPlanEnv, opts *BootstrapOptions) (bootstrapStep, error) {
	selection, err := selectOpenStackNetworkPlugin(cfg)
	if err != nil {
		return bootstrapStep{}, err
	}
	action := fmt.Sprintf("Install %s network plugin using %s", selection.Name, selection.InstallMethod)
	notes := []string{"Plan only; Helm, kubectl, Kustomize rendering, and Kubernetes API access were not checked."}

	return bootstrapStep{
		ID:          openStackNetworkPluginStepID,
		Description: fmt.Sprintf("Install %s network plugin", selection.Name),
		Plan: BootstrapPlanStep{
			ID:          openStackNetworkPluginStepID,
			Action:      action,
			WorkingDir:  clusterDir,
			Commands:    openStackNetworkPluginPlanCommands(selection, opts.KubeconfigPath),
			Environment: planEnv,
			Reads:       []string{opts.KubeconfigPath},
			Writes:      []string{"Kubernetes CNI resources"},
			Notes:       notes,
		},
		Run: func(ctx context.Context) error {
			return p.installOpenStackNetworkPlugin(ctx, cfg, opts.KubeconfigPath)
		},
	}, nil
}

func (p *openstackBootstrapProvider) installOpenStackNetworkPlugin(ctx context.Context, cfg *v2.Config, kubeconfigPath string) error {
	selection, err := selectOpenStackNetworkPlugin(cfg)
	if err != nil {
		return err
	}
	if strings.TrimSpace(kubeconfigPath) == "" {
		return fmt.Errorf("kubeconfig path must be set before installing %s", selection.Name)
	}

	env, err := buildOpenStackBootstrapEnvironment(cfg, kubeconfigPath)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "opencenter-openstack-cni-*")
	if err != nil {
		return fmt.Errorf("create temporary CNI install directory: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	if selection.Name == "calico" {
		if err := p.installOpenStackCalicoWithHelm(ctx, cfg, selection, kubeconfigPath, tmpDir, env); err != nil {
			return err
		}
		return p.waitForOpenStackNetworkPlugin(ctx, selection, kubeconfigPath, tmpDir, env)
	}

	switch selection.InstallMethod {
	case openStackNetworkPluginMethodHelm:
		if err := p.installOpenStackNetworkPluginWithHelm(ctx, cfg, selection, kubeconfigPath, tmpDir, env); err != nil {
			return err
		}
	case openStackNetworkPluginMethodKustomizeHelm:
		if err := p.installOpenStackNetworkPluginWithKustomizeHelm(ctx, cfg, selection, kubeconfigPath, tmpDir, env); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported OpenStack network plugin install_method %q for %s; use %q or %q", selection.InstallMethod, selection.Name, openStackNetworkPluginMethodHelm, openStackNetworkPluginMethodKustomizeHelm)
	}

	return p.waitForOpenStackNetworkPlugin(ctx, selection, kubeconfigPath, tmpDir, env)
}

// installOpenStackCalicoWithHelm installs Calico via the official Helm chart
// from https://docs.tigera.io/calico/charts. It uses a values override file
// located in the GitOps repository at:
//
//	<organization>/applications/overlays/<cluster>/services/calico/helm-values/override_values.yaml
func (p *openstackBootstrapProvider) installOpenStackCalicoWithHelm(ctx context.Context, cfg *v2.Config, selection openStackNetworkPluginSelection, kubeconfigPath, tmpDir string, env map[string]string) error {
	// Add the projectcalico Helm repository
	if _, err := p.runner.Run(ctx, tmpDir, env, "helm", "repo", "add", calicoHelmRepoName, calicoHelmRepo); err != nil {
		return fmt.Errorf("add Calico Helm repo: %w", err)
	}

	// Resolve the override values file path from the GitOps repository
	valuesPath, err := resolveOpenStackCalicoValuesPath(cfg)
	if err != nil {
		return err
	}

	// Verify the values file exists
	if _, err := os.Stat(valuesPath); err != nil {
		return fmt.Errorf("calico helm values file not found at %s: %w", valuesPath, err)
	}

	// Install/upgrade Calico using Helm
	if _, err := p.runner.Run(ctx, tmpDir, env, "helm",
		"upgrade", "--install",
		selection.ReleaseName,
		calicoHelmChart,
		"--namespace", selection.Namespace,
		"--create-namespace",
		"-f", valuesPath,
	); err != nil {
		return fmt.Errorf("helm install Calico %s: %w", selection.Version, err)
	}
	return nil
}

// resolveOpenStackCalicoValuesPath builds the path to the Calico Helm override
// values file within the GitOps repository.
func resolveOpenStackCalicoValuesPath(cfg *v2.Config) (string, error) {
	gitDir := cfg.GitDir()
	if gitDir == "" {
		return "", fmt.Errorf("gitops.git_dir must be configured for Calico Helm install")
	}
	organization := cfg.Organization()
	if organization == "" {
		return "", fmt.Errorf("meta.organization must be configured for Calico Helm install")
	}
	clusterName := cfg.ClusterName()
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set for Calico Helm install")
	}

	return filepath.Join(gitDir, organization, "applications", "overlays", clusterName, "services", "calico", "helm-values", "override_values.yaml"), nil
}

func (p *openstackBootstrapProvider) installOpenStackNetworkPluginWithHelm(ctx context.Context, cfg *v2.Config, selection openStackNetworkPluginSelection, kubeconfigPath, tmpDir string, env map[string]string) error {
	valuesPath, err := writeOpenStackNetworkPluginValues(cfg, selection, tmpDir)
	if err != nil {
		return err
	}

	switch selection.Name {
	case "cilium", "kube-ovn":
		if _, err := p.runner.Run(ctx, tmpDir, env, "helm", "upgrade", "--install", selection.ReleaseName, selection.Chart, "--namespace", selection.Namespace, "--version", selection.Version, "--values", valuesPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported OpenStack network plugin %q", selection.Name)
	}

	return nil
}

func (p *openstackBootstrapProvider) installOpenStackNetworkPluginWithKustomizeHelm(ctx context.Context, cfg *v2.Config, selection openStackNetworkPluginSelection, kubeconfigPath, tmpDir string, env map[string]string) error {
	overlayDir := filepath.Join(tmpDir, selection.Name)
	if err := os.MkdirAll(overlayDir, 0o755); err != nil {
		return fmt.Errorf("create %s Kustomize overlay: %w", selection.Name, err)
	}
	if _, err := writeOpenStackNetworkPluginValues(cfg, selection, overlayDir); err != nil {
		return err
	}
	if err := writeOpenStackNetworkPluginKustomization(selection, overlayDir); err != nil {
		return err
	}

	rendered, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "kustomize", "--enable-helm", overlayDir)...)
	if err != nil {
		return err
	}
	renderedPath := filepath.Join(tmpDir, selection.Name+"-rendered.yaml")
	if err := os.WriteFile(renderedPath, rendered, 0o600); err != nil {
		return fmt.Errorf("write rendered %s manifests: %w", selection.Name, err)
	}
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "apply", "-f", renderedPath)...); err != nil {
		return err
	}
	return nil
}

func (p *openstackBootstrapProvider) waitForOpenStackNetworkPlugin(ctx context.Context, selection openStackNetworkPluginSelection, kubeconfigPath, tmpDir string, env map[string]string) error {
	switch selection.Name {
	case "calico":
		if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", "tigera-operator", "rollout", "status", "deployment/tigera-operator", "--timeout=5m")...); err != nil {
			return err
		}
		if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=create", "tigerastatus/calico", "--timeout=5m")...); err != nil {
			return err
		}
		if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=condition=Available", "tigerastatus/calico", "--timeout=10m")...); err != nil {
			return err
		}
		_, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", "calico-system", "wait", "--for=condition=Ready", "pods", "--all", "--timeout=10m")...)
		return err
	case "cilium":
		if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "ds/cilium", "--timeout=10m")...); err != nil {
			return err
		}
		_, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "deploy/cilium-operator", "--timeout=10m")...)
		return err
	case "kube-ovn":
		_, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "wait", "--for=condition=Ready", "pods", "-l", "app.kubernetes.io/part-of=kube-ovn", "--timeout=10m")...)
		return err
	default:
		return fmt.Errorf("unsupported OpenStack network plugin %q", selection.Name)
	}
}

func selectOpenStackNetworkPlugin(cfg *v2.Config) (openStackNetworkPluginSelection, error) {
	if cfg == nil {
		return openStackNetworkPluginSelection{}, fmt.Errorf("configuration is nil")
	}

	var enabled []openStackNetworkPluginSelection
	plugins := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin
	if plugins.Calico != nil && plugins.Calico.Enabled {
		selection, err := openStackCalicoSelection(plugins.Calico)
		if err != nil {
			return openStackNetworkPluginSelection{}, err
		}
		enabled = append(enabled, selection)
	}
	if plugins.Cilium != nil && plugins.Cilium.Enabled {
		enabled = append(enabled, openStackCiliumSelection(plugins.Cilium))
	}
	if plugins.KubeOVN != nil && plugins.KubeOVN.Enabled {
		enabled = append(enabled, openStackKubeOVNSelection(plugins.KubeOVN))
	}

	switch len(enabled) {
	case 0:
		return openStackNetworkPluginSelection{}, fmt.Errorf("exactly one network plugin must be enabled at opencenter.cluster.kubernetes.network_plugin")
	case 1:
		selection := enabled[0]
		switch selection.InstallMethod {
		case openStackNetworkPluginMethodHelm, openStackNetworkPluginMethodKustomizeHelm:
			return selection, nil
		case "kubespray":
			return openStackNetworkPluginSelection{}, fmt.Errorf("OpenStack no longer installs network plugins with kubespray for %s; use %q or %q", selection.Name, openStackNetworkPluginMethodHelm, openStackNetworkPluginMethodKustomizeHelm)
		default:
			return openStackNetworkPluginSelection{}, fmt.Errorf("unsupported OpenStack network plugin install_method %q for %s; use %q or %q", selection.InstallMethod, selection.Name, openStackNetworkPluginMethodHelm, openStackNetworkPluginMethodKustomizeHelm)
		}
	default:
		names := make([]string, 0, len(enabled))
		for _, plugin := range enabled {
			names = append(names, plugin.Name)
		}
		return openStackNetworkPluginSelection{}, fmt.Errorf("only one network plugin may be enabled at opencenter.cluster.kubernetes.network_plugin; enabled: %s", strings.Join(names, ", "))
	}
}

func openStackCalicoSelection(calico *v2.CalicoConfig) (openStackNetworkPluginSelection, error) {
	version := strings.TrimSpace(calico.Version)
	if version == "" {
		version = defaultCalicoChartVersion
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return openStackNetworkPluginSelection{
		Name:          "calico",
		InstallMethod: normalizeOpenStackNetworkPluginInstallMethod(calico.InstallMethod),
		Version:       version,
		Namespace:     "tigera-operator",
		ReleaseName:   "calico",
		Chart:         calicoHelmChart,
		Repo:          calicoHelmRepo,
		ChartName:     "tigera-operator",
	}, nil
}

func openStackCiliumSelection(cilium *v2.CiliumConfig) openStackNetworkPluginSelection {
	version := strings.TrimPrefix(strings.TrimSpace(cilium.Version), "v")
	if version == "" {
		version = defaultCiliumChartVersion
	}
	return openStackNetworkPluginSelection{
		Name:          "cilium",
		InstallMethod: normalizeOpenStackNetworkPluginInstallMethod(cilium.InstallMethod),
		Version:       version,
		Namespace:     "kube-system",
		ReleaseName:   "cilium",
		Chart:         "oci://quay.io/cilium/charts/cilium",
		Repo:          "oci://quay.io/cilium/charts",
		ChartName:     "cilium",
	}
}

func openStackKubeOVNSelection(kubeOVN *v2.KubeOVNConfig) openStackNetworkPluginSelection {
	version := strings.TrimSpace(kubeOVN.Version)
	if version == "" {
		version = defaultKubeOVNChartVersion
	}
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	return openStackNetworkPluginSelection{
		Name:          "kube-ovn",
		InstallMethod: normalizeOpenStackNetworkPluginInstallMethod(kubeOVN.InstallMethod),
		Version:       version,
		Namespace:     "kube-system",
		ReleaseName:   "kube-ovn",
		Chart:         "oci://ghcr.io/kubeovn/charts/kube-ovn-v2",
		Repo:          "oci://ghcr.io/kubeovn/charts",
		ChartName:     "kube-ovn-v2",
	}
}

func normalizeOpenStackNetworkPluginInstallMethod(method string) string {
	method = strings.ToLower(strings.TrimSpace(method))
	if method == "" {
		return openStackNetworkPluginMethodHelm
	}
	return method
}

func openStackNetworkPluginPlanCommands(selection openStackNetworkPluginSelection, kubeconfigPath string) []BootstrapPlanCommand {
	if selection.Name == "calico" {
		commands := []BootstrapPlanCommand{
			commandPlan("helm", "repo", "add", calicoHelmRepoName, calicoHelmRepo),
			commandPlan("helm", "upgrade", "--install", selection.ReleaseName, calicoHelmChart, "--namespace", selection.Namespace, "--create-namespace", "-f", "<organization>/applications/overlays/<cluster>/services/calico/helm-values/override_values.yaml"),
		}
		return append(commands, openStackNetworkPluginReadinessPlanCommands(selection, kubeconfigPath)...)
	}
	switch selection.InstallMethod {
	case openStackNetworkPluginMethodKustomizeHelm:
		commands := []BootstrapPlanCommand{
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "kustomize", "--enable-helm", "<generated overlay>")...),
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "apply", "-f", "<rendered manifests>")...),
		}
		return append(commands, openStackNetworkPluginReadinessPlanCommands(selection, kubeconfigPath)...)
	default:
		switch selection.Name {
		case "cilium":
			return []BootstrapPlanCommand{
				commandPlan("helm", "upgrade", "--install", "cilium", "oci://quay.io/cilium/charts/cilium", "--namespace", "kube-system", "--version", selection.Version, "--values", "<generated values.yaml>"),
				commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "ds/cilium", "--timeout=10m")...),
				commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "deploy/cilium-operator", "--timeout=10m")...),
			}
		case "kube-ovn":
			return []BootstrapPlanCommand{
				commandPlan("helm", "upgrade", "--install", "kube-ovn", "oci://ghcr.io/kubeovn/charts/kube-ovn-v2", "--namespace", "kube-system", "--version", selection.Version, "--values", "<generated values.yaml>"),
				commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "wait", "--for=condition=Ready", "pods", "-l", "app.kubernetes.io/part-of=kube-ovn", "--timeout=10m")...),
			}
		default:
			return nil
		}
	}
}

func openStackNetworkPluginReadinessPlanCommands(selection openStackNetworkPluginSelection, kubeconfigPath string) []BootstrapPlanCommand {
	switch selection.Name {
	case "calico":
		return []BootstrapPlanCommand{
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "tigera-operator", "rollout", "status", "deployment/tigera-operator", "--timeout=5m")...),
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=create", "tigerastatus/calico", "--timeout=5m")...),
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=condition=Available", "tigerastatus/calico", "--timeout=10m")...),
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "calico-system", "wait", "--for=condition=Ready", "pods", "--all", "--timeout=10m")...),
		}
	case "cilium":
		return []BootstrapPlanCommand{
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "ds/cilium", "--timeout=10m")...),
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "rollout", "status", "deploy/cilium-operator", "--timeout=10m")...),
		}
	case "kube-ovn":
		return []BootstrapPlanCommand{
			commandPlan("kubectl", kubectlArgs(kubeconfigPath, "-n", "kube-system", "wait", "--for=condition=Ready", "pods", "-l", "app.kubernetes.io/part-of=kube-ovn", "--timeout=10m")...),
		}
	default:
		return nil
	}
}

func writeOpenStackNetworkPluginValues(cfg *v2.Config, selection openStackNetworkPluginSelection, dir string) (string, error) {
	values, err := openStackNetworkPluginValues(cfg, selection)
	if err != nil {
		return "", err
	}
	data, err := yaml.Marshal(values)
	if err != nil {
		return "", fmt.Errorf("marshal %s Helm values: %w", selection.Name, err)
	}
	path := filepath.Join(dir, "values.yaml")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write %s Helm values: %w", selection.Name, err)
	}
	return path, nil
}

func openStackNetworkPluginValues(cfg *v2.Config, selection openStackNetworkPluginSelection) (map[string]any, error) {
	k8s := cfg.OpenCenter.Cluster.Kubernetes
	switch selection.Name {
	case "cilium":
		cilium := k8s.NetworkPlugin.Cilium
		values := map[string]any{
			"ipam": map[string]any{
				"mode": "cluster-pool",
				"operator": map[string]any{
					"clusterPoolIPv4PodCIDRList": []string{k8s.SubnetPods},
				},
			},
		}
		if cilium != nil {
			switch strings.ToLower(strings.TrimSpace(cilium.TunnelMode)) {
			case "vxlan", "geneve":
				values["routingMode"] = "tunnel"
				values["tunnelProtocol"] = strings.ToLower(strings.TrimSpace(cilium.TunnelMode))
			case "disabled":
				values["routingMode"] = "native"
			}
			if cilium.Hubble {
				values["hubble"] = map[string]any{
					"enabled": true,
					"relay":   map[string]any{"enabled": true},
					"ui":      map[string]any{"enabled": true},
				}
			}
			if !cilium.NetworkPolicy {
				values["policyEnforcementMode"] = "never"
			}
		}
		return values, nil
	case "kube-ovn":
		kubeOVN := k8s.NetworkPlugin.KubeOVN
		networkPolicyEnforcement := "standard"
		if kubeOVN != nil && !kubeOVN.NetworkPolicy {
			networkPolicyEnforcement = "lax"
		}
		return map[string]any{
			"networkPolicies": map[string]any{
				"enforcement": networkPolicyEnforcement,
			},
			"networking": map[string]any{
				"stack": "IPv4",
				"pods": map[string]any{
					"cidr": map[string]any{
						"v4": k8s.SubnetPods,
					},
					"gateways": map[string]any{
						"v4": firstAddressInCIDR(k8s.SubnetPods),
					},
				},
				"services": map[string]any{
					"cidr": map[string]any{
						"v4": k8s.SubnetServices,
					},
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported OpenStack network plugin %q", selection.Name)
	}
}

func writeOpenStackNetworkPluginKustomization(selection openStackNetworkPluginSelection, overlayDir string) error {
	kustomization := map[string]any{
		"apiVersion": "kustomize.config.k8s.io/v1beta1",
		"kind":       "Kustomization",
	}
	kustomization["helmCharts"] = openStackNetworkPluginHelmCharts(selection)

	data, err := yaml.Marshal(kustomization)
	if err != nil {
		return fmt.Errorf("marshal %s Kustomize overlay: %w", selection.Name, err)
	}
	if err := os.WriteFile(filepath.Join(overlayDir, "kustomization.yaml"), data, 0o600); err != nil {
		return fmt.Errorf("write %s Kustomize overlay: %w", selection.Name, err)
	}
	return nil
}

func openStackNetworkPluginHelmCharts(selection openStackNetworkPluginSelection) []map[string]any {
	return []map[string]any{
		{
			"name":        selection.ChartName,
			"repo":        selection.Repo,
			"version":     selection.Version,
			"releaseName": selection.ReleaseName,
			"namespace":   selection.Namespace,
			"valuesFile":  "values.yaml",
		},
	}
}

func firstAddressInCIDR(raw string) string {
	prefix, err := netip.ParsePrefix(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	addr := prefix.Addr()
	if !addr.Is4() {
		return ""
	}
	next := addr.Next()
	if !prefix.Contains(next) {
		return ""
	}
	return next.String()
}

func kubectlArgs(kubeconfigPath string, args ...string) []string {
	if strings.TrimSpace(kubeconfigPath) == "" {
		return append([]string(nil), args...)
	}
	withKubeconfig := []string{"--kubeconfig", kubeconfigPath}
	withKubeconfig = append(withKubeconfig, args...)
	return withKubeconfig
}
