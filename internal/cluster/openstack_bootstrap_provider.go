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

func (p *openstackBootstrapProvider) BuildSteps(cfg *v2.Config, _ *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}
	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for openstack bootstrap")
	}

	if _, err := os.Stat(clusterDir); err != nil {
		return nil, fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
	}

	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}

	env := buildBootstrapEnvironment(opts.KubeconfigPath)
	mergeBootstrapEnvironment(env, creds.ToEnvMap())

	openTofuPath := strings.TrimSpace(cfg.OpenTofu.Path)
	if openTofuPath == "" {
		openTofuPath = "opentofu"
	}

	return []bootstrapStep{
		{
			ID:          "openstack-preflight",
			Description: "Validate OpenStack credentials and bootstrap prerequisites",
			Run: func(ctx context.Context) error {
				return validateOpenStackBootstrap(creds)
			},
		},
		{
			ID:          "opentofu-init",
			Description: "Initialize OpenTofu",
			Run: func(ctx context.Context) error {
				_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
				return err
			},
		},
		{
			ID:          "opentofu-apply",
			Description: "Apply OpenTofu infrastructure",
			Run: func(ctx context.Context) error {
				_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "apply", "-auto-approve")
				return err
			},
		},
		{
			ID:          "openstack-normalize-kubeconfig",
			Description: "Normalize kubeconfig into the cluster-owned path",
			Run: func(ctx context.Context) error {
				return normalizeOpenStackKubeconfig(clusterDir, opts.KubeconfigPath)
			},
		},
	}, nil
}

func validateOpenStackBootstrap(creds *credentials.OpenStackCredentials) error {
	if creds == nil || creds.IsEmpty() {
		return fmt.Errorf("openstack credentials are incomplete; set auth_url and application credentials or username/password before bootstrap")
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

func normalizeOpenStackKubeconfig(clusterDir, targetPath string) error {
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

	if sameFilePath(sourcePath, targetPath) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}

	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return fmt.Errorf("read kubeconfig %s: %w", sourcePath, err)
	}

	if err := os.WriteFile(targetPath, data, 0o600); err != nil {
		return fmt.Errorf("write kubeconfig %s: %w", targetPath, err)
	}

	return nil
}

func sameFilePath(a, b string) bool {
	if a == "" || b == "" {
		return false
	}

	absA, err := filepath.Abs(a)
	if err != nil {
		return a == b
	}
	absB, err := filepath.Abs(b)
	if err != nil {
		return a == b
	}

	return absA == absB
}
