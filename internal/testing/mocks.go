// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testing

import (
	"context"
	"sync"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

// MockTemplateEngine provides a mock implementation of template.TemplateEngine for testing.
type MockTemplateEngine struct {
	mu sync.RWMutex

	// RenderFunc allows customizing the Render behavior
	RenderFunc func(ctx context.Context, templatePath string, data interface{}) ([]byte, error)

	// ValidateTemplateFunc allows customizing the ValidateTemplate behavior
	ValidateTemplateFunc func(templatePath string) error

	// RegisterFunctionFunc allows customizing the RegisterFunction behavior
	RegisterFunctionFunc func(name string, fn interface{})

	// SetCacheEnabledFunc allows customizing the SetCacheEnabled behavior
	SetCacheEnabledFunc func(enabled bool)

	// ClearCacheFunc allows customizing the ClearCache behavior
	ClearCacheFunc func()

	// Tracking fields for verification
	RenderCalls          []RenderCall
	ValidateTemplateCalls []string
	RegisteredFunctions  map[string]interface{}
	CacheEnabled         bool
	CacheClearCount      int
}

// RenderCall tracks a call to Render
type RenderCall struct {
	TemplatePath string
	Data         interface{}
}

// NewMockTemplateEngine creates a new mock template engine with default behaviors.
func NewMockTemplateEngine() *MockTemplateEngine {
	return &MockTemplateEngine{
		RegisteredFunctions: make(map[string]interface{}),
		RenderCalls:         make([]RenderCall, 0),
		ValidateTemplateCalls: make([]string, 0),
	}
}

// Render implements template.TemplateEngine.Render
func (m *MockTemplateEngine) Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error) {
	m.mu.Lock()
	m.RenderCalls = append(m.RenderCalls, RenderCall{
		TemplatePath: templatePath,
		Data:         data,
	})
	m.mu.Unlock()

	if m.RenderFunc != nil {
		return m.RenderFunc(ctx, templatePath, data)
	}

	// Default behavior: return empty result
	return []byte{}, nil
}

// ValidateTemplate implements template.TemplateEngine.ValidateTemplate
func (m *MockTemplateEngine) ValidateTemplate(templatePath string) error {
	m.mu.Lock()
	m.ValidateTemplateCalls = append(m.ValidateTemplateCalls, templatePath)
	m.mu.Unlock()

	if m.ValidateTemplateFunc != nil {
		return m.ValidateTemplateFunc(templatePath)
	}

	// Default behavior: validation passes
	return nil
}

// RegisterFunction implements template.TemplateEngine.RegisterFunction
func (m *MockTemplateEngine) RegisterFunction(name string, fn interface{}) {
	m.mu.Lock()
	m.RegisteredFunctions[name] = fn
	m.mu.Unlock()

	if m.RegisterFunctionFunc != nil {
		m.RegisterFunctionFunc(name, fn)
	}
}

// SetCacheEnabled implements template.TemplateEngine.SetCacheEnabled
func (m *MockTemplateEngine) SetCacheEnabled(enabled bool) {
	m.mu.Lock()
	m.CacheEnabled = enabled
	m.mu.Unlock()

	if m.SetCacheEnabledFunc != nil {
		m.SetCacheEnabledFunc(enabled)
	}
}

// ClearCache implements template.TemplateEngine.ClearCache
func (m *MockTemplateEngine) ClearCache() {
	m.mu.Lock()
	m.CacheClearCount++
	m.mu.Unlock()

	if m.ClearCacheFunc != nil {
		m.ClearCacheFunc()
	}
}

// MockConfigBuilder provides a mock implementation of config.ConfigBuilder for testing.
type MockConfigBuilder struct {
	mu sync.RWMutex

	// Configuration state
	provider     string
	organization string
	clusterName  string
	kubeVersion  string
	masterCount  int
	workerCount  int
	networking   interface{}
	services     []string
	overrides    map[string]interface{}

	// BuildFunc allows customizing the Build behavior
	BuildFunc func() (config.Config, error)

	// ValidateFunc allows customizing the Validate behavior
	ValidateFunc func() []error

	// Tracking fields
	BuildCalls    int
	ValidateCalls int
}

// NewMockConfigBuilder creates a new mock config builder.
func NewMockConfigBuilder() *MockConfigBuilder {
	return &MockConfigBuilder{
		services:  make([]string, 0),
		overrides: make(map[string]interface{}),
	}
}

// WithProvider implements config.ConfigBuilder.WithProvider
func (m *MockConfigBuilder) WithProvider(provider string) *MockConfigBuilder {
	m.mu.Lock()
	m.provider = provider
	m.mu.Unlock()
	return m
}

