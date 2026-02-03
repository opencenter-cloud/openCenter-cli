package plugins

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
	svc "github.com/rackerlabs/opencenter-cli/internal/services"
)

// PrometheusStackPlugin implements the ServicePlugin interface for kube-prometheus-stack
type PrometheusStackPlugin struct{}

// NewPrometheusStackPlugin creates a new PrometheusStackPlugin
func NewPrometheusStackPlugin() svc.ServicePlugin {
	return &PrometheusStackPlugin{}
}

// Name returns the service name
func (p *PrometheusStackPlugin) Name() string {
	return "kube-prometheus-stack"
}

// Type returns the service type
func (p *PrometheusStackPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeMonitoring
}

// Render renders the service templates to the workspace
func (p *PrometheusStackPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// Status returns the current status of the service
func (p *PrometheusStackPlugin) Status(config interface{}) svc.ServiceStatus {
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
