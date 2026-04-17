package openstack

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/availabilityzones"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/flavors"
	"github.com/gophercloud/gophercloud/openstack/imageservice/v2/images"
	networkexternal "github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/external"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/networks"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/subnets"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type CatalogItem struct {
	ID   string
	Name string
}

type DiscoveryCatalog struct {
	Images             []CatalogItem
	Flavors            []CatalogItem
	Networks           []CatalogItem
	Subnets            []CatalogItem
	ExternalNetworks   []CatalogItem
	AvailabilityZones  []CatalogItem
	DesignateAvailable bool
}

type DiscoveryClient interface {
	Discover(ctx context.Context, cfg *v2.Config) (*DiscoveryCatalog, error)
}

type GophercloudDiscoveryClient struct{}

func NewDiscoveryClient() DiscoveryClient {
	return &GophercloudDiscoveryClient{}
}

func (c *GophercloudDiscoveryClient) Discover(ctx context.Context, cfg *v2.Config) (*DiscoveryCatalog, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}

	authURL := strings.TrimSpace(creds.AuthURL)
	appCredID := strings.TrimSpace(creds.ApplicationCredentialID)
	appCredSecret := strings.TrimSpace(creds.ApplicationCredentialSecret)
	if authURL == "" || appCredID == "" || appCredSecret == "" {
		return nil, fmt.Errorf("openstack discovery requires auth_url and application credentials")
	}

	authOpts := gophercloud.AuthOptions{
		IdentityEndpoint:            authURL,
		ApplicationCredentialID:     appCredID,
		ApplicationCredentialSecret: appCredSecret,
		DomainName:                  strings.TrimSpace(creds.Domain),
		TenantName:                  firstNonEmpty(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectName), strings.TrimSpace(cfg.OpenCenter.Infrastructure.Cloud.OpenStack.TenantName)),
		AllowReauth:                 true,
	}

	provider := NewProvider(authOpts, strings.TrimSpace(creds.Region))

	computeClient, err := provider.getComputeClient()
	if err != nil {
		return nil, err
	}
	networkClient, err := provider.getNetworkClient()
	if err != nil {
		return nil, err
	}

	providerClient, err := provider.getProviderClient()
	if err != nil {
		return nil, err
	}
	imageClient, err := openstack.NewImageServiceV2(providerClient, gophercloud.EndpointOpts{Region: strings.TrimSpace(creds.Region)})
	if err != nil {
		return nil, fmt.Errorf("create image client: %w", err)
	}

	catalog := &DiscoveryCatalog{}

	if catalog.Images, err = listOpenStackImages(ctx, imageClient); err != nil {
		return nil, err
	}
	if catalog.Flavors, err = listOpenStackFlavors(ctx, computeClient); err != nil {
		return nil, err
	}
	if catalog.AvailabilityZones, err = listOpenStackAvailabilityZones(ctx, computeClient); err != nil {
		return nil, err
	}
	if catalog.Networks, catalog.ExternalNetworks, err = listOpenStackNetworks(ctx, networkClient); err != nil {
		return nil, err
	}
	if catalog.Subnets, err = listOpenStackSubnets(ctx, networkClient); err != nil {
		return nil, err
	}
	catalog.DesignateAvailable = designateAvailable(providerClient, strings.TrimSpace(creds.Region))

	return catalog, nil
}

func listOpenStackImages(ctx context.Context, client *gophercloud.ServiceClient) ([]CatalogItem, error) {
	_ = ctx
	allPages, err := images.List(client, images.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list openstack images: %w", err)
	}
	allImages, err := images.ExtractImages(allPages)
	if err != nil {
		return nil, fmt.Errorf("extract openstack images: %w", err)
	}

	items := make([]CatalogItem, 0, len(allImages))
	for _, image := range allImages {
		if !strings.EqualFold(string(image.Status), "active") {
			continue
		}
		items = append(items, CatalogItem{ID: image.ID, Name: image.Name})
	}
	sortCatalogItems(items)
	return items, nil
}

