package cluster

import (
	"context"
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type objectStorageCapabilityHandler struct {
	registry *configservices.ServiceProviderRegistry
}

func newObjectStorageCapabilityHandler(registry *configservices.ServiceProviderRegistry) orchestration.CapabilityHandler {
	return &objectStorageCapabilityHandler{registry: registry}
}

func (h *objectStorageCapabilityHandler) Name() string {
	return "object-storage"
}

func (h *objectStorageCapabilityHandler) Applies(cfg *v2.Config, providerCtx orchestration.ProviderContext) bool {
	return enabledLokiConfig(cfg) != nil || enabledTempoConfig(cfg) != nil
}

func (h *objectStorageCapabilityHandler) Discover(ctx context.Context, cfg *v2.Config, providerCtx orchestration.ProviderContext) (orchestration.DiscoveryResult, error) {
	return orchestration.DiscoveryResult{}, nil
}

func (h *objectStorageCapabilityHandler) Prompts(cfg *v2.Config, providerCtx orchestration.ProviderContext, discovery orchestration.DiscoveryResult) []orchestration.PromptSpec {
	prompts := make([]orchestration.PromptSpec, 0)

	if loki := enabledLokiConfig(cfg); loki != nil {
		storageType := strings.TrimSpace(loki.StorageType)
		// Platform bulk storage is S3-compatible only (Swift is no longer
		// supported). Always drive the guided flow to the S3 prompts; a stored
		// non-s3 storage_type is treated as unset and re-prompted as S3.
		if storageType != "" && storageType != "s3" {
			storageType = ""
		}
		prompts = append(prompts, s3PromptsForLoki(cfg, loki)...)
	}

	if tempo := enabledTempoConfig(cfg); tempo != nil {
		storageType := strings.TrimSpace(tempo.StorageType)
		// Platform bulk storage is S3-compatible only (Swift is no longer
		// supported). Always drive the guided flow to the S3 prompts; a stored
		// non-s3 storage_type is treated as unset and re-prompted as S3.
		if storageType != "" && storageType != "s3" {
			storageType = ""
		}
		prompts = append(prompts, s3PromptsForTempo(cfg, tempo)...)
	}

	return prompts
}

func (h *objectStorageCapabilityHandler) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers, providerCtx orchestration.ProviderContext) (orchestration.ChangeSet, error) {
	changes := orchestration.ChangeSet{}
	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack

	// Platform bulk storage is S3-compatible only; always record storage_type=s3.
	changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.storage_type", Label: "Loki storage backend", Value: "s3"})
	lokiType := "s3"
	_ = openstackCfg
	switch lokiType {
	case "s3":
		if value := strings.TrimSpace(answers["storage.loki.s3_bucket"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.bucket_name", Label: "Loki S3 bucket", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_endpoint"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.s3_endpoint", Label: "Loki S3 endpoint", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_region"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.s3_region", Label: "Loki S3 region", Value: value})
		}
		if value, ok := answers["storage.loki.s3_force_path_style"]; ok {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.s3_force_path_style", Label: "Loki S3 path style", Value: normalizeBoolString(value)})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_access_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.loki.s3_access_key_id", Label: "Loki S3 access key", Value: value, Masked: true})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_secret_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.loki.s3_secret_access_key", Label: "Loki S3 secret key", Value: value, Masked: true})
		}
	}

	// Platform bulk storage is S3-compatible only; always record storage_type=s3.
	changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.storage_type", Label: "Tempo storage backend", Value: "s3"})
	tempoType := "s3"
	switch tempoType {
	case "s3":
		if value := strings.TrimSpace(answers["storage.tempo.s3_bucket"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.bucket_name", Label: "Tempo S3 bucket", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.tempo.s3_endpoint"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.s3_endpoint", Label: "Tempo S3 endpoint", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.tempo.s3_region"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.s3_region", Label: "Tempo S3 region", Value: value})
		}
		if value, ok := answers["storage.tempo.s3_force_path_style"]; ok {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.s3_force_path_style", Label: "Tempo S3 path style", Value: normalizeBoolString(value)})
		}
		if value := strings.TrimSpace(answers["storage.tempo.s3_access_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.tempo.access_key", Label: "Tempo S3 access key", Value: value, Masked: true})
		}
		if value := strings.TrimSpace(answers["storage.tempo.s3_secret_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.tempo.secret_key", Label: "Tempo S3 secret key", Value: value, Masked: true})
		}
	}

	return changes, nil
}

