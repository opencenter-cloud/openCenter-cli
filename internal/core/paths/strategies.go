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
	"regexp"
	"strings"
)

var allowedPathVariables = map[string]struct{}{
	"HOME":                    {},
	"OPENCENTER_CLUSTERS_DIR": {},
	"OPENCENTER_CONFIG_DIR":   {},
	"OPENCENTER_PLUGINS_DIR":  {},
	"OPENCENTER_STATE_DIR":    {},
	"XDG_STATE_HOME":          {},
}

var pathVariablePattern = regexp.MustCompile(`\$(\w+)|\$\{([^}]+)\}`)

// ResolutionStrategy defines the interface for path resolution strategies.
// Currently only supports organization-based structure.
//
// Implementations must:
//   - Provide a unique strategy name
//   - Detect if they can resolve paths for a cluster
//   - Resolve all paths for a cluster
//
// Thread Safety:
//
// Implementations must be thread-safe and support concurrent calls to all methods.
//
// Example implementation:
//
//	type MyStrategy struct {
//	    baseDir string
//	}
//
//	func (s *MyStrategy) Name() string {
//	    return "my-strategy"
//	}
//
//	func (s *MyStrategy) CanResolve(ctx context.Context, clusterName, organization string) (bool, error) {
//	    // Check if cluster exists in this strategy's structure
//	    clusterDir := filepath.Join(s.baseDir, organization, clusterName)
//	    _, err := os.Stat(clusterDir)
//	    return err == nil, nil
//	}
//
//	func (s *MyStrategy) Resolve(ctx context.Context, clusterName, organization string) (*ClusterPaths, error) {
//	    // Resolve all paths for the cluster
//	    return &ClusterPaths{
//	        ClusterDir: filepath.Join(s.baseDir, organization, clusterName),
//	        // ... other paths
//	    }, nil
//	}
type ResolutionStrategy interface {
	// Name returns the name of the strategy
	//
	// The name should be unique and descriptive (e.g., "org-based", "legacy").
	// It's used for logging, debugging, and cache keys.
	Name() string

	// CanResolve checks if this strategy can resolve paths for the given cluster
	//
	// This method should be fast and only check for existence, not perform
	// full path resolution. It's called during strategy selection.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - clusterName: Name of the cluster
	//   - organization: Organization name (may be empty)
	//
	// Returns:
	//   - bool: true if this strategy can resolve the cluster
	//   - error: Check failure (not found is not an error)
	CanResolve(ctx context.Context, clusterName, organization string) (bool, error)

	// Resolve resolves all paths for the given cluster
	//
	// This method performs the actual path resolution and returns a complete
	// ClusterPaths structure with all paths populated.
	//
	// Parameters:
	//   - ctx: Context for cancellation
	//   - clusterName: Name of the cluster
	//   - organization: Organization name (may be empty)
	//
	// Returns:
	//   - *ClusterPaths: Resolved paths for the cluster
	//   - error: Resolution failure
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
		ConfigPath:      filepath.Join(orgDir, "."+clusterName+"-config.yaml"),
		SSHKeyPath:      filepath.Join(secretsDir, "ssh", clusterName),
	}, nil
}

// ExpandPath expands environment variables and tilde in a path.
// This is a public utility function that can be used by other packages.
func ExpandPath(path string) string {
	// Expand tilde
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(homeDir, path[2:])
		}
	}

	// Expand the documented allowlist of path variables only.
	path = expandAllowedPathVariables(path, os.LookupEnv)

	return path
}

func expandAllowedPathVariables(path string, lookupEnv func(string) (string, bool)) string {
	return pathVariablePattern.ReplaceAllStringFunc(path, func(match string) string {
		groups := pathVariablePattern.FindStringSubmatch(match)
		if len(groups) != 3 {
			return match
		}

		name := groups[1]
		if name == "" {
			name = groups[2]
		}

		if _, allowed := allowedPathVariables[name]; !allowed {
			return match
		}

		if value, ok := lookupEnv(name); ok {
			return value
		}

		return match
	})
}

// expandPath is an internal alias for ExpandPath for backward compatibility
func expandPath(path string) string {
	return ExpandPath(path)
}
