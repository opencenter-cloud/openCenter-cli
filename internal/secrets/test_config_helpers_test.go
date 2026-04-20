package secrets

import (
	"os"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

func newSecretsTestConfig(clusterName, provider string) *v2.Config {
	cfg, err := v2.NewV2Default(clusterName, provider)
	if err != nil {
		panic(err)
	}
	return cfg
}

type partialSecretsTestConfig struct {
	OpenCenter struct {
		Meta struct {
			Name         string `yaml:"name"`
			Organization string `yaml:"organization"`
		} `yaml:"meta"`
		Cluster struct {
			ClusterName string `yaml:"cluster_name"`
		} `yaml:"cluster"`
		GitOps struct {
			GitDir string `yaml:"git_dir"`
		} `yaml:"gitops"`
		Infrastructure struct {
			Provider string `yaml:"provider"`
		} `yaml:"infrastructure"`
	} `yaml:"opencenter"`
	Secrets struct {
		SopsAgeKeyFile string                `yaml:"sops_age_key_file"`
		SSHPrivateKey  string                `yaml:"ssh_private_key_file"`
		SSHPublicKey   string                `yaml:"ssh_public_key_file"`
		CertManager    v2.CertManagerSecrets `yaml:"cert_manager"`
		Keycloak       v2.KeycloakSecrets    `yaml:"keycloak"`
	} `yaml:"secrets"`
}

func normalizeSecretsConfigYAML(t *testing.T, clusterName, raw string) []byte {
	t.Helper()

	var partial partialSecretsTestConfig
	if err := yaml.Unmarshal([]byte(raw), &partial); err != nil {
		t.Fatalf("unmarshal partial config: %v", err)
	}

	name := clusterName
	if partial.OpenCenter.Cluster.ClusterName != "" {
		name = partial.OpenCenter.Cluster.ClusterName
	}
	provider := partial.OpenCenter.Infrastructure.Provider
	if provider == "" {
		provider = "openstack"
	}

	cfg := newSecretsTestConfig(name, provider)
	if partial.OpenCenter.Meta.Name != "" {
		cfg.OpenCenter.Meta.Name = partial.OpenCenter.Meta.Name
	}
	if partial.OpenCenter.Meta.Organization != "" {
		cfg.OpenCenter.Meta.Organization = partial.OpenCenter.Meta.Organization
	}
	if partial.OpenCenter.GitOps.Repository.LocalDir != "" {
		cfg.OpenCenter.GitOps.Repository.LocalDir = partial.OpenCenter.GitOps.Repository.LocalDir
	}
	if partial.Secrets.SopsAgeKeyFile != "" {
		cfg.Secrets.SopsAgeKeyFile = partial.Secrets.SopsAgeKeyFile
	}
	if partial.Secrets.SSHPrivateKey != "" {
		cfg.Secrets.SSHKey.Private = partial.Secrets.SSHPrivateKey
	}
	if partial.Secrets.SSHPublicKey != "" {
		cfg.Secrets.SSHKey.Public = partial.Secrets.SSHPublicKey
	}
	if partial.Secrets.CertManager != (v2.CertManagerSecrets{}) {
		cfg.Secrets.CertManager = partial.Secrets.CertManager
	}
	if partial.Secrets.Keycloak != (v2.KeycloakSecrets{}) {
		cfg.Secrets.Keycloak = partial.Secrets.Keycloak
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal normalized config: %v", err)
	}
	return data
}

func writeNormalizedSecretsConfigFile(t *testing.T, path, clusterName, raw string) {
	t.Helper()

	data := normalizeSecretsConfigYAML(t, clusterName, raw)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write normalized config: %v", err)
	}
}

func normalizeSecretsConfigYAMLBytes(clusterName, raw string) ([]byte, error) {
	var partial partialSecretsTestConfig
	if err := yaml.Unmarshal([]byte(raw), &partial); err != nil {
		return nil, err
	}

	name := clusterName
	if partial.OpenCenter.Cluster.ClusterName != "" {
		name = partial.OpenCenter.Cluster.ClusterName
	}
	provider := partial.OpenCenter.Infrastructure.Provider
	if provider == "" {
		provider = "openstack"
	}

	cfg := newSecretsTestConfig(name, provider)
	if partial.OpenCenter.Meta.Name != "" {
		cfg.OpenCenter.Meta.Name = partial.OpenCenter.Meta.Name
	}
	if partial.OpenCenter.Meta.Organization != "" {
		cfg.OpenCenter.Meta.Organization = partial.OpenCenter.Meta.Organization
	}
	if partial.OpenCenter.GitOps.Repository.LocalDir != "" {
		cfg.OpenCenter.GitOps.Repository.LocalDir = partial.OpenCenter.GitOps.Repository.LocalDir
	}
	if partial.Secrets.SopsAgeKeyFile != "" {
		cfg.Secrets.SopsAgeKeyFile = partial.Secrets.SopsAgeKeyFile
	}
	if partial.Secrets.SSHPrivateKey != "" {
		cfg.Secrets.SSHKey.Private = partial.Secrets.SSHPrivateKey
	}
	if partial.Secrets.SSHPublicKey != "" {
		cfg.Secrets.SSHKey.Public = partial.Secrets.SSHPublicKey
	}
	if partial.Secrets.CertManager != (v2.CertManagerSecrets{}) {
		cfg.Secrets.CertManager = partial.Secrets.CertManager
	}
	if partial.Secrets.Keycloak != (v2.KeycloakSecrets{}) {
		cfg.Secrets.Keycloak = partial.Secrets.Keycloak
	}

	return yaml.Marshal(cfg)
}
