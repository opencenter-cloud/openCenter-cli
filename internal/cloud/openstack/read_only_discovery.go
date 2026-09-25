package openstack

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gophercloud/gophercloud"
	gopenstack "github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/identity/v2/tokens"
	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	networkexternal "github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"
)

// Resource is a stable, redacted inventory item suitable for selection output.
type Resource struct {
	ID   string `json:"id" yaml:"id"`
	Name string `json:"name" yaml:"name"`
}

// Subnet carries the network relationship needed to validate provider wiring.
type Subnet struct {
	Resource
	NetworkID string `json:"network_id" yaml:"network_id"`
}

// DiscoverySnapshot is a read-only inventory and profile metadata snapshot.
type DiscoverySnapshot struct {
	AuthURL                    string
	Region                     string
	ProjectID                  string
	ProjectName                string
	UserDomainName             string
	ProjectDomainName          string
	DomainName                 string
	Insecure                   bool
	CA                         string
	AvailabilityZonesAvailable bool
	Images                     []Resource
	WindowsImages              []Resource
	Networks                   []Resource
	Subnets                    []Subnet
	ExternalNetworks           []Resource
	AvailabilityZones          []Resource
	Flavors                    []Resource
	Warnings                   []string
}

// ProfileDiscovery performs only OpenStack reads. Optional compute inventory
// failures are returned as warnings because provider wiring does not require
// flavor matching or availability-zone enumeration by default.
type ProfileDiscovery struct{ Profile Profile }

// DiscoveryOptions controls optional inventory reads.
type DiscoveryOptions struct {
	SkipInternalNetworkAndSubnet bool
}

func NewProfileDiscovery(profile Profile) *ProfileDiscovery {
	return &ProfileDiscovery{Profile: profile}
}

func (d *ProfileDiscovery) Discover(ctx context.Context) (*DiscoverySnapshot, error) {
	return d.DiscoverWithOptions(ctx, DiscoveryOptions{})
}

func (d *ProfileDiscovery) DiscoverWithOptions(ctx context.Context, opts DiscoveryOptions) (*DiscoverySnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	provider, err := d.Profile.provider(ctx)
	if err != nil {
		return nil, err
	}
	endpoint := gophercloud.EndpointOpts{Region: strings.TrimSpace(d.Profile.Region)}
	if strings.TrimSpace(d.Profile.Interface) != "" {
		endpoint.Availability = gophercloud.Availability(strings.TrimSpace(d.Profile.Interface))
	}
	imageClient, err := gopenstack.NewImageServiceV2(provider, endpoint)
	if err != nil {
		return nil, fmt.Errorf("create image client: %w", err)
	}
	networkClient, err := gopenstack.NewNetworkV2(provider, endpoint)
	if err != nil {
		return nil, fmt.Errorf("create network client: %w", err)
	}
	imageItems, err := listProfileImages(imageClient)
	if err != nil {
		return nil, err
	}
	var networkItems, externalItems []Resource
	if opts.SkipInternalNetworkAndSubnet {
		externalItems, err = listProfileExternalNetworks(networkClient)
	} else {
		networkItems, externalItems, err = listProfileNetworks(networkClient)
	}
	if err != nil {
		return nil, err
	}
	var subnetItems []Subnet
	if !opts.SkipInternalNetworkAndSubnet {
		subnetItems, err = listProfileSubnets(networkClient)
		if err != nil {
			return nil, err
		}
	}
	projectID, projectName, projectDomain, scopeErr := authenticatedProjectScope(provider)
	if scopeErr != nil {
		// Some Keystone deployments omit project details from the token response;
		// profile values remain a compatibility fallback for those clouds.
		projectID, projectName, projectDomain = d.Profile.ProjectID, d.Profile.ProjectName, d.Profile.ProjectDomain
	}
	if strings.TrimSpace(projectID) == "" {
		projectID = d.Profile.ProjectID
	}
	if strings.TrimSpace(projectName) == "" {
		projectName = d.Profile.ProjectName
	}
	if strings.TrimSpace(projectDomain) == "" {
		projectDomain = firstProfileValue(d.Profile.ProjectDomain, d.Profile.DomainName)
	}
	result := &DiscoverySnapshot{
		AuthURL: d.Profile.AuthURL, Region: d.Profile.Region, ProjectID: projectID,
		ProjectName: projectName, UserDomainName: firstProfileValue(d.Profile.UserDomain, d.Profile.DomainName),
		ProjectDomainName: projectDomain, DomainName: projectDomain,
		Insecure: d.Profile.Insecure || (d.Profile.Verify != nil && !*d.Profile.Verify), CA: d.Profile.CACert,
		Images: imageItems, Networks: networkItems, Subnets: subnetItems, ExternalNetworks: externalItems, Warnings: []string{},
	}
	if scopeErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("authenticated project scope unavailable; using clouds.yaml fallback: %v", scopeErr))
	}
	for _, item := range imageItems {
		if strings.Contains(strings.ToLower(item.Name), "windows") {
			result.WindowsImages = append(result.WindowsImages, item)
		}
	}
	result.Images = filterNonWindows(imageItems)
	computeClient, computeErr := gopenstack.NewComputeV2(provider, endpoint)
	if computeErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("compute inventory unavailable: %v", computeErr))
		return result, nil
	}
	if result.Flavors, err = listProfileFlavors(computeClient); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	}
	if result.AvailabilityZones, err = listProfileZones(computeClient); err != nil {
		result.Warnings = append(result.Warnings, err.Error())
	} else {
		result.AvailabilityZonesAvailable = true
	}
	return result, nil
}

