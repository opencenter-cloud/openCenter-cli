package cluster

import (
	"context"
	"encoding/json"
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
		return nil, fmt.Errorf("opentofu must be enabled for bootstrap")
	}

	openTofuPath, err := resolveTofuBinary(cfg.OpenTofu.Path)
	if err != nil {
		return nil, err
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	planEnv := providerPlanEnv(provider, opts.KubeconfigPath)

	steps := []bootstrapStep{
		p.buildPreflightStep(cfg, provider, clusterDir),
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
				env, err := buildProviderBootstrapEnvironment(cfg, opts.KubeconfigPath)
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
				Writes:      []string{"infrastructure resources", filepath.Join(clusterDir, "terraform.tfstate")},
				Notes:       []string{"Plan only; API access and infrastructure changes were not simulated."},
			},
			Run: func(ctx context.Context) error {
				env, err := buildProviderBootstrapEnvironment(cfg, opts.KubeconfigPath)
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

	// Flux bootstrap runs after the CNI is installed so the cluster is
	// network-ready when FluxCD source-controller starts reconciling.
	// Only add the step when token auth is configured and a real (non-placeholder)
	// repository URL is present.
	if cfg.OpenCenter.GitOps.Auth.Token != nil &&
		strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider) != "" &&
		cfg.ConfiguredGitURL() != "" {
		fluxStep, err := p.buildFluxBootstrapStep(cfg, clusterDir, planEnv, opts)
		if err != nil {
			return nil, fmt.Errorf("building flux bootstrap step: %w", err)
		}
		steps = append(steps, fluxStep)
	}

	// No automatic git push — the user commits and pushes manually after deploy.

	return steps, nil
}

func extractOpenStackBootstrapCredentials(cfg *v2.Config) (*credentials.OpenStackCredentials, error) {
	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}
	return creds, nil
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

// buildPreflightStep returns a provider-aware preflight validation step.
func (p *openstackBootstrapProvider) buildPreflightStep(cfg *v2.Config, provider, clusterDir string) bootstrapStep {
	return bootstrapStep{
		ID:          "preflight",
		Description: "Validate credentials and bootstrap prerequisites",
		Plan: BootstrapPlanStep{
			ID:         "preflight",
			Action:     "Validate credentials and bootstrap prerequisites",
			WorkingDir: clusterDir,
			Reads:      []string{clusterDir},
			Notes:      []string{"Plan only; credentials, infrastructure directory, and OpenTofu availability were not checked."},
		},
		Run: func(ctx context.Context) error {
			if _, err := os.Stat(clusterDir); err != nil {
				return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
			}
			if err := validateProviderBootstrap(cfg, provider); err != nil {
				return err
			}
			return p.checkBastionSSH(ctx, cfg)
		},
	}
}

// validateProviderBootstrap performs provider-specific preflight validation.
func validateProviderBootstrap(cfg *v2.Config, provider string) error {
	switch provider {
	case "openstack":
		creds, err := extractOpenStackBootstrapCredentials(cfg)
		if err != nil {
			return err
		}
		return validateOpenStackBootstrap(creds)
	case "vmware", "vsphere":
		secret := extractVSphereBootstrapCredentials(cfg)
		if isEmptyOrPlaceholder(secret.VCenterHost) || isEmptyOrPlaceholder(secret.Username) || isEmptyOrPlaceholder(secret.Password) {
			return fmt.Errorf("vmware credentials incomplete; set secrets.vsphere_csi (vcenter_host, username, password)")
		}
		return validateStaticNodes(cfg)
	case "baremetal":
		return validateStaticNodes(cfg)
	default:
		return fmt.Errorf("unsupported provider %q for bootstrap", provider)
	}
}

