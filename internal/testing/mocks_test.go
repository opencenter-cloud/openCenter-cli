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
	"errors"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockTemplateEngine(t *testing.T) {
	t.Run("default behavior", func(t *testing.T) {
		engine := NewMockTemplateEngine()

		// Test Render
		result, err := engine.Render(context.Background(), "test.tmpl", map[string]string{"key": "value"})
		require.NoError(t, err)
		assert.Empty(t, result)
		assert.Len(t, engine.RenderCalls, 1)
		assert.Equal(t, "test.tmpl", engine.RenderCalls[0].TemplatePath)

		// Test ValidateTemplate
		err = engine.ValidateTemplate("test.tmpl")
		require.NoError(t, err)
		assert.Len(t, engine.ValidateTemplateCalls, 1)

		// Test RegisterFunction
		engine.RegisterFunction("testFunc", func() string { return "test" })
		assert.Len(t, engine.RegisteredFunctions, 1)

		// Test SetCacheEnabled
		engine.SetCacheEnabled(true)
		assert.True(t, engine.CacheEnabled)

		// Test ClearCache
		engine.ClearCache()
		assert.Equal(t, 1, engine.CacheClearCount)
	})

	t.Run("custom behavior", func(t *testing.T) {
		engine := NewMockTemplateEngine()
		engine.RenderFunc = func(ctx context.Context, templatePath string, data interface{}) ([]byte, error) {
			return []byte("custom result"), nil
		}

		result, err := engine.Render(context.Background(), "test.tmpl", nil)
		require.NoError(t, err)
		assert.Equal(t, []byte("custom result"), result)
	})
}

func TestMockConfigBuilder(t *testing.T) {
	t.Run("fluent API", func(t *testing.T) {
		builder := NewMockConfigBuilder()

		// Test method chaining
		result := builder.
			WithProvider("openstack").
			WithOrganization("test-org").
			WithClusterName("test-cluster").
			WithKubernetesVersion("1.28.0").
			WithNodeCounts(3, 5).
			WithServices("prometheus", "loki").
			WithOverride("path.to.value", "override")

		assert.NotNil(t, result)
		assert.Equal(t, "openstack", builder.provider)
		assert.Equal(t, "test-org", builder.organization)
		assert.Equal(t, "test-cluster", builder.clusterName)
		assert.Equal(t, "1.28.0", builder.kubeVersion)
		assert.Equal(t, 3, builder.masterCount)
		assert.Equal(t, 5, builder.workerCount)
		assert.Len(t, builder.services, 2)
		assert.Len(t, builder.overrides, 1)
	})

	t.Run("build and validate", func(t *testing.T) {
		builder := NewMockConfigBuilder()
		builder.WithProvider("openstack").WithOrganization("test-org").WithClusterName("test-cluster")

		cfg, err := builder.Build()
		require.NoError(t, err)
		assert.Equal(t, "test-cluster", cfg.OpenCenter.Meta.Name)
		assert.Equal(t, 1, builder.BuildCalls)

		errors := builder.Validate()
		assert.Nil(t, errors)
		assert.Equal(t, 1, builder.ValidateCalls)
	})
}

func TestMockConfigValidator(t *testing.T) {
	t.Run("default behavior", func(t *testing.T) {
		validator := NewMockConfigValidator()
		cfg := config.Config{}

		errors := validator.Validate(cfg)
		assert.Nil(t, errors)
		assert.Len(t, validator.ValidateCalls, 1)
	})

	t.Run("custom behavior", func(t *testing.T) {
		validator := NewMockConfigValidator()
		validator.ValidateFunc = func(cfg config.Config) []error {
			return []error{
				errors.New("test error"),
			}
		}

		errs := validator.Validate(config.Config{})
		assert.Len(t, errs, 1)
		assert.Equal(t, "test error", errs[0].Error())
	})
}