func authenticatedProjectScope(provider *gophercloud.ProviderClient) (string, string, string, error) {
	if provider == nil || provider.GetAuthResult() == nil {
		return "", "", "", fmt.Errorf("authenticated Keystone token is unavailable")
	}
	switch result := provider.GetAuthResult().(type) {
	case tokens3.CreateResult:
		project, err := result.ExtractProject()
		if err != nil {
			return "", "", "", fmt.Errorf("extract Keystone v3 project: %w", err)
		}
		if project == nil {
			return "", "", "", fmt.Errorf("keystone v3 token has no project scope")
		}
		return project.ID, project.Name, project.Domain.Name, nil
	case tokens3.GetResult:
		project, err := result.ExtractProject()
		if err != nil {
			return "", "", "", fmt.Errorf("extract Keystone v3 project: %w", err)
		}
		if project == nil {
			return "", "", "", fmt.Errorf("keystone v3 token has no project scope")
		}
		return project.ID, project.Name, project.Domain.Name, nil
	case tokens.CreateResult:
		token, err := result.ExtractToken()
		if err != nil {
			return "", "", "", fmt.Errorf("extract Keystone v2 project: %w", err)
		}
		if token == nil {
			return "", "", "", fmt.Errorf("keystone v2 token has no project scope")
		}
		return token.Tenant.ID, token.Tenant.Name, "", nil
	default:
		return "", "", "", fmt.Errorf("unsupported authenticated Keystone result %T", provider.GetAuthResult())
	}
}

// authenticatedUserID returns the user ID embedded in the already-authenticated
// Keystone v3 token. It intentionally performs no identity API lookup.
func authenticatedUserID(provider *gophercloud.ProviderClient) (string, error) {
	if provider == nil || provider.GetAuthResult() == nil {
		return "", fmt.Errorf("authenticated Keystone token is unavailable; cannot derive user ID")
	}

	var (
		user *tokens3.User
		err  error
	)
	switch result := provider.GetAuthResult().(type) {
	case tokens3.CreateResult:
		user, err = result.ExtractUser()
	case *tokens3.CreateResult:
		user, err = result.ExtractUser()
	case tokens3.GetResult:
		user, err = result.ExtractUser()
	case *tokens3.GetResult:
		user, err = result.ExtractUser()
	default:
		return "", fmt.Errorf("unsupported authenticated Keystone result %T for user ID derivation", provider.GetAuthResult())
	}
	if err != nil {
		return "", fmt.Errorf("extract Keystone v3 token user: %w", err)
	}
	if user == nil {
		return "", fmt.Errorf("keystone v3 token user is absent")
	}
	if strings.TrimSpace(user.ID) == "" {
		return "", fmt.Errorf("keystone v3 token user ID is blank")
	}
	return strings.TrimSpace(user.ID), nil
}

func listProfileImages(client *gophercloud.ServiceClient) ([]Resource, error) {
	pages, err := images.List(client, images.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list OpenStack images: %w", err)
	}
	items, err := images.ExtractImages(pages)
	if err != nil {
		return nil, fmt.Errorf("extract OpenStack images: %w", err)
	}
	result := make([]Resource, 0, len(items))
	for _, item := range items {
		if strings.EqualFold(string(item.Status), "active") {
			result = append(result, Resource{ID: item.ID, Name: item.Name})
		}
	}
	sortResources(result)
	return result, nil
}