func s3PromptsForLoki(cfg *v2.Config, loki *configservices.LokiConfig) []orchestration.PromptSpec {
	prompts := make([]orchestration.PromptSpec, 0)

	if strings.TrimSpace(loki.BucketName) == "" || strings.TrimSpace(loki.S3Endpoint) == "" || strings.TrimSpace(loki.S3Region) == "" {
		prompts = append(prompts,
			orchestration.PromptSpec{ID: "storage.loki.s3_bucket", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Loki S3 bucket", Default: firstNonEmptyString(strings.TrimSpace(loki.BucketName), fmt.Sprintf("%s-loki", cfg.OpenCenter.Meta.Name)), Required: true},
			orchestration.PromptSpec{ID: "storage.loki.s3_endpoint", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Loki S3 endpoint", Default: strings.TrimSpace(loki.S3Endpoint), Required: true},
			orchestration.PromptSpec{ID: "storage.loki.s3_region", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Loki S3 region", Default: firstNonEmptyString(strings.TrimSpace(loki.S3Region), cfg.OpenCenter.Meta.Region), Required: true},
			orchestration.PromptSpec{ID: "storage.loki.s3_force_path_style", Group: configureGroupStorage, Kind: orchestration.PromptKindConfirm, Label: "Use S3 path-style addressing for Loki?", Default: fmt.Sprintf("%t", loki.S3ForcePathStyle)},
		)
	}

	if !hasUsableAWSApplicationCredentials(cfg) && (strings.TrimSpace(cfg.Secrets.Loki.S3AccessKeyID) == "" || strings.TrimSpace(cfg.Secrets.Loki.S3SecretAccessKey) == "") {
		prompts = append(prompts,
			orchestration.PromptSpec{ID: "storage.loki.s3_access_key", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Loki S3 access key", Required: true},
			orchestration.PromptSpec{ID: "storage.loki.s3_secret_key", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Loki S3 secret key", Required: true},
		)
	}

	return prompts
}

func s3PromptsForTempo(cfg *v2.Config, tempo *configservices.TempoConfig) []orchestration.PromptSpec {
	prompts := make([]orchestration.PromptSpec, 0)

	if strings.TrimSpace(tempo.BucketName) == "" || strings.TrimSpace(tempo.S3Endpoint) == "" || strings.TrimSpace(tempo.S3Region) == "" {
		prompts = append(prompts,
			orchestration.PromptSpec{ID: "storage.tempo.s3_bucket", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Tempo S3 bucket", Default: firstNonEmptyString(strings.TrimSpace(tempo.BucketName), fmt.Sprintf("%s-tempo", cfg.OpenCenter.Meta.Name)), Required: true},
			orchestration.PromptSpec{ID: "storage.tempo.s3_endpoint", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Tempo S3 endpoint", Default: strings.TrimSpace(tempo.S3Endpoint), Required: true},
			orchestration.PromptSpec{ID: "storage.tempo.s3_region", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Tempo S3 region", Default: firstNonEmptyString(strings.TrimSpace(tempo.S3Region), cfg.OpenCenter.Meta.Region), Required: true},
			orchestration.PromptSpec{ID: "storage.tempo.s3_force_path_style", Group: configureGroupStorage, Kind: orchestration.PromptKindConfirm, Label: "Use S3 path-style addressing for Tempo?", Default: fmt.Sprintf("%t", tempo.S3ForcePathStyle)},
		)
	}

	if !hasUsableAWSApplicationCredentials(cfg) && (strings.TrimSpace(cfg.Secrets.Tempo.AccessKey) == "" || strings.TrimSpace(cfg.Secrets.Tempo.SecretKey) == "") {
		prompts = append(prompts,
			orchestration.PromptSpec{ID: "storage.tempo.s3_access_key", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Tempo S3 access key", Required: true},
			orchestration.PromptSpec{ID: "storage.tempo.s3_secret_key", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Tempo S3 secret key", Required: true},
		)
	}

	return prompts
}

func hasUsableAWSApplicationCredentials(cfg *v2.Config) bool {
	accessKey, secretKey := cfg.GetAWSApplicationCredentials()
	return strings.TrimSpace(accessKey) != "" && strings.TrimSpace(secretKey) != ""
}

func enabledLokiConfig(cfg *v2.Config) *configservices.LokiConfig {
	if cfg == nil {
		return nil
	}
	serviceAny, ok := cfg.OpenCenter.Services["loki"]
	if !ok {
		return nil
	}
	switch typed := serviceAny.(type) {
	case *configservices.LokiConfig:
		if typed.Enabled {
			return typed
		}
	case *configservices.DefaultServiceConfig:
		if typed.Enabled {
			converted := &configservices.LokiConfig{BaseConfig: typed.BaseConfig}
			cfg.OpenCenter.Services["loki"] = converted
			return converted
		}
	}
	return nil
}

func enabledTempoConfig(cfg *v2.Config) *configservices.TempoConfig {
	if cfg == nil {
		return nil
	}
	serviceAny, ok := cfg.OpenCenter.Services["tempo"]
	if !ok {
		return nil
	}
	switch typed := serviceAny.(type) {
	case *configservices.TempoConfig:
		if typed.Enabled {
			return typed
		}
	case *configservices.DefaultServiceConfig:
		if typed.Enabled {
			converted := &configservices.TempoConfig{BaseConfig: typed.BaseConfig}
			cfg.OpenCenter.Services["tempo"] = converted
			return converted
		}
	}
	return nil
}