// WithOrganization implements config.ConfigBuilder.WithOrganization
func (m *MockConfigBuilder) WithOrganization(org string) *MockConfigBuilder {
	m.mu.Lock()
	m.organization = org
	m.mu.Unlock()
	return m
}

// WithClusterName implements config.ConfigBuilder.WithClusterName
func (m *MockConfigBuilder) WithClusterName(name string) *MockConfigBuilder {
	m.mu.Lock()
	m.clusterName = name
	m.mu.Unlock()
	return m
}

// WithKubernetesVersion implements config.ConfigBuilder.WithKubernetesVersion
func (m *MockConfigBuilder) WithKubernetesVersion(version string) *MockConfigBuilder {
	m.mu.Lock()
	m.kubeVersion = version
	m.mu.Unlock()
	return m
}

// WithNodeCounts implements config.ConfigBuilder.WithNodeCounts
func (m *MockConfigBuilder) WithNodeCounts(masters, workers int) *MockConfigBuilder {
	m.mu.Lock()
	m.masterCount = masters
	m.workerCount = workers
	m.mu.Unlock()
	return m
}

// WithNetworking implements config.ConfigBuilder.WithNetworking
func (m *MockConfigBuilder) WithNetworking(cfg interface{}) *MockConfigBuilder {
	m.mu.Lock()
	m.networking = cfg
	m.mu.Unlock()
	return m
}

// WithServices implements config.ConfigBuilder.WithServices
func (m *MockConfigBuilder) WithServices(services ...string) *MockConfigBuilder {
	m.mu.Lock()
	m.services = append(m.services, services...)
	m.mu.Unlock()
	return m
}

// WithOverride implements config.ConfigBuilder.WithOverride
func (m *MockConfigBuilder) WithOverride(path string, value interface{}) *MockConfigBuilder {
	m.mu.Lock()
	m.overrides[path] = value
	m.mu.Unlock()
	return m
}

// Build implements config.ConfigBuilder.Build
func (m *MockConfigBuilder) Build() (config.Config, error) {
	m.mu.Lock()
	m.BuildCalls++
	m.mu.Unlock()

	if m.BuildFunc != nil {
		return m.BuildFunc()
	}

	// Default behavior: return a basic config
	return config.Config{
		OpenCenter: config.SimplifiedOpenCenter{
			Meta: config.ClusterMeta{
				Name:         m.clusterName,
				Organization: m.organization,
			},
		},
	}, nil
}

// Validate implements config.ConfigBuilder.Validate
func (m *MockConfigBuilder) Validate() []error {
	m.mu.Lock()
	m.ValidateCalls++
	m.mu.Unlock()

	if m.ValidateFunc != nil {
		return m.ValidateFunc()
	}

	// Default behavior: no validation errors
	return nil
}

// MockConfigValidator provides a mock implementation of config.ConfigValidator for testing.
type MockConfigValidator struct {
	mu sync.RWMutex

	// ValidateFunc allows customizing the Validate behavior
	ValidateFunc func(cfg config.Config) []error

	// Tracking fields
	ValidateCalls []config.Config
}

// NewMockConfigValidator creates a new mock config validator.
func NewMockConfigValidator() *MockConfigValidator {
	return &MockConfigValidator{
		ValidateCalls: make([]config.Config, 0),
	}
}

// Validate implements config.ConfigValidator.Validate
func (m *MockConfigValidator) Validate(cfg config.Config) []error {
	m.mu.Lock()
	m.ValidateCalls = append(m.ValidateCalls, cfg)
	m.mu.Unlock()

	if m.ValidateFunc != nil {
		return m.ValidateFunc(cfg)
	}

	// Default behavior: no validation errors
	return nil
}

// MockTemplateRegistry provides a mock implementation of template registry for testing.
type MockTemplateRegistry struct {
	mu sync.RWMutex

	// Templates storage
	templates map[string]interface{}

	// Function customization
	RegisterTemplateFunc            func(template interface{}) error
	GetTemplateFunc                 func(name string) (interface{}, error)
	GetTemplatesForProviderFunc     func(provider string) []interface{}
	GetTemplatesForServiceFunc      func(service string) []interface{}
	ResolveTemplateDependenciesFunc func(templates []string) ([]interface{}, error)

	// Tracking fields
	RegisterTemplateCalls            []interface{}
	GetTemplateCalls                 []string
	GetTemplatesForProviderCalls     []string
	GetTemplatesForServiceCalls      []string
	ResolveTemplateDependenciesCalls [][]string
}

// NewMockTemplateRegistry creates a new mock template registry.
func NewMockTemplateRegistry() *MockTemplateRegistry {
	return &MockTemplateRegistry{
		templates:                        make(map[string]interface{}),
		RegisterTemplateCalls:            make([]interface{}, 0),
		GetTemplateCalls:                 make([]string, 0),
		GetTemplatesForProviderCalls:     make([]string, 0),
		GetTemplatesForServiceCalls:      make([]string, 0),
		ResolveTemplateDependenciesCalls: make([][]string, 0),
	}
}

// RegisterTemplate implements TemplateRegistry.RegisterTemplate
func (m *MockTemplateRegistry) RegisterTemplate(template interface{}) error {
	m.mu.Lock()
	m.RegisterTemplateCalls = append(m.RegisterTemplateCalls, template)
	m.mu.Unlock()

	if m.RegisterTemplateFunc != nil {
		return m.RegisterTemplateFunc(template)
	}

	return nil
}

// GetTemplate implements TemplateRegistry.GetTemplate
func (m *MockTemplateRegistry) GetTemplate(name string) (interface{}, error) {
	m.mu.Lock()
	m.GetTemplateCalls = append(m.GetTemplateCalls, name)
	m.mu.Unlock()

	if m.GetTemplateFunc != nil {
		return m.GetTemplateFunc(name)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if template, ok := m.templates[name]; ok {
		return template, nil
	}

	return nil, nil
}

// GetTemplatesForProvider implements TemplateRegistry.GetTemplatesForProvider
func (m *MockTemplateRegistry) GetTemplatesForProvider(provider string) []interface{} {
	m.mu.Lock()
	m.GetTemplatesForProviderCalls = append(m.GetTemplatesForProviderCalls, provider)
	m.mu.Unlock()

	if m.GetTemplatesForProviderFunc != nil {
		return m.GetTemplatesForProviderFunc(provider)
	}

	return []interface{}{}
}

// GetTemplatesForService implements TemplateRegistry.GetTemplatesForService
func (m *MockTemplateRegistry) GetTemplatesForService(service string) []interface{} {
	m.mu.Lock()
	m.GetTemplatesForServiceCalls = append(m.GetTemplatesForServiceCalls, service)
	m.mu.Unlock()

	if m.GetTemplatesForServiceFunc != nil {
		return m.GetTemplatesForServiceFunc(service)
	}

	return []interface{}{}
}

// ResolveTemplateDependencies implements TemplateRegistry.ResolveTemplateDependencies
func (m *MockTemplateRegistry) ResolveTemplateDependencies(templates []string) ([]interface{}, error) {
	m.mu.Lock()
	m.ResolveTemplateDependenciesCalls = append(m.ResolveTemplateDependenciesCalls, templates)
	m.mu.Unlock()

	if m.ResolveTemplateDependenciesFunc != nil {
		return m.ResolveTemplateDependenciesFunc(templates)
	}

	return []interface{}{}, nil
}

// MockGitOpsGenerator provides a mock implementation of GitOps generator for testing.
type MockGitOpsGenerator struct {
	mu sync.RWMutex

	// Function customization
	GenerateFunc       func(ctx context.Context, cfg config.Config) error
	GenerateDryRunFunc func(ctx context.Context, cfg config.Config) (interface{}, error)
	RollbackFunc       func(ctx context.Context, checkpointID string) error

	// Tracking fields
	GenerateCalls       []config.Config
	GenerateDryRunCalls []config.Config
	RollbackCalls       []string
}

// NewMockGitOpsGenerator creates a new mock GitOps generator.
func NewMockGitOpsGenerator() *MockGitOpsGenerator {
	return &MockGitOpsGenerator{
		GenerateCalls:       make([]config.Config, 0),
		GenerateDryRunCalls: make([]config.Config, 0),
		RollbackCalls:       make([]string, 0),
	}
}

// Generate implements GitOpsGenerator.Generate
func (m *MockGitOpsGenerator) Generate(ctx context.Context, cfg config.Config) error {
	m.mu.Lock()
	m.GenerateCalls = append(m.GenerateCalls, cfg)
	m.mu.Unlock()

	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, cfg)
	}

	return nil
}

// GenerateDryRun implements GitOpsGenerator.GenerateDryRun
func (m *MockGitOpsGenerator) GenerateDryRun(ctx context.Context, cfg config.Config) (interface{}, error) {
	m.mu.Lock()
	m.GenerateDryRunCalls = append(m.GenerateDryRunCalls, cfg)
	m.mu.Unlock()

	if m.GenerateDryRunFunc != nil {
		return m.GenerateDryRunFunc(ctx, cfg)
	}

	return nil, nil
}

// Rollback implements GitOpsGenerator.Rollback
func (m *MockGitOpsGenerator) Rollback(ctx context.Context, checkpointID string) error {
	m.mu.Lock()
	m.RollbackCalls = append(m.RollbackCalls, checkpointID)
	m.mu.Unlock()

	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx, checkpointID)
	}

	return nil
}

// MockGenerationStage provides a mock implementation of generation stage for testing.
type MockGenerationStage struct {
	mu sync.RWMutex

	// Stage configuration
	stageName string

	// Function customization
	NameFunc     func() string
	ExecuteFunc  func(ctx context.Context, workspace interface{}) error
	RollbackFunc func(ctx context.Context, workspace interface{}) error
	ValidateFunc func(ctx context.Context, workspace interface{}) error

	// Tracking fields
	ExecuteCalls  []interface{}
	RollbackCalls []interface{}
	ValidateCalls []interface{}
}

// NewMockGenerationStage creates a new mock generation stage.
func NewMockGenerationStage(name string) *MockGenerationStage {
	return &MockGenerationStage{
		stageName:     name,
		ExecuteCalls:  make([]interface{}, 0),
		RollbackCalls: make([]interface{}, 0),
		ValidateCalls: make([]interface{}, 0),
	}
}

// Name implements GenerationStage.Name
func (m *MockGenerationStage) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return m.stageName
}

// Execute implements GenerationStage.Execute
func (m *MockGenerationStage) Execute(ctx context.Context, workspace interface{}) error {
	m.mu.Lock()
	m.ExecuteCalls = append(m.ExecuteCalls, workspace)
	m.mu.Unlock()

	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(ctx, workspace)
	}

	return nil
}

// Rollback implements GenerationStage.Rollback
func (m *MockGenerationStage) Rollback(ctx context.Context, workspace interface{}) error {
	m.mu.Lock()
	m.RollbackCalls = append(m.RollbackCalls, workspace)
	m.mu.Unlock()

	if m.RollbackFunc != nil {
		return m.RollbackFunc(ctx, workspace)
	}

	return nil
}

// Validate implements GenerationStage.Validate
func (m *MockGenerationStage) Validate(ctx context.Context, workspace interface{}) error {
	m.mu.Lock()
	m.ValidateCalls = append(m.ValidateCalls, workspace)
	m.mu.Unlock()

	if m.ValidateFunc != nil {
		return m.ValidateFunc(ctx, workspace)
	}

	return nil
}

// MockServiceRegistry provides a mock implementation of service registry for testing.
type MockServiceRegistry struct {
	mu sync.RWMutex

	// Services storage
	services map[string]interface{}

	// Function customization
	RegisterServiceFunc      func(service interface{}) error
	GetServiceFunc           func(name string) (interface{}, error)
	GetEnabledServicesFunc   func(cfg config.Config) []interface{}
	ResolveDependenciesFunc  func(services []string) ([]interface{}, error)
	ValidateDependenciesFunc func(services []string) error

	// Tracking fields
	RegisterServiceCalls      []interface{}
	GetServiceCalls           []string
	GetEnabledServicesCalls   []config.Config
	ResolveDependenciesCalls  [][]string
	ValidateDependenciesCalls [][]string
}

// NewMockServiceRegistry creates a new mock service registry.
func NewMockServiceRegistry() *MockServiceRegistry {
	return &MockServiceRegistry{
		services:                  make(map[string]interface{}),
		RegisterServiceCalls:      make([]interface{}, 0),
		GetServiceCalls:           make([]string, 0),
		GetEnabledServicesCalls:   make([]config.Config, 0),
		ResolveDependenciesCalls:  make([][]string, 0),
		ValidateDependenciesCalls: make([][]string, 0),
	}
}

// RegisterService implements ServiceRegistry.RegisterService
func (m *MockServiceRegistry) RegisterService(service interface{}) error {
	m.mu.Lock()
	m.RegisterServiceCalls = append(m.RegisterServiceCalls, service)
	m.mu.Unlock()

	if m.RegisterServiceFunc != nil {
		return m.RegisterServiceFunc(service)
	}

	return nil
}

