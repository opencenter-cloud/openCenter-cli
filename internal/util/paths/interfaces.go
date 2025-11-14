/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package paths

// PathResolver interface for resolving paths with organization support
type PathResolver interface {
	ResolveClusterPaths(clusterName, organization string) ClusterPaths
	ExpandPath(path string) string
	ValidatePath(path string) error
	CreateOrganizationStructure(organization string) error
	CreateClusterDirectories(clusterName, organization string) error
	GetLegacyClusterPath(clusterName string) (string, error)
	IsLegacyCluster(clusterName string) (bool, error)
	OrganizationAwareClusterDirectoryPath(clusterName string) (string, error)
	OrganizationAwareConfigPath(clusterName string) (string, error)
	OrganizationAwareSecretsPath(clusterName string) (string, error)
	ClearCache()
	InvalidateCacheForCluster(clusterName string)
}

// MigrationManager interface for handling path structure migrations
type MigrationManager interface {
	DetectLegacyStructure() ([]string, error)
	MigrateClusterToOrganization(clusterName, organization string) error
	ValidatePostMigration(clusterName, organization string) error
	BackupCluster(clusterName string) (string, error)
	RestoreCluster(clusterName, backupPath string) error
	MigrateAllLegacyClusters(organization string) ([]string, []error)
}

// PathValidator interface for validating paths and permissions
type PathValidator interface {
	ValidatePath(path string) error
	ValidateDirectoryPermissions(dir string) error
	ValidateClusterName(name string) error
	ValidateOrganizationName(name string) error
	ValidatePathIsDirectory(path string) error
	ValidatePathIsFile(path string) error
}

// DirectoryManager interface for creating and managing directories
type DirectoryManager interface {
	CreateDirectory(path string, mode uint32) error
	CreateDirectoryStructure(paths []string, mode uint32) error
	EnsureDirectoryExists(path string, mode uint32) error
	RemoveDirectoryIfEmpty(path string) error
	CopyDirectory(src, dst string) error
	MoveDirectory(src, dst string) error
}

// ClusterPaths contains all organization-aware paths for a cluster
type ClusterPaths struct {
	OrganizationDir   string // ~/.config/openCenter/clusters/<organization>
	GitOpsDir         string // ~/.config/openCenter/clusters/<organization>
	ClusterDir        string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>
	ApplicationsDir   string // ~/.config/openCenter/clusters/<organization>/applications/overlays/<cluster>
	SecretsDir        string // ~/.config/openCenter/clusters/<organization>/secrets
	SOPSKeyPath       string // ~/.config/openCenter/clusters/<organization>/secrets/age/keys/<cluster>.txt
	SOPSConfigPath    string // ~/.config/openCenter/clusters/<organization>/.sops.yaml
	KubeconfigPath    string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml
	InventoryPath     string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/inventory/
	VenvPath          string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/venv/
	BinPath           string // ~/.config/openCenter/clusters/<organization>/infrastructure/clusters/<cluster>/.bin/
}

// PathExpander interface for expanding environment variables and user paths
type PathExpander interface {
	ExpandPath(path string) string
	ExpandEnvironmentVariables(path string) string
	ExpandUserHome(path string) string
	ResolvePath(path string) (string, error)
}