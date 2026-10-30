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

	// DefaultLinuxImageName is the operator preference applied when no
	// --image-name is given. It is a NAME, resolved against live Glance
	// inventory exactly like an explicit selector, so nothing is ever persisted
	// unless the cloud actually has that image. Image IDs are project-scoped and
	// must never be baked in (see the stale-image detection in Plan); a name
	// carries no such risk. When the cloud does not have it, plan falls through
	// to the ordinary ambiguous path and prints the candidate list.
	DefaultLinuxImageName = "Ubuntu 24.04"
)

// windowsImageNeeded reports whether the plan must resolve image_id_windows.
// Readiness only requires that value when the cluster runs Windows workers
// without a per-pool image override (config/v2/validator.go), so requiring a
// selection otherwise blocked every Linux-only cluster over an image it will
// never boot. A stored value or an explicit --windows-image-name also keeps the
// field in scope, so a stale Windows image stays repairable on any cluster.
func windowsImageNeeded(cfg *v2.Config, osCfg *v2.OpenStackCloudConfig, opts Options) bool {
	if strings.TrimSpace(osCfg.ImageIDWindows) != "" || strings.TrimSpace(opts.WindowsImageName) != "" {
		return true
	}
	compute := cfg.OpenCenter.Infrastructure.Compute
	if compute.WorkerCountWindows > 0 {
		return true
	}
	for _, pool := range compute.AdditionalServerPoolsWorkerWindows {
		if pool.Count > 0 && strings.TrimSpace(pool.Image) == "" {
			return true
		}
	}
	return false
}

type AuthImport struct {
	ApplicationCredentialID     string
	ApplicationCredentialSecret string
}

// EnsureProfileAuth is the seam through which the command layer obtains a
// persistable application credential for a cloud profile, minting and
// persisting one into clouds.yaml when the profile lacks an ID/secret pair.
// Tests replace it to exercise the auto-import flow without a live Keystone.
var EnsureProfileAuth = func(ctx context.Context, profile cloudopenstack.Profile, name string) (cloudopenstack.AppCredential, cloudopenstack.Profile, error) {
	return cloudopenstack.EnsureApplicationCredential(ctx, profile, cloudopenstack.NewAppCredentialProvisioner(), name)
}

