package v2

import (
	"encoding/json"
	"strings"
)

// ClusterName returns the cluster's canonical name.
func (c Config) ClusterName() string {
	if value := strings.TrimSpace(c.OpenCenter.Cluster.ClusterName); value != "" {
		return value
	}
	return strings.TrimSpace(c.OpenCenter.Meta.Name)
}

// Organization returns the cluster organization.
func (c Config) Organization() string {
	return strings.TrimSpace(c.OpenCenter.Meta.Organization)
}

// Provider returns the normalized infrastructure provider name.
func (c Config) Provider() string {
	return strings.TrimSpace(c.OpenCenter.Infrastructure.Provider)
}

// GitOps returns the GitOps configuration block.
func (c Config) GitOps() GitOpsConfig {
	return c.OpenCenter.GitOps
}

// GitDir returns the configured GitOps working directory.
func (c Config) GitDir() string {
	return strings.TrimSpace(c.OpenCenter.GitOps.GitDir)
}

// GitBranchOrDefault returns the configured Git branch, defaulting to main.
func (c Config) GitBranchOrDefault() string {
	if branch := strings.TrimSpace(c.OpenCenter.GitOps.GitBranch); branch != "" {
		return branch
	}
	return "main"
}

// IsKind reports whether the cluster uses the kind provider.
func (c Config) IsKind() bool {
	return strings.EqualFold(c.Provider(), "kind")
}

// KindDisableDefaultCNI reports whether kind should disable its default CNI.
func (c Config) KindDisableDefaultCNI() bool {
	return c.OpenCenter.Infrastructure.Kind != nil && c.OpenCenter.Infrastructure.Kind.DisableDefaultCNI
}

// ToJSON marshals the configuration to indented JSON.
func (c Config) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "  ")
}
