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

// ClusterPaths contains all organization-aware paths for a cluster.
// This structure supports the organization-based directory layout and provides
// a single source of truth for all cluster-related paths.
//
// Directory Structure:
//
//	~/.config/opencenter/clusters/
//	└── <organization>/
//	    ├── infrastructure/
//	    │   └── clusters/
//	    │       └── <cluster>/
//	    │           ├── .<cluster>-config.yaml  (ConfigPath)
//	    │           ├── kubeconfig.yaml         (KubeconfigPath)
//	    │           ├── inventory/              (InventoryPath)
//	    │           ├── venv/                   (VenvPath)
//	    │           └── .bin/                   (BinPath)
//	    ├── applications/
//	    │   └── overlays/
//	    │       └── <cluster>/                  (ApplicationsDir)
//	    ├── secrets/
//	    │   ├── age/
//	    │   │   └── keys/
//	    │   │       └── <cluster>-key.txt       (SOPSKeyPath)
//	    │   └── ssh/
//	    │       └── <cluster>                   (SSHKeyPath)
//	    └── .sops.yaml                          (SOPSConfigPath)
//
// All paths are absolute and fully resolved (no ~ or environment variables).
type ClusterPaths struct {
	// OrganizationDir is the root directory for the organization
	// Example: ~/.config/opencenter/clusters/<organization>
	OrganizationDir string

	// GitOpsDir is the root directory for GitOps manifests
	// Example: ~/.config/opencenter/clusters/<organization>
	GitOpsDir string

	// ClusterDir is the directory containing cluster-specific infrastructure
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>
	ClusterDir string

	// ApplicationsDir is the directory containing cluster-specific application overlays
	// Example: ~/.config/opencenter/clusters/<organization>/applications/overlays/<cluster>
	ApplicationsDir string

	// SecretsDir is the directory containing encrypted secrets
	// Example: ~/.config/opencenter/clusters/<organization>/secrets
	SecretsDir string

	// SOPSKeyPath is the path to the SOPS Age encryption key
	// Example: ~/.config/opencenter/clusters/<organization>/secrets/age/keys/<cluster>-key.txt
	SOPSKeyPath string

	// SOPSConfigPath is the path to the SOPS configuration file
	// Example: ~/.config/opencenter/clusters/<organization>/.sops.yaml
	SOPSConfigPath string

	// KubeconfigPath is the path to the cluster kubeconfig file
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml
	KubeconfigPath string

	// InventoryPath is the path to the Ansible inventory directory
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/inventory
	InventoryPath string

	// VenvPath is the path to the Python virtual environment
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/venv
	VenvPath string

	// BinPath is the path to cluster-specific binaries
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/.bin
	BinPath string

	// ConfigPath is the path to the cluster configuration file
	// Example: ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/.<cluster>-config.yaml
	ConfigPath string

	// SSHKeyPath is the path to the cluster SSH key
	// Example: ~/.config/opencenter/clusters/<organization>/secrets/ssh/<cluster>-<env>-<region>
	SSHKeyPath string
}

// ResolutionOptions contains options for path resolution.
//
// These options control the behavior of PathResolver:
//   - Organization: Default organization when not specified in Resolve()
//   - CacheResults: Enable caching for performance (recommended: true)
//   - ValidatePaths: Validate write permissions (expensive, use in tests)
//
// Example:
//
//	opts := paths.ResolutionOptions{
//	    Organization: "my-company",
//	    CacheResults: true,
//	    ValidatePaths: false, // Skip expensive validation
//	}
type ResolutionOptions struct {
	// Organization is the organization name for the cluster
	// If empty, defaults to "opencenter"
	Organization string

	// CacheResults enables caching of resolved paths
	// Default: true
	CacheResults bool

	// ValidatePaths validates that resolved paths are safe and accessible
	// Default: false (validation is expensive)
	ValidatePaths bool
}

// DefaultResolutionOptions returns the default resolution options.
//
// Defaults:
//   - Organization: "opencenter"
//   - CacheResults: true (caching enabled for performance)
//   - ValidatePaths: false (validation disabled for performance)
//
// Example:
//
//	opts := paths.DefaultResolutionOptions()
//	opts.Organization = "my-company" // Override default
func DefaultResolutionOptions() ResolutionOptions {
	return ResolutionOptions{
		Organization:  "opencenter",
		CacheResults:  true,
		ValidatePaths: false,
	}
}

// PathType represents the type of path being resolved.
type PathType int

const (
	// PathTypeConfig represents a cluster configuration file path
	PathTypeConfig PathType = iota

	// PathTypeSecrets represents a secrets directory path
	PathTypeSecrets

	// PathTypeGitOps represents a GitOps directory path
	PathTypeGitOps

	// PathTypeCluster represents a cluster directory path
	PathTypeCluster

	// PathTypeApplications represents an applications directory path
	PathTypeApplications

	// PathTypeSOPS represents a SOPS key or config path
	PathTypeSOPS

	// PathTypeKubeconfig represents a kubeconfig file path
	PathTypeKubeconfig

	// PathTypeInventory represents an Ansible inventory path
	PathTypeInventory

	// PathTypeSSH represents an SSH key path
	PathTypeSSH
)

// String returns the string representation of a PathType.
func (pt PathType) String() string {
	switch pt {
	case PathTypeConfig:
		return "config"
	case PathTypeSecrets:
		return "secrets"
	case PathTypeGitOps:
		return "gitops"
	case PathTypeCluster:
		return "cluster"
	case PathTypeApplications:
		return "applications"
	case PathTypeSOPS:
		return "sops"
	case PathTypeKubeconfig:
		return "kubeconfig"
	case PathTypeInventory:
		return "inventory"
	case PathTypeSSH:
		return "ssh"
	default:
		return "unknown"
	}
}

// StructureType represents the type of directory structure.
type StructureType int

const (
	// StructureTypeOrganization represents the organization-based structure
	StructureTypeOrganization StructureType = iota

	// StructureTypeUnknown represents an unknown structure type
	StructureTypeUnknown
)

// String returns the string representation of a StructureType.
func (st StructureType) String() string {
	switch st {
	case StructureTypeOrganization:
		return "organization"
	case StructureTypeUnknown:
		return "unknown"
	default:
		return "unknown"
	}
}
