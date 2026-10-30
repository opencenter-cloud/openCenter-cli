// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package services

import (
	"fmt"
	"strings"
	"sync"
)

// ServiceProviderType represents different provider types for services
type ServiceProviderType string

const (
	// DNS providers for cert-manager
	DNSProviderRoute53    ServiceProviderType = "route53"
	DNSProviderDesignate  ServiceProviderType = "designate"
	DNSProviderCloudflare ServiceProviderType = "cloudflare"
	DNSProviderCloudDNS   ServiceProviderType = "clouddns"
	DNSProviderAzureDNS   ServiceProviderType = "azuredns"
	// DNSProviderNone disables ACME entirely: no DNS-01 solver, no public
	// ClusterIssuer, and every Gateway/Harbor certificate falls back to the
	// in-cluster self-signed rackspace-ca. It is a sentinel, not a provider:
	// AutoSelectDNSProvider never picks it, so it is strictly opt-in.
	//
	// A bool would not work here. AutoSelectDNSProvider always returns a
	// non-empty provider (OpenStack without Designate falls back to
	// cloudflare), so ResolvedDNSProvider() == "" is unreachable while
	// cert-manager is enabled and the internal-CA branch of issuer selection
	// could never be reached deliberately.
	DNSProviderNone ServiceProviderType = "none"

	// Storage providers for backup services
	StorageProviderS3    ServiceProviderType = "s3"
	StorageProviderSwift ServiceProviderType = "swift"
	StorageProviderGCS   ServiceProviderType = "gcs"
	StorageProviderAzure ServiceProviderType = "azure"
	StorageProviderNone  ServiceProviderType = "none"
)

// InfrastructureProvider represents the infrastructure provider type
type InfrastructureProvider string

const (
	ProviderOpenStack InfrastructureProvider = "openstack"
	ProviderAWS       InfrastructureProvider = "aws"
	ProviderGCP       InfrastructureProvider = "gcp"
	ProviderAzure     InfrastructureProvider = "azure"
	ProviderBareMetal InfrastructureProvider = "baremetal"
	ProviderVSphere   InfrastructureProvider = "vsphere"
)

// ServiceProviderCompatibility defines compatibility between infrastructure and service providers
type ServiceProviderCompatibility struct {
	InfraProvider   InfrastructureProvider
	ServiceProvider ServiceProviderType
	Compatible      bool
	Reason          string
}

// ServiceProviderRegistry manages service provider compatibility and auto-selection
type ServiceProviderRegistry struct {
	mu                    sync.RWMutex
	compatibilityMatrix   map[string]map[string]ServiceProviderCompatibility
	defaultProviders      map[string]map[InfrastructureProvider]ServiceProviderType
	designateAvailability map[string]bool // Track if Designate is available per cluster
}

var (
	globalProviderRegistry *ServiceProviderRegistry
	registryOnce           sync.Once
)

// GetProviderRegistry returns the global service provider registry
func GetProviderRegistry() *ServiceProviderRegistry {
	registryOnce.Do(func() {
		globalProviderRegistry = newServiceProviderRegistry()
	})
	return globalProviderRegistry
}

// newServiceProviderRegistry creates and initializes a new service provider registry
func newServiceProviderRegistry() *ServiceProviderRegistry {
	r := &ServiceProviderRegistry{
		compatibilityMatrix:   make(map[string]map[string]ServiceProviderCompatibility),
		defaultProviders:      make(map[string]map[InfrastructureProvider]ServiceProviderType),
		designateAvailability: make(map[string]bool),
	}

	r.initializeCompatibilityMatrix()
	r.initializeDefaultProviders()

	return r
}

