package plugins

import (
	"context"
	"fmt"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	svc "github.com/opencenter-cloud/opencenter-cli/internal/services"
)

// DefaultServicePlugin implements the ServicePlugin interface for services with no specific configuration
type DefaultServicePlugin struct {
	name        string
	serviceType svc.ServiceType
}

// NewDefaultServicePlugin creates a new DefaultServicePlugin
func NewDefaultServicePlugin(name string, serviceType svc.ServiceType) svc.ServicePlugin {
	return &DefaultServicePlugin{
		name:        name,
		serviceType: serviceType,
	}
}

// Name returns the service name
func (p *DefaultServicePlugin) Name() string {
	return p.name
}

// Type returns the service type
func (p *DefaultServicePlugin) Type() svc.ServiceType {
	return p.serviceType
}

// Validate validates the service configuration
func (p *DefaultServicePlugin) Validate(config interface{}) error {
	// Validation is handled by validators
	// This method is here to satisfy the ServicePlugin interface
	return nil
}

// Render renders the service templates to the workspace
func (p *DefaultServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	return nil
}

// Status returns the current status of the service
func (p *DefaultServicePlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.DefaultServiceConfig)
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
		Message: fmt.Sprintf("%s service", p.name),
	}
}

// HeadlampPlugin implements the ServicePlugin interface for Headlamp
type HeadlampPlugin struct{}

func NewHeadlampPlugin() svc.ServicePlugin {
	return &HeadlampPlugin{}
}

func (p *HeadlampPlugin) Name() string {
	return "headlamp"
}

func (p *HeadlampPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeCore
}

func (p *HeadlampPlugin) Validate(config interface{}) error {
	// Validation is handled by validators
	// This method is here to satisfy the ServicePlugin interface
	return nil
}

func (p *HeadlampPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *HeadlampPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.HeadlampConfig)
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
		Message: "Headlamp dashboard service",
		Details: map[string]interface{}{
			"oidc_issuer_url": cfg.OIDCIssuerURL,
			"oidc_client_id":  cfg.OIDCClientID,
		},
	}
}

// WeaveGitOpsPlugin implements the ServicePlugin interface for Weave GitOps
type WeaveGitOpsPlugin struct{}

func NewWeaveGitOpsPlugin() svc.ServicePlugin {
	return &WeaveGitOpsPlugin{}
}

func (p *WeaveGitOpsPlugin) Name() string {
	return "weave-gitops"
}

func (p *WeaveGitOpsPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeGitOps
}

func (p *WeaveGitOpsPlugin) Validate(config interface{}) error {
	return nil
}

func (p *WeaveGitOpsPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *WeaveGitOpsPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.DefaultServiceConfig)
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

	return svc.ServiceStatus{
		State:   "pending",
		Message: "Weave GitOps service",
	}
}

// AlertProxyPlugin implements the ServicePlugin interface for Alert Proxy
type AlertProxyPlugin struct{}

func NewAlertProxyPlugin() svc.ServicePlugin {
	return &AlertProxyPlugin{}
}

func (p *AlertProxyPlugin) Name() string {
	return "alert-proxy"
}

func (p *AlertProxyPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeMonitoring
}

func (p *AlertProxyPlugin) Validate(config interface{}) error {
	// Validation is handled by validators
	// This method is here to satisfy the ServicePlugin interface
	return nil
}

func (p *AlertProxyPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *AlertProxyPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.AlertProxyConfig)
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
		Message: "Alert proxy service",
		Details: map[string]interface{}{
			"alert_manager_base_url": cfg.AlertManagerBaseURL,
			"http_route_fqdn":        cfg.HTTPRouteFQDN,
		},
	}
}

// EtcdBackupPlugin implements the ServicePlugin interface for Etcd Backup
// using composition with BaseServicePlugin
type EtcdBackupPlugin struct {
	*svc.BaseServicePlugin
}

func NewEtcdBackupPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "etcd-backup",
		Version:     "1.0.0",
		Description: "Automated etcd backup and restore",
		Type:        svc.ServiceTypeStorage,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &EtcdBackupPlugin{
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

// validate implements etcd-backup specific validation
func (p *EtcdBackupPlugin) validate(config interface{}) error {
	// Validation is handled by validators
	// This method is here to satisfy the ServicePlugin interface
	return nil
}

// render implements etcd-backup specific rendering
func (p *EtcdBackupPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

// status implements etcd-backup specific status logic
func (p *EtcdBackupPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.EtcdBackupConfig)
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
		Message: "Etcd backup service",
		Details: map[string]interface{}{
			"s3_host":   cfg.S3Host,
			"s3_region": cfg.S3Region,
		},
	}
}

// VSphereCSIPlugin implements the ServicePlugin interface for vSphere CSI
// using composition with BaseServicePlugin
type VSphereCSIPlugin struct {
	*svc.BaseServicePlugin
}

func NewVSphereCSIPlugin() svc.ServicePlugin {
	// Create base plugin with metadata
	base := svc.NewBasePlugin(svc.PluginMetadata{
		Name:        "vsphere-csi",
		Version:     "1.0.0",
		Description: "VMware vSphere Container Storage Interface driver",
		Type:        svc.ServiceTypeStorage,
		Author:      "opencenter",
		License:     "Apache-2.0",
	})

	plugin := &VSphereCSIPlugin{
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

// validate implements vsphere-csi specific validation
func (p *VSphereCSIPlugin) validate(config interface{}) error {
	// Validation is handled by validators
	// This method is here to satisfy the ServicePlugin interface
	return nil
}

// render implements vsphere-csi specific rendering
func (p *VSphereCSIPlugin) render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

// status implements vsphere-csi specific status logic
func (p *VSphereCSIPlugin) status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.VSphereCSIConfig)
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
		Message: "vSphere CSI service",
	}
}