// GetService implements ServiceRegistry.GetService
func (m *MockServiceRegistry) GetService(name string) (interface{}, error) {
	m.mu.Lock()
	m.GetServiceCalls = append(m.GetServiceCalls, name)
	m.mu.Unlock()

	if m.GetServiceFunc != nil {
		return m.GetServiceFunc(name)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if service, ok := m.services[name]; ok {
		return service, nil
	}

	return nil, nil
}

// GetEnabledServices implements ServiceRegistry.GetEnabledServices
func (m *MockServiceRegistry) GetEnabledServices(cfg config.Config) []interface{} {
	m.mu.Lock()
	m.GetEnabledServicesCalls = append(m.GetEnabledServicesCalls, cfg)
	m.mu.Unlock()

	if m.GetEnabledServicesFunc != nil {
		return m.GetEnabledServicesFunc(cfg)
	}

	return []interface{}{}
}

// ResolveDependencies implements ServiceRegistry.ResolveDependencies
func (m *MockServiceRegistry) ResolveDependencies(services []string) ([]interface{}, error) {
	m.mu.Lock()
	m.ResolveDependenciesCalls = append(m.ResolveDependenciesCalls, services)
	m.mu.Unlock()

	if m.ResolveDependenciesFunc != nil {
		return m.ResolveDependenciesFunc(services)
	}

	return []interface{}{}, nil
}

// ValidateDependencies implements ServiceRegistry.ValidateDependencies
func (m *MockServiceRegistry) ValidateDependencies(services []string) error {
	m.mu.Lock()
	m.ValidateDependenciesCalls = append(m.ValidateDependenciesCalls, services)
	m.mu.Unlock()

	if m.ValidateDependenciesFunc != nil {
		return m.ValidateDependenciesFunc(services)
	}

	return nil
}

// MockServicePlugin provides a mock implementation of service plugin for testing.
type MockServicePlugin struct {
	mu sync.RWMutex

	// Plugin configuration
	pluginName string
	pluginType string

	// Function customization
	NameFunc     func() string
	TypeFunc     func() string
	ValidateFunc func(cfg config.Config) error
	RenderFunc   func(ctx context.Context, cfg config.Config, workspace interface{}) error
	StatusFunc   func(cfg config.Config) interface{}

	// Tracking fields
	ValidateCalls []config.Config
	RenderCalls   []RenderPluginCall
	StatusCalls   []config.Config
}

// RenderPluginCall tracks a call to Render
type RenderPluginCall struct {
	Config    config.Config
	Workspace interface{}
}

// NewMockServicePlugin creates a new mock service plugin.
func NewMockServicePlugin(name, pluginType string) *MockServicePlugin {
	return &MockServicePlugin{
		pluginName:    name,
		pluginType:    pluginType,
		ValidateCalls: make([]config.Config, 0),
		RenderCalls:   make([]RenderPluginCall, 0),
		StatusCalls:   make([]config.Config, 0),
	}
}

// Name implements ServicePlugin.Name
func (m *MockServicePlugin) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return m.pluginName
}

// Type implements ServicePlugin.Type
func (m *MockServicePlugin) Type() string {
	if m.TypeFunc != nil {
		return m.TypeFunc()
	}
	return m.pluginType
}

// Validate implements ServicePlugin.Validate
func (m *MockServicePlugin) Validate(cfg config.Config) error {
	m.mu.Lock()
	m.ValidateCalls = append(m.ValidateCalls, cfg)
	m.mu.Unlock()

	if m.ValidateFunc != nil {
		return m.ValidateFunc(cfg)
	}

	return nil
}

// Render implements ServicePlugin.Render
func (m *MockServicePlugin) Render(ctx context.Context, cfg config.Config, workspace interface{}) error {
	m.mu.Lock()
	m.RenderCalls = append(m.RenderCalls, RenderPluginCall{
		Config:    cfg,
		Workspace: workspace,
	})
	m.mu.Unlock()

	if m.RenderFunc != nil {
		return m.RenderFunc(ctx, cfg, workspace)
	}

	return nil
}

// Status implements ServicePlugin.Status
func (m *MockServicePlugin) Status(cfg config.Config) interface{} {
	m.mu.Lock()
	m.StatusCalls = append(m.StatusCalls, cfg)
	m.mu.Unlock()

	if m.StatusFunc != nil {
		return m.StatusFunc(cfg)
	}

	return nil
}

// MockMigrationManager provides a mock implementation of migration manager for testing.
type MockMigrationManager struct {
	mu sync.RWMutex

	// Configuration
	currentVersion     string
	supportedVersions  []string

	// Function customization
	MigrateConfigFunc          func(cfg config.Config, targetVersion string) (config.Config, error)
	GetCurrentVersionFunc      func() string
	GetSupportedVersionsFunc   func() []string
	ValidateMigrationPathFunc  func(fromVersion, toVersion string) error

	// Tracking fields
	MigrateConfigCalls         []MigrateConfigCall
	GetCurrentVersionCalls     int
	GetSupportedVersionsCalls  int
	ValidateMigrationPathCalls []MigrationPathCall
}