func TestMockTemplateRegistry(t *testing.T) {
	t.Run("register and get template", func(t *testing.T) {
		registry := NewMockTemplateRegistry()

		err := registry.RegisterTemplate("test-template")
		require.NoError(t, err)
		assert.Len(t, registry.RegisterTemplateCalls, 1)

		template, err := registry.GetTemplate("test")
		require.NoError(t, err)
		assert.Nil(t, template)
		assert.Len(t, registry.GetTemplateCalls, 1)
	})

	t.Run("filter templates", func(t *testing.T) {
		registry := NewMockTemplateRegistry()

		templates := registry.GetTemplatesForProvider("openstack")
		assert.Empty(t, templates)
		assert.Len(t, registry.GetTemplatesForProviderCalls, 1)

		templates = registry.GetTemplatesForService("prometheus")
		assert.Empty(t, templates)
		assert.Len(t, registry.GetTemplatesForServiceCalls, 1)
	})

	t.Run("resolve dependencies", func(t *testing.T) {
		registry := NewMockTemplateRegistry()

		templates, err := registry.ResolveTemplateDependencies([]string{"template1", "template2"})
		require.NoError(t, err)
		assert.Empty(t, templates)
		assert.Len(t, registry.ResolveTemplateDependenciesCalls, 1)
	})
}

func TestMockGitOpsGenerator(t *testing.T) {
	t.Run("generate", func(t *testing.T) {
		generator := NewMockGitOpsGenerator()
		cfg := config.Config{}

		err := generator.Generate(context.Background(), cfg)
		require.NoError(t, err)
		assert.Len(t, generator.GenerateCalls, 1)
	})

	t.Run("dry run", func(t *testing.T) {
		generator := NewMockGitOpsGenerator()
		cfg := config.Config{}

		plan, err := generator.GenerateDryRun(context.Background(), cfg)
		require.NoError(t, err)
		assert.Nil(t, plan)
		assert.Len(t, generator.GenerateDryRunCalls, 1)
	})

	t.Run("rollback", func(t *testing.T) {
		generator := NewMockGitOpsGenerator()

		err := generator.Rollback(context.Background(), "checkpoint-1")
		require.NoError(t, err)
		assert.Len(t, generator.RollbackCalls, 1)
		assert.Equal(t, "checkpoint-1", generator.RollbackCalls[0])
	})
}

func TestMockGenerationStage(t *testing.T) {
	t.Run("stage operations", func(t *testing.T) {
		stage := NewMockGenerationStage("test-stage")

		assert.Equal(t, "test-stage", stage.Name())

		err := stage.Execute(context.Background(), "workspace")
		require.NoError(t, err)
		assert.Len(t, stage.ExecuteCalls, 1)

		err = stage.Rollback(context.Background(), "workspace")
		require.NoError(t, err)
		assert.Len(t, stage.RollbackCalls, 1)

		err = stage.Validate(context.Background(), "workspace")
		require.NoError(t, err)
		assert.Len(t, stage.ValidateCalls, 1)
	})
}

func TestMockServiceRegistry(t *testing.T) {
	t.Run("register and get service", func(t *testing.T) {
		registry := NewMockServiceRegistry()

		err := registry.RegisterService("test-service")
		require.NoError(t, err)
		assert.Len(t, registry.RegisterServiceCalls, 1)

		service, err := registry.GetService("test")
		require.NoError(t, err)
		assert.Nil(t, service)
		assert.Len(t, registry.GetServiceCalls, 1)
	})

	t.Run("get enabled services", func(t *testing.T) {
		registry := NewMockServiceRegistry()
		cfg := config.Config{}

		services := registry.GetEnabledServices(cfg)
		assert.Empty(t, services)
		assert.Len(t, registry.GetEnabledServicesCalls, 1)
	})

	t.Run("resolve and validate dependencies", func(t *testing.T) {
		registry := NewMockServiceRegistry()

		services, err := registry.ResolveDependencies([]string{"service1", "service2"})
		require.NoError(t, err)
		assert.Empty(t, services)
		assert.Len(t, registry.ResolveDependenciesCalls, 1)

		err = registry.ValidateDependencies([]string{"service1"})
		require.NoError(t, err)
		assert.Len(t, registry.ValidateDependenciesCalls, 1)
	})
}

func TestMockServicePlugin(t *testing.T) {
	t.Run("plugin operations", func(t *testing.T) {
		plugin := NewMockServicePlugin("test-plugin", "monitoring")

		assert.Equal(t, "test-plugin", plugin.Name())
		assert.Equal(t, "monitoring", plugin.Type())

		cfg := config.Config{}
		err := plugin.Validate(cfg)
		require.NoError(t, err)
		assert.Len(t, plugin.ValidateCalls, 1)

		err = plugin.Render(context.Background(), cfg, "workspace")
		require.NoError(t, err)
		assert.Len(t, plugin.RenderCalls, 1)

		status := plugin.Status(cfg)
		assert.Nil(t, status)
		assert.Len(t, plugin.StatusCalls, 1)
	})
}

