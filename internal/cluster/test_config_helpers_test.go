package cluster

import v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"

func mustNewClusterTestConfig(name, provider string) v2.Config {
	cfg, err := v2.NewV2Default(name, provider)
	if err != nil {
		panic(err)
	}
	return *cfg
}

func applyClusterProviderDefaults(cfg *v2.Config, provider string) error {
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
