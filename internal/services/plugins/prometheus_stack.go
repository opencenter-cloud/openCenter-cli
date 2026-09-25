package plugins

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

// PrometheusStackPlugin implements the ServicePlugin interface for kube-prometheus-stack
// using composition with BaseServicePlugin
type PrometheusStackPlugin struct {
	*svc.BaseServicePlugin
}

// NewPrometheusStackPlugin creates a new PrometheusStackPlugin
func NewPrometheusStackPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "kube-prometheus-stack",
		Version:     "1.0.0",
		Description: "Complete monitoring stack with Prometheus, Grafana, and Alertmanager",
		Type:        svc.ServiceTypeMonitoring,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &PrometheusStackPlugin{
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

// validate implements kube-prometheus-stack specific validation
func (p *PrometheusStackPlugin) validate(config interface{}) error {
	cfg, ok := config.(*services.PrometheusStackConfig)
	if !ok {
		return fmt.Errorf("invalid config type for kube-prometheus-stack: expected *PrometheusStackConfig")
	}

	if err := services.ValidatePrometheusStackConfig(cfg); err != nil {
		return err
	}

	// Basic validation
	if cfg.GrafanaVolumeSize < 0 {
		return fmt.Errorf("grafana_volume_size must be non-negative")
	}
	if cfg.PrometheusVolumeSize < 0 {
		return fmt.Errorf("prometheus_volume_size must be non-negative")
	}
	if cfg.AlertmanagerVolumeSize < 0 {
		return fmt.Errorf("alertmanager_volume_size must be non-negative")
	}

	return nil
}

// render implements kube-prometheus-stack specific rendering
func (p *PrometheusStackPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// status implements kube-prometheus-stack specific status logic
func (p *PrometheusStackPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.PrometheusStackConfig)
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
		Message: "Prometheus stack service",
		Details: map[string]interface{}{
			"grafana_volume_size":      cfg.GrafanaVolumeSize,
			"prometheus_volume_size":   cfg.PrometheusVolumeSize,
			"alertmanager_volume_size": cfg.AlertmanagerVolumeSize,
		},
	}
}
