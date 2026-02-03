package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
	svc "github.com/rackerlabs/opencenter-cli/internal/services"
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
