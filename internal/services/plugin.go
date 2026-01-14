package services

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// ServiceType represents the type of service
type ServiceType string

const (
	ServiceTypeCore       ServiceType = "core"
	ServiceTypeMonitoring ServiceType = "monitoring"
	ServiceTypeLogging    ServiceType = "logging"
	ServiceTypeStorage    ServiceType = "storage"
	ServiceTypeNetworking ServiceType = "networking"
	ServiceTypeSecurity   ServiceType = "security"
	ServiceTypeGitOps     ServiceType = "gitops"
	ServiceTypeCustom     ServiceType = "custom"
)

// ServicePlugin defines the interface that all service plugins must implement
type ServicePlugin interface {
	// Name returns the unique name of the service
	Name() string

	// Type returns the service type
	Type() ServiceType

	// Validate validates the service configuration
	Validate(config interface{}) error

	// Render renders the service templates to the workspace
	Render(ctx context.Context, config interface{}, workspace interface{}) error

	// Status returns the current status of the service
	Status(config interface{}) ServiceStatus
}

// ServiceStatus represents the status of a service
type ServiceStatus struct {
	State   string                 `json:"state"`   // pending, running, success, failed
	Message string                 `json:"message"` // Status message
	Details map[string]interface{} `json:"details"` // Additional status details
}

// ServicePluginManifest defines the structure of a service plugin manifest file
type ServicePluginManifest struct {
	Name         string            `yaml:"name" json:"name"`
	Version      string            `yaml:"version" json:"version"`
	Type         ServiceType       `yaml:"type" json:"type"`
	Description  string            `yaml:"description" json:"description"`
	Dependencies []string          `yaml:"dependencies" json:"dependencies"`
	Templates    []TemplateRef     `yaml:"templates" json:"templates"`
	Config       ServiceConfig     `yaml:"config" json:"config"`
	Metadata     map[string]string `yaml:"metadata" json:"metadata"`
}

// TemplateRef references a template used by the service
type TemplateRef struct {
	Name      string            `yaml:"name" json:"name"`
	Path      string            `yaml:"path" json:"path"`
	Condition map[string]string `yaml:"condition,omitempty" json:"condition,omitempty"`
}

// ServiceConfig defines the configuration schema for a service
type ServiceConfig struct {
	Schema     map[string]interface{} `yaml:"schema" json:"schema"`
	Defaults   map[string]interface{} `yaml:"defaults" json:"defaults"`
	Required   []string               `yaml:"required" json:"required"`
	Validation []ValidationRule       `yaml:"validation" json:"validation"`
}

// ValidationRule defines a validation rule for service configuration
type ValidationRule struct {
	Field    string `yaml:"field" json:"field"`
	Type     string `yaml:"type" json:"type"`
	Message  string `yaml:"message" json:"message"`
	Operator string `yaml:"operator,omitempty" json:"operator,omitempty"`
	Value    string `yaml:"value,omitempty" json:"value,omitempty"`
}

// ServiceLifecycle defines lifecycle hooks for a service
type ServiceLifecycle struct {
	PreInstall  func(ctx context.Context, config interface{}) error
	PostInstall func(ctx context.Context, config interface{}) error
	PreUpdate   func(ctx context.Context, config interface{}) error
	PostUpdate  func(ctx context.Context, config interface{}) error
	PreRemove   func(ctx context.Context, config interface{}) error
	PostRemove  func(ctx context.Context, config interface{}) error
}

// LoadManifest loads a service plugin manifest from a file
func LoadManifest(path string) (*ServicePluginManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest file %s: %w", path, err)
	}

	var manifest ServicePluginManifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest file %s: %w", path, err)
	}

	// Validate required fields
	if manifest.Name == "" {
		return nil, fmt.Errorf("manifest %s: name is required", path)
	}
	if manifest.Version == "" {
		return nil, fmt.Errorf("manifest %s: version is required", path)
	}
	if manifest.Type == "" {
		manifest.Type = ServiceTypeCustom
	}

	return &manifest, nil
}

// LoadManifestsFromDirectory loads all service plugin manifests from a directory
func LoadManifestsFromDirectory(dir string) ([]*ServicePluginManifest, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // Directory doesn't exist, return empty list
		}
		return nil, fmt.Errorf("failed to read directory %s: %w", dir, err)
	}

	var manifests []*ServicePluginManifest
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Only process YAML files
		name := entry.Name()
		if filepath.Ext(name) != ".yaml" && filepath.Ext(name) != ".yml" {
			continue
		}

		manifestPath := filepath.Join(dir, name)
		manifest, err := LoadManifest(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load manifest %s: %w", manifestPath, err)
		}

		manifests = append(manifests, manifest)
	}

	return manifests, nil
}

// ValidateManifest validates a service plugin manifest
func ValidateManifest(manifest *ServicePluginManifest) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	if manifest.Name == "" {
		return fmt.Errorf("manifest name is required")
	}

	if manifest.Version == "" {
		return fmt.Errorf("manifest version is required")
	}

	// Validate service type
	validTypes := map[ServiceType]bool{
		ServiceTypeCore:       true,
		ServiceTypeMonitoring: true,
		ServiceTypeLogging:    true,
		ServiceTypeStorage:    true,
		ServiceTypeNetworking: true,
		ServiceTypeSecurity:   true,
		ServiceTypeGitOps:     true,
		ServiceTypeCustom:     true,
	}

	if !validTypes[manifest.Type] {
		return fmt.Errorf("invalid service type: %s", manifest.Type)
	}

	return nil
}