// MigrateConfigCall tracks a call to MigrateConfig
type MigrateConfigCall struct {
	Config        config.Config
	TargetVersion string
}

// MigrationPathCall tracks a call to ValidateMigrationPath
type MigrationPathCall struct {
	FromVersion string
	ToVersion   string
}

// NewMockMigrationManager creates a new mock migration manager.
func NewMockMigrationManager() *MockMigrationManager {
	return &MockMigrationManager{
		currentVersion:             "1.0.0",
		supportedVersions:          []string{"1.0.0"},
		MigrateConfigCalls:         make([]MigrateConfigCall, 0),
		ValidateMigrationPathCalls: make([]MigrationPathCall, 0),
	}
}

// MigrateConfig implements MigrationManager.MigrateConfig
func (m *MockMigrationManager) MigrateConfig(cfg config.Config, targetVersion string) (config.Config, error) {
	m.mu.Lock()
	m.MigrateConfigCalls = append(m.MigrateConfigCalls, MigrateConfigCall{
		Config:        cfg,
		TargetVersion: targetVersion,
	})
	m.mu.Unlock()

	if m.MigrateConfigFunc != nil {
		return m.MigrateConfigFunc(cfg, targetVersion)
	}

	return cfg, nil
}

// GetCurrentVersion implements MigrationManager.GetCurrentVersion
func (m *MockMigrationManager) GetCurrentVersion() string {
	m.mu.Lock()
	m.GetCurrentVersionCalls++
	m.mu.Unlock()

	if m.GetCurrentVersionFunc != nil {
		return m.GetCurrentVersionFunc()
	}

	return m.currentVersion
}

// GetSupportedVersions implements MigrationManager.GetSupportedVersions
func (m *MockMigrationManager) GetSupportedVersions() []string {
	m.mu.Lock()
	m.GetSupportedVersionsCalls++
	m.mu.Unlock()

	if m.GetSupportedVersionsFunc != nil {
		return m.GetSupportedVersionsFunc()
	}

	return m.supportedVersions
}

// ValidateMigrationPath implements MigrationManager.ValidateMigrationPath
func (m *MockMigrationManager) ValidateMigrationPath(fromVersion, toVersion string) error {
	m.mu.Lock()
	m.ValidateMigrationPathCalls = append(m.ValidateMigrationPathCalls, MigrationPathCall{
		FromVersion: fromVersion,
		ToVersion:   toVersion,
	})
	m.mu.Unlock()

	if m.ValidateMigrationPathFunc != nil {
		return m.ValidateMigrationPathFunc(fromVersion, toVersion)
	}

	return nil
}

// MockMCPServer provides a mock implementation of MCP server for testing.
type MockMCPServer struct {
	mu sync.RWMutex

	// Function customization
	StartFunc             func(ctx context.Context) error
	StopFunc              func(ctx context.Context) error
	RegisterToolsFunc     func(tools []interface{}) error
	RegisterResourcesFunc func(resources []interface{}) error
	RegisterPromptsFunc   func(prompts []interface{}) error
	SetAuthProviderFunc   func(provider interface{}) error

	// Tracking fields
	StartCalls             int
	StopCalls              int
	RegisterToolsCalls     [][]interface{}
	RegisterResourcesCalls [][]interface{}
	RegisterPromptsCalls   [][]interface{}
	SetAuthProviderCalls   []interface{}
}

// NewMockMCPServer creates a new mock MCP server.
func NewMockMCPServer() *MockMCPServer {
	return &MockMCPServer{
		RegisterToolsCalls:     make([][]interface{}, 0),
		RegisterResourcesCalls: make([][]interface{}, 0),
		RegisterPromptsCalls:   make([][]interface{}, 0),
		SetAuthProviderCalls:   make([]interface{}, 0),
	}
}

// Start implements MCPServer.Start
func (m *MockMCPServer) Start(ctx context.Context) error {
	m.mu.Lock()
	m.StartCalls++
	m.mu.Unlock()

	if m.StartFunc != nil {
		return m.StartFunc(ctx)
	}

	return nil
}

// Stop implements MCPServer.Stop
func (m *MockMCPServer) Stop(ctx context.Context) error {
	m.mu.Lock()
	m.StopCalls++
	m.mu.Unlock()

	if m.StopFunc != nil {
		return m.StopFunc(ctx)
	}

	return nil
}

// RegisterTools implements MCPServer.RegisterTools
func (m *MockMCPServer) RegisterTools(tools []interface{}) error {
	m.mu.Lock()
	m.RegisterToolsCalls = append(m.RegisterToolsCalls, tools)
	m.mu.Unlock()

	if m.RegisterToolsFunc != nil {
		return m.RegisterToolsFunc(tools)
	}

	return nil
}