// validateStaticNodes checks that pre-provisioned nodes are defined in the config.
func validateStaticNodes(cfg *v2.Config) error {
	compute := cfg.OpenCenter.Infrastructure.Compute
	hasNodes := len(compute.MasterNodes) > 0

	// Also accept vmware cloud nodes as a source.
	if !hasNodes && cfg.OpenCenter.Infrastructure.Cloud.VMware != nil {
		// VMware nodes are defined in cloud.vmware.nodes via the schema;
		// the Terraform module reads them directly from the config.
		hasNodes = true
	}

	if !hasNodes {
		return fmt.Errorf("no master nodes defined; set infrastructure.compute.master_nodes for static node deployment")
	}

	if strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.User) == "" &&
		strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.Username) == "" {
		return fmt.Errorf("ssh user must be set for static node deployment; set infrastructure.ssh.user")
	}
	return nil
}

// checkBastionSSH verifies SSH connectivity through the bastion host.
// It SSHs to the bastion and confirms it can reach localhost via SSH.
// Skipped when bastion is not enabled or has no address.
func (p *openstackBootstrapProvider) checkBastionSSH(ctx context.Context, cfg *v2.Config) error {
	bastion := cfg.OpenCenter.Infrastructure.Bastion
	if !bastion.Enabled {
		return nil
	}
	bastionAddr := strings.TrimSpace(bastion.Address)
	if bastionAddr == "" {
		return nil
	}

	sshUser := strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.Username)
	if sshUser == "" {
		sshUser = strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.User)
	}
	if sshUser == "" {
		sshUser = "ubuntu"
	}

	keyPath := strings.TrimSpace(cfg.OpenCenter.Infrastructure.SSH.KeyPath)

	// Build SSH args for bastion connectivity check
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "ConnectTimeout=10",
		"-o", "BatchMode=yes",
	}
	if keyPath != "" {
		args = append(args, "-i", keyPath)
	}
	args = append(args, fmt.Sprintf("%s@%s", sshUser, bastionAddr), "ssh", "-o", "StrictHostKeyChecking=no", "-o", "ConnectTimeout=5", "-o", "BatchMode=yes", "localhost", "true")

	_, err := p.runner.Run(ctx, "", nil, "ssh", args...)
	if err != nil {
		return fmt.Errorf("bastion SSH check failed: cannot SSH to localhost from bastion %s: %w", bastionAddr, err)
	}
	return nil
}

// buildProviderBootstrapEnvironment returns environment variables for the
// OpenTofu run, selecting credentials based on the infrastructure provider.
func buildProviderBootstrapEnvironment(cfg *v2.Config, kubeconfigPath string) (map[string]string, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	env := buildBootstrapEnvironment(kubeconfigPath)

	switch provider {
	case "openstack":
		creds, err := extractOpenStackBootstrapCredentials(cfg)
		if err != nil {
			return nil, err
		}
		mergeBootstrapEnvironment(env, creds.ToEnvMap())
	case "vmware", "vsphere":
		secret := extractVSphereBootstrapCredentials(cfg)
		if strings.TrimSpace(secret.VCenterHost) != "" {
			env["VSPHERE_SERVER"] = secret.VCenterHost
		}
		if strings.TrimSpace(secret.Username) != "" {
			env["VSPHERE_USER"] = secret.Username
		}
		if strings.TrimSpace(secret.Password) != "" {
			env["VSPHERE_PASSWORD"] = secret.Password
		}
		if strings.TrimSpace(secret.InsecureFlag) != "" {
			env["VSPHERE_ALLOW_UNVERIFIED_SSL"] = secret.InsecureFlag
		}
	case "baremetal":
		// No extra credentials needed.
	}

	return env, nil
}

// vSphereBootstrapSecret holds credentials extracted from the cluster config.
type vSphereBootstrapSecret struct {
	VCenterHost  string `json:"vcenter_host" yaml:"vcenter_host"`
	Username     string `json:"username" yaml:"username"`
	Password     string `json:"password" yaml:"password"`
	InsecureFlag string `json:"insecure_flag" yaml:"insecure_flag"`
}