// initializeCompatibilityMatrix sets up the compatibility matrix for service providers
func (r *ServiceProviderRegistry) initializeCompatibilityMatrix() {
	// DNS provider compatibility for cert-manager
	r.registerCompatibility("cert-manager", "dns", ProviderAWS, DNSProviderRoute53, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderAWS, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderAWS, DNSProviderDesignate, false, "Designate is OpenStack-specific")

	r.registerCompatibility("cert-manager", "dns", ProviderOpenStack, DNSProviderDesignate, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderOpenStack, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderOpenStack, DNSProviderRoute53, false, "Route53 is AWS-specific")

	r.registerCompatibility("cert-manager", "dns", ProviderGCP, DNSProviderCloudDNS, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderGCP, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderGCP, DNSProviderRoute53, false, "Route53 is AWS-specific")
	r.registerCompatibility("cert-manager", "dns", ProviderGCP, DNSProviderDesignate, false, "Designate is OpenStack-specific")

	r.registerCompatibility("cert-manager", "dns", ProviderAzure, DNSProviderAzureDNS, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderAzure, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderAzure, DNSProviderRoute53, false, "Route53 is AWS-specific")
	r.registerCompatibility("cert-manager", "dns", ProviderAzure, DNSProviderDesignate, false, "Designate is OpenStack-specific")

	r.registerCompatibility("cert-manager", "dns", ProviderBareMetal, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderBareMetal, DNSProviderRoute53, false, "Route53 requires AWS infrastructure")
	r.registerCompatibility("cert-manager", "dns", ProviderBareMetal, DNSProviderDesignate, false, "Designate requires OpenStack infrastructure")

	r.registerCompatibility("cert-manager", "dns", ProviderVSphere, DNSProviderCloudflare, true, "")
	r.registerCompatibility("cert-manager", "dns", ProviderVSphere, DNSProviderRoute53, false, "Route53 requires AWS infrastructure")
	r.registerCompatibility("cert-manager", "dns", ProviderVSphere, DNSProviderDesignate, false, "Designate requires OpenStack infrastructure")

	// Platform bulk data is S3-compatible regardless of infrastructure provider.
	// Legacy Swift, GCS, and Azure selections remain recognized only to return an
	// actionable migration error rather than silently selecting another backend.
	for _, serviceName := range []string{"loki", "velero", "tempo"} {
		for _, infrastructureProvider := range []InfrastructureProvider{ProviderAWS, ProviderOpenStack, ProviderGCP, ProviderAzure, ProviderBareMetal, ProviderVSphere} {
			r.registerCompatibility(serviceName, "storage", infrastructureProvider, StorageProviderS3, true, "")
			if serviceName == "loki" || serviceName == "velero" {
				r.registerCompatibility(serviceName, "storage", infrastructureProvider, StorageProviderNone, true, "")
			}
			if serviceName == "tempo" {
				r.registerCompatibility(serviceName, "storage", infrastructureProvider, ServiceProviderType("filesystem"), true, "")
			}
			r.registerCompatibility(serviceName, "storage", infrastructureProvider, StorageProviderSwift, false, "Swift is no longer supported; migrate to S3-compatible storage")
			r.registerCompatibility(serviceName, "storage", infrastructureProvider, StorageProviderGCS, false, "GCS is not a platform bulk-data contract; use an S3-compatible endpoint")
			r.registerCompatibility(serviceName, "storage", infrastructureProvider, StorageProviderAzure, false, "Azure Blob is not a platform bulk-data contract; use an S3-compatible endpoint")
		}
	}

}

// initializeDefaultProviders sets up default provider selections based on infrastructure
func (r *ServiceProviderRegistry) initializeDefaultProviders() {
	// DNS provider defaults for cert-manager
	dnsDefaults := make(map[InfrastructureProvider]ServiceProviderType)
	dnsDefaults[ProviderAWS] = DNSProviderRoute53
	dnsDefaults[ProviderGCP] = DNSProviderCloudDNS
	dnsDefaults[ProviderAzure] = DNSProviderAzureDNS
	dnsDefaults[ProviderBareMetal] = DNSProviderCloudflare
	dnsDefaults[ProviderVSphere] = DNSProviderCloudflare
	// OpenStack default is conditional - set by SetDesignateAvailability
	dnsDefaults[ProviderOpenStack] = DNSProviderCloudflare // fallback default
	r.defaultProviders["cert-manager:dns"] = dnsDefaults

	// Object storage is not inferred from infrastructure. All service defaults
	// use the portable S3 contract; the cluster storage profile decides whether
	// that endpoint is external or managed RustFS.
	for _, serviceName := range []string{"loki", "velero", "tempo"} {
		defaults := make(map[InfrastructureProvider]ServiceProviderType)
		for _, infrastructureProvider := range []InfrastructureProvider{ProviderAWS, ProviderOpenStack, ProviderGCP, ProviderAzure, ProviderBareMetal, ProviderVSphere} {
			defaults[infrastructureProvider] = StorageProviderS3
		}
		r.defaultProviders[serviceName+":storage"] = defaults
	}

}

// registerCompatibility registers a compatibility entry
func (r *ServiceProviderRegistry) registerCompatibility(
	serviceName, providerType string,
	infraProvider InfrastructureProvider,
	serviceProvider ServiceProviderType,
	compatible bool,
	reason string,
) {
	key := fmt.Sprintf("%s:%s", serviceName, providerType)
	if r.compatibilityMatrix[key] == nil {
		r.compatibilityMatrix[key] = make(map[string]ServiceProviderCompatibility)
	}

	compatKey := fmt.Sprintf("%s:%s", infraProvider, serviceProvider)
	r.compatibilityMatrix[key][compatKey] = ServiceProviderCompatibility{
		InfraProvider:   infraProvider,
		ServiceProvider: serviceProvider,
		Compatible:      compatible,
		Reason:          reason,
	}
}

