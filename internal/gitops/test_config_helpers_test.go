package gitops

import v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"

func mustNewGitOpsTestConfig(name, provider string) v2.Config {
	cfg, err := v2.NewV2Default(name, provider)
	if err != nil {
		panic(err)
	}

	return *cfg
}

func newDefault(name string) v2.Config {
	return mustNewGitOpsTestConfig(name, "openstack")
}

// useMetalLBTestOwnership selects MetalLB as the sole LoadBalancer controller
// for fixtures that exercise MetalLB while starting from OpenStack defaults.
func useMetalLBTestOwnership(cfg *v2.Config) {
	if service := cfg.OpenCenter.Services["openstack-ccm"]; service != nil {
		if base := extractBaseConfig(service); base != nil {
			base.Enabled = false
		}
	}
}

func applyProviderDefaults(cfg *v2.Config, provider string) error {
	next, err := v2.NewV2Default(cfg.ClusterName(), provider)
	if err != nil {
		return err
	}

	gitDir := cfg.OpenCenter.GitOps.Repository.LocalDir
	organization := cfg.OpenCenter.Meta.Organization
	*cfg = *next
	if gitDir != "" {
		cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	}
	if organization != "" {
		cfg.OpenCenter.Meta.Organization = organization
	}

	return nil
}
