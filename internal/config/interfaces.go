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

package config

import (
	"context"
	"fmt"
)

// ConfigLoader defines the interface for loading cluster configurations.
type ConfigLoader interface {
	// LoadConfig loads a cluster configuration by name
	LoadConfig(ctx context.Context, clusterName string) (*Config, error)

	// GetConfigPath returns the path to a cluster's configuration file
	GetConfigPath(ctx context.Context, clusterName string) (string, error)
}

// ConfigPersister defines the interface for saving and deleting configurations.
type ConfigPersister interface {
	// SaveConfig saves a cluster configuration
	SaveConfig(ctx context.Context, config *Config) error

	// DeleteConfig removes a cluster configuration
	DeleteConfig(ctx context.Context, clusterName string) error
}

// ConfigLister defines the interface for listing available configurations.
type ConfigLister interface {
	// ListConfigs returns a list of available cluster configurations
	ListConfigs(ctx context.Context) ([]string, error)
}

// ActiveConfigManager defines the interface for managing the active cluster configuration.
type ActiveConfigManager interface {
	// SetActiveConfig sets the active cluster configuration
	SetActiveConfig(ctx context.Context, clusterName string) error

	// GetActiveConfig returns the name of the active cluster configuration
	GetActiveConfig(ctx context.Context) (string, error)
}

// ConfigManagerInterface defines the interface for configuration management operations.
// This interface provides abstraction for loading, validating, and managing cluster configurations.
// Deprecated: Use specific interfaces (ConfigLoader, ConfigPersister, ConfigLister, ActiveConfigManager)
// This interface is maintained for backward compatibility and will be removed in v2.0.0
type ConfigManagerInterface interface {
	ConfigLoader
	ConfigPersister
	ConfigLister
	ActiveConfigManager

	// ValidateConfig validates a cluster configuration
	ValidateConfig(ctx context.Context, config *Config) *ConfigValidationResult
}

// ConfigLoaderInterface defines the interface for loading configurations from various sources.
type ConfigLoaderInterface interface {
	// LoadFromFile loads configuration from a file path
	LoadFromFile(ctx context.Context, filePath string) (*Config, error)

	// LoadFromBytes loads configuration from byte data
	LoadFromBytes(ctx context.Context, data []byte, clusterName string) (*Config, error)

	// LoadDefault creates a default configuration for a cluster
	LoadDefault(ctx context.Context, clusterName string) (*Config, error)

	// GenerateCompleteConfig generates a complete configuration with defaults merged
	GenerateCompleteConfig(ctx context.Context, clusterName string) (*Config, error)

	// LoadFromPath loads configuration using organization-aware path resolution
	LoadFromPath(ctx context.Context, clusterName string) (*Config, error)
}

// ConfigValidatorInterface defines the interface for configuration validation.
type ConfigValidatorInterface interface {
	// Validate performs comprehensive validation on a configuration
	Validate(ctx context.Context, config *Config) *ConfigValidationResult

	// ValidateStructure validates the basic structure of a configuration
	ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult

	// ValidateSemantics validates the semantic correctness of a configuration
	ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult

	// ValidateNetworking validates network plugin configuration
	ValidateNetworking(ctx context.Context, config *Config) *ConfigValidationResult

	// ValidateCloudProvider validates cloud provider specific configuration
	ValidateCloudProvider(ctx context.Context, config *Config) *ConfigValidationResult
}

// PathResolverInterface defines the interface for path resolution operations.
type PathResolverInterface interface {
	// ResolveClusterPaths resolves all paths for a cluster
	ResolveClusterPaths(ctx context.Context, clusterName, organization string) (*OrganizationClusterPaths, error)

	// CreateClusterDirectories creates all necessary directories for a cluster
	CreateClusterDirectories(ctx context.Context, clusterName, organization string) error

	// CreateOrganizationStructure creates the organization directory structure
	CreateOrganizationStructure(ctx context.Context, organization string) error

	// ValidatePath validates that a path is safe and accessible
	ValidatePath(ctx context.Context, path string) error

	// IsLegacyCluster checks if a cluster uses legacy structure
	IsLegacyCluster(ctx context.Context, clusterName string) (bool, error)

	// GetClusterOrganization determines the organization for a cluster
	GetClusterOrganization(ctx context.Context, clusterName string) (string, error)
}

// ConfigCacheInterface defines the interface for configuration caching.
// This is an internal interface used by ConfigurationManager.
type ConfigCacheInterface interface {
	// Get retrieves a cached configuration
	Get(ctx context.Context, key string) (*Config, bool)

	// Set stores a configuration in cache
	Set(ctx context.Context, key string, config *Config) error

	// Delete removes a configuration from cache
	Delete(ctx context.Context, key string) error

	// Clear clears all cached configurations
	Clear(ctx context.Context) error

	// InvalidateCluster invalidates all cache entries for a cluster
	InvalidateCluster(ctx context.Context, clusterName string) error
}

// ConfigValidationResult represents the result of configuration validation.
type ConfigValidationResult struct {
	Valid    bool                     `json:"valid"`
	Errors   []*ConfigValidationError `json:"errors,omitempty"`
	Warnings []*ConfigValidationError `json:"warnings,omitempty"`
	Repaired []*ConfigValidationError `json:"repaired,omitempty"`
}

// ConfigValidationError represents a configuration error with context and suggestions.
type ConfigValidationError struct {
	Type        string      `json:"type"`        // validation, permission, path, format
	Field       string      `json:"field"`       // dot notation path to problematic field
	Value       interface{} `json:"value"`       // the problematic value
	Message     string      `json:"message"`     // human-readable error message
	Suggestions []string    `json:"suggestions"` // actionable suggestions for fixing the error
	Repaired    bool        `json:"repaired"`    // indicates if the error was automatically repaired
}

// Error implements the error interface for ConfigValidationError.
func (ce *ConfigValidationError) Error() string {
	prefix := ""
	if ce.Repaired {
		prefix = "[AUTO-REPAIRED] "
	}

	if ce.Field != "" {
		return fmt.Sprintf("%s%s error in field '%s': %s", prefix, ce.Type, ce.Field, ce.Message)
	}
	return fmt.Sprintf("%s%s error: %s", prefix, ce.Type, ce.Message)
}

// OrganizationClusterPaths contains all organization-aware paths for a cluster.
type OrganizationClusterPaths struct {
	OrganizationDir string `json:"organization_dir"` // ~/.config/opencenter/clusters/<organization>
	GitOpsDir       string `json:"gitops_dir"`       // ~/.config/opencenter/clusters/<organization>
	ClusterDir      string `json:"cluster_dir"`      // ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>
	ApplicationsDir string `json:"applications_dir"` // ~/.config/opencenter/clusters/<organization>/applications/overlays/<cluster>
	SecretsDir      string `json:"secrets_dir"`      // ~/.config/opencenter/clusters/<organization>/secrets
	SOPSKeyPath     string `json:"sops_key_path"`    // ~/.config/opencenter/clusters/<organization>/secrets/age/keys/<cluster>.txt
	SOPSConfigPath  string `json:"sops_config_path"` // ~/.config/opencenter/clusters/<organization>/.sops.yaml
	KubeconfigPath  string `json:"kubeconfig_path"`  // ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/kubeconfig.yaml
	InventoryPath   string `json:"inventory_path"`   // ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/inventory/inventory.yaml
	VenvPath        string `json:"venv_path"`        // ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/venv/
	BinPath         string `json:"bin_path"`         // ~/.config/opencenter/clusters/<organization>/infrastructure/clusters/<cluster>/.bin/
}
