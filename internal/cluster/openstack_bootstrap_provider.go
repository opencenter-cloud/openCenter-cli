package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	openstackprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type openstackBootstrapProvider struct {
	runner lifecycleCommandRunner
}

func newOpenStackBootstrapProvider(runner lifecycleCommandRunner) lifecycleBootstrapProvider {
	return &openstackBootstrapProvider{runner: runner}
}

func (p *openstackBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for openstack bootstrap")
	}

	openTofuPath := strings.TrimSpace(cfg.OpenTofu.Path)
	if openTofuPath == "" {
		openTofuPath = "opentofu"
	}
	planEnv := openStackPlanEnv(opts.KubeconfigPath)

	steps := []bootstrapStep{
		{
			ID:          "openstack-preflight",
			Description: "Validate OpenStack credentials and bootstrap prerequisites",
			Plan: BootstrapPlanStep{
				ID:         "openstack-preflight",
				Action:     "Validate OpenStack credentials and bootstrap prerequisites",
				WorkingDir: clusterDir,
				Reads:      []string{clusterDir},
				Notes:      []string{"Plan only; OpenStack credentials, infrastructure directory, and OpenTofu availability were not checked."},
			},
			Run: func(ctx context.Context) error {
				if _, err := os.Stat(clusterDir); err != nil {
					return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
				}
				creds, err := extractOpenStackBootstrapCredentials(cfg)
				if err != nil {
					return err
				}
				return validateOpenStackBootstrap(creds)
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
				Writes:      []string{"OpenStack infrastructure resources", filepath.Join(clusterDir, "terraform.tfstate")},
				Notes:       []string{"Plan only; OpenStack API access and infrastructure changes were not simulated."},
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
	}

	// DISABLED: kubespray steps (kubespray-venv-create, kubespray-pip-install,
	// kubespray-ansible-playbook) are intentionally skipped here.
	//
	// The OpenTofu module for this provider already embeds a
	// null_resource.run_kubespray with a local-exec provisioner that runs the
	// full Ansible/Kubespray playbook as part of opentofu-apply (step 3).
	// Appending these steps caused Ansible to run a second time against a
	// cluster that was already provisioned, wasting ~1h of deploy time.
	//
	// Long-term fix: remove the null_resource.run_kubespray local-exec from
	// the OpenTofu templates so that OpenTofu only provisions infrastructure
	// and these steps own the Ansible run exclusively.
	//
	// if cfg.Deployment.Method == "kubespray" {
	// 	kubespraySteps, err := p.buildKubespraySteps(cfg, clusterPaths, clusterDir, planEnv, opts)
	// 	if err != nil {
	// 		return nil, fmt.Errorf("building kubespray steps: %w", err)
	// 	}
	// 	steps = append(steps, kubespraySteps...)
	// }

	apiEndpointIP := resolveAPIEndpointIP(cfg)
	steps = append(steps, bootstrapStep{
		ID:          "openstack-normalize-kubeconfig",
		Description: "Normalize kubeconfig into the cluster-owned path and replace localhost with VIP",
		Plan: BootstrapPlanStep{
			ID:         "openstack-normalize-kubeconfig",
			Action:     "Normalize kubeconfig into the cluster-owned path and replace localhost with VIP",
			WorkingDir: clusterDir,
			Reads:      kubeconfigCandidatePaths(clusterDir, opts.KubeconfigPath),
			Writes:     []string{opts.KubeconfigPath},
			Notes:      []string{"Plan only; kubeconfig candidates were not checked."},
		},
		Run: func(ctx context.Context) error {
			return normalizeOpenStackKubeconfig(clusterDir, opts.KubeconfigPath, apiEndpointIP)
		},
	})

	networkPluginStep, err := p.buildNetworkPluginInstallStep(cfg, clusterDir, planEnv, opts)
	if err != nil {
		return nil, err
	}
	steps = append(steps, networkPluginStep)

	return steps, nil
}

// resolveVenvPath returns the Python virtual environment path for the cluster.
// It prefers ClusterPaths.VenvPath when available, falling back to a
// conventional path under the cluster infrastructure directory.
func resolveVenvPath(clusterPaths *paths.ClusterPaths, clusterDir string) string {
	if clusterPaths != nil && strings.TrimSpace(clusterPaths.VenvPath) != "" {
		return clusterPaths.VenvPath
	}
	return filepath.Join(clusterDir, "venv")
}

// buildKubespraySteps returns the bootstrap steps that create a Python venv,
// install Kubespray's requirements, and run ansible-playbook. Every Python
// and Ansible binary is called by its absolute venv path so that no shell
// "source activate" is needed.
func (p *openstackBootstrapProvider) buildKubespraySteps(
	cfg *v2.Config,
	clusterPaths *paths.ClusterPaths,
	clusterDir string,
	planEnv []BootstrapPlanEnv,
	opts *BootstrapOptions,
) ([]bootstrapStep, error) {
	venvDir := resolveVenvPath(clusterPaths, clusterDir)
	pipPath := filepath.Join(venvDir, "bin", "pip")
	ansiblePlaybookPath := filepath.Join(venvDir, "bin", "ansible-playbook")

	kubesprayDir := filepath.Join(clusterDir, "kubespray")
	inventoryFile := filepath.Join(clusterDir, "inventory", "inventory.yaml")
	requirementsFile := filepath.Join(kubesprayDir, "requirements.txt")

	return []bootstrapStep{
		{
			ID:          "kubespray-venv-create",
			Description: "Create Python virtual environment for Kubespray",
			Plan: BootstrapPlanStep{
				ID:         "kubespray-venv-create",
				Action:     "Create Python virtual environment for Kubespray",
				WorkingDir: clusterDir,
				Commands:   []BootstrapPlanCommand{commandPlan("python3", "-m", "venv", venvDir)},
				Writes:     []string{venvDir},
				Notes:      []string{"Plan only; Python 3 availability was not checked."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildOpenStackBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				_, runErr := p.runner.Run(ctx, clusterDir, env, "python3", "-m", "venv", venvDir)
				return runErr
			},
		},
		{
			ID:          "kubespray-pip-install",
			Description: "Install Kubespray requirements into virtual environment",
			Plan: BootstrapPlanStep{
				ID:          "kubespray-pip-install",
				Action:      "Install Kubespray requirements into virtual environment",
				WorkingDir:  clusterDir,
				Commands:    []BootstrapPlanCommand{commandPlan(pipPath, "install", "-r", requirementsFile)},
				Environment: planEnv,
				Reads:       []string{requirementsFile},
				Writes:      []string{filepath.Join(venvDir, "lib")},
				Notes:       []string{"Plan only; requirements.txt existence and network access were not checked."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildOpenStackBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				// Set VIRTUAL_ENV so pip and any post-install hooks see the
				// correct environment, matching the behavior of "source activate".
				env["VIRTUAL_ENV"] = venvDir
				_, runErr := p.runner.Run(ctx, clusterDir, env, pipPath, "install", "-r", requirementsFile)
				return runErr
			},
		},
		{
			ID:          "kubespray-ansible-playbook",
			Description: "Run Kubespray Ansible playbook to deploy the cluster",
			Plan: BootstrapPlanStep{
				ID:          "kubespray-ansible-playbook",
				Action:      "Run Kubespray Ansible playbook to deploy the cluster",
				WorkingDir:  kubesprayDir,
				Commands:    []BootstrapPlanCommand{commandPlan(ansiblePlaybookPath, "-i", inventoryFile, "cluster.yml", "-b")},
				Environment: planEnv,
				Reads:       []string{kubesprayDir, inventoryFile},
				Writes:      []string{"Kubernetes cluster nodes"},
				Notes:       []string{"Plan only; inventory, SSH access, and node connectivity were not checked."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildOpenStackBootstrapEnvironment(cfg, opts.KubeconfigPath)
				if err != nil {
					return err
				}
				env["VIRTUAL_ENV"] = venvDir
				// Prepend the venv bin directory to PATH so Ansible can find
				// its own helper binaries (e.g. ansible-connection).
				if existing, ok := env["PATH"]; ok && existing != "" {
					env["PATH"] = filepath.Join(venvDir, "bin") + string(os.PathListSeparator) + existing
				} else {
					env["PATH"] = filepath.Join(venvDir, "bin")
				}
				env["ANSIBLE_HOST_KEY_CHECKING"] = "False"
				_, runErr := p.runner.Run(ctx, kubesprayDir, env, ansiblePlaybookPath, "-i", inventoryFile, "cluster.yml", "-b")
				return runErr
			},
		},
	}, nil
}

func extractOpenStackBootstrapCredentials(cfg *v2.Config) (*credentials.OpenStackCredentials, error) {
	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}
	return creds, nil
}

func buildOpenStackBootstrapEnvironment(cfg *v2.Config, kubeconfigPath string) (map[string]string, error) {
	creds, err := extractOpenStackBootstrapCredentials(cfg)
	if err != nil {
		return nil, err
	}
	env := buildBootstrapEnvironment(kubeconfigPath)
	mergeBootstrapEnvironment(env, creds.ToEnvMap())
	return env, nil
}

func validateOpenStackBootstrap(creds *credentials.OpenStackCredentials) error {
	if creds == nil || creds.IsEmpty() {
		return fmt.Errorf("openstack credentials are incomplete; set auth_url and application credentials or username/password before bootstrap")
	}

	// Reject placeholder credentials
	if creds.ApplicationCredentialID == "CHANGEME" || creds.ApplicationCredentialSecret == "CHANGEME" {
		return fmt.Errorf("openstack credentials are incomplete; application_credential_id and application_credential_secret must be replaced before bootstrap")
	}

	for _, warning := range openstackprovider.PreflightOpenStack(creds.AuthURL) {
		if strings.Contains(warning, "auth_url is empty") {
			return fmt.Errorf("%s", warning)
		}
	}

	return nil
}

func buildBootstrapEnvironment(kubeconfigPath string) map[string]string {
	env := make(map[string]string)

	if strings.TrimSpace(kubeconfigPath) != "" {
		env["KUBECONFIG"] = kubeconfigPath
	}
	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}

func mergeBootstrapEnvironment(target, extra map[string]string) {
	for key, value := range extra {
		if strings.TrimSpace(value) == "" {
			continue
		}
		target[key] = value
	}
}

// resolveAPIEndpointIP returns the IP address that should replace localhost
// in the kubeconfig server URL. It prefers the explicit k8s_api_ip override
// and falls back to the VRRP VIP when VRRP is enabled.
func resolveAPIEndpointIP(cfg *v2.Config) string {
	if ip := strings.TrimSpace(cfg.OpenCenter.Infrastructure.K8sAPIIP); ip != "" {
		return ip
	}
	net := cfg.OpenCenter.Infrastructure.Networking
	if net.VRRPEnabled {
		return strings.TrimSpace(net.VRRPIP)
	}
	return ""
}

func infrastructureClusterDir(cfg *v2.Config) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}

	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" {
		return "", fmt.Errorf("gitops.git_dir must be configured for provider %q", cfg.Provider())
	}

	clusterName := strings.TrimSpace(cfg.ClusterName())
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set")
	}

	return filepath.Join(gitDir, "infrastructure", "clusters", clusterName), nil
}

func normalizeOpenStackKubeconfig(clusterDir, targetPath, apiEndpointIP string) error {
	if strings.TrimSpace(targetPath) == "" {
		return fmt.Errorf("kubeconfig path must be set")
	}

	candidates := []string{
		targetPath,
		filepath.Join(clusterDir, "kubeconfig.yaml"),
		filepath.Join(clusterDir, "kubeconfig"),
		filepath.Join(clusterDir, "kube_config_cluster.yml"),
	}

	var sourcePath string
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			sourcePath = candidate
			break
		}
	}

	if sourcePath == "" {
		return fmt.Errorf("kubeconfig not found after bootstrap in %s", clusterDir)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read kubeconfig %s: %w", sourcePath, err)
	}

	data = replaceLocalhostInKubeconfig(data, apiEndpointIP)

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig %s: %w", targetPath, err)
	}

	return nil
}

// replaceLocalhostInKubeconfig rewrites cluster server URLs that point to
// localhost (127.0.0.1 or ::1) so they use the cluster's VIP instead.
// This is necessary for OpenStack deployments where the bootstrap tooling
// (e.g. Kubespray) writes a kubeconfig with a localhost endpoint that is
// only reachable from the control-plane node itself.
//
// When apiEndpointIP is empty the data is returned unchanged.
func replaceLocalhostInKubeconfig(data []byte, apiEndpointIP string) []byte {
	if strings.TrimSpace(apiEndpointIP) == "" {
		return data
	}

	// Match server lines whose host portion is a localhost address.
	// Kubeconfig server values follow the pattern:
	//   server: https://<host>:<port>
	// We replace only the host, preserving scheme and port.
	localhostHosts := []string{"127.0.0.1", "localhost", "[::1]"}
	result := string(data)
	for _, host := range localhostHosts {
		// Replace https://<localhost>: with https://<vip>:
		result = strings.ReplaceAll(result,
			"https://"+host+":",
			"https://"+apiEndpointIP+":")
		// Also handle the less common http:// variant
		result = strings.ReplaceAll(result,
			"http://"+host+":",
			"http://"+apiEndpointIP+":")
	}
	return []byte(result)
}
