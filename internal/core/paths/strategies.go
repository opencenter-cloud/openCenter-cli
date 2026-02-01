// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package paths

import (
	"context"
	"os"
	"path/filepath"
	"strings"
)

// ResolutionStrategy defines the interface for path resolution strategies.
// Currently only supports organization-based structure.
type ResolutionStrategy interface {
	// Name returns the name of the strategy
	Name() string

	// CanResolve checks if this strategy can resolve paths for the given cluster
	CanResolve(ctx context.Context, clusterName, organization string) (bool, error)

	// Resolve resolves all paths for the given cluster
	Resolve(ctx context.Context, clusterName, organization string) (*ClusterPaths, error)
}

// OrgBasedStrategy implements organization-based path resolution.
// Structure: clusters/<org>/infrastructure/clusters/<cluster>/
type OrgBasedStrategy struct {
	baseDir string
}

// NewOrgBasedStrategy creates a new organization-based strategy.
func NewOrgBasedStrategy(baseDir string) *OrgBasedStrategy {
	return &OrgBasedStrategy{
		baseDir: expandPath(baseDir),
	}
}

// Name returns the strategy name.
func (s *OrgBasedStrategy) Name() string {
	return "org-based"
}

// CanResolve checks if organization-based structure exists for the cluster.
func (s *OrgBasedStrategy) CanResolve(ctx context.Context, clusterName, organization string) (bool, error) {
	if organization == "" {
		organization = "opencenter"
	}

	// Check if organization directory exists
	orgDir := filepath.Join(s.baseDir, organization)
	if _, err := os.Stat(orgDir); os.IsNotExist(err) {
		return false, nil
	}

	// Check if cluster directory exists in organization structure
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		return false, nil
	}

	return true, nil
}

// Resolve resolves paths using organization-based structure.
func (s *OrgBasedStrategy) Resolve(ctx context.Context, clusterName, organization string) (*ClusterPaths, error) {
	if organization == "" {
		organization = "opencenter"
	}

	orgDir := filepath.Join(s.baseDir, organization)
	clusterDir := filepath.Join(orgDir, "infrastructure", "clusters", clusterName)
	applicationsDir := filepath.Join(orgDir, "applications", "overlays", clusterName)
	secretsDir := filepath.Join(orgDir, "secrets")

	return &ClusterPaths{
		OrganizationDir: orgDir,
		GitOpsDir:       orgDir,
		ClusterDir:      clusterDir,
		ApplicationsDir: applicationsDir,
		SecretsDir:      secretsDir,
		SOPSKeyPath:     filepath.Join(secretsDir, "age", "keys", clusterName+"-key.txt"),
		SOPSConfigPath:  filepath.Join(orgDir, ".sops.yaml"),
		KubeconfigPath:  filepath.Join(clusterDir, "kubeconfig.yaml"),
		InventoryPath:   filepath.Join(clusterDir, "inventory"),
		VenvPath:        filepath.Join(clusterDir, "venv"),
		BinPath:         filepath.Join(clusterDir, ".bin"),
		ConfigPath:      filepath.Join(clusterDir, "."+clusterName+"-config.yaml"),
		SSHKeyPath:      filepath.Join(secretsDir, "ssh", clusterName),
	}, nil
}

// expandPath expands environment variables and tilde in a path.
func expandPath(path string) string {
	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	return path
}