func listProfileNetworks(client *gophercloud.ServiceClient) ([]Resource, []Resource, error) {
	pages, err := networks.List(client, networks.ListOpts{}).AllPages()
	if err != nil {
		return nil, nil, fmt.Errorf("list OpenStack networks: %w", err)
	}
	items, err := networks.ExtractNetworks(pages)
	if err != nil {
		return nil, nil, fmt.Errorf("extract OpenStack networks: %w", err)
	}
	all := make([]Resource, 0, len(items))
	for _, item := range items {
		all = append(all, Resource{ID: item.ID, Name: item.Name})
	}
	externalResult, err := listProfileExternalNetworks(client)
	if err != nil {
		return nil, nil, err
	}
	sortResources(all)
	sortResources(externalResult)
	internal := make([]Resource, 0, len(all))
	externalIDs := make(map[string]struct{}, len(externalResult))
	for _, item := range externalResult {
		externalIDs[item.ID] = struct{}{}
	}
	for _, item := range all {
		if _, isExternal := externalIDs[item.ID]; !isExternal {
			internal = append(internal, item)
		}
	}
	return internal, externalResult, nil
}

func listProfileExternalNetworks(client *gophercloud.ServiceClient) ([]Resource, error) {
	external := true
	externalPages, err := networks.List(client, networkexternal.ListOptsExt{ListOptsBuilder: networks.ListOpts{}, External: &external}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list OpenStack external networks: %w", err)
	}
	externalItems, err := networks.ExtractNetworks(externalPages)
	if err != nil {
		return nil, fmt.Errorf("extract OpenStack external networks: %w", err)
	}
	result := make([]Resource, 0, len(externalItems))
	for _, item := range externalItems {
		result = append(result, Resource{ID: item.ID, Name: item.Name})
	}
	sortResources(result)
	return result, nil
}

func listProfileSubnets(client *gophercloud.ServiceClient) ([]Subnet, error) {
	pages, err := subnets.List(client, subnets.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list OpenStack subnets: %w", err)
	}
	items, err := subnets.ExtractSubnets(pages)
	if err != nil {
		return nil, fmt.Errorf("extract OpenStack subnets: %w", err)
	}
	result := make([]Subnet, 0, len(items))
	for _, item := range items {
		result = append(result, Subnet{Resource: Resource{ID: item.ID, Name: item.Name}, NetworkID: item.NetworkID})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Name == result[j].Name {
			return result[i].ID < result[j].ID
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}

func listProfileFlavors(client *gophercloud.ServiceClient) ([]Resource, error) {
	pages, err := flavors.ListDetail(client, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list OpenStack flavors: %w", err)
	}
	items, err := flavors.ExtractFlavors(pages)
	if err != nil {
		return nil, fmt.Errorf("extract OpenStack flavors: %w", err)
	}
	result := make([]Resource, 0, len(items))
	for _, item := range items {
		result = append(result, Resource{ID: item.ID, Name: item.Name})
	}
	sortResources(result)
	return result, nil
}

func listProfileZones(client *gophercloud.ServiceClient) ([]Resource, error) {
	pages, err := availabilityzones.List(client).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list OpenStack availability zones: %w", err)
	}
	items, err := availabilityzones.ExtractAvailabilityZones(pages)
	if err != nil {
		return nil, fmt.Errorf("extract OpenStack availability zones: %w", err)
	}
	result := make([]Resource, 0, len(items))
	for _, item := range items {
		if item.ZoneState.Available {
			result = append(result, Resource{ID: item.ZoneName, Name: item.ZoneName})
		}
	}
	sortResources(result)
	return result, nil
}

func filterNonWindows(items []Resource) []Resource {
	result := make([]Resource, 0, len(items))
	for _, item := range items {
		if !strings.Contains(strings.ToLower(item.Name), "windows") {
			result = append(result, item)
		}
	}
	return result
}

func sortResources(items []Resource) {
	sort.Slice(items, func(i, j int) bool {
		if items[i].Name == items[j].Name {
			return items[i].ID < items[j].ID
		}
		return items[i].Name < items[j].Name
	})
}