// SetDesignateAvailability sets whether Designate is available for a cluster
// This affects the default DNS provider selection for OpenStack
func (r *ServiceProviderRegistry) SetDesignateAvailability(clusterName string, available bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.designateAvailability[clusterName] = available

	// Update OpenStack DNS default based on Designate availability
	if dnsDefaults, ok := r.defaultProviders["cert-manager:dns"]; ok {
		if available {
			dnsDefaults[ProviderOpenStack] = DNSProviderDesignate
		} else {
			dnsDefaults[ProviderOpenStack] = DNSProviderCloudflare
		}
	}
}

// AutoSelectDNSProvider resolves the cert-manager DNS provider for an
// infrastructure provider without mutating registry state. It is the pure form
// of AutoSelectProvider for the cert-manager/dns capability: Designate only for
// OpenStack clusters that enable it, otherwise the per-provider default
// (route53 on AWS, clouddns on GCP, azuredns on Azure, cloudflare elsewhere).
// An unknown or empty infrastructure provider yields cloudflare, the portable
// choice.
func AutoSelectDNSProvider(infraProvider, clusterName string, designateAvailable bool) ServiceProviderType {
	if InfrastructureProvider(strings.ToLower(strings.TrimSpace(infraProvider))) == ProviderOpenStack {
		if designateAvailable {
			return DNSProviderDesignate
		}
		return DNSProviderCloudflare
	}
	provider, err := GetProviderRegistry().GetDefaultProvider("cert-manager", "dns", InfrastructureProvider(strings.ToLower(strings.TrimSpace(infraProvider))))
	if err != nil {
		return DNSProviderCloudflare
	}
	return provider
}

// GetDefaultProvider returns the default service provider for a given infrastructure provider
func (r *ServiceProviderRegistry) GetDefaultProvider(
	serviceName, providerType string,
	infraProvider InfrastructureProvider,
) (ServiceProviderType, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", serviceName, providerType)
	defaults, ok := r.defaultProviders[key]
	if !ok {
		return "", fmt.Errorf("no default providers configured for %s", key)
	}

	provider, ok := defaults[infraProvider]
	if !ok {
		return "", fmt.Errorf("no default provider for infrastructure %s", infraProvider)
	}

	return provider, nil
}

// ValidateCompatibility validates if a service provider is compatible with an infrastructure provider
func (r *ServiceProviderRegistry) ValidateCompatibility(
	serviceName, providerType string,
	infraProvider InfrastructureProvider,
	serviceProvider ServiceProviderType,
) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", serviceName, providerType)
	compatMap, ok := r.compatibilityMatrix[key]
	if !ok {
		return fmt.Errorf("no compatibility matrix for %s", key)
	}

	compatKey := fmt.Sprintf("%s:%s", infraProvider, serviceProvider)
	compat, ok := compatMap[compatKey]
	if !ok {
		return fmt.Errorf("unknown compatibility for %s with %s on %s", serviceName, serviceProvider, infraProvider)
	}

	if !compat.Compatible {
		return fmt.Errorf("%s provider %s is not compatible with %s infrastructure: %s",
			serviceName, serviceProvider, infraProvider, compat.Reason)
	}

	return nil
}

// GetCompatibleProviders returns all compatible service providers for an infrastructure provider
func (r *ServiceProviderRegistry) GetCompatibleProviders(
	serviceName, providerType string,
	infraProvider InfrastructureProvider,
) []ServiceProviderType {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := fmt.Sprintf("%s:%s", serviceName, providerType)
	compatMap, ok := r.compatibilityMatrix[key]
	if !ok {
		return nil
	}

	var compatible []ServiceProviderType
	for _, compat := range compatMap {
		if compat.InfraProvider == infraProvider && compat.Compatible {
			compatible = append(compatible, compat.ServiceProvider)
		}
	}

	return compatible
}

// AutoSelectProvider automatically selects the best service provider based on infrastructure
// It considers Designate availability for OpenStack when selecting DNS providers
func (r *ServiceProviderRegistry) AutoSelectProvider(
	serviceName, providerType string,
	infraProvider InfrastructureProvider,
	clusterName string,
) (ServiceProviderType, error) {
	// Special handling for OpenStack DNS provider selection
	if serviceName == "cert-manager" && providerType == "dns" && infraProvider == ProviderOpenStack {
		r.mu.RLock()
		designateAvailable := r.designateAvailability[clusterName]
		r.mu.RUnlock()

		if designateAvailable {
			return DNSProviderDesignate, nil
		}
		return DNSProviderCloudflare, nil
	}

	// Use default provider for other cases
	return r.GetDefaultProvider(serviceName, providerType, infraProvider)
}
