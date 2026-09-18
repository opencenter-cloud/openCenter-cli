package cluster

import (
	"context"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestConfigureServiceDefaultProviderRegistryResolvesMagnum(t *testing.T) {
	service := NewConfigureService(nil, nil, nil)
	provider, err := service.providers.Resolve(" MAGNUM ")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if provider.Name() != "magnum" {
		t.Fatalf("Resolve() provider = %q, want magnum", provider.Name())
	}
}

func TestMagnumConfigurePrompts(t *testing.T) {
	cfg := mustMagnumConfigureConfig(t)
	cfg.OpenCenter.Meta.Region = "region-from-meta"
	cfg.OpenCenter.Infrastructure.Cloud.Magnum.Region = ""

	provider := newMagnumConfigureOrchestrator()
	discovery, err := provider.Discover(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Discover() error = %v", err)
	}
	if len(discovery.Warnings) != 0 || len(discovery.Metadata) != 0 {
		t.Fatalf("Discover() = %#v, want empty result", discovery)
	}

	prompts := provider.Prompts(cfg, discovery)
	wantIDs := []string{
		"magnum.auth_url",
		"magnum.region",
		"magnum.project_id",
		"magnum.application_credential_id",
		"magnum.application_credential_secret",
		"magnum.cluster_template",
		"magnum.keypair",
		"magnum.master_flavor_id",
		"magnum.node_flavor_id",
		"magnum.master_count",
		"magnum.worker_count",
		"magnum.create_timeout",
		"magnum.insecure",
	}
	if got := promptIDs(prompts); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("prompt IDs = %v, want %v", got, wantIDs)
	}

	for _, prompt := range prompts {
		if strings.Contains(prompt.ID, "image") || strings.Contains(prompt.ID, "network") {
			t.Fatalf("unexpected image/network prompt: %#v", prompt)
		}
	}
	secret := prompts[4]
	if secret.Kind != orchestration.PromptKindSecret {
		t.Fatalf("application credential secret kind = %q, want secret", secret.Kind)
	}
	if !prompts[1].Required || prompts[1].Default != "region-from-meta" {
		t.Fatalf("region prompt = %#v, want required meta-region default", prompts[1])
	}
	if !prompts[0].Required || !prompts[5].Required {
		t.Fatal("auth URL and cluster template should be required")
	}
}

func TestMagnumConfigureApplyAnswersPatchesConfig(t *testing.T) {
	cfg := mustMagnumConfigureConfig(t)
	provider := newMagnumConfigureOrchestrator()
	changes, err := provider.ApplyAnswers(cfg, orchestration.PromptAnswers{
		"magnum.auth_url":                      "https://identity.example.com/v3",
		"magnum.region":                        "DFW3",
		"magnum.project_id":                    "project-id",
		"magnum.application_credential_id":     "application-id",
		"magnum.application_credential_secret": "secret-value",
		"magnum.cluster_template":              "kubernetes-template",
		"magnum.keypair":                       "cluster-key",
		"magnum.master_flavor_id":              "master-flavor",
		"magnum.node_flavor_id":                "worker-flavor",
		"magnum.master_count":                  "3",
		"magnum.worker_count":                  "5",
		"magnum.create_timeout":                "120",
		"magnum.insecure":                      "true",
	})
	if err != nil {
		t.Fatalf("ApplyAnswers() error = %v", err)
	}

	if err := applyChangeSet(cfg, changes); err != nil {
		t.Fatalf("applyChangeSet() error = %v", err)
	}
	magnum := cfg.OpenCenter.Infrastructure.Cloud.Magnum
	if magnum == nil || magnum.AuthURL != "https://identity.example.com/v3" || magnum.Region != "DFW3" || magnum.ProjectID != "project-id" || magnum.ClusterTemplate != "kubernetes-template" {
		t.Fatalf("unexpected Magnum configuration: %#v", magnum)
	}
	if magnum.ApplicationCredentialID != "application-id" || magnum.ApplicationCredentialSecret != "secret-value" || !magnum.Insecure {
		t.Fatalf("unexpected Magnum credentials/TLS configuration: %#v", magnum)
	}
	if magnum.Keypair != "cluster-key" || magnum.MasterFlavorID != "master-flavor" || magnum.NodeFlavorID != "worker-flavor" || magnum.CreateTimeout != 120 {
		t.Fatalf("unexpected Magnum deployment configuration: %#v", magnum)
	}
	if cfg.OpenCenter.Meta.Region != "DFW3" || cfg.OpenCenter.Infrastructure.Compute.MasterCount != 3 || cfg.OpenCenter.Infrastructure.Compute.WorkerCount != 5 {
		t.Fatalf("unexpected generic configuration: region=%q masters=%d workers=%d", cfg.OpenCenter.Meta.Region, cfg.OpenCenter.Infrastructure.Compute.MasterCount, cfg.OpenCenter.Infrastructure.Compute.WorkerCount)
	}

	secretMasked := false
	for _, patch := range changes.Patches {
		if patch.Path == "opencenter.infrastructure.cloud.magnum.application_credential_secret" {
			secretMasked = patch.Masked
		}
	}
	if !secretMasked {
		t.Fatal("application credential secret patch should be masked")
	}
}

func TestMagnumConfigureRejectsInvalidNumbers(t *testing.T) {
	provider := newMagnumConfigureOrchestrator()
	cfg := mustMagnumConfigureConfig(t)
	for _, answerID := range []string{"magnum.master_count", "magnum.worker_count", "magnum.create_timeout"} {
		_, err := provider.ApplyAnswers(cfg, orchestration.PromptAnswers{answerID: "not-a-number"})
		if err == nil {
			t.Errorf("ApplyAnswers(%s) error = nil, want error", answerID)
		}
	}
}

func TestMagnumConfigureCapabilities(t *testing.T) {
	requests := newMagnumConfigureOrchestrator().CapabilityRequests(nil, orchestration.DiscoveryResult{})
	if len(requests) != 2 || requests[0].Name != "git-auth" || requests[1].Name != "object-storage" {
		t.Fatalf("CapabilityRequests() = %#v, want git-auth and object-storage", requests)
	}
}

func mustMagnumConfigureConfig(t *testing.T) *v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("guided-magnum", "magnum")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Infrastructure.Cloud.Magnum = &v2.MagnumCloudConfig{}
	return cfg
}
