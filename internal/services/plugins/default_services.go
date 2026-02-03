package plugins

import (
	"context"
	"fmt"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
	svc "github.com/rackerlabs/opencenter-cli/internal/services"
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

// CalicoPlugin implements the ServicePlugin interface for Calico
type CalicoPlugin struct{}

func NewCalicoPlugin() svc.ServicePlugin {
	return &CalicoPlugin{}
}

func (p *CalicoPlugin) Name() string {
	return "calico"
}

func (p *CalicoPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeNetworking
}

func (p *CalicoPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *CalicoPlugin) Status(config interface{}) svc.ServiceStatus {
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

func (p *WeaveGitOpsPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *WeaveGitOpsPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.WeaveGitOpsConfig)
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
			"alert_manager_base_url": cfg.AlertManagerBaseUrl,
			"http_route_fqdn":        cfg.HTTPRouteFQDN,
		},
	}
}

// EtcdBackupPlugin implements the ServicePlugin interface for Etcd Backup
type EtcdBackupPlugin struct{}

func NewEtcdBackupPlugin() svc.ServicePlugin {
	return &EtcdBackupPlugin{}
}

func (p *EtcdBackupPlugin) Name() string {
	return "etcd-backup"
}

func (p *EtcdBackupPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeStorage
}

func (p *EtcdBackupPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *EtcdBackupPlugin) Status(config interface{}) svc.ServiceStatus {
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
type VSphereCSIPlugin struct{}

func NewVSphereCSIPlugin() svc.ServicePlugin {
	return &VSphereCSIPlugin{}
}

func (p *VSphereCSIPlugin) Name() string {
	return "vsphere-csi"
}

func (p *VSphereCSIPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeStorage
}

func (p *VSphereCSIPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *VSphereCSIPlugin) Status(config interface{}) svc.ServiceStatus {
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
