package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

// CertManagerPlugin implements the ServicePlugin interface for cert-manager
type CertManagerPlugin struct{}

// NewCertManagerPlugin creates a new CertManagerPlugin
func NewCertManagerPlugin() svc.ServicePlugin {
	return &CertManagerPlugin{}
}

// Name returns the service name
func (p *CertManagerPlugin) Name() string {
	return "cert-manager"
}

// Type returns the service type
func (p *CertManagerPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeSecurity
}

// Validate validates the service configuration
func (p *CertManagerPlugin) Validate(config interface{}) error {
	cfg, ok := config.(*services.CertManagerConfig)
	if !ok {
		return fmt.Errorf("invalid config type for cert-manager: expected *CertManagerConfig")
	}

	// Validate LetsEncrypt server URL if specified
	if cfg.LetsEncryptServer != "" {
		if !strings.HasPrefix(cfg.LetsEncryptServer, "https://") {
			return fmt.Errorf("letsencrypt_server must be an HTTPS URL")
		}
	}

	// Validate email if specified
	if cfg.Email != "" && !strings.Contains(cfg.Email, "@") {
		return fmt.Errorf("email must be a valid email address")
	}

	return nil
}

// Render renders the service templates to the workspace
func (p *CertManagerPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	// This is a placeholder for future implementation
	return nil
}

// Status returns the current status of the service
func (p *CertManagerPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.CertManagerConfig)
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
		Message: "Cert-manager service",
		Details: map[string]interface{}{
			"letsencrypt_server": cfg.LetsEncryptServer,
			"email":              cfg.Email,
		},
	}
}
