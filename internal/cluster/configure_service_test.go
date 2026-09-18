package cluster

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func TestConfigureServiceMagnumInitToGuidedConfigurePersistsAnswers(t *testing.T) {
	tmpDir := t.TempDir()
	pathResolver := paths.NewPathResolver(tmpDir)
	validationEngine := setupValidationEngine(t)
	configManager, err := config.NewConfigManager(filepath.Join(tmpDir, "cli-settings.yaml"))
	if err != nil {
		t.Fatalf("create config manager: %v", err)
	}

	initService := NewInitService(pathResolver, validationEngine, configManager)
	initResult, err := initService.Initialize(context.Background(), InitOptions{
		ClusterName: "guided-magnum",
		Provider:    "magnum",
		NoGitInit:   true,
		NoKeyGen:    true,
	})
	if err != nil {
		t.Fatalf("initialize Magnum config: %v", err)
	}
	if initResult.Config.OpenCenter.Infrastructure.Cloud.OpenStack != nil || initResult.Config.OpenCenter.Infrastructure.Cloud.Magnum == nil {
		t.Fatalf("init produced wrong cloud sections: %#v", initResult.Config.OpenCenter.Infrastructure.Cloud)
	}

	capabilities := orchestration.NewCapabilityRegistry(
		&disabledConfigureCapability{name: "git-auth"},
		&disabledConfigureCapability{name: "object-storage"},
	)
	service := NewConfigureServiceWithDeps(
		pathResolver,
		validationEngine,
		configManager,
		nil,
		nil,
		orchestration.NewProviderRegistry(newMagnumConfigureOrchestrator()),
		capabilities,
	)
	runner := &magnumConfigureTestRunner{answers: orchestration.PromptAnswers{
		"magnum.auth_url":                      "https://keystone.example.com/v3",
		"magnum.region":                        "RegionOne",
		"magnum.project_id":                    "project-id",
		"magnum.application_credential_id":     "application-credential-id",
		"magnum.application_credential_secret": "application-credential-secret",
		"magnum.cluster_template":              "kubernetes-template",
		"magnum.master_flavor_id":              "master-flavor",
		"magnum.node_flavor_id":                "worker-flavor",
		"magnum.master_count":                  "3",
		"magnum.worker_count":                  "3",
	}}

	result, err := service.Configure(context.Background(), ConfigureOptions{Identifier: "guided-magnum"}, runner)
	if err != nil {
		t.Fatalf("guided Magnum configure: %v", err)
	}
	if !runner.prompted || len(runner.promptIDs) == 0 {
		t.Fatal("guided configure did not present Magnum prompts")
	}
	for _, want := range []string{"magnum.auth_url", "magnum.application_credential_secret", "magnum.cluster_template"} {
		if !containsString(runner.promptIDs, want) {
			t.Fatalf("guided configure prompts = %v, missing %q", runner.promptIDs, want)
		}
	}

	magnum := result.Config.OpenCenter.Infrastructure.Cloud.Magnum
	if magnum == nil || magnum.AuthURL != "https://keystone.example.com/v3" || magnum.ApplicationCredentialSecret != "application-credential-secret" || magnum.ClusterTemplate != "kubernetes-template" {
		t.Fatalf("configured Magnum values = %#v", magnum)
	}
	persisted, err := os.ReadFile(result.ConfigPath)
	if err != nil {
		t.Fatalf("read persisted config: %v", err)
	}
	persistedCfg, err := v2.DecodePublicConfig(persisted)
	if err != nil {
		t.Fatalf("decode persisted config: %v", err)
	}
	if persistedCfg.OpenCenter.Infrastructure.Cloud.Magnum == nil || persistedCfg.OpenCenter.Infrastructure.Cloud.Magnum.ClusterTemplate != "kubernetes-template" {
		t.Fatalf("persisted Magnum config = %#v", persistedCfg.OpenCenter.Infrastructure.Cloud.Magnum)
	}
}

func TestConfigureServiceRejectsOrdinaryInvalidConfig(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "invalid-config.yaml")
	cfg, err := v2.NewV2Default("ordinary-invalid", "kind")
	if err != nil {
		t.Fatalf("create config: %v", err)
	}
	cfg.OpenTofu.Backend.Local.Path = ""
	data, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(tmpFile, data, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	service := NewConfigureService(nil, nil, nil)
	if _, err := service.loadV2Config(tmpFile); err == nil {
		t.Fatal("ordinary invalid config unexpectedly entered guided loading")
	}
}

type magnumConfigureTestRunner struct {
	answers   orchestration.PromptAnswers
	prompted  bool
	promptIDs []string
}

func (r *magnumConfigureTestRunner) Message(string) {}
func (r *magnumConfigureTestRunner) Warning(string) {}
func (r *magnumConfigureTestRunner) Prompt(_ context.Context, prompts []orchestration.PromptSpec) (orchestration.PromptAnswers, error) {
	r.prompted = true
	r.promptIDs = promptIDs(prompts)
	return r.answers, nil
}
func (r *magnumConfigureTestRunner) Review(context.Context, orchestration.ReviewSpec) (bool, error) {
	return true, nil
}

type disabledConfigureCapability struct {
	name string
}

func (h *disabledConfigureCapability) Name() string { return h.name }
func (h *disabledConfigureCapability) Applies(*v2.Config, orchestration.ProviderContext) bool {
	return false
}
func (h *disabledConfigureCapability) Discover(context.Context, *v2.Config, orchestration.ProviderContext) (orchestration.DiscoveryResult, error) {
	return orchestration.DiscoveryResult{}, nil
}
func (h *disabledConfigureCapability) Prompts(*v2.Config, orchestration.ProviderContext, orchestration.DiscoveryResult) []orchestration.PromptSpec {
	return nil
}
func (h *disabledConfigureCapability) ApplyAnswers(*v2.Config, orchestration.PromptAnswers, orchestration.ProviderContext) (orchestration.ChangeSet, error) {
	return orchestration.ChangeSet{}, nil
}