// RegisterResources implements MCPServer.RegisterResources
func (m *MockMCPServer) RegisterResources(resources []interface{}) error {
	m.mu.Lock()
	m.RegisterResourcesCalls = append(m.RegisterResourcesCalls, resources)
	m.mu.Unlock()

	if m.RegisterResourcesFunc != nil {
		return m.RegisterResourcesFunc(resources)
	}

	return nil
}

// RegisterPrompts implements MCPServer.RegisterPrompts
func (m *MockMCPServer) RegisterPrompts(prompts []interface{}) error {
	m.mu.Lock()
	m.RegisterPromptsCalls = append(m.RegisterPromptsCalls, prompts)
	m.mu.Unlock()

	if m.RegisterPromptsFunc != nil {
		return m.RegisterPromptsFunc(prompts)
	}

	return nil
}

// SetAuthProvider implements MCPServer.SetAuthProvider
func (m *MockMCPServer) SetAuthProvider(provider interface{}) error {
	m.mu.Lock()
	m.SetAuthProviderCalls = append(m.SetAuthProviderCalls, provider)
	m.mu.Unlock()

	if m.SetAuthProviderFunc != nil {
		return m.SetAuthProviderFunc(provider)
	}

	return nil
}

// MockMCPSession provides a mock implementation of MCP session for testing.
type MockMCPSession struct {
	mu sync.RWMutex

	// Session data
	userID       string
	organization string
	permissions  []interface{}
	auditLog     interface{}
	configScope  interface{}

	// Function customization
	UserIDFunc       func() string
	OrganizationFunc func() string
	PermissionsFunc  func() []interface{}
	AuditLogFunc     func() interface{}
	ConfigScopeFunc  func() interface{}

	// Tracking fields
	UserIDCalls       int
	OrganizationCalls int
	PermissionsCalls  int
	AuditLogCalls     int
	ConfigScopeCalls  int
}

// NewMockMCPSession creates a new mock MCP session.
func NewMockMCPSession(userID, organization string) *MockMCPSession {
	return &MockMCPSession{
		userID:       userID,
		organization: organization,
		permissions:  make([]interface{}, 0),
	}
}

// UserID implements MCPSession.UserID
func (m *MockMCPSession) UserID() string {
	m.mu.Lock()
	m.UserIDCalls++
	m.mu.Unlock()

	if m.UserIDFunc != nil {
		return m.UserIDFunc()
	}

	return m.userID
}

// Organization implements MCPSession.Organization
func (m *MockMCPSession) Organization() string {
	m.mu.Lock()
	m.OrganizationCalls++
	m.mu.Unlock()

	if m.OrganizationFunc != nil {
		return m.OrganizationFunc()
	}

	return m.organization
}

// Permissions implements MCPSession.Permissions
func (m *MockMCPSession) Permissions() []interface{} {
	m.mu.Lock()
	m.PermissionsCalls++
	m.mu.Unlock()

	if m.PermissionsFunc != nil {
		return m.PermissionsFunc()
	}

	return m.permissions
}

// AuditLog implements MCPSession.AuditLog
func (m *MockMCPSession) AuditLog() interface{} {
	m.mu.Lock()
	m.AuditLogCalls++
	m.mu.Unlock()

	if m.AuditLogFunc != nil {
		return m.AuditLogFunc()
	}

	return m.auditLog
}

// ConfigScope implements MCPSession.ConfigScope
func (m *MockMCPSession) ConfigScope() interface{} {
	m.mu.Lock()
	m.ConfigScopeCalls++
	m.mu.Unlock()

	if m.ConfigScopeFunc != nil {
		return m.ConfigScopeFunc()
	}

	return m.configScope
}

// MockAuthProvider provides a mock implementation of auth provider for testing.
type MockAuthProvider struct {
	mu sync.RWMutex

	// Function customization
	AuthenticateSessionFunc func(ctx context.Context, credentials map[string]string) (interface{}, error)
	ValidatePermissionFunc  func(session interface{}, permission interface{}) error
	RefreshSessionFunc      func(ctx context.Context, session interface{}) error

	// Tracking fields
	AuthenticateSessionCalls []map[string]string
	ValidatePermissionCalls  []ValidatePermissionCall
	RefreshSessionCalls      []interface{}
}

// ValidatePermissionCall tracks a call to ValidatePermission
type ValidatePermissionCall struct {
	Session    interface{}
	Permission interface{}
}

