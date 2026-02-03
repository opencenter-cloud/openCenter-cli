package services

import (
	"context"
	"fmt"
	"sync"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// ServiceRegistry manages service definitions and plugins
type ServiceRegistry interface {
	// RegisterService registers a service definition
	RegisterService(service ServiceDefinition) error

	// RegisterFromManifest registers a service from a manifest
	RegisterFromManifest(manifest *ServicePluginManifest, plugin ServicePlugin) error

	// GetService retrieves a service definition by name
	GetService(name string) (ServiceDefinition, error)

	// GetEnabledServices returns all enabled services for a configuration
	GetEnabledServices(config interface{}) []ServiceDefinition

	// ResolveDependencies resolves service dependencies in correct order
	ResolveDependencies(services []string) ([]ServiceDefinition, error)

	// ValidateDependencies validates that all dependencies are available
	ValidateDependencies(services []string) error

	// ListServices returns all registered services
	ListServices() []ServiceDefinition

	// LoadManifestsFromDirectory loads service manifests from a directory
	LoadManifestsFromDirectory(dir string) error

	// ExecuteLifecycleHook executes a lifecycle hook for a specific service
	ExecuteLifecycleHook(ctx context.Context, serviceName string, hook string, config interface{}) error

	// ExecuteLifecycleHooks executes a lifecycle hook for multiple services in dependency order
	ExecuteLifecycleHooks(ctx context.Context, services []string, hook string, config interface{}) error

	// GetValidationEngine returns the validation engine used by the registry
	GetValidationEngine() *validation.ValidationEngine

	// ValidateService validates a service configuration using the ValidationEngine
	ValidateService(ctx context.Context, serviceName string, config interface{}) (*validation.ValidationResult, error)
}

// ServiceDefinition defines a complete service with its plugin and metadata
type ServiceDefinition struct {
	Name         string
	Type         ServiceType
	Version      string
	Description  string
	Dependencies []string
	Templates    []TemplateRef
	Plugin       ServicePlugin
	Lifecycle    ServiceLifecycle
	Metadata     ServiceMetadata
}

// ExecuteLifecycleHook executes a specific lifecycle hook if it's defined
func (s *ServiceDefinition) ExecuteLifecycleHook(ctx context.Context, hook string, config interface{}) error {
	var hookFunc func(context.Context, interface{}) error

	switch hook {
	case "PreInstall":
		hookFunc = s.Lifecycle.PreInstall
	case "PostInstall":
		hookFunc = s.Lifecycle.PostInstall
	case "PreUpdate":
		hookFunc = s.Lifecycle.PreUpdate
	case "PostUpdate":
		hookFunc = s.Lifecycle.PostUpdate
	case "PreRemove":
		hookFunc = s.Lifecycle.PreRemove
	case "PostRemove":
		hookFunc = s.Lifecycle.PostRemove
	default:
		return fmt.Errorf("unknown lifecycle hook: %s", hook)
	}

	// If hook is not defined, skip it
	if hookFunc == nil {
		return nil
	}

	// Execute the hook
	if err := hookFunc(ctx, config); err != nil {
		return fmt.Errorf("lifecycle hook %s failed for service %s: %w", hook, s.Name, err)
	}

	return nil
}

// ServiceMetadata contains additional metadata about a service
type ServiceMetadata struct {
	Author      string            `json:"author,omitempty"`
	Homepage    string            `json:"homepage,omitempty"`
	Repository  string            `json:"repository,omitempty"`
	License     string            `json:"license,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
}

// DefaultServiceRegistry is the default implementation of ServiceRegistry
type DefaultServiceRegistry struct {
	mu               sync.RWMutex
	services         map[string]ServiceDefinition
	validationEngine *validation.ValidationEngine
}

// NewServiceRegistry creates a new service registry
func NewServiceRegistry() ServiceRegistry {
	return &DefaultServiceRegistry{
		services:         make(map[string]ServiceDefinition),
		validationEngine: validation.NewValidationEngine(),
	}
}

// NewServiceRegistryWithEngine creates a new service registry with a custom validation engine
func NewServiceRegistryWithEngine(engine *validation.ValidationEngine) ServiceRegistry {
	return &DefaultServiceRegistry{
		services:         make(map[string]ServiceDefinition),
		validationEngine: engine,
	}
}

// RegisterService registers a service definition
func (r *DefaultServiceRegistry) RegisterService(service ServiceDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if service.Name == "" {
		return fmt.Errorf("service name is required")
	}

	if _, exists := r.services[service.Name]; exists {
		return fmt.Errorf("service %s is already registered", service.Name)
	}

	// Validate dependencies exist (except for the service being registered)
	for _, dep := range service.Dependencies {
		if dep == service.Name {
			return fmt.Errorf("service %s cannot depend on itself", service.Name)
		}
	}

	r.services[service.Name] = service
	
	// Register service validator with the validation engine
	// Only register if not already registered
	serviceValidator := validators.NewServiceValidator(service.Name)
	if !r.validationEngine.Has(serviceValidator.Name()) {
		if err := r.validationEngine.Register(serviceValidator); err != nil {
			return fmt.Errorf("failed to register validator for service %s: %w", service.Name, err)
		}
	}
	
	return nil
}

// RegisterFromManifest registers a service from a manifest
func (r *DefaultServiceRegistry) RegisterFromManifest(manifest *ServicePluginManifest, plugin ServicePlugin) error {
	if manifest == nil {
		return fmt.Errorf("manifest is nil")
	}

	if plugin == nil {
		return fmt.Errorf("plugin is nil")
	}

	// Validate manifest
	if err := ValidateManifest(manifest); err != nil {
		return fmt.Errorf("invalid manifest: %w", err)
	}

	// Ensure plugin name matches manifest name
	if plugin.Name() != manifest.Name {
		return fmt.Errorf("plugin name %s does not match manifest name %s", plugin.Name(), manifest.Name)
	}

	// Create service definition from manifest
	service := ServiceDefinition{
		Name:         manifest.Name,
		Type:         manifest.Type,
		Version:      manifest.Version,
		Description:  manifest.Description,
		Dependencies: manifest.Dependencies,
		Templates:    manifest.Templates,
		Plugin:       plugin,
		Metadata: ServiceMetadata{
			Annotations: manifest.Metadata,
		},
	}

	return r.RegisterService(service)
}

// GetService retrieves a service definition by name
func (r *DefaultServiceRegistry) GetService(name string) (ServiceDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	service, exists := r.services[name]
	if !exists {
		return ServiceDefinition{}, fmt.Errorf("service %s not found", name)
	}

	return service, nil
}

// GetEnabledServices returns all enabled services for a configuration
func (r *DefaultServiceRegistry) GetEnabledServices(config interface{}) []ServiceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var enabled []ServiceDefinition
	for _, service := range r.services {
		// For now, return all services
		// In a real implementation, this would check the config to see which services are enabled
		enabled = append(enabled, service)
	}

	return enabled
}

// ResolveDependencies resolves service dependencies in correct order
func (r *DefaultServiceRegistry) ResolveDependencies(services []string) ([]ServiceDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Check for circular dependencies first
	if err := r.checkCircularDependencies(services); err != nil {
		return nil, err
	}

	// Build dependency graph
	visited := make(map[string]bool)
	var result []ServiceDefinition

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}

		service, exists := r.services[name]
		if !exists {
			return fmt.Errorf("service %s not found", name)
		}

		// Visit dependencies first
		for _, dep := range service.Dependencies {
			if err := visit(dep); err != nil {
				return err
			}
		}

		visited[name] = true
		result = append(result, service)
		return nil
	}

	// Visit all requested services
	for _, name := range services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return result, nil
}

// ValidateDependencies validates that all dependencies are available
func (r *DefaultServiceRegistry) ValidateDependencies(services []string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, name := range services {
		service, exists := r.services[name]
		if !exists {
			return fmt.Errorf("service %s not found", name)
		}

		// Check all dependencies exist
		for _, dep := range service.Dependencies {
			if _, exists := r.services[dep]; !exists {
				return fmt.Errorf("service %s depends on %s which is not registered", name, dep)
			}
		}
	}

	// Check for circular dependencies
	return r.checkCircularDependencies(services)
}

// checkCircularDependencies checks for circular dependencies in the service graph
func (r *DefaultServiceRegistry) checkCircularDependencies(services []string) error {
	visiting := make(map[string]bool)
	visited := make(map[string]bool)

	var visit func(name string, path []string) error
	visit = func(name string, path []string) error {
		if visiting[name] {
			// Found a cycle
			cycle := append(path, name)
			return fmt.Errorf("circular dependency detected: %v", cycle)
		}

		if visited[name] {
			return nil
		}

		service, exists := r.services[name]
		if !exists {
			return fmt.Errorf("service %s not found", name)
		}

		visiting[name] = true
		path = append(path, name)

		for _, dep := range service.Dependencies {
			if err := visit(dep, path); err != nil {
				return err
			}
		}

		visiting[name] = false
		visited[name] = true
		return nil
	}

	for _, name := range services {
		if err := visit(name, []string{}); err != nil {
			return err
		}
	}

	return nil
}

// ListServices returns all registered services
func (r *DefaultServiceRegistry) ListServices() []ServiceDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make([]ServiceDefinition, 0, len(r.services))
	for _, service := range r.services {
		services = append(services, service)
	}

	return services
}

// LoadManifestsFromDirectory loads service manifests from a directory
func (r *DefaultServiceRegistry) LoadManifestsFromDirectory(dir string) error {
	manifests, err := LoadManifestsFromDirectory(dir)
	if err != nil {
		return fmt.Errorf("failed to load manifests from directory %s: %w", dir, err)
	}

	for _, manifest := range manifests {
		// Create a basic plugin implementation for the manifest
		// In a real implementation, this would load the actual plugin code
		plugin := &BasicServicePlugin{
			name:        manifest.Name,
			serviceType: manifest.Type,
		}

		if err := r.RegisterFromManifest(manifest, plugin); err != nil {
			return fmt.Errorf("failed to register service from manifest %s: %w", manifest.Name, err)
		}
	}

	return nil
}

// ExecuteLifecycleHook executes a lifecycle hook for a specific service
func (r *DefaultServiceRegistry) ExecuteLifecycleHook(ctx context.Context, serviceName string, hook string, config interface{}) error {
	service, err := r.GetService(serviceName)
	if err != nil {
		return fmt.Errorf("failed to get service %s: %w", serviceName, err)
	}

	return service.ExecuteLifecycleHook(ctx, hook, config)
}

// ExecuteLifecycleHooks executes a lifecycle hook for multiple services in dependency order
func (r *DefaultServiceRegistry) ExecuteLifecycleHooks(ctx context.Context, services []string, hook string, config interface{}) error {
	// Resolve dependencies to get correct execution order
	resolved, err := r.ResolveDependencies(services)
	if err != nil {
		return fmt.Errorf("failed to resolve dependencies: %w", err)
	}

	// For removal hooks, execute in reverse order (remove dependents before dependencies)
	if hook == "PreRemove" || hook == "PostRemove" {
		for i := len(resolved) - 1; i >= 0; i-- {
			if err := resolved[i].ExecuteLifecycleHook(ctx, hook, config); err != nil {
				return err
			}
		}
	} else {
		// For install/update hooks, execute in dependency order
		for _, service := range resolved {
			if err := service.ExecuteLifecycleHook(ctx, hook, config); err != nil {
				return err
			}
		}
	}

	return nil
}

// GetValidationEngine returns the validation engine used by the registry
func (r *DefaultServiceRegistry) GetValidationEngine() *validation.ValidationEngine {
	return r.validationEngine
}

// ValidateService validates a service configuration using the ValidationEngine
func (r *DefaultServiceRegistry) ValidateService(ctx context.Context, serviceName string, config interface{}) (*validation.ValidationResult, error) {
	// Check if service exists
	r.mu.RLock()
	_, exists := r.services[serviceName]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("service %s not found", serviceName)
	}
	
	// Use ValidationEngine to validate the service
	validatorName := fmt.Sprintf("service:%s", serviceName)
	result, err := r.validationEngine.Validate(ctx, validatorName, config)
	if err != nil {
		return nil, fmt.Errorf("validation failed for service %s: %w", serviceName, err)
	}
	
	return result, nil
}

// BasicServicePlugin is a basic implementation of ServicePlugin for testing
type BasicServicePlugin struct {
	name        string
	serviceType ServiceType
}

func (p *BasicServicePlugin) Name() string {
	return p.name
}

func (p *BasicServicePlugin) Type() ServiceType {
	return p.serviceType
}

func (p *BasicServicePlugin) Validate(config interface{}) error {
	return nil
}

func (p *BasicServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	return nil
}

func (p *BasicServicePlugin) Status(config interface{}) ServiceStatus {
	// For basic plugin, we can't determine the actual status
	// Return pending as a safe default
	return ServiceStatus{
		State:   "pending",
		Message: "Service not yet deployed",
	}
}