func TestMockMigrationManager(t *testing.T) {
	t.Run("migration operations", func(t *testing.T) {
		manager := NewMockMigrationManager()

		version := manager.GetCurrentVersion()
		assert.Equal(t, "1.0.0", version)
		assert.Equal(t, 1, manager.GetCurrentVersionCalls)

		versions := manager.GetSupportedVersions()
		assert.Len(t, versions, 1)
		assert.Equal(t, 1, manager.GetSupportedVersionsCalls)

		cfg := config.Config{}
		migratedCfg, err := manager.MigrateConfig(cfg, "2.0.0")
		require.NoError(t, err)
		assert.NotNil(t, migratedCfg)
		assert.Len(t, manager.MigrateConfigCalls, 1)

		err = manager.ValidateMigrationPath("1.0.0", "2.0.0")
		require.NoError(t, err)
		assert.Len(t, manager.ValidateMigrationPathCalls, 1)
	})
}

func TestMockMCPServer(t *testing.T) {
	t.Run("server lifecycle", func(t *testing.T) {
		server := NewMockMCPServer()

		err := server.Start(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, server.StartCalls)

		err = server.Stop(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, server.StopCalls)
	})

	t.Run("registration", func(t *testing.T) {
		server := NewMockMCPServer()

		err := server.RegisterTools([]interface{}{"tool1", "tool2"})
		require.NoError(t, err)
		assert.Len(t, server.RegisterToolsCalls, 1)

		err = server.RegisterResources([]interface{}{"resource1"})
		require.NoError(t, err)
		assert.Len(t, server.RegisterResourcesCalls, 1)

		err = server.RegisterPrompts([]interface{}{"prompt1"})
		require.NoError(t, err)
		assert.Len(t, server.RegisterPromptsCalls, 1)

		err = server.SetAuthProvider("auth-provider")
		require.NoError(t, err)
		assert.Len(t, server.SetAuthProviderCalls, 1)
	})
}

func TestMockMCPSession(t *testing.T) {
	t.Run("session data", func(t *testing.T) {
		session := NewMockMCPSession("user-123", "org-456")

		assert.Equal(t, "user-123", session.UserID())
		assert.Equal(t, 1, session.UserIDCalls)

		assert.Equal(t, "org-456", session.Organization())
		assert.Equal(t, 1, session.OrganizationCalls)

		permissions := session.Permissions()
		assert.Empty(t, permissions)
		assert.Equal(t, 1, session.PermissionsCalls)

		auditLog := session.AuditLog()
		assert.Nil(t, auditLog)
		assert.Equal(t, 1, session.AuditLogCalls)

		configScope := session.ConfigScope()
		assert.Nil(t, configScope)
		assert.Equal(t, 1, session.ConfigScopeCalls)
	})
}

func TestMockAuthProvider(t *testing.T) {
	t.Run("authentication", func(t *testing.T) {
		provider := NewMockAuthProvider()

		credentials := map[string]string{"username": "test", "password": "secret"}
		session, err := provider.AuthenticateSession(context.Background(), credentials)
		require.NoError(t, err)
		assert.NotNil(t, session)
		assert.Len(t, provider.AuthenticateSessionCalls, 1)

		err = provider.ValidatePermission(session, "read")
		require.NoError(t, err)
		assert.Len(t, provider.ValidatePermissionCalls, 1)

		err = provider.RefreshSession(context.Background(), session)
		require.NoError(t, err)
		assert.Len(t, provider.RefreshSessionCalls, 1)
	})
}

func TestMockErrorAggregator(t *testing.T) {
	t.Run("error collection", func(t *testing.T) {
		aggregator := NewMockErrorAggregator()

		assert.False(t, aggregator.HasErrors())

		err1 := errors.New("error 1")
		aggregator.Add(err1)
		assert.Len(t, aggregator.AddCalls, 1)
		assert.True(t, aggregator.HasErrors())

		err2 := errors.New("error 2")
		aggregator.AddWithContext(err2, "context")
		assert.Len(t, aggregator.AddWithContextCalls, 1)

		errors := aggregator.Errors()
		assert.Len(t, errors, 2)

		reportErr := aggregator.Report()
		require.Error(t, reportErr)
		assert.Equal(t, err1, reportErr)
	})

	t.Run("no errors", func(t *testing.T) {
		aggregator := NewMockErrorAggregator()

		assert.False(t, aggregator.HasErrors())
		assert.Empty(t, aggregator.Errors())

		reportErr := aggregator.Report()
		assert.NoError(t, reportErr)
	})
}
