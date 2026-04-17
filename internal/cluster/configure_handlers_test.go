package cluster

import (
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestDNSCapabilityHandlerDesignateNeedsNoAdditionalSecrets(t *testing.T) {
	cfg, err := v2.NewV2Default("guided-designate", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	certManager := certManagerConfig(cfg)
	if certManager == nil {
		t.Fatal("expected cert-manager config")
	}
	certManager.DNSProvider = string(configservices.DNSProviderDesignate)

	handler := newDNSCapabilityHandler(configservices.GetProviderRegistry())
	prompts := handler.Prompts(cfg, orchestration.ProviderContext{
		Provider:    "openstack",
		ClusterName: cfg.ClusterName(),
	}, orchestration.DiscoveryResult{})
	if len(prompts) != 0 {
		t.Fatalf("expected no designate prompts, got %#v", prompts)
	}
}

func TestObjectStorageCapabilityHandlerUsesGlobalAWSApplicationCredentials(t *testing.T) {
	cfg, err := v2.NewV2Default("guided-storage", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	cfg.OpenCenter.Services["loki"] = &configservices.LokiConfig{
		BaseConfig:  configservices.BaseConfig{Enabled: true},
		StorageType: "s3",
	}
	cfg.Secrets.Global.AWS.Application.AccessKey = "AKIA-GLOBAL"
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-secret"

	handler := newObjectStorageCapabilityHandler(configservices.GetProviderRegistry())
	prompts := handler.Prompts(cfg, orchestration.ProviderContext{
		Provider: "openstack",
	}, orchestration.DiscoveryResult{})

	ids := promptIDs(prompts)
	assertPromptContains(t, ids, "storage.loki.s3_bucket")
	assertPromptContains(t, ids, "storage.loki.s3_endpoint")
	assertPromptContains(t, ids, "storage.loki.s3_region")
	assertPromptContains(t, ids, "storage.loki.s3_force_path_style")
	assertPromptAbsent(t, ids, "storage.loki.s3_access_key")
	assertPromptAbsent(t, ids, "storage.loki.s3_secret_key")
}

func TestObjectStorageCapabilityHandlerSkipsS3PromptsWhenConfigIsComplete(t *testing.T) {
	cfg, err := v2.NewV2Default("guided-storage-complete", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	cfg.OpenCenter.Services["tempo"] = &configservices.TempoConfig{
		BaseConfig:       configservices.BaseConfig{Enabled: true},
		StorageType:      "s3",
		BucketName:       "tempo-traces",
		S3Endpoint:       "https://s3.example.com",
		S3Region:         "us-east-1",
		S3ForcePathStyle: true,
	}
	cfg.Secrets.Global.AWS.Application.AccessKey = "AKIA-GLOBAL"
	cfg.Secrets.Global.AWS.Application.SecretAccessKey = "global-secret"

	handler := newObjectStorageCapabilityHandler(configservices.GetProviderRegistry())
	prompts := handler.Prompts(cfg, orchestration.ProviderContext{
		Provider: "openstack",
	}, orchestration.DiscoveryResult{})
	if len(prompts) != 0 {
		t.Fatalf("expected no prompts when Tempo S3 config is complete, got %#v", prompts)
	}
}

func promptIDs(prompts []orchestration.PromptSpec) []string {
	ids := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		ids = append(ids, prompt.ID)
	}
	return ids
}

func assertPromptContains(t *testing.T, ids []string, want string) {
	t.Helper()
	for _, id := range ids {
		if id == want {
			return
		}
	}
	t.Fatalf("expected prompt %q in %v", want, ids)
}

func assertPromptAbsent(t *testing.T, ids []string, want string) {
	t.Helper()
	for _, id := range ids {
		if id == want {
			t.Fatalf("did not expect prompt %q in %v", want, ids)
		}
	}
}