func listOpenStackFlavors(ctx context.Context, client *gophercloud.ServiceClient) ([]CatalogItem, error) {
	_ = ctx
	allPages, err := flavors.ListDetail(client, nil).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list openstack flavors: %w", err)
	}
	allFlavors, err := flavors.ExtractFlavors(allPages)
	if err != nil {
		return nil, fmt.Errorf("extract openstack flavors: %w", err)
	}

	items := make([]CatalogItem, 0, len(allFlavors))
	for _, flavor := range allFlavors {
		items = append(items, CatalogItem{ID: flavor.ID, Name: flavor.Name})
	}
	sortCatalogItems(items)
	return items, nil
}

func listOpenStackAvailabilityZones(ctx context.Context, client *gophercloud.ServiceClient) ([]CatalogItem, error) {
	_ = ctx
	allPages, err := availabilityzones.List(client).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list openstack availability zones: %w", err)
	}
	allZones, err := availabilityzones.ExtractAvailabilityZones(allPages)
	if err != nil {
		return nil, fmt.Errorf("extract openstack availability zones: %w", err)
	}

	items := make([]CatalogItem, 0, len(allZones))
	for _, zone := range allZones {
		if !zone.ZoneState.Available {
			continue
		}
		items = append(items, CatalogItem{ID: zone.ZoneName, Name: zone.ZoneName})
	}
	sortCatalogItems(items)
	return items, nil
}

func listOpenStackNetworks(ctx context.Context, client *gophercloud.ServiceClient) ([]CatalogItem, []CatalogItem, error) {
	_ = ctx
	allPages, err := networks.List(client, networks.ListOpts{}).AllPages()
	if err != nil {
		return nil, nil, fmt.Errorf("list openstack networks: %w", err)
	}
	allNetworks, err := networks.ExtractNetworks(allPages)
	if err != nil {
		return nil, nil, fmt.Errorf("extract openstack networks: %w", err)
	}

	items := make([]CatalogItem, 0, len(allNetworks))
	for _, network := range allNetworks {
		items = append(items, CatalogItem{ID: network.ID, Name: network.Name})
	}

	isExternal := true
	externalPages, err := networks.List(client, networkexternal.ListOptsExt{
		ListOptsBuilder: networks.ListOpts{},
		External:        &isExternal,
	}).AllPages()
	if err != nil {
		return nil, nil, fmt.Errorf("list external openstack networks: %w", err)
	}
	externalNetworks, err := networks.ExtractNetworks(externalPages)
	if err != nil {
		return nil, nil, fmt.Errorf("extract external openstack networks: %w", err)
	}
	externalItems := make([]CatalogItem, 0, len(externalNetworks))
	for _, network := range externalNetworks {
		externalItems = append(externalItems, CatalogItem{ID: network.ID, Name: network.Name})
	}

	sortCatalogItems(items)
	sortCatalogItems(externalItems)
	return items, externalItems, nil
}

func listOpenStackSubnets(ctx context.Context, client *gophercloud.ServiceClient) ([]CatalogItem, error) {
	_ = ctx
	allPages, err := subnets.List(client, subnets.ListOpts{}).AllPages()
	if err != nil {
		return nil, fmt.Errorf("list openstack subnets: %w", err)
	}
	allSubnets, err := subnets.ExtractSubnets(allPages)
	if err != nil {
		return nil, fmt.Errorf("extract openstack subnets: %w", err)
	}

	items := make([]CatalogItem, 0, len(allSubnets))
	for _, subnet := range allSubnets {
		items = append(items, CatalogItem{ID: subnet.ID, Name: subnet.Name})
	}
	sortCatalogItems(items)
	return items, nil
}

func designateAvailable(providerClient *gophercloud.ProviderClient, region string) bool {
	if providerClient == nil {
		return false
	}
	_, err := openstack.NewDNSV2(providerClient, gophercloud.EndpointOpts{Region: region})
	return err == nil
}

func sortCatalogItems(items []CatalogItem) {
	sort.Slice(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].Name)
		right := strings.TrimSpace(items[j].Name)
		if left == right {
			return items[i].ID < items[j].ID
		}
		return left < right
	})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
