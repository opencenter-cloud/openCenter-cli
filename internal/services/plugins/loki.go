package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

// LokiPlugin implements the ServicePlugin interface for Loki
type LokiPlugin struct{}

// NewLokiPlugin creates a new LokiPlugin
func NewLokiPlugin() svc.ServicePlugin {
	return &LokiPlugin{}
}

// Name returns the service name
func (p *LokiPlugin) Name() string {
	return "loki"
}

// Type returns the service type
func (p *LokiPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeLogging
}

// Validate validates the service configuration
func (p *LokiPlugin) Validate(config interface{}) error {
	cfg, ok := config.(*services.LokiConfig)
	if !ok {
		return fmt.Errorf("invalid config type for loki: expected *LokiConfig")
	}

	if !cfg.IsEnabled() {
		return nil
	}

	// Validate storage type
	if cfg.StorageType != "" && cfg.StorageType != "s3" && cfg.StorageType != "swift" {
		return fmt.Errorf("loki_storage_type must be 's3' or 'swift'")
	}

	// Validate Swift configuration if using Swift storage
	if cfg.StorageType == "swift" {
		if cfg.SwiftAuthURL == "" {
			return fmt.Errorf("swift_auth_url is required when using Swift storage")
		}
		if !strings.HasSuffix(cfg.SwiftAuthURL, "/v3") {
			return fmt.Errorf("swift_auth_url must end with /v3")
		}
		if cfg.SwiftContainerName == "" {
			return fmt.Errorf("swift_container_name is required when using Swift storage")
		}
	}

	// Validate S3 configuration if using S3 storage
	if cfg.StorageType == "s3" {
		if cfg.S3Endpoint == "" {
			return fmt.Errorf("loki_s3_endpoint is required when using S3 storage")
		}
		if cfg.BucketName == "" {
			return fmt.Errorf("loki_bucket_name is required when using S3 storage")
		}
	}

	// Validate volume size
	if cfg.VolumeSize < 0 {
		return fmt.Errorf("loki_volume_size must be non-negative")
	}

	return nil
}

// Render renders the service templates to the workspace
func (p *LokiPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	return nil
}

// Status returns the current status of the service
func (p *LokiPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.LokiConfig)
	if !ok {
		return svc.ServiceStatus{
			State:   "failed",
			Message: "Invalid configuration type",
		}
	}

	if !cfg.IsEnabled() {
		return svc.ServiceStatus{
			State:   "disabled",
			Message: "Service is disabled",
		}
	}

	// Get status from config, default to "pending" if not set
	state := cfg.GetStatus()
	if state == "" {
		state = "pending"
	}

	return svc.ServiceStatus{
		State:   state,
		Message: "Loki logging service",
		Details: map[string]interface{}{
			"storage_type": cfg.StorageType,
			"bucket_name":  cfg.BucketName,
			"volume_size":  cfg.VolumeSize,
		},
	}
}
