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
		if storageType == "" {
			prompts = append(prompts, orchestration.PromptSpec{
				ID:       "storage.loki.type",
				Group:    configureGroupStorage,
				Kind:     orchestration.PromptKindSelect,
				Label:    "Loki storage backend",
				Default:  h.defaultStorageProvider("loki", providerCtx),
				Required: true,
				Options: []orchestration.PromptOption{
					{Value: "swift", Label: "Swift"},
					{Value: "s3", Label: "S3"},
				},
			})
		} else if storageType == "swift" {
			if strings.TrimSpace(loki.SwiftContainerName) == "" || strings.TrimSpace(cfg.Secrets.Loki.SwiftApplicationCredentialSecret) == "" {
				prompts = append(prompts,
					orchestration.PromptSpec{ID: "storage.loki.swift_container", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Loki Swift container", Default: firstNonEmptyString(strings.TrimSpace(loki.SwiftContainerName), fmt.Sprintf("%s-loki", cfg.OpenCenter.Meta.Name)), Required: true},
				)
				if strings.TrimSpace(cfg.Secrets.Loki.SwiftApplicationCredentialSecret) == "" {
					prompts = append(prompts,
						orchestration.PromptSpec{ID: "storage.loki.swift_secret", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Loki Swift application credential secret", Required: true},
					)
				}
			}
		} else if storageType == "s3" {
			prompts = append(prompts, s3PromptsForLoki(cfg, loki)...)
		}
	}

	if tempo := enabledTempoConfig(cfg); tempo != nil {
		storageType := strings.TrimSpace(tempo.StorageType)
		if storageType == "" {
			prompts = append(prompts, orchestration.PromptSpec{
				ID:       "storage.tempo.type",
				Group:    configureGroupStorage,
				Kind:     orchestration.PromptKindSelect,
				Label:    "Tempo storage backend",
				Default:  h.defaultStorageProvider("tempo", providerCtx),
				Required: true,
				Options: []orchestration.PromptOption{
					{Value: "swift", Label: "Swift"},
					{Value: "s3", Label: "S3"},
				},
			})
		} else if storageType == "swift" {
			if strings.TrimSpace(tempo.SwiftContainerName) == "" || strings.TrimSpace(cfg.Secrets.Tempo.SwiftApplicationCredentialSecret) == "" {
				prompts = append(prompts,
					orchestration.PromptSpec{ID: "storage.tempo.swift_container", Group: configureGroupStorage, Kind: orchestration.PromptKindInput, Label: "Tempo Swift container", Default: firstNonEmptyString(strings.TrimSpace(tempo.SwiftContainerName), fmt.Sprintf("%s-tempo", cfg.OpenCenter.Meta.Name)), Required: true},
				)
				if strings.TrimSpace(cfg.Secrets.Tempo.SwiftApplicationCredentialSecret) == "" {
					prompts = append(prompts,
						orchestration.PromptSpec{ID: "storage.tempo.swift_secret", Group: configureGroupStorage, Kind: orchestration.PromptKindSecret, Label: "Tempo Swift application credential secret", Required: true},
					)
				}
			}
		} else if storageType == "s3" {
			prompts = append(prompts, s3PromptsForTempo(cfg, tempo)...)
		}
	}

	return prompts
}