type Options struct {
	ImageName             string
	WindowsImageName      string
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
	// No explicit Linux image: prefer the default name, but only if this cloud
	// actually has it. Resolution is the ordinary selector path, so an absent
	// default leaves the plan ambiguous and the candidate list is printed.
	if strings.TrimSpace(opts.ImageName) == "" && resolveSelector(snapshot.Images, DefaultLinuxImageName).resource.ID != "" {
		opts.ImageName = DefaultLinuxImageName
		result.Warnings = append(result.Warnings, fmt.Sprintf("no --image-name specified; defaulting to %q (%s) discovered in this project", DefaultLinuxImageName, resolveSelector(snapshot.Images, DefaultLinuxImageName).resource.ID))
	}
	prospective := cloneProviderConfig(cfg)
	cloneBastionAndPools(prospective)
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

	// isDerived reports whether a stored provider value is a `cluster init`
	// guess rather than an operator selection. Most baked defaults contain a
	// placeholder marker; the region is the exception — init copies a real
	// region name (the CLI default, else InitDefaultRegion) straight into the
	// provider block, so the init default is the second replaceable shape.
	//
	// auth_url is the third shape, and the dangerous one. init derives it from
	// the same region guess, so it bakes a REAL, fully-formed keystone endpoint
	// for a region that may have nothing to do with the cloud the cluster is
	// later bound to. Neither rule above catches it: it contains no placeholder
	// marker and is not equal to a bare region name. Treated as an operator
	// choice, it survives the binding while the region moves - producing the
	// auth_url/region split that makes every in-cluster consumer (CCM cloud.conf,
	// cinder CSI) authenticate in the wrong region and reject the credential.
	// So an auth_url that is exactly what init would have generated for the init
	// default region is a guess, and is replaceable.
	initDefaultRegion := strings.TrimSpace(v2.InitDefaultRegion)
	initDefaultAuthURL := ""
	if initDefaultRegion != "" {
		initDefaultAuthURL = v2.KeystoneAuthURL(initDefaultRegion)
	}
	isDerived := func(value string) bool {
		value = strings.TrimSpace(value)
		if isPlaceholder(value) || strings.EqualFold(value, initDefaultRegion) {
			return true
		}
		return initDefaultAuthURL != "" && value == initDefaultAuthURL
	}
	setDerived := func(path string, current *string, desired string) {
		currentValue := strings.TrimSpace(*current)
		desired = strings.TrimSpace(desired)
		if desired == "" || currentValue == desired {
			return
		}
		if !isDerived(currentValue) && !opts.Replace {
			return
		}
		*current = desired
		changes = append(changes, Change{Path: path, Old: currentValue, New: desired})
	}

	// setRegionFollowers repoints the fields OUTSIDE the provider block that
	// are derived from the cloud region (meta.region, the Barbican secret
	// backend). `cluster init` bakes a default region long before any cloud
	// is chosen, and that guess is not an operator decision worth protecting:
	// a stored region equal to the init default (or empty/placeholder) is
	// corrected silently. A region that differs from the discovered cloud AND
	// from the init default was set deliberately, so it blocks the plan and
	// names --replace rather than half-propagating.
	//
	// The init default is recognised by value, not by provenance: the CLI
	// config (cluster_defaults.region) is deliberately not consulted here,
	// because the cloud selected by --os-cloud outranks a global default.
	setRegionFollowers := func(desired string) {
		desired = strings.TrimSpace(desired)
		if desired == "" {
			return
		}
		followers := []struct {
			path    string
			current *string
		}{
			// meta.region is the human-facing cluster region: `cluster
			// describe`, `cluster use`, and the GitOps renderers that fall
			// back to it all read this field.
			{"opencenter.meta.region", &prospective.OpenCenter.Meta.Region},
			// The Barbican secret backend authenticates against the same
			// Keystone scope as the infrastructure provider; its region must
			// track the cloud, or `opencenter secrets` would target the wrong
			// endpoint.
			{"opencenter.secrets.barbican.region", &prospective.OpenCenter.Secrets.Barbican.Region},
		}
		// All-or-nothing: a cluster whose meta.region and Barbican region
		// point at different clouds is either mid-migration or hand-edited,
		// and rewriting only the unguarded half would leave it split across
		// two regions. Any protected follower blocks the whole set.
		for _, follower := range followers {
			currentValue := strings.TrimSpace(*follower.current)
			if currentValue != desired && !isDerived(currentValue) && !opts.Replace {
				blocked = true
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s is populated with %q; the discovered cloud region is %q; use --replace to repoint the cluster", follower.path, currentValue, desired))
				return
			}
		}
		for _, follower := range followers {
			currentValue := strings.TrimSpace(*follower.current)
			if currentValue == desired {
				continue
			}
			*follower.current = desired
			changes = append(changes, Change{Path: follower.path, Old: currentValue, New: desired})
		}
	}

	// rederiveIdentityAfterRegionChange re-derives cluster_fqdn and heals any
	// service hostname still baked from the pre-binding identity. The region the
	// cloud reports arrives AFTER defaults were built from the init default
	// region, so both cluster_fqdn and every hostname scalar materialized from
	// it still name the guessed region. Without this step a cluster bound to
	// IAD3 keeps auth.<name>.dfw3.k8s.opencenter.cloud and the Gateway ends up
	// with listeners in two DNS domains.
	rederiveIdentityAfterRegionChange := func(desired string) {
		region := strings.ToLower(strings.TrimSpace(desired))
		if region == "" {
			return
		}
		if derived := prospective.OpenCenter.Cluster.DeriveClusterFQDN(region); derived != "" &&
			derived != prospective.OpenCenter.Cluster.ClusterFQDN {
			changes = append(changes, Change{
				Path: "opencenter.cluster.cluster_fqdn",
				Old:  prospective.OpenCenter.Cluster.ClusterFQDN,
				New:  derived,
			})
			prospective.OpenCenter.Cluster.ClusterFQDN = derived
		}
		for _, field := range v2.RepairPlaceholderHostnames(prospective) {
			changes = append(changes, Change{
				Path: field,
				Old:  "(placeholder-derived)",
				New:  "re-derived from cluster_fqdn",
			})
		}
	}

	// setRegionListFollowers repoints a region-derived field that holds a LIST,
	// which the scalar follower loop above cannot express. The same
	// all-or-nothing rule applies: a value that is neither the discovered
	// region's default nor a placeholder was chosen by the operator, so it
	// blocks instead of being overwritten.
	setRegionListFollowers := func(path, regionName string, current *[]string, desired []string) {
		if len(desired) == 0 {
			return
		}
		currentValues := *current
		same := len(currentValues) == len(desired)
		if same {
			for i := range desired {
				if strings.TrimSpace(currentValues[i]) != desired[i] {
					same = false
					break
				}
			}
		}
		if same {
			return
		}
		// A list is derived when every entry is a placeholder or when it is
		// exactly what init would have baked for the region init guessed.
		initGuess := v2.DefaultNTPServers(prospective.OpenCenter.Infrastructure.Provider, initDefaultRegion)
		matchesInitGuess := len(currentValues) == len(initGuess)
		if matchesInitGuess {
			for i := range initGuess {
				if strings.TrimSpace(currentValues[i]) != initGuess[i] {
					matchesInitGuess = false
					break
				}
			}
		}
		if !matchesInitGuess && len(currentValues) > 0 && !opts.Replace {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s is populated with %s; the discovered cloud region %q uses %s; use --replace to repoint it", path, strings.Join(currentValues, ","), regionName, strings.Join(desired, ",")))
			return
		}
		*current = append([]string(nil), desired...)
		changes = append(changes, Change{Path: path, Old: strings.Join(currentValues, ","), New: strings.Join(desired, ",")})
	}

	// setFlavorsFromRegion applies the registry flavors for the discovered
	// region to the three node roles, then checks every flavor that will be
	// used - registry default or operator choice - against the flavors Nova
	// actually offers in that region.
	//
	// The registry is the weak link: its flavor table was copied verbatim into
	// all four regions from one dev-tools import, so it asserted a gp.5.* family
	// that no Rackspace region offers. kit-kat (IAD3) was hand-corrected to the
	// real gp.0.* family and deployed; clusters left on the registry default die
	// in `tofu apply` with Nova's "Unable to find flavor with name gp.5.4.8" -
	// after provisioning has begun. Discovery already lists the region's real
	// flavors, so the binding has the truth in hand and must use it: a flavor the
	// cloud does not have blocks the plan and names the available ones, instead
	// of shipping a config that cannot boot.
	setFlavorsFromRegion := func(desiredRegion string) {
		flavors := v2.DefaultFlavorsForRegion(prospective.OpenCenter.Infrastructure.Provider, desiredRegion)
		initFlavors := v2.DefaultFlavorsForRegion(prospective.OpenCenter.Infrastructure.Provider, initDefaultRegion)
		compute := &prospective.OpenCenter.Infrastructure.Compute
		roles := []struct {
			path    string
			key     string
			current *string
		}{
			{"opencenter.infrastructure.compute.flavor_bastion", "bastion", &compute.FlavorBastion},
			{"opencenter.infrastructure.compute.flavor_master", "master", &compute.FlavorMaster},
			{"opencenter.infrastructure.compute.flavor_worker", "worker", &compute.FlavorWorker},
		}
		for _, role := range roles {
			desired, ok := flavors[role.key]
			if !ok {
				continue
			}
			currentValue := strings.TrimSpace(*role.current)
			if currentValue == desired {
				continue
			}
			// Replaceable when empty, a placeholder, or exactly what init baked
			// for its guessed region.
			replaceable := isPlaceholder(currentValue) || currentValue == "" || currentValue == initFlavors[role.key]
			if !replaceable && !opts.Replace {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s is populated with %q; the discovered cloud region %q prescribes %q; use --replace to change it", role.path, currentValue, desiredRegion, desired))
				continue
			}
			*role.current = desired
			changes = append(changes, Change{Path: role.path, Old: currentValue, New: desired})
		}

		// Validate against live Nova inventory when it was read. Availability is
		// not guaranteed - the compute service may be unreachable or the
		// credential may lack the flavor-list policy - and an unavailable list
		// must not block a binding that would otherwise succeed, so the check is
		// skipped with a warning rather than failing closed.
		if len(snapshot.Flavors) == 0 {
			return
		}
		available := make(map[string]string, len(snapshot.Flavors))
		for _, flavor := range snapshot.Flavors {
			available[flavor.Name] = flavor.ID
			available[flavor.ID] = flavor.ID
		}
		for _, role := range roles {
			value := strings.TrimSpace(*role.current)
			if value == "" || isPlaceholder(value) {
				continue
			}
			if _, ok := available[value]; ok {
				continue
			}
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s=%q does not exist in the discovered cloud region %q; available flavors: %s", role.path, value, desiredRegion, strings.Join(flavorNames(snapshot.Flavors), ", ")))
		}
	}

	// The region is the load-bearing value the profile carries: OpenTofu
	// renders the Swift/S3 service regions and the GitOps provider block from
	// cloud.openstack.region, and describe/use/reporting read meta.region.
	// `cluster init` bakes a default region long before any cloud is chosen,
	// so a cluster bound to a different cloud would otherwise deploy against
	// init's guess. The discovered region is authoritative and propagates to
	// every dependent field.
	setDerived("opencenter.infrastructure.cloud.openstack.auth_url", &osCfg.AuthURL, snapshot.AuthURL)
	setDerived("opencenter.infrastructure.cloud.openstack.region", &osCfg.Region, snapshot.Region)
	setRegionFollowers(snapshot.Region)
	// NTP hosts and instance flavors are region-derived too: init baked them
	// from its own region guess, so binding to a profile in another region has
	// to repoint them or the cluster boots against the wrong cloud's
	// infrastructure.
	if snapshot.Region != "" {
		setRegionListFollowers("opencenter.infrastructure.networking.ntp_servers", snapshot.Region,
			&prospective.OpenCenter.Infrastructure.Networking.NTPServers,
			v2.DefaultNTPServers(prospective.OpenCenter.Infrastructure.Provider, snapshot.Region))
		setFlavorsFromRegion(snapshot.Region)
		rederiveIdentityAfterRegionChange(snapshot.Region)
	}
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
	if !opts.Replace && strings.TrimSpace(opts.ImageName) == "" {
		if current := strings.TrimSpace(osCfg.ImageID); current != "" && !isPlaceholder(current) {
			if _, exists := findResource(snapshot.Images, current); !exists {
				blocked = true
				result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id %q does not match any discovered Linux image; specify --image-name with --replace to select a valid image", current))
			}
		}
	}
	if windowsImageNeeded(prospective, osCfg, opts) && !opts.Replace && strings.TrimSpace(opts.WindowsImageName) == "" {
		if current := strings.TrimSpace(osCfg.ImageIDWindows); current != "" && !isPlaceholder(current) {
			if _, exists := findResource(snapshot.WindowsImages, current); !exists {
				blocked = true
				result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id_windows %q does not match any discovered Windows image; specify --windows-image-name with --replace to select a valid image", current))
			}
		}
	}
	if resourceConflict(osCfg.ImageID, opts.ImageName, snapshot.Images, opts.Replace, true) {
		blocked = true
		result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id is populated with %q; use --replace or --image-name", osCfg.ImageID))
	}
	if unresolvedSelection(osCfg.ImageID, opts.ImageName, snapshot.Images) {
		blocked = true
		result.Warnings = append(result.Warnings, "no viable Linux image was discovered; specify --image-name or provide a profile with an image")
	}
	// resolvedLinuxImage is the Linux image this plan binds the cluster to;
	// the bastion and Linux worker pool propagation below reads it.
	var resolvedLinuxImage cloudopenstack.Resource
	if selected, ok, ambiguous, err := chooseResource("image", osCfg.ImageID, opts.ImageName, snapshot.Images, opts.Replace); err != nil {
		return result, cfg, err
	} else if ambiguous {
		ambiguousSelection(&result, "image", "--image-name", "opencenter.infrastructure.cloud.openstack.image_id", snapshot.Images)
		blocked = true
	} else if ok {
		resolvedLinuxImage = selected
		set("opencenter.infrastructure.cloud.openstack.image_id", &osCfg.ImageID, selected.ID, false)
		set("opencenter.infrastructure.cloud.openstack.image_name", &osCfg.ImageName, selected.Name, false)
	}
	if windowsImageNeeded(prospective, osCfg, opts) {
		if resourceConflict(osCfg.ImageIDWindows, opts.WindowsImageName, snapshot.WindowsImages, opts.Replace, true) {
			blocked = true
			result.Warnings = append(result.Warnings, fmt.Sprintf("opencenter.infrastructure.cloud.openstack.image_id_windows is populated with %q; use --replace or --windows-image-name", osCfg.ImageIDWindows))
		}
		if selected, ok, ambiguous, err := chooseResource("windows image", osCfg.ImageIDWindows, opts.WindowsImageName, snapshot.WindowsImages, opts.Replace); err != nil {
			return result, cfg, err
		} else if ambiguous {
			ambiguousSelection(&result, "windows image", "--windows-image-name", "opencenter.infrastructure.cloud.openstack.image_id_windows", snapshot.WindowsImages)
			blocked = true
		} else if ok {
			set("opencenter.infrastructure.cloud.openstack.image_id_windows", &osCfg.ImageIDWindows, selected.ID, false)
		}
	}
	// Propagate the resolved Linux image to every consumer that stores its
	// own copy of it. The provider block (image_id) is only what OpenTofu
	// renders for the control plane and main worker nodes; the bastion and
	// each Linux worker pool carry independent image fields, and `cluster
	// init` bakes provider-default (often project-scoped placeholder) image
	// IDs into both long before any cloud is chosen. A cluster bound to a
	// real cloud would otherwise boot bastion and pool nodes from a dead
	// image ID belonging to another project.
	//
	// The propagation follows the region-follower rules above: a follower
	// image that is empty or placeholder-shaped is a stale init guess and is
	// corrected silently; an image the operator set deliberately survives
	// unless --replace. Disabled pools are skipped — their baked image is
	// inert, and rewriting it would be noise in every plan.
	if resolvedLinuxImage.ID != "" {
		propagateLinuxImage := func(path string, current *string) {
			currentValue := strings.TrimSpace(*current)
			if currentValue == resolvedLinuxImage.ID {
				return
			}
			if !isDerived(currentValue) && !opts.Replace {
				result.Warnings = append(result.Warnings, fmt.Sprintf("%s is populated with %q; the resolved Linux image is %q; use --replace to repoint it", path, currentValue, resolvedLinuxImage.ID))
				return
			}
			*current = resolvedLinuxImage.ID
			changes = append(changes, Change{Path: path, Old: currentValue, New: resolvedLinuxImage.ID})
		}
		if prospective.OpenCenter.Infrastructure.Bastion.Enabled {
			propagateLinuxImage("opencenter.infrastructure.bastion.image", &prospective.OpenCenter.Infrastructure.Bastion.Image)
		}
		for i := range prospective.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
			pool := &prospective.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker[i]
			if pool.Count <= 0 {
				continue
			}
			propagateLinuxImage(fmt.Sprintf("opencenter.infrastructure.compute.additional_server_pools_worker[%d].image", i), &pool.Image)
		}
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

// selectorMatch is a resource resolved from a `--*-id` selector value.
type selectorMatch struct {
	resource  cloudopenstack.Resource
	ambiguous bool
}

// resolveSelector matches a selector value against discovered inventory by
// resource ID or by name. Names are the human-facing handle (they are what the
// candidate list prints and what operators type), while IDs remain accepted
// because they are what the config stores and what OpenTofu consumes. A
// selector matching several resources is ambiguous rather than
// first-match-wins: Glance does not enforce unique names, and silently picking
// one would bind the cluster to a resource the operator did not choose.
func resolveSelector(candidates []cloudopenstack.Resource, selector string) selectorMatch {
	matches := selectorMatches(candidates, selector)
	switch len(matches) {
	case 0:
		return selectorMatch{}
	case 1:
		return selectorMatch{resource: matches[0]}
	default:
		return selectorMatch{ambiguous: true}
	}
}

// selectorMatches lists every resource a selector value resolves to, so an
// ambiguous selector can name the near misses instead of a bare count.
func selectorMatches(candidates []cloudopenstack.Resource, selector string) []cloudopenstack.Resource {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil
	}
	matches := make([]cloudopenstack.Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ID == selector || strings.EqualFold(strings.TrimSpace(candidate.Name), selector) {
			matches = append(matches, candidate)
		}
	}
	sortResources(matches)
	return matches
}

