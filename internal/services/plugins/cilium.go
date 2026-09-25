package plugins

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

// CiliumPlugin implements the ServicePlugin interface for Cilium
// using composition with BaseServicePlugin
type CiliumPlugin struct {
	*svc.BaseServicePlugin
}

// NewCiliumPlugin creates a new CiliumPlugin
func NewCiliumPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "cilium",
		Version:     "1.0.0",
		Description: "Cilium eBPF-based networking for Kubernetes",
		Type:        svc.ServiceTypeNetworking,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &CiliumPlugin{
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

// validate implements cilium specific validation
func (p *CiliumPlugin) validate(config interface{}) error {
	_, ok := config.(*services.CiliumConfig)
	if !ok {
		return fmt.Errorf("invalid config type for cilium: expected *CiliumConfig")
	}

	return nil
}

// render implements cilium specific rendering
func (p *CiliumPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// status implements cilium specific status logic
func (p *CiliumPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.CiliumConfig)
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
		Message: "Cilium networking service",
		Details: map[string]interface{}{
			"operator_enabled":       cfg.OperatorEnabled,
			"kube_proxy_replacement": cfg.KubeProxyReplacement,
		},
	}
}