// NewMockAuthProvider creates a new mock auth provider.
func NewMockAuthProvider() *MockAuthProvider {
	return &MockAuthProvider{
		AuthenticateSessionCalls: make([]map[string]string, 0),
		ValidatePermissionCalls:  make([]ValidatePermissionCall, 0),
		RefreshSessionCalls:      make([]interface{}, 0),
	}
}

// AuthenticateSession implements AuthProvider.AuthenticateSession
func (m *MockAuthProvider) AuthenticateSession(ctx context.Context, credentials map[string]string) (interface{}, error) {
	m.mu.Lock()
	m.AuthenticateSessionCalls = append(m.AuthenticateSessionCalls, credentials)
	m.mu.Unlock()

	if m.AuthenticateSessionFunc != nil {
		return m.AuthenticateSessionFunc(ctx, credentials)
	}

	return NewMockMCPSession("test-user", "test-org"), nil
}

// ValidatePermission implements AuthProvider.ValidatePermission
func (m *MockAuthProvider) ValidatePermission(session interface{}, permission interface{}) error {
	m.mu.Lock()
	m.ValidatePermissionCalls = append(m.ValidatePermissionCalls, ValidatePermissionCall{
		Session:    session,
		Permission: permission,
	})
	m.mu.Unlock()

	if m.ValidatePermissionFunc != nil {
		return m.ValidatePermissionFunc(session, permission)
	}

	return nil
}

// RefreshSession implements AuthProvider.RefreshSession
func (m *MockAuthProvider) RefreshSession(ctx context.Context, session interface{}) error {
	m.mu.Lock()
	m.RefreshSessionCalls = append(m.RefreshSessionCalls, session)
	m.mu.Unlock()

	if m.RefreshSessionFunc != nil {
		return m.RefreshSessionFunc(ctx, session)
	}

	return nil
}

// MockErrorAggregator provides a mock implementation of error aggregator for testing.
type MockErrorAggregator struct {
	mu sync.RWMutex

	// Errors storage
	errors []error

	// Function customization
	AddFunc            func(err error)
	AddWithContextFunc func(err error, context interface{})
	HasErrorsFunc      func() bool
	ErrorsFunc         func() []error
	ReportFunc         func() error

	// Tracking fields
	AddCalls            []error
	AddWithContextCalls []AddWithContextCall
	HasErrorsCalls      int
	ErrorsCalls         int
	ReportCalls         int
}

// AddWithContextCall tracks a call to AddWithContext
type AddWithContextCall struct {
	Error   error
	Context interface{}
}

// NewMockErrorAggregator creates a new mock error aggregator.
func NewMockErrorAggregator() *MockErrorAggregator {
	return &MockErrorAggregator{
		errors:              make([]error, 0),
		AddCalls:            make([]error, 0),
		AddWithContextCalls: make([]AddWithContextCall, 0),
	}
}

// Add implements ErrorAggregator.Add
func (m *MockErrorAggregator) Add(err error) {
	m.mu.Lock()
	m.AddCalls = append(m.AddCalls, err)
	m.errors = append(m.errors, err)
	m.mu.Unlock()

	if m.AddFunc != nil {
		m.AddFunc(err)
	}
}

// AddWithContext implements ErrorAggregator.AddWithContext
func (m *MockErrorAggregator) AddWithContext(err error, context interface{}) {
	m.mu.Lock()
	m.AddWithContextCalls = append(m.AddWithContextCalls, AddWithContextCall{
		Error:   err,
		Context: context,
	})
	m.errors = append(m.errors, err)
	m.mu.Unlock()

	if m.AddWithContextFunc != nil {
		m.AddWithContextFunc(err, context)
	}
}

// HasErrors implements ErrorAggregator.HasErrors
func (m *MockErrorAggregator) HasErrors() bool {
	m.mu.Lock()
	m.HasErrorsCalls++
	m.mu.Unlock()

	if m.HasErrorsFunc != nil {
		return m.HasErrorsFunc()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.errors) > 0
}

// Errors implements ErrorAggregator.Errors
func (m *MockErrorAggregator) Errors() []error {
	m.mu.Lock()
	m.ErrorsCalls++
	m.mu.Unlock()

	if m.ErrorsFunc != nil {
		return m.ErrorsFunc()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.errors
}

// Report implements ErrorAggregator.Report
func (m *MockErrorAggregator) Report() error {
	m.mu.Lock()
	m.ReportCalls++
	m.mu.Unlock()

	if m.ReportFunc != nil {
		return m.ReportFunc()
	}

	m.mu.RLock()
	defer m.mu.RUnlock()
	if len(m.errors) == 0 {
		return nil
	}

	// Return the first error as a simple implementation
	return m.errors[0]
}