// extractVSphereBootstrapCredentials extracts vSphere credentials from the
// secrets.vsphere_csi or secrets.vsphere-csi config block, falling back to
// the typed secrets.VSphereCsi struct and infrastructure.cloud.vmware for the host.
func extractVSphereBootstrapCredentials(cfg *v2.Config) vSphereBootstrapSecret {
	var secret vSphereBootstrapSecret
	for _, key := range []string{"vsphere_csi", "vsphere-csi"} {
		raw, ok := cfg.Secrets.ServiceSecrets[key]
		if !ok || raw == nil {
			continue
		}
		data, err := json.Marshal(raw)
		if err != nil {
			continue
		}
		if err := json.Unmarshal(data, &secret); err == nil {
			break
		}
	}

	// Fall back to the typed secrets.vsphere_csi struct fields.
	if strings.TrimSpace(secret.VCenterHost) == "" {
		secret.VCenterHost = strings.TrimSpace(cfg.Secrets.VSphereCsi.VCenterHost)
	}
	if strings.TrimSpace(secret.Username) == "" {
		secret.Username = strings.TrimSpace(cfg.Secrets.VSphereCsi.Username)
	}
	if strings.TrimSpace(secret.Password) == "" {
		secret.Password = cfg.Secrets.VSphereCsi.Password
	}
	if strings.TrimSpace(secret.InsecureFlag) == "" {
		secret.InsecureFlag = strings.TrimSpace(cfg.Secrets.VSphereCsi.InsecureFlag)
	}

	// Fall back to cloud.vmware.vcenter_server if still no host.
	if strings.TrimSpace(secret.VCenterHost) == "" {
		if vmwareCfg := cfg.OpenCenter.Infrastructure.Cloud.VMware; vmwareCfg != nil {
			secret.VCenterHost = strings.TrimSpace(vmwareCfg.VCenterServer)
		}
	}

	// Normalize: strip https:// scheme from vcenter_host so VSPHERE_SERVER
	// receives a bare hostname. Users may paste a full URL.
	secret.VCenterHost = normalizeVCenterHost(secret.VCenterHost)

	return secret
}

// normalizeVCenterHost strips an https:// or http:// prefix from a vCenter
// host value, returning just the hostname (with port if present). This allows
// users to set secrets.vsphere_csi.vcenter_host to either
// "vcenter.example.com" or "https://vcenter.example.com".
func normalizeVCenterHost(host string) string {
	host = strings.TrimSpace(host)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(strings.ToLower(host), prefix) {
			host = host[len(prefix):]
			// Strip trailing path (e.g. "/sdk")
			if idx := strings.Index(host, "/"); idx != -1 {
				host = host[:idx]
			}
			break
		}
	}
	return host
}

// isEmptyOrPlaceholder returns true if the value is empty or still set to the
// default placeholder ("CHANGEME") that must be replaced before deployment.
func isEmptyOrPlaceholder(value string) bool {
	v := strings.TrimSpace(value)
	return v == "" || strings.EqualFold(v, "CHANGEME")
}

// providerPlanEnv returns the dry-run plan environment for the given provider.
func providerPlanEnv(provider, kubeconfigPath string) []BootstrapPlanEnv {
	switch provider {
	case "vmware", "vsphere":
		env := map[string]string{
			"VSPHERE_SERVER":             "",
			"VSPHERE_USER":              "",
			"VSPHERE_PASSWORD":          "",
			"VSPHERE_ALLOW_UNVERIFIED_SSL": "",
			"PATH":                      "<current PATH>",
		}
		if strings.TrimSpace(kubeconfigPath) != "" {
			env["KUBECONFIG"] = kubeconfigPath
		}
		redacted := map[string]bool{
			"VSPHERE_SERVER":   true,
			"VSPHERE_USER":     true,
			"VSPHERE_PASSWORD": true,
		}
		return envPlanFromMap(env, redacted)
	case "baremetal":
		env := map[string]string{
			"PATH": "<current PATH>",
		}
		if strings.TrimSpace(kubeconfigPath) != "" {
			env["KUBECONFIG"] = kubeconfigPath
		}
		return envPlanFromMap(env, nil)
	default:
		return openStackPlanEnv(kubeconfigPath)
	}
}
