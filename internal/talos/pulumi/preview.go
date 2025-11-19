package pulumi

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// PreviewEngine handles Pulumi preview operations.
type PreviewEngine struct {
	manager *Manager
	logger  Logger
}

// NewPreviewEngine creates a new preview engine.
func NewPreviewEngine(manager *Manager, logger Logger) (*PreviewEngine, error) {
	if manager == nil {
		return nil, fmt.Errorf("manager cannot be nil")
	}
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &PreviewEngine{
		manager: manager,
		logger:  logger,
	}, nil
}

// ExecutePreview executes a Pulumi preview operation via Go SDK.
func (p *PreviewEngine) ExecutePreview(ctx context.Context) (*talos.PulumiPreview, error) {
	p.logger.Info("executing Pulumi preview", "stack", p.manager.config.StackName)

	// Validate stack configuration
	if err := p.validatePreviewConfig(); err != nil {
		return nil, err
	}

	// Placeholder for actual Pulumi SDK preview execution
	// In real implementation, this would:
	// 1. Initialize Pulumi automation API
	// 2. Load the stack
	// 3. Execute preview operation
	// 4. Parse preview results
	// 5. Structure the output

	preview := &talos.PulumiPreview{
		Creates:  []talos.ResourceChange{},
		Updates:  []talos.ResourceChange{},
		Deletes:  []talos.ResourceChange{},
		Replaces: []talos.ResourceChange{},
	}

	p.logger.Info("Pulumi preview executed", "stack", p.manager.config.StackName)
	return preview, nil
}

// ParsePreviewOutput parses and structures preview output.
func (p *PreviewEngine) ParsePreviewOutput(ctx context.Context, rawOutput string) (*talos.PulumiPreview, error) {
	p.logger.Debug("parsing Pulumi preview output")

	// Placeholder for parsing logic
	// In real implementation, this would:
	// 1. Parse the raw preview output
	// 2. Extract resource changes
	// 3. Categorize changes (create, update, delete, replace)
	// 4. Extract properties and reasons

	preview := &talos.PulumiPreview{
		Creates:  []talos.ResourceChange{},
		Updates:  []talos.ResourceChange{},
		Deletes:  []talos.ResourceChange{},
		Replaces: []talos.ResourceChange{},
	}

	p.logger.Debug("Pulumi preview output parsed")
	return preview, nil
}

// DisplayPlannedChanges formats and displays planned changes.
func (p *PreviewEngine) DisplayPlannedChanges(ctx context.Context, preview *talos.PulumiPreview) error {
	p.logger.Info("displaying planned changes")

	if preview == nil {
		return fmt.Errorf("preview cannot be nil")
	}

	// Log summary of changes
	p.logger.Info("planned changes summary",
		"creates", len(preview.Creates),
		"updates", len(preview.Updates),
		"deletes", len(preview.Deletes),
		"replaces", len(preview.Replaces),
	)

	// Display creates
	for _, change := range preview.Creates {
		p.logger.Info("will create", "type", change.Type, "name", change.Name)
	}

	// Display updates
	for _, change := range preview.Updates {
		p.logger.Info("will update", "type", change.Type, "name", change.Name, "reason", change.Reason)
	}

	// Display deletes
	for _, change := range preview.Deletes {
		p.logger.Info("will delete", "type", change.Type, "name", change.Name)
	}

	// Display replaces
	for _, change := range preview.Replaces {
		p.logger.Info("will replace", "type", change.Type, "name", change.Name, "reason", change.Reason)
	}

	return nil
}

// validatePreviewConfig validates the configuration before preview.
func (p *PreviewEngine) validatePreviewConfig() error {
	if p.manager.config.StackName == "" {
		return &ConfigError{
			Field:   "stack_name",
			Message: "stack name is required for preview",
		}
	}

	if p.manager.config.SwiftContainer == "" {
		return &ConfigError{
			Field:   "swift_container",
			Message: "Swift container is required for preview",
		}
	}

	return nil
}

// GetChangeCount returns the total number of changes in a preview.
func (p *PreviewEngine) GetChangeCount(preview *talos.PulumiPreview) int {
	if preview == nil {
		return 0
	}

	return len(preview.Creates) + len(preview.Updates) + len(preview.Deletes) + len(preview.Replaces)
}

// HasNodeReplacements checks if the preview includes node replacements.
func (p *PreviewEngine) HasNodeReplacements(preview *talos.PulumiPreview) bool {
	if preview == nil {
		return false
	}

	for _, change := range preview.Replaces {
		// Check if the resource is a compute instance (node)
		if change.Type == "openstack:compute/instance:Instance" ||
			change.Type == "openstack:compute:Instance" {
			return true
		}
	}

	return false
}

// GetNodeReplacements returns all node replacement changes.
func (p *PreviewEngine) GetNodeReplacements(preview *talos.PulumiPreview) []talos.ResourceChange {
	if preview == nil {
		return []talos.ResourceChange{}
	}

	var nodeReplacements []talos.ResourceChange
	for _, change := range preview.Replaces {
		// Check if the resource is a compute instance (node)
		if change.Type == "openstack:compute/instance:Instance" ||
			change.Type == "openstack:compute:Instance" {
			nodeReplacements = append(nodeReplacements, change)
		}
	}

	return nodeReplacements
}

// GetNetworkUpdates returns all network-related updates.
func (p *PreviewEngine) GetNetworkUpdates(preview *talos.PulumiPreview) []talos.ResourceChange {
	if preview == nil {
		return []talos.ResourceChange{}
	}

	var networkUpdates []talos.ResourceChange
	for _, change := range preview.Updates {
		// Check if the resource is network-related
		if change.Type == "openstack:networking/network:Network" ||
			change.Type == "openstack:networking/subnet:Subnet" ||
			change.Type == "openstack:networking/router:Router" ||
			change.Type == "openstack:networking/securityGroup:SecurityGroup" {
			networkUpdates = append(networkUpdates, change)
		}
	}

	return networkUpdates
}

// GetSecurityPolicyChanges returns all security policy changes.
func (p *PreviewEngine) GetSecurityPolicyChanges(preview *talos.PulumiPreview) []talos.ResourceChange {
	if preview == nil {
		return []talos.ResourceChange{}
	}

	var securityChanges []talos.ResourceChange

	// Check creates
	for _, change := range preview.Creates {
		if p.isSecurityResource(change.Type) {
			securityChanges = append(securityChanges, change)
		}
	}

	// Check updates
	for _, change := range preview.Updates {
		if p.isSecurityResource(change.Type) {
			securityChanges = append(securityChanges, change)
		}
	}

	// Check deletes
	for _, change := range preview.Deletes {
		if p.isSecurityResource(change.Type) {
			securityChanges = append(securityChanges, change)
		}
	}

	return securityChanges
}

// isSecurityResource checks if a resource type is security-related.
func (p *PreviewEngine) isSecurityResource(resourceType string) bool {
	securityTypes := []string{
		"openstack:networking/securityGroup:SecurityGroup",
		"openstack:networking/securityGroupRule:SecurityGroupRule",
		"openstack:keymanager/secret:Secret",
		"openstack:keymanager/container:Container",
	}

	for _, secType := range securityTypes {
		if resourceType == secType {
			return true
		}
	}

	return false
}
