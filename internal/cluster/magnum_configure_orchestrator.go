package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type magnumConfigureOrchestrator struct{}

func newMagnumConfigureOrchestrator() orchestration.ProviderOrchestrator {
	return &magnumConfigureOrchestrator{}
}

func (o *magnumConfigureOrchestrator) Name() string {
	return "magnum"
}

func (o *magnumConfigureOrchestrator) Supports(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "magnum")
}

// Magnum discovery is intentionally a no-op. Cluster templates are deployment-
// specific and must be entered manually rather than discovered from a cloud.
func (o *magnumConfigureOrchestrator) Discover(_ context.Context, _ *v2.Config) (orchestration.DiscoveryResult, error) {
	return orchestration.DiscoveryResult{}, nil
}

func (o *magnumConfigureOrchestrator) Prompts(cfg *v2.Config, _ orchestration.DiscoveryResult) []orchestration.PromptSpec {
	magnumCfg := magnumConfigOrEmpty(cfg)
	return []orchestration.PromptSpec{
		{ID: "magnum.auth_url", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Magnum auth URL", Default: strings.TrimSpace(magnumCfg.AuthURL), Required: true},
		{ID: "magnum.region", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Magnum region", Default: firstNonEmptyString(strings.TrimSpace(magnumCfg.Region), strings.TrimSpace(cfg.OpenCenter.Meta.Region)), Required: true},
		{ID: "magnum.project_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Magnum project ID", Default: strings.TrimSpace(magnumCfg.ProjectID), Required: true},
		{ID: "magnum.application_credential_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Application credential ID", Default: strings.TrimSpace(magnumCfg.ApplicationCredentialID), Required: true},
		{ID: "magnum.application_credential_secret", Group: configureGroupProvider, Kind: orchestration.PromptKindSecret, Label: "Application credential secret", Default: strings.TrimSpace(magnumCfg.ApplicationCredentialSecret), Required: true},
		{ID: "magnum.cluster_template", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Cluster template", Default: strings.TrimSpace(magnumCfg.ClusterTemplate), Required: true},
		{ID: "magnum.keypair", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Keypair (optional)", Default: strings.TrimSpace(magnumCfg.Keypair)},
		{ID: "magnum.master_flavor_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Control plane flavor", Default: strings.TrimSpace(magnumCfg.MasterFlavorID), Required: true},
		{ID: "magnum.node_flavor_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Worker flavor", Default: strings.TrimSpace(magnumCfg.NodeFlavorID), Required: true},
		{ID: "magnum.master_count", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Control plane count", Default: strconv.Itoa(cfg.OpenCenter.Infrastructure.Compute.MasterCount), Required: true, Validate: validatePositiveInt},
		{ID: "magnum.worker_count", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Worker count", Default: strconv.Itoa(cfg.OpenCenter.Infrastructure.Compute.WorkerCount), Required: true, Validate: validatePositiveInt},
		{ID: "magnum.create_timeout", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Create timeout (optional)", Default: strconv.Itoa(magnumCfg.CreateTimeout), Validate: validatePositiveInt},
		{ID: "magnum.insecure", Group: configureGroupProvider, Kind: orchestration.PromptKindConfirm, Label: "Allow insecure TLS to Keystone?", Default: strconv.FormatBool(magnumCfg.Insecure)},
	}
}

func (o *magnumConfigureOrchestrator) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers) (orchestration.ChangeSet, error) {
	if cfg == nil {
		return orchestration.ChangeSet{}, fmt.Errorf("config is nil")
	}

	changes := orchestration.ChangeSet{}
	addStringPatch := func(answerID, path, label string, masked bool) {
		if value := strings.TrimSpace(answers[answerID]); value != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{
				Group:  configureGroupProvider,
				Path:   path,
				Label:  label,
				Value:  value,
				Masked: masked,
			})
		}
	}

	addStringPatch("magnum.auth_url", "opencenter.infrastructure.cloud.magnum.auth_url", "Auth URL", false)
	if value := strings.TrimSpace(answers["magnum.region"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.meta.region", Label: "Region", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.magnum.region", Label: "Magnum region", Value: value},
		)
	}
	addStringPatch("magnum.project_id", "opencenter.infrastructure.cloud.magnum.project_id", "Project ID", false)
	addStringPatch("magnum.application_credential_id", "opencenter.infrastructure.cloud.magnum.application_credential_id", "Application credential ID", false)
	addStringPatch("magnum.application_credential_secret", "opencenter.infrastructure.cloud.magnum.application_credential_secret", "Application credential secret", true)
	addStringPatch("magnum.cluster_template", "opencenter.infrastructure.cloud.magnum.cluster_template", "Cluster template", false)
	addStringPatch("magnum.keypair", "opencenter.infrastructure.cloud.magnum.keypair", "Keypair", false)
	addStringPatch("magnum.master_flavor_id", "opencenter.infrastructure.cloud.magnum.master_flavor_id", "Control plane flavor", false)
	addStringPatch("magnum.node_flavor_id", "opencenter.infrastructure.cloud.magnum.node_flavor_id", "Worker flavor", false)

	for _, count := range []struct {
		answerID string
		path     string
		label    string
	}{
		{answerID: "magnum.master_count", path: "opencenter.infrastructure.compute.master_count", label: "Control plane count"},
		{answerID: "magnum.worker_count", path: "opencenter.infrastructure.compute.worker_count", label: "Worker count"},
	} {
		if value := strings.TrimSpace(answers[count.answerID]); value != "" {
			if err := validatePositiveInt(value); err != nil {
				return orchestration.ChangeSet{}, fmt.Errorf("%s: %w", count.label, err)
			}
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: count.path, Label: count.label, Value: value})
		}
	}

	if value := strings.TrimSpace(answers["magnum.create_timeout"]); value != "" {
		if err := validatePositiveInt(value); err != nil {
			return orchestration.ChangeSet{}, fmt.Errorf("Create timeout: %w", err)
		}
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.magnum.create_timeout", Label: "Create timeout", Value: value})
	}
	if value, ok := answers["magnum.insecure"]; ok && strings.TrimSpace(value) != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.magnum.insecure", Label: "Insecure TLS", Value: normalizeBoolString(value)})
	}

	return changes, nil
}

func (o *magnumConfigureOrchestrator) CapabilityRequests(_ *v2.Config, _ orchestration.DiscoveryResult) []orchestration.CapabilityRequest {
	return []orchestration.CapabilityRequest{
		{Name: "git-auth"},
		{Name: "object-storage"},
	}
}

func magnumConfigOrEmpty(cfg *v2.Config) *v2.MagnumCloudConfig {
	if cfg != nil && cfg.OpenCenter.Infrastructure.Cloud.Magnum != nil {
		return cfg.OpenCenter.Infrastructure.Cloud.Magnum
	}
	return &v2.MagnumCloudConfig{}
}
