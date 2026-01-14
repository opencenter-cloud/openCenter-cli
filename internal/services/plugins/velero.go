package plugins

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

// VeleroPlugin implements the ServicePlugin interface for Velero
type VeleroPlugin struct{}

// NewVeleroPlugin creates a new VeleroPlugin
func NewVeleroPlugin() svc.ServicePlugin {
	return &VeleroPlugin{}
}

// Name returns the service name
func (p *VeleroPlugin) Name() string {
	return "velero"
}

// Type returns the service type
func (p *VeleroPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeStorage
}

// Validate validates the service configuration
func (p *VeleroPlugin) Validate(config interface{}) error {
	cfg, ok := config.(*services.VeleroConfig)
	if !ok {
		return fmt.Errorf("invalid config type for velero: expected *VeleroConfig")
	}

	// Validate backup bucket if service is enabled
	if cfg.IsEnabled() && cfg.BackupBucket == "" {
		return fmt.Errorf("velero_backup_bucket is required when velero is enabled")
	}

	return nil
}

// Render renders the service templates to the workspace
func (p *VeleroPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	return nil
}

// Status returns the current status of the service
func (p *VeleroPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.VeleroConfig)
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
		Message: "Velero backup service",
		Details: map[string]interface{}{
			"backup_bucket": cfg.BackupBucket,
			"region":        cfg.Region,
		},
	}
}
