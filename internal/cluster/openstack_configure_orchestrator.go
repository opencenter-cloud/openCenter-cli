package cluster

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	openstackcloud "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const (
	configureGroupProvider = "Provider"
	configureGroupGit      = "Git Auth"
	configureGroupDNS      = "DNS"
	configureGroupStorage  = "Storage"
)

type openStackConfigureOrchestrator struct {
	discovery openstackcloud.DiscoveryClient
}

func newOpenStackConfigureOrchestrator(discovery openstackcloud.DiscoveryClient) orchestration.ProviderOrchestrator {
	return &openStackConfigureOrchestrator{discovery: discovery}
}

func (o *openStackConfigureOrchestrator) Name() string {
	return "openstack"
}

func (o *openStackConfigureOrchestrator) Supports(provider string) bool {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "openstack", "":
		return true
	default:
		return false
	}
}

func (o *openStackConfigureOrchestrator) Discover(ctx context.Context, cfg *v2.Config) (orchestration.DiscoveryResult, error) {
	if o.discovery == nil {
		return orchestration.DiscoveryResult{}, nil
	}

	catalog, err := o.discovery.Discover(ctx, cfg)
	if err != nil {
		return orchestration.DiscoveryResult{}, err
	}

	return orchestration.DiscoveryResult{
		Metadata: map[string]any{
			"catalog": catalog,
		},
	}, nil
}

