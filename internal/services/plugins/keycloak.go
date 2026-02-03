package plugins

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/config/services"
	svc "github.com/rackerlabs/opencenter-cli/internal/services"
)

// KeycloakPlugin implements the ServicePlugin interface for Keycloak
type KeycloakPlugin struct{}

// NewKeycloakPlugin creates a new KeycloakPlugin
func NewKeycloakPlugin() svc.ServicePlugin {
	return &KeycloakPlugin{}
}

// Name returns the service name
func (p *KeycloakPlugin) Name() string {
	return "keycloak"
}

// Type returns the service type
func (p *KeycloakPlugin) Type() svc.ServiceType {
	return svc.ServiceTypeSecurity
}

// Render renders the service templates to the workspace
func (p *KeycloakPlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	// Template rendering will be handled by the template system
	return nil
}

// Status returns the current status of the service
func (p *KeycloakPlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.KeycloakConfig)
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
		Message: "Keycloak identity service",
		Details: map[string]interface{}{
			"realm":        cfg.Realm,
			"frontend_url": cfg.FrontendURL,
			"client_id":    cfg.ClientID,
		},
	}
}
