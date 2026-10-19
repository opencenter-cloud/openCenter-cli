package openstack

import (
	"bytes"
	"context"
	"fmt"
	"sort"
	"strings"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

const (
	StatusPlanned  = "planned"
	StatusApplied  = "applied"
	StatusNoOp     = "no-op"
	StatusBlocked  = "blocked"
	StatusDeclined = "declined"
)

type AuthImport struct {
	ApplicationCredentialID     string
	ApplicationCredentialSecret string
}

type Options struct {
	ImageID               string
	WindowsImageID        string
	NetworkID             string
	ExternalNetworkID     string
	SubnetID              string
	AvailabilityZone      string
	CreateInternalNetwork bool
	Replace               bool
	ImportAuth            *AuthImport
	ImportTLS             bool
}

type Change struct {
	Path string `json:"path" yaml:"path"`
	Old  string `json:"old" yaml:"old"`
	New  string `json:"new" yaml:"new"`
}

type Selection struct {
	Field      string                    `json:"field" yaml:"field"`
	Candidates []cloudopenstack.Resource `json:"candidates" yaml:"candidates"`
}

type Result struct {
	SchemaVersion           int         `json:"schema_version" yaml:"schema_version"`
	Operation               string      `json:"operation" yaml:"operation"`
	Status                  string      `json:"status" yaml:"status"`
	Cluster                 string      `json:"cluster" yaml:"cluster"`
	Provider                string      `json:"provider" yaml:"provider"`
	CloudProfile            string      `json:"cloud_profile" yaml:"cloud_profile"`
	InternalNetworkMode     string      `json:"internal_network_mode,omitempty" yaml:"internal_network_mode,omitempty"`
	Changes                 []Change    `json:"changes" yaml:"changes"`
	Selections              []Selection `json:"selections" yaml:"selections"`
	Warnings                []string    `json:"warnings" yaml:"warnings"`
	RemoteActions           []string    `json:"remote_actions" yaml:"remote_actions"`
	SensitiveValuesRedacted bool        `json:"sensitive_values_redacted" yaml:"sensitive_values_redacted"`
}

func Plan(ctx context.Context, cfg *v2.Config, snapshot *cloudopenstack.DiscoverySnapshot, opts Options) (Result, *v2.Config, error) {
	result := Result{SchemaVersion: 1, Operation: "cluster.provider.openstack.plan", Provider: "openstack", Changes: []Change{}, Selections: []Selection{}, Warnings: []string{}, RemoteActions: []string{}, SensitiveValuesRedacted: true}
	if err := ctx.Err(); err != nil {
		return result, cfg, err
	}
	if cfg == nil {
		return result, cfg, fmt.Errorf("configuration cannot be nil")
	}
	result.Cluster = cfg.OpenCenter.Meta.Organization + "/" + cfg.OpenCenter.Meta.Name
	if strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider) != "openstack" {
		return result, cfg, fmt.Errorf("provider must be openstack")
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		return result, cfg, fmt.Errorf("openstack provider configuration is missing")
	}
	if err := validateCreateInternalNetworkOptions(cfg.OpenCenter.Infrastructure.Cloud.OpenStack, opts); err != nil {
		return result, cfg, err
	}
	if opts.CreateInternalNetwork {
		result.InternalNetworkMode = "tofu-managed"
	}
	if snapshot == nil {
		return result, cfg, fmt.Errorf("OpenStack discovery snapshot cannot be nil")
	}
	result.Warnings = append(result.Warnings, snapshot.Warnings...)
	prospective := cloneProviderConfig(cfg)
	osCfg := prospective.OpenCenter.Infrastructure.Cloud.OpenStack
	changes := make([]Change, 0)
	blocked := false

	set := func(path string, current *string, desired string, sensitive bool) {
		currentValue := strings.TrimSpace(*current)
		desired = strings.TrimSpace(desired)
		if desired == "" || currentValue == desired {
			return
		}
		if !isPlaceholder(currentValue) && !opts.Replace {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s is populated with %q; use --replace to change it", path, redact(currentValue, sensitive)))
			return
		}
		*current = desired
		changes = append(changes, Change{Path: path, Old: redact(currentValue, sensitive), New: redact(desired, sensitive)})
	}

	setDerived := func(path string, current *string, desired string) {
		currentValue := strings.TrimSpace(*current)
		desired = strings.TrimSpace(desired)
		if desired == "" || currentValue == desired {
			return
		}
		if !isPlaceholder(currentValue) && !opts.Replace {
			return
		}
		*current = desired
		changes = append(changes, Change{Path: path, Old: currentValue, New: desired})
	}

	setDerived("opencenter.infrastructure.cloud.openstack.auth_url", &osCfg.AuthURL, snapshot.AuthURL)
	setDerived("opencenter.infrastructure.cloud.openstack.region", &osCfg.Region, snapshot.Region)
	setDerived("opencenter.infrastructure.cloud.openstack.project_id", &osCfg.ProjectID, snapshot.ProjectID)
	setDerived("opencenter.infrastructure.cloud.openstack.project_name", &osCfg.ProjectName, snapshot.ProjectName)
	setDerived("opencenter.infrastructure.cloud.openstack.tenant_name", &osCfg.TenantName, snapshot.ProjectName)
	setDerived("opencenter.infrastructure.cloud.openstack.domain", &osCfg.Domain, snapshot.DomainName)
	setDerived("opencenter.infrastructure.cloud.openstack.user_domain_name", &osCfg.UserDomainName, snapshot.UserDomainName)
	setDerived("opencenter.infrastructure.cloud.openstack.project_domain_name", &osCfg.ProjectDomainName, snapshot.ProjectDomainName)
	setDerived("opencenter.infrastructure.cloud.openstack.domain_name", &osCfg.DomainName, snapshot.DomainName)
	if opts.ImportTLS {
		if snapshot.CA != "" {
			set("opencenter.infrastructure.cloud.openstack.ca", &osCfg.CA, snapshot.CA, false)
		}
		if snapshot.Insecure && !osCfg.Insecure {
			if opts.Replace {
				osCfg.Insecure = true
				changes = append(changes, Change{Path: "opencenter.infrastructure.cloud.openstack.insecure", Old: "false", New: "true"})
			} else {
				blocked = true
				result.Warnings = append(result.Warnings, "OpenStack TLS verification is disabled in the profile; use --replace with --import-tls to persist insecure=true")
			}
		}
	}
	if opts.ImportAuth != nil {
		set("opencenter.infrastructure.cloud.openstack.application_credential_id", &osCfg.ApplicationCredentialID, opts.ImportAuth.ApplicationCredentialID, true)
		set("opencenter.infrastructure.cloud.openstack.application_credential_secret", &osCfg.ApplicationCredentialSecret, opts.ImportAuth.ApplicationCredentialSecret, true)
	}

	// A stored image ID that matches neither the requested selector nor any
	// discovered resource is stale (for example a baked-in default from a
	// different project). It must be surfaced explicitly: otherwise chooseResource
	// silently preserves the dead value and plan reports a misleading no-op.
	if !opts.Replace && strings.TrimSpace(opts.ImageID) == "" {
		if current := strings.TrimSpace(osCfg.ImageID); current != "" && !isPlaceholder(current) {
			if _, exists := findResource(snapshot.Images, current); !exists {
				blocked = true
				result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id %q does not match any discovered Linux image; specify --image-id with --replace to select a valid image", current))
			}
		}
	}
	if !opts.Replace && strings.TrimSpace(opts.WindowsImageID) == "" {
		if current := strings.TrimSpace(osCfg.ImageIDWindows); current != "" && !isPlaceholder(current) {
			if _, exists := findResource(snapshot.WindowsImages, current); !exists {
				blocked = true
				result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id_windows %q does not match any discovered Windows image; specify --windows-image-id with --replace to select a valid image", current))
			}
		}
	}
	if resourceConflict(osCfg.ImageID, opts.ImageID, snapshot.Images, opts.Replace, true) {
		blocked = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id is populated with %q; use --replace or --image-id", osCfg.ImageID))
	}
	if unresolvedSelection(osCfg.ImageID, opts.ImageID, snapshot.Images) {
		blocked = true
		result.Warnings = append(result.Warnings, "no viable Linux image was discovered; specify --image-id or provide a profile with an image")
	}
	if selected, ok, ambiguous, err := chooseResource("image", osCfg.ImageID, opts.ImageID, snapshot.Images, opts.Replace); err != nil {
		return result, cfg, err
	} else if ambiguous {
		ambiguousSelection(&result, "image", "--image-id", "opencenter.infrastructure.cloud.openstack.image_id", snapshot.Images)
		blocked = true
	} else if ok {
		set("opencenter.infrastructure.cloud.openstack.image_id", &osCfg.ImageID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.image_name", &osCfg.ImageName, selected.Name, false)
	}
	if resourceConflict(osCfg.ImageIDWindows, opts.WindowsImageID, snapshot.WindowsImages, opts.Replace, true) {
		blocked = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id_windows is populated with %q; use --replace or --windows-image-id", osCfg.ImageIDWindows))
	}
	if selected, ok, ambiguous, err := chooseResource("windows image", osCfg.ImageIDWindows, opts.WindowsImageID, snapshot.WindowsImages, opts.Replace); err != nil {
		return result, cfg, err
	} else if ambiguous {
		ambiguousSelection(&result, "windows image", "--windows-image-id", "opencenter.infrastructure.cloud.openstack.image_id_windows", snapshot.WindowsImages)
		blocked = true
	} else if ok {
		set("opencenter.infrastructure.cloud.openstack.image_id_windows", &osCfg.ImageIDWindows, selected.ID, false)
	}
	if opts.CreateInternalNetwork {
		if !clearInternalNetworkSelections(osCfg, opts.Replace, &changes, &result.Warnings) {
			blocked = true
		}
	} else {
		internalNetworks := filterInternalNetworks(snapshot.Networks, snapshot.ExternalNetworks)
		if resourceConflict(osCfg.NetworkID, opts.NetworkID, internalNetworks, opts.Replace, true) {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.network_id is populated with %q; use --replace or --network-id", osCfg.NetworkID))
		}
		if unresolvedSelection(osCfg.NetworkID, opts.NetworkID, internalNetworks) {
			blocked = true
			result.Warnings = append(result.Warnings, "no viable internal network was discovered; specify --network-id")
		}
		if selected, ok, ambiguous, err := chooseResource("network", osCfg.NetworkID, opts.NetworkID, internalNetworks, opts.Replace); err != nil {
			return result, cfg, err
		} else if ambiguous {
			ambiguousSelection(&result, "network", "--network-id", "opencenter.infrastructure.cloud.openstack.network_id", internalNetworks)
			blocked = true
		} else if ok {
			set("opencenter.infrastructure.cloud.openstack.network_id", &osCfg.NetworkID, selected.ID, false)
			set("opencenter.infrastructure.cloud.openstack.network_name", &osCfg.NetworkName, selected.Name, false)
			ensureNetworking(osCfg)
			set("opencenter.infrastructure.cloud.openstack.networking.network_id", &osCfg.Networking.NetworkID, selected.ID, false)
		}
	}
	if resourceConflict(osCfg.RouterExternalNetworkID, opts.ExternalNetworkID, snapshot.ExternalNetworks, opts.Replace, false) {
		blocked = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.router_external_network_id is populated with %q; use --replace or --external-network-id", osCfg.RouterExternalNetworkID))
	}
	if unresolvedSelection(osCfg.RouterExternalNetworkID, opts.ExternalNetworkID, snapshot.ExternalNetworks) {
		blocked = true
		result.Warnings = append(result.Warnings, "no viable external network was discovered; specify --external-network-id")
	}
	if selected, ok, ambiguous, err := chooseResource("external network", osCfg.RouterExternalNetworkID, opts.ExternalNetworkID, snapshot.ExternalNetworks, opts.Replace); err != nil {
		return result, cfg, err
	} else if ambiguous {
		ambiguousSelection(&result, "external network", "--external-network-id", "opencenter.infrastructure.cloud.openstack.router_external_network_id", snapshot.ExternalNetworks)
		blocked = true
	} else if ok {
		set("opencenter.infrastructure.cloud.openstack.floating_network_id", &osCfg.FloatingNetworkID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.router_external_network_id", &osCfg.RouterExternalNetworkID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.floating_ip_pool", &osCfg.FloatingIPPool, selected.Name, false)
		set("opencenter.infrastructure.cloud.openstack.external_network_name", &osCfg.ExternalNetworkName, selected.Name, false)
		ensureNetworking(osCfg)
		set("opencenter.infrastructure.cloud.openstack.networking.floating_network_id", &osCfg.Networking.FloatingNetworkID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.networking.router_external_network_id", &osCfg.Networking.RouterExternalNetworkID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.networking.floating_ip_pool", &osCfg.Networking.FloatingIPPool, selected.Name, false)
	}

	if !opts.CreateInternalNetwork {
		subnetCandidates := snapshot.Subnets
		if strings.TrimSpace(osCfg.NetworkID) != "" {
			filtered := make([]cloudopenstack.Subnet, 0, len(subnetCandidates))
			for _, subnet := range subnetCandidates {
				if subnet.NetworkID == osCfg.NetworkID {
					filtered = append(filtered, subnet)
				}
			}
			subnetCandidates = filtered
		}
		subnetResources := make([]cloudopenstack.Resource, 0, len(subnetCandidates))
		for _, subnet := range subnetCandidates {
			subnetResources = append(subnetResources, subnet.Resource)
		}
		if resourceConflict(osCfg.SubnetID, opts.SubnetID, subnetResources, opts.Replace, true) {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.subnet_id is populated with %q; use --replace or --subnet-id", osCfg.SubnetID))
		}
		if unresolvedSelection(osCfg.SubnetID, opts.SubnetID, subnetResources) {
			blocked = true
			result.Warnings = append(result.Warnings, "no viable subnet was discovered; specify --subnet-id")
		}
		if selected, ok, ambiguous, err := chooseResource("subnet", osCfg.SubnetID, opts.SubnetID, subnetResources, opts.Replace); err != nil {
			return result, cfg, err
		} else if ambiguous {
			ambiguousSelection(&result, "subnet", "--subnet-id", "opencenter.infrastructure.cloud.openstack.subnet_id", subnetResources)
			blocked = true
		} else if ok {
			set("opencenter.infrastructure.cloud.openstack.subnet_id", &osCfg.SubnetID, selected.ID, false)
			ensureNetworking(osCfg)
			set("opencenter.infrastructure.cloud.openstack.networking.subnet_id", &osCfg.Networking.SubnetID, selected.ID, false)
		}
	}
	if snapshot.AvailabilityZonesAvailable {
		if resourceConflict(osCfg.AvailabilityZone, opts.AvailabilityZone, snapshot.AvailabilityZones, opts.Replace, true) {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.availability_zone is populated with %q; use --replace or --availability-zone", osCfg.AvailabilityZone))
		}
		if unresolvedSelection(osCfg.AvailabilityZone, opts.AvailabilityZone, snapshot.AvailabilityZones) {
			blocked = true
			result.Warnings = append(result.Warnings, "no viable availability zone was discovered; specify --availability-zone")
		}
		if selected, ok, ambiguous, err := chooseResource("availability zone", osCfg.AvailabilityZone, opts.AvailabilityZone, snapshot.AvailabilityZones, opts.Replace); err != nil {
			return result, cfg, err
		} else if ambiguous {
			ambiguousSelection(&result, "availability zone", "--availability-zone", "opencenter.infrastructure.cloud.openstack.availability_zone", snapshot.AvailabilityZones)
			blocked = true
		} else if ok {
			set("opencenter.infrastructure.cloud.openstack.availability_zone", &osCfg.AvailabilityZone, selected.ID, false)
		}
		if len(osCfg.AvailabilityZones) == 0 && len(snapshot.AvailabilityZones) > 0 {
			zones := make([]string, 0, len(snapshot.AvailabilityZones))
			for _, zone := range snapshot.AvailabilityZones {
				zones = append(zones, zone.ID)
			}
			osCfg.AvailabilityZones = zones
			changes = append(changes, Change{Path: "opencenter.infrastructure.cloud.openstack.availability_zones", Old: "", New: strings.Join(zones, ",")})
		}
	}

	if blocked {
		result.Status = StatusBlocked
	} else if len(changes) == 0 {
		result.Status = StatusNoOp
	} else {
		result.Status = StatusPlanned
	}
	result.Changes = sortedChanges(changes)
	result.Warnings = sortedStrings(result.Warnings)
	result.Selections = sortedSelections(result.Selections)
	return result, prospective, nil
}

func validateCreateInternalNetworkOptions(cfg *v2.OpenStackCloudConfig, opts Options) error {
	if !opts.CreateInternalNetwork {
		return nil
	}
	if strings.TrimSpace(opts.NetworkID) != "" || strings.TrimSpace(opts.SubnetID) != "" {
		return fmt.Errorf("--create-internal-network cannot be combined with --network-id or --subnet-id")
	}
	if cfg.Networking != nil && strings.TrimSpace(cfg.Networking.VLAN.ID) != "" {
		return fmt.Errorf("--create-internal-network cannot be used when networking.vlan.id is set")
	}
	return nil
}

func clearInternalNetworkSelections(cfg *v2.OpenStackCloudConfig, replace bool, changes *[]Change, warnings *[]string) bool {
	populated := strings.TrimSpace(cfg.NetworkID) != "" || strings.TrimSpace(cfg.NetworkName) != "" || strings.TrimSpace(cfg.SubnetID) != ""
	if cfg.Networking != nil {
		populated = populated || strings.TrimSpace(cfg.Networking.NetworkID) != "" || strings.TrimSpace(cfg.Networking.SubnetID) != ""
	}
	if populated && !replace {
		*warnings = append(*warnings, "internal network selections are populated; use --replace with --create-internal-network to clear them")
		return false
	}
	clear := func(path string, current *string) {
		old := strings.TrimSpace(*current)
		if old == "" {
			return
		}
		*current = ""
		*changes = append(*changes, Change{Path: path, Old: old, New: ""})
	}
	clear("opencenter.infrastructure.cloud.openstack.network_id", &cfg.NetworkID)
	clear("opencenter.infrastructure.cloud.openstack.network_name", &cfg.NetworkName)
	clear("opencenter.infrastructure.cloud.openstack.subnet_id", &cfg.SubnetID)
	if cfg.Networking != nil {
		clear("opencenter.infrastructure.cloud.openstack.networking.network_id", &cfg.Networking.NetworkID)
		clear("opencenter.infrastructure.cloud.openstack.networking.subnet_id", &cfg.Networking.SubnetID)
	}
	return true
}

func resourceConflict(current, explicit string, candidates []cloudopenstack.Resource, replace, preserveCurrent bool) bool {
	if replace || preserveCurrent || strings.TrimSpace(explicit) != "" || isPlaceholder(current) {
		return false
	}
	for _, candidate := range candidates {
		if candidate.ID == strings.TrimSpace(current) {
			return false
		}
	}
	return true
}

func chooseResource(field, current, explicit string, candidates []cloudopenstack.Resource, replace bool) (cloudopenstack.Resource, bool, bool, error) {
	candidates = append([]cloudopenstack.Resource(nil), candidates...)
	sortResources(candidates)
	if strings.TrimSpace(explicit) != "" {
		for _, candidate := range candidates {
			if candidate.ID == strings.TrimSpace(explicit) {
				return candidate, true, false, nil
			}
		}
		return cloudopenstack.Resource{}, false, false, fmt.Errorf("%s selector %q was not found in OpenStack discovery", field, explicit)
	}
	current = strings.TrimSpace(current)
	if current != "" && !isPlaceholder(current) {
		for _, candidate := range candidates {
			if candidate.ID == current {
				return candidate, false, false, nil
			}
		}
		if !replace {
			return cloudopenstack.Resource{}, false, false, nil
		}
	}
	if len(candidates) == 1 {
		return candidates[0], true, false, nil
	}
	return cloudopenstack.Resource{}, false, len(candidates) > 1, nil
}

func findResource(candidates []cloudopenstack.Resource, id string) (cloudopenstack.Resource, bool) {
	for _, candidate := range candidates {
		if candidate.ID == strings.TrimSpace(id) {
			return candidate, true
		}
	}
	return cloudopenstack.Resource{}, false
}

func cloneProviderConfig(cfg *v2.Config) *v2.Config {
	copyCfg := *cfg
	copyCfg.OpenCenter = cfg.OpenCenter
	copyCfg.OpenCenter.Infrastructure = cfg.OpenCenter.Infrastructure
	copyCfg.OpenCenter.Infrastructure.Cloud = cfg.OpenCenter.Infrastructure.Cloud
	original := *cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	copyCfg.OpenCenter.Infrastructure.Cloud.OpenStack = &original
	if original.Networking != nil {
		networking := *original.Networking
		copyCfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking = &networking
	}
	return &copyCfg
}

func ensureNetworking(cfg *v2.OpenStackCloudConfig) {
	if cfg.Networking == nil {
		cfg.Networking = &v2.OpenStackNetworkingConfig{}
	}
}

func isPlaceholder(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	return value == "" || value == "changeme" || value == "change-me" || strings.Contains(value, "placeholder") || strings.Contains(value, "example") || strings.Contains(value, "todo")
}

func redact(value string, sensitive bool) string {
	if sensitive && strings.TrimSpace(value) != "" {
		return "<redacted>"
	}
	return value
}

func sortedChanges(changes []Change) []Change {
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func sortedStrings(values []string) []string {
	sort.Strings(values)
	return values
}

func sortedSelections(values []Selection) []Selection {
	sort.Slice(values, func(i, j int) bool { return values[i].Field < values[j].Field })
	return values
}

func sortResources(values []cloudopenstack.Resource) {
	sort.Slice(values, func(i, j int) bool {
		if values[i].Name == values[j].Name {
			return values[i].ID < values[j].ID
		}
		return values[i].Name < values[j].Name
	})
}

func filterInternalNetworks(networks, external []cloudopenstack.Resource) []cloudopenstack.Resource {
	externalIDs := make(map[string]struct{}, len(external))
	for _, network := range external {
		externalIDs[network.ID] = struct{}{}
	}
	result := make([]cloudopenstack.Resource, 0, len(networks))
	for _, network := range networks {
		if _, isExternal := externalIDs[network.ID]; !isExternal {
			result = append(result, network)
		}
	}
	return result
}

func unresolvedSelection(current, explicit string, candidates []cloudopenstack.Resource) bool {
	return strings.TrimSpace(explicit) == "" && isPlaceholder(current) && len(candidates) == 0
}

// ambiguousSelection records that a field could not be resolved automatically.
// The candidate list alone does not explain the blocked status, so every
// ambiguity also emits a warning naming the selector that resolves it.
func ambiguousSelection(result *Result, field, selector, configPath string, candidates []cloudopenstack.Resource) {
	result.Selections = append(result.Selections, Selection{Field: field, Candidates: sortedResourceCopy(candidates)})
	result.Warnings = append(result.Warnings, fmt.Sprintf("%s is ambiguous: %d candidates discovered; specify %s with one of the listed IDs", configPath, len(candidates), selector))
}

func sortedResourceCopy(values []cloudopenstack.Resource) []cloudopenstack.Resource {
	copyValues := append([]cloudopenstack.Resource(nil), values...)
	sortResources(copyValues)
	return copyValues
}

// ApplyPersistence persists a non-empty provider patch after prospective
// validation. It deliberately has no remote mutation capability.
type ApplyPersistence struct {
	FileSystem fs.FileSystem
	Validate   func(context.Context, *v2.Config) error
}

func (p ApplyPersistence) Apply(ctx context.Context, path string, original, prospective *v2.Config, originalData []byte, result Result) (Result, error) {
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if result.Status == StatusBlocked {
		return result, nil
	}
	if len(result.Changes) == 0 {
		result.Status = StatusNoOp
		return result, nil
	}
	if prospective == nil || original == nil {
		return result, fmt.Errorf("configuration cannot be nil")
	}
	if p.Validate != nil {
		if err := p.Validate(ctx, prospective); err != nil {
			return result, fmt.Errorf("prospective configuration validation failed: %w", err)
		}
	}
	if p.FileSystem == nil {
		return result, fmt.Errorf("filesystem is required")
	}
	data, err := v2.MarshalPublicConfig(prospective)
	if err != nil {
		return result, fmt.Errorf("marshal prospective configuration: %w", err)
	}
	currentData, err := p.FileSystem.ReadFile(path)
	if err != nil {
		return result, fmt.Errorf("read configuration before apply: %w", err)
	}
	if !bytes.Equal(currentData, originalData) {
		return result, fmt.Errorf("configuration changed since planning; re-plan before applying")
	}
	if err := p.FileSystem.WriteFile(path+".backup", currentData, 0o600); err != nil {
		return result, fmt.Errorf("create configuration backup: %w", err)
	}
	if err := p.FileSystem.WriteFileAtomic(path, data, 0o600); err != nil {
		return result, fmt.Errorf("write configuration atomically: %w", err)
	}
	result.Status = StatusApplied
	return result, nil
}