func describeResources(values []cloudopenstack.Resource) string {
	described := make([]string, 0, len(values))
	for _, value := range values {
		described = append(described, fmt.Sprintf("%s (%s)", value.Name, value.ID))
	}
	return strings.Join(described, ", ")
}

func chooseResource(field, current, explicit string, candidates []cloudopenstack.Resource, replace bool) (cloudopenstack.Resource, bool, bool, error) {
	candidates = append([]cloudopenstack.Resource(nil), candidates...)
	sortResources(candidates)
	if strings.TrimSpace(explicit) != "" {
		match := resolveSelector(candidates, explicit)
		if match.ambiguous {
			matches := selectorMatches(candidates, explicit)
			return cloudopenstack.Resource{}, false, false, fmt.Errorf("%s selector %q matches %d discovered resources: %s; specify exactly one", field, strings.TrimSpace(explicit), len(matches), describeResources(matches))
		}
		if match.resource.ID != "" {
			return match.resource, true, false, nil
		}
		return cloudopenstack.Resource{}, false, false, fmt.Errorf("%s selector %q was not found in OpenStack discovery by ID or name", field, explicit)
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
	// Meta and Secrets are value structs that Plan may rewrite (meta.region
	// follows the discovered cloud region; the Barbican block inherits it).
	// Copying them by value keeps every write confined to the prospective
	// config, exactly like the OpenStack block below.
	copyCfg.OpenCenter.Meta = cfg.OpenCenter.Meta
	copyCfg.OpenCenter.Secrets = cfg.OpenCenter.Secrets
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

// cloneBastionAndPools detaches the fields Plan rewrites OUTSIDE the
// OpenStack provider block — infrastructure.bastion.image and each Linux
// worker pool's image — from the caller's config. cloneProviderConfig only
// deep-copies the provider block, so without this the writes below would
// mutate the caller's (and, after apply, the on-disk baseline's) bastion
// struct and pool slice in place. Bastion is a value struct reached through
// the copied Infrastructure value, so copying the slice header of the pools
// into a fresh backing array is what needs doing.
func cloneBastionAndPools(cfg *v2.Config) {
	if cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker != nil {
		pools := make([]v2.WorkerPoolConfig, len(cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker))
		copy(pools, cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker)
		cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker = pools
	}
}

func ensureNetworking(cfg *v2.OpenStackCloudConfig) {
	if cfg.Networking == nil {
		cfg.Networking = &v2.OpenStackNetworkingConfig{}
	}
}

// flavorNames returns the discovered flavor names, sorted, for error text.
func flavorNames(flavors []cloudopenstack.Resource) []string {
	names := make([]string, 0, len(flavors))
	for _, flavor := range flavors {
		if name := strings.TrimSpace(flavor.Name); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
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
	result.Warnings = append(result.Warnings, fmt.Sprintf("%s is ambiguous: %d candidates discovered; specify %s with one of the listed names or IDs", configPath, len(candidates), selector))
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