func (h *objectStorageCapabilityHandler) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers, providerCtx orchestration.ProviderContext) (orchestration.ChangeSet, error) {
	changes := orchestration.ChangeSet{}
	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack

	if value := strings.TrimSpace(answers["storage.loki.type"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_storage_type", Label: "Loki storage backend", Value: value})
	}
	lokiType := strings.TrimSpace(answers["storage.loki.type"])
	if lokiType == "" {
		if loki := enabledLokiConfig(cfg); loki != nil {
			lokiType = strings.TrimSpace(loki.StorageType)
		}
	}
	switch lokiType {
	case "swift":
		container := strings.TrimSpace(answers["storage.loki.swift_container"])
		if container != "" && openstackCfg != nil {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_bucket_name", Label: "Loki Swift container", Value: container},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_auth_url", Label: "Loki Swift auth URL", Value: strings.TrimSpace(openstackCfg.AuthURL)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_region", Label: "Loki Swift region", Value: strings.TrimSpace(openstackCfg.Region)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_application_credential_id", Label: "Loki Swift application credential ID", Value: strings.TrimSpace(openstackCfg.ApplicationCredentialID)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_container_name", Label: "Loki Swift container", Value: container},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_user_domain_name", Label: "Loki Swift user domain", Value: firstNonEmptyString(strings.TrimSpace(openstackCfg.UserDomainName), strings.TrimSpace(openstackCfg.Domain))},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.swift_domain_name", Label: "Loki Swift domain", Value: firstNonEmptyString(strings.TrimSpace(openstackCfg.DomainName), strings.TrimSpace(openstackCfg.Domain))},
			)
		}
		if secret, ok := answers["storage.loki.swift_secret"]; ok && strings.TrimSpace(secret) != "" {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.loki.swift_application_credential_secret", Label: "Loki Swift secret", Value: strings.TrimSpace(secret), Masked: true},
			)
		}
	case "s3":
		if value := strings.TrimSpace(answers["storage.loki.s3_bucket"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_bucket_name", Label: "Loki S3 bucket", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_endpoint"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_s3_endpoint", Label: "Loki S3 endpoint", Value: value})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_region"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_s3_region", Label: "Loki S3 region", Value: value})
		}
		if value, ok := answers["storage.loki.s3_force_path_style"]; ok {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.loki.loki_s3_force_path_style", Label: "Loki S3 path style", Value: normalizeBoolString(value)})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_access_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.loki.s3_access_key_id", Label: "Loki S3 access key", Value: value, Masked: true})
		}
		if value := strings.TrimSpace(answers["storage.loki.s3_secret_key"]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.loki.s3_secret_access_key", Label: "Loki S3 secret key", Value: value, Masked: true})
		}
	}

	if value := strings.TrimSpace(answers["storage.tempo.type"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.storage_type", Label: "Tempo storage backend", Value: value})
	}
	tempoType := strings.TrimSpace(answers["storage.tempo.type"])
	if tempoType == "" {
		if tempo := enabledTempoConfig(cfg); tempo != nil {
			tempoType = strings.TrimSpace(tempo.StorageType)
		}
	}
	switch tempoType {
	case "swift":
		container := strings.TrimSpace(answers["storage.tempo.swift_container"])
		if container != "" && openstackCfg != nil {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.bucket_name", Label: "Tempo Swift container", Value: container},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_auth_url", Label: "Tempo Swift auth URL", Value: strings.TrimSpace(openstackCfg.AuthURL)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_region", Label: "Tempo Swift region", Value: strings.TrimSpace(openstackCfg.Region)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_application_credential_id", Label: "Tempo Swift application credential ID", Value: strings.TrimSpace(openstackCfg.ApplicationCredentialID)},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_container_name", Label: "Tempo Swift container", Value: container},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_user_domain_name", Label: "Tempo Swift user domain", Value: firstNonEmptyString(strings.TrimSpace(openstackCfg.UserDomainName), strings.TrimSpace(openstackCfg.Domain))},
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "opencenter.services.tempo.swift_domain_name", Label: "Tempo Swift domain", Value: firstNonEmptyString(strings.TrimSpace(openstackCfg.DomainName), strings.TrimSpace(openstackCfg.Domain))},
			)
		}
		if secret, ok := answers["storage.tempo.swift_secret"]; ok && strings.TrimSpace(secret) != "" {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupStorage, Path: "secrets.tempo.swift_application_credential_secret", Label: "Tempo Swift secret", Value: strings.TrimSpace(secret), Masked: true},
			)
		}
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

func (h *objectStorageCapabilityHandler) defaultStorageProvider(service string, providerCtx orchestration.ProviderContext) string {
	if h.registry == nil {
		return "swift"
	}
	infraProvider := configservices.InfrastructureProvider(strings.ToLower(strings.TrimSpace(providerCtx.Provider)))
	provider, err := h.registry.GetDefaultProvider(service, "storage", infraProvider)
	if err != nil {
		return "swift"
	}
	return string(provider)
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