func (o *openStackConfigureOrchestrator) Prompts(cfg *v2.Config, discovery orchestration.DiscoveryResult) []orchestration.PromptSpec {
	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstackCfg == nil {
		return nil
	}

	if needsOpenStackAuthPrompts(openstackCfg) {
		return []orchestration.PromptSpec{
			{ID: "openstack.auth_url", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "OpenStack auth URL", Default: strings.TrimSpace(openstackCfg.AuthURL), Required: true},
			{ID: "openstack.region", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "OpenStack region", Default: firstNonEmptyString(strings.TrimSpace(openstackCfg.Region), strings.TrimSpace(cfg.OpenCenter.Meta.Region)), Required: true},
			{ID: "openstack.project_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "OpenStack project ID", Default: strings.TrimSpace(openstackCfg.ProjectID), Required: true},
			{ID: "openstack.project_name", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "OpenStack project name", Default: firstNonEmptyString(strings.TrimSpace(openstackCfg.ProjectName), strings.TrimSpace(openstackCfg.TenantName)), Required: true},
			{ID: "openstack.domain", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "OpenStack domain", Default: firstNonEmptyString(strings.TrimSpace(openstackCfg.Domain), strings.TrimSpace(openstackCfg.DomainName), "Default"), Required: true},
			{ID: "openstack.application_credential_id", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Application credential ID", Default: strings.TrimSpace(openstackCfg.ApplicationCredentialID), Required: true},
			{ID: "openstack.application_credential_secret", Group: configureGroupProvider, Kind: orchestration.PromptKindSecret, Label: "Application credential secret", Default: strings.TrimSpace(openstackCfg.ApplicationCredentialSecret), Required: true},
			{ID: "openstack.insecure", Group: configureGroupProvider, Kind: orchestration.PromptKindConfirm, Label: "Allow insecure TLS to Keystone?", Default: strconv.FormatBool(openstackCfg.Insecure)},
			{ID: "openstack.ca", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "CA bundle path (optional)", Default: strings.TrimSpace(openstackCfg.CA)},
		}
	}

	catalog := discoveryCatalogFromMetadata(discovery)
	imageOptions := promptOptionsFromCatalog(catalog.Images)
	flavorOptions := promptOptionsFromCatalog(catalog.Flavors)
	networkOptions := promptOptionsFromCatalog(catalog.Networks)
	subnetOptions := promptOptionsFromCatalog(catalog.Subnets)
	externalNetworkOptions := promptOptionsFromCatalog(catalog.ExternalNetworks)
	azOptions := promptOptionsFromCatalog(catalog.AvailabilityZones)

	return []orchestration.PromptSpec{
		selectOrInputPrompt("openstack.image_id", "Base image", strings.TrimSpace(openstackCfg.ImageID), imageOptions, true),
		selectOrInputPrompt("openstack.flavor_bastion", "Bastion flavor", strings.TrimSpace(cfg.OpenCenter.Infrastructure.Compute.FlavorBastion), flavorOptions, cfg.OpenCenter.Infrastructure.Bastion.Enabled),
		selectOrInputPrompt("openstack.flavor_master", "Control plane flavor", strings.TrimSpace(cfg.OpenCenter.Infrastructure.Compute.FlavorMaster), flavorOptions, true),
		selectOrInputPrompt("openstack.flavor_worker", "Worker flavor", strings.TrimSpace(cfg.OpenCenter.Infrastructure.Compute.FlavorWorker), flavorOptions, true),
		{ID: "openstack.master_count", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Control plane count", Default: strconv.Itoa(cfg.OpenCenter.Infrastructure.Compute.MasterCount), Required: true, Validate: validatePositiveInt},
		{ID: "openstack.worker_count", Group: configureGroupProvider, Kind: orchestration.PromptKindInput, Label: "Worker count", Default: strconv.Itoa(cfg.OpenCenter.Infrastructure.Compute.WorkerCount), Required: true, Validate: validatePositiveInt},
		selectOrInputPrompt("openstack.network_id", "Cluster network", strings.TrimSpace(openstackCfg.NetworkID), networkOptions, true),
		selectOrInputPrompt("openstack.subnet_id", "Cluster subnet", strings.TrimSpace(openstackCfg.SubnetID), subnetOptions, true),
		selectOrInputPrompt("openstack.floating_network_id", "Floating network", strings.TrimSpace(openstackCfg.FloatingNetworkID), externalNetworkOptions, true),
		selectOrInputPrompt("openstack.router_external_network_id", "Router external network", strings.TrimSpace(openstackCfg.RouterExternalNetworkID), externalNetworkOptions, true),
		selectOrInputPrompt("openstack.availability_zone", "Availability zone", strings.TrimSpace(openstackCfg.AvailabilityZone), azOptions, true),
	}
}

func (o *openStackConfigureOrchestrator) ApplyAnswers(cfg *v2.Config, answers orchestration.PromptAnswers) (orchestration.ChangeSet, error) {
	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstackCfg == nil {
		return orchestration.ChangeSet{}, fmt.Errorf("openstack configuration is nil")
	}

	changes := orchestration.ChangeSet{}

	if value := strings.TrimSpace(answers["openstack.auth_url"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.auth_url", Label: "Auth URL", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.region"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.meta.region", Label: "Region", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.region", Label: "OpenStack region", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.project_id"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.project_id", Label: "Project ID", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.project_name"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.project_name", Label: "Project name", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.tenant_name", Label: "Tenant name", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.domain"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.domain", Label: "Domain", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.domain_name", Label: "Domain name", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.user_domain_name", Label: "User domain", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.project_domain_name", Label: "Project domain", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.application_credential_id"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.application_credential_id", Label: "Application credential ID", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.application_credential_secret"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.application_credential_secret", Label: "Application credential secret", Value: value, Masked: true})
	}
	if value, ok := answers["openstack.insecure"]; ok && strings.TrimSpace(value) != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.insecure", Label: "Insecure TLS", Value: normalizeBoolString(value)})
	}
	if value, ok := answers["openstack.ca"]; ok {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.ca", Label: "CA bundle", Value: strings.TrimSpace(value)})
	}
	if value := strings.TrimSpace(answers["openstack.image_id"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.image_id", Label: "Image", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.bastion.image", Label: "Bastion image", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.flavor_bastion"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.compute.flavor_bastion", Label: "Bastion flavor", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.bastion.flavor", Label: "Bastion flavor", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.flavor_master"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.compute.flavor_master", Label: "Control plane flavor", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.flavor_worker"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.compute.flavor_worker", Label: "Worker flavor", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.master_count"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.compute.master_count", Label: "Control plane count", Value: value})
	}
	if value := strings.TrimSpace(answers["openstack.worker_count"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.compute.worker_count", Label: "Worker count", Value: value})
	}

	catalog := discoveryCatalogFromConfig(cfg)
	if value := strings.TrimSpace(answers["openstack.network_id"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.network_id", Label: "Network", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.networking.network_id", Label: "Network", Value: value},
		)
		if name := catalog.findName(catalog.Networks, value); name != "" {
			changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.network_name", Label: "Network name", Value: name})
		}
	}
	if value := strings.TrimSpace(answers["openstack.subnet_id"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.subnet_id", Label: "Subnet", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.networking.subnet_id", Label: "Subnet", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.floating_network_id"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.floating_network_id", Label: "Floating network", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.networking.floating_network_id", Label: "Floating network", Value: value},
		)
		if name := catalog.findName(catalog.ExternalNetworks, value); name != "" {
			changes.Patches = append(changes.Patches,
				orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.floating_ip_pool", Label: "Floating pool", Value: name},
				orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.external_network_name", Label: "External network name", Value: name},
				orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.networking.floating_ip_pool", Label: "Floating pool", Value: name},
			)
		}
	}
	if value := strings.TrimSpace(answers["openstack.router_external_network_id"]); value != "" {
		changes.Patches = append(changes.Patches,
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.router_external_network_id", Label: "Router external network", Value: value},
			orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.networking.router_external_network_id", Label: "Router external network", Value: value},
		)
	}
	if value := strings.TrimSpace(answers["openstack.availability_zone"]); value != "" {
		changes.Patches = append(changes.Patches, orchestration.ConfigPatch{Group: configureGroupProvider, Path: "opencenter.infrastructure.cloud.openstack.availability_zone", Label: "Availability zone", Value: value})
	}

	return changes, nil
}

func (o *openStackConfigureOrchestrator) CapabilityRequests(cfg *v2.Config, discovery orchestration.DiscoveryResult) []orchestration.CapabilityRequest {
	return []orchestration.CapabilityRequest{
		{Name: "git-auth"},
		{Name: "dns"},
		{Name: "object-storage"},
	}
}

func needsOpenStackAuthPrompts(cfg *v2.OpenStackCloudConfig) bool {
	if cfg == nil {
		return true
	}

	return strings.TrimSpace(cfg.AuthURL) == "" ||
		isPlaceholderValue(cfg.ProjectID) ||
		isPlaceholderValue(cfg.ProjectName) ||
		strings.TrimSpace(cfg.ApplicationCredentialID) == "" ||
		strings.TrimSpace(cfg.ApplicationCredentialSecret) == ""
}

func selectOrInputPrompt(id, label, defaultValue string, options []orchestration.PromptOption, required bool) orchestration.PromptSpec {
	prompt := orchestration.PromptSpec{
		ID:       id,
		Group:    configureGroupProvider,
		Label:    label,
		Default:  defaultValue,
		Required: required,
	}
	if len(options) == 0 {
		prompt.Kind = orchestration.PromptKindInput
		return prompt
	}
	prompt.Kind = orchestration.PromptKindSelect
	prompt.Options = options
	return prompt
}

func promptOptionsFromCatalog(items []openstackcloud.CatalogItem) []orchestration.PromptOption {
	options := make([]orchestration.PromptOption, 0, len(items))
	for _, item := range items {
		label := item.Name
		if label == "" {
			label = item.ID
		}
		if item.ID != "" && item.ID != label {
			label = fmt.Sprintf("%s (%s)", label, item.ID)
		}
		options = append(options, orchestration.PromptOption{
			Value:       item.ID,
			Label:       label,
			Description: item.Name,
		})
	}
	return options
}

type openstackDiscoveryCatalog struct {
	Images            []openstackcloud.CatalogItem
	Flavors           []openstackcloud.CatalogItem
	Networks          []openstackcloud.CatalogItem
	Subnets           []openstackcloud.CatalogItem
	ExternalNetworks  []openstackcloud.CatalogItem
	AvailabilityZones []openstackcloud.CatalogItem
}

func (c openstackDiscoveryCatalog) findName(items []openstackcloud.CatalogItem, id string) string {
	for _, item := range items {
		if item.ID == id {
			return item.Name
		}
	}
	return ""
}

func discoveryCatalogFromMetadata(discovery orchestration.DiscoveryResult) *openstackcloud.DiscoveryCatalog {
	if discovery.Metadata == nil {
		return &openstackcloud.DiscoveryCatalog{}
	}
	catalog, _ := discovery.Metadata["catalog"].(*openstackcloud.DiscoveryCatalog)
	if catalog == nil {
		return &openstackcloud.DiscoveryCatalog{}
	}
	return catalog
}

func discoveryCatalogFromConfig(cfg *v2.Config) openstackDiscoveryCatalog {
	openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstackCfg == nil {
		return openstackDiscoveryCatalog{}
	}
	return openstackDiscoveryCatalog{
		Networks: []openstackcloud.CatalogItem{
			{ID: strings.TrimSpace(openstackCfg.NetworkID), Name: strings.TrimSpace(openstackCfg.NetworkName)},
		},
		ExternalNetworks: []openstackcloud.CatalogItem{
			{ID: strings.TrimSpace(openstackCfg.FloatingNetworkID), Name: strings.TrimSpace(openstackCfg.ExternalNetworkName)},
			{ID: strings.TrimSpace(openstackCfg.RouterExternalNetworkID), Name: strings.TrimSpace(openstackCfg.ExternalNetworkName)},
		},
	}
}

func validatePositiveInt(value string) error {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fmt.Errorf("must be an integer")
	}
	if parsed < 0 {
		return fmt.Errorf("must be zero or greater")
	}
	return nil
}

func normalizeBoolString(value string) string {
	parsed, err := strconv.ParseBool(strings.TrimSpace(value))
	if err != nil {
		return "false"
	}
	return strconv.FormatBool(parsed)
}

func isPlaceholderValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return true
	}
	return strings.Contains(trimmed, "placeholder") || strings.Contains(trimmed, "example.com")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
