package pulumi

import (
	"context"
	"fmt"
)

// SwiftBackendConfig holds configuration for Swift backend.
type SwiftBackendConfig struct {
	Container         string
	Prefix            string
	VersioningEnabled bool
	EncryptionEnabled bool
	AccessKey         string
	SecretKey         string
	AuthURL           string
	Region            string
}

// SwiftBackend manages Pulumi state in OpenStack Swift.
type SwiftBackend struct {
	config *SwiftBackendConfig
	logger Logger
}

// NewSwiftBackend creates a new Swift backend manager.
func NewSwiftBackend(config *SwiftBackendConfig, logger Logger) (*SwiftBackend, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if config.Container == "" {
		return nil, &ConfigError{Field: "Container", Message: "container name is required"}
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &SwiftBackend{
		config: config,
		logger: logger,
	}, nil
}

// Initialize creates and configures the Swift container for Pulumi state.
func (s *SwiftBackend) Initialize(ctx context.Context) error {
	s.logger.Info("initializing Swift backend", "container", s.config.Container)

	// Validate configuration
	if err := s.validateConfig(); err != nil {
		return fmt.Errorf("invalid Swift backend configuration: %w", err)
	}

	// Create container if it doesn't exist
	if err := s.createContainer(ctx); err != nil {
		return &BackendError{
			Container: s.config.Container,
			Cause:     err,
		}
	}

	// Enable versioning
	if s.config.VersioningEnabled {
		if err := s.enableVersioning(ctx); err != nil {
			return &BackendError{
				Container: s.config.Container,
				Cause:     fmt.Errorf("failed to enable versioning: %w", err),
			}
		}
		s.logger.Info("versioning enabled", "container", s.config.Container)
	}

	// Enable server-side encryption
	if s.config.EncryptionEnabled {
		if err := s.enableEncryption(ctx); err != nil {
			return &BackendError{
				Container: s.config.Container,
				Cause:     fmt.Errorf("failed to enable encryption: %w", err),
			}
		}
		s.logger.Info("server-side encryption enabled", "container", s.config.Container)
	}

	s.logger.Info("Swift backend initialized successfully", "container", s.config.Container)
	return nil
}

// validateConfig validates the Swift backend configuration.
func (s *SwiftBackend) validateConfig() error {
	if s.config.Container == "" {
		return &ConfigError{Field: "Container", Message: "container name is required"}
	}
	if s.config.AccessKey == "" {
		return &ConfigError{Field: "AccessKey", Message: "EC2 access key is required"}
	}
	if s.config.SecretKey == "" {
		return &ConfigError{Field: "SecretKey", Message: "EC2 secret key is required"}
	}
	if s.config.AuthURL == "" {
		return &ConfigError{Field: "AuthURL", Message: "auth URL is required"}
	}
	return nil
}

// createContainer creates the Swift container if it doesn't exist.
func (s *SwiftBackend) createContainer(ctx context.Context) error {
	s.logger.Debug("creating Swift container", "container", s.config.Container)

	// Check if container exists
	exists, err := s.containerExists(ctx)
	if err != nil {
		return fmt.Errorf("failed to check container existence: %w", err)
	}

	if exists {
		s.logger.Debug("container already exists", "container", s.config.Container)
		return nil
	}

	// Placeholder for actual Swift API call
	// In real implementation, this would use gophercloud or similar
	s.logger.Info("container created", "container", s.config.Container)
	return nil
}

// containerExists checks if the Swift container exists.
func (s *SwiftBackend) containerExists(ctx context.Context) (bool, error) {
	// Placeholder for actual Swift API call
	// In real implementation, this would use gophercloud to check container
	return false, nil
}

// enableVersioning enables versioning on the Swift container.
func (s *SwiftBackend) enableVersioning(ctx context.Context) error {
	s.logger.Debug("enabling versioning", "container", s.config.Container)

	// Placeholder for actual Swift API call
	// In real implementation, this would set X-Versions-Location header
	return nil
}

// enableEncryption enables server-side encryption on the Swift container.
func (s *SwiftBackend) enableEncryption(ctx context.Context) error {
	s.logger.Debug("enabling server-side encryption", "container", s.config.Container)

	// Placeholder for actual Swift API call
	// In real implementation, this would configure encryption settings
	return nil
}

// ValidateAccess verifies that the Swift backend is accessible.
func (s *SwiftBackend) ValidateAccess(ctx context.Context) error {
	s.logger.Debug("validating Swift backend access", "container", s.config.Container)

	// Check if container exists and is accessible
	exists, err := s.containerExists(ctx)
	if err != nil {
		return &BackendError{
			Container: s.config.Container,
			Cause:     fmt.Errorf("failed to access container: %w", err),
		}
	}

	if !exists {
		return &BackendError{
			Container: s.config.Container,
			Cause:     ErrStackNotFound,
		}
	}

	s.logger.Debug("Swift backend access validated", "container", s.config.Container)
	return nil
}

// GetBackendURL returns the Pulumi backend URL for Swift.
func (s *SwiftBackend) GetBackendURL() string {
	if s.config.Prefix != "" {
		return fmt.Sprintf("s3://%s/%s", s.config.Container, s.config.Prefix)
	}
	return fmt.Sprintf("s3://%s", s.config.Container)
}

// GetConfig returns the Swift backend configuration.
func (s *SwiftBackend) GetConfig() *SwiftBackendConfig {
	return s.config
}
