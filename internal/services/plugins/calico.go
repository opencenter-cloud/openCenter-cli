package plugins

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

// CalicoPlugin implements the ServicePlugin interface for Calico
// using composition with BaseServicePlugin
type CalicoPlugin struct {
	*svc.BaseServicePlugin
}

// NewCalicoPlugin creates a new CalicoPlugin
func NewCalicoPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "calico",
		Version:     "1.0.0",
		Description: "Calico networking for Kubernetes",
		Type:        svc.ServiceTypeNetworking,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &CalicoPlugin{
		BaseServicePlugin: base,
	}

	// Inject service-specific validation logic
	base.SetValidator(plugin.validate)

	// Inject service-specific rendering logic
	base.SetRenderer(plugin.render)

	// Inject service-specific status logic
	base.SetStatusFunc(plugin.status)

	return plugin
}

// validate implements calico specific validation
func (p *CalicoPlugin) validate(config interface{}) error {
	_, ok := config.(*services.CalicoConfig)
	if !ok {
		return fmt.Errorf("invalid config type for calico: expected *CalicoConfig")
	}

	return nil
}

// render implements calico specific rendering
func (p *CalicoPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// status implements calico specific status logic
func (p *CalicoPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.CalicoConfig)
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
		Message: "Calico networking service",
		Details: map[string]interface{}{
			"kube_api_server": cfg.KubeAPIServer,
		},
	}
}
