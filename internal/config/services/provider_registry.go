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

	// Storage providers for backup services
	StorageProviderS3    ServiceProviderType = "s3"
	StorageProviderSwift ServiceProviderType = "swift"
	StorageProviderGCS   ServiceProviderType = "gcs"
	StorageProviderAzure ServiceProviderType = "azure"
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

	// Storage provider compatibility for backup services (loki, velero, tempo)
	r.registerCompatibility("loki", "storage", ProviderAWS, StorageProviderS3, true, "")
	r.registerCompatibility("loki", "storage", ProviderAWS, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("loki", "storage", ProviderOpenStack, StorageProviderSwift, true, "")
	r.registerCompatibility("loki", "storage", ProviderOpenStack, StorageProviderS3, true, "")

	r.registerCompatibility("loki", "storage", ProviderGCP, StorageProviderGCS, true, "")
	r.registerCompatibility("loki", "storage", ProviderGCP, StorageProviderS3, true, "")
	r.registerCompatibility("loki", "storage", ProviderGCP, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("loki", "storage", ProviderAzure, StorageProviderAzure, true, "")
	r.registerCompatibility("loki", "storage", ProviderAzure, StorageProviderS3, true, "")
	r.registerCompatibility("loki", "storage", ProviderAzure, StorageProviderSwift, false, "Swift is OpenStack-specific")

	// Velero storage compatibility
	r.registerCompatibility("velero", "storage", ProviderAWS, StorageProviderS3, true, "")
	r.registerCompatibility("velero", "storage", ProviderAWS, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("velero", "storage", ProviderOpenStack, StorageProviderSwift, true, "")
	r.registerCompatibility("velero", "storage", ProviderOpenStack, StorageProviderS3, true, "")

	r.registerCompatibility("velero", "storage", ProviderGCP, StorageProviderGCS, true, "")
	r.registerCompatibility("velero", "storage", ProviderGCP, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("velero", "storage", ProviderAzure, StorageProviderAzure, true, "")
	r.registerCompatibility("velero", "storage", ProviderAzure, StorageProviderSwift, false, "Swift is OpenStack-specific")

	// Tempo storage compatibility
	r.registerCompatibility("tempo", "storage", ProviderAWS, StorageProviderS3, true, "")
	r.registerCompatibility("tempo", "storage", ProviderAWS, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("tempo", "storage", ProviderOpenStack, StorageProviderSwift, true, "")
	r.registerCompatibility("tempo", "storage", ProviderOpenStack, StorageProviderS3, true, "")

	r.registerCompatibility("tempo", "storage", ProviderGCP, StorageProviderGCS, true, "")
	r.registerCompatibility("tempo", "storage", ProviderGCP, StorageProviderSwift, false, "Swift is OpenStack-specific")

	r.registerCompatibility("tempo", "storage", ProviderAzure, StorageProviderAzure, true, "")
	r.registerCompatibility("tempo", "storage", ProviderAzure, StorageProviderSwift, false, "Swift is OpenStack-specific")
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

	// Storage provider defaults for loki
	lokiStorageDefaults := make(map[InfrastructureProvider]ServiceProviderType)
	lokiStorageDefaults[ProviderAWS] = StorageProviderS3
	lokiStorageDefaults[ProviderOpenStack] = StorageProviderSwift
	lokiStorageDefaults[ProviderGCP] = StorageProviderGCS
	lokiStorageDefaults[ProviderAzure] = StorageProviderAzure
	lokiStorageDefaults[ProviderBareMetal] = StorageProviderS3
	lokiStorageDefaults[ProviderVSphere] = StorageProviderS3
	r.defaultProviders["loki:storage"] = lokiStorageDefaults

	// Storage provider defaults for velero
	veleroStorageDefaults := make(map[InfrastructureProvider]ServiceProviderType)
	veleroStorageDefaults[ProviderAWS] = StorageProviderS3
	veleroStorageDefaults[ProviderOpenStack] = StorageProviderSwift
	veleroStorageDefaults[ProviderGCP] = StorageProviderGCS
	veleroStorageDefaults[ProviderAzure] = StorageProviderAzure
	veleroStorageDefaults[ProviderBareMetal] = StorageProviderS3
	veleroStorageDefaults[ProviderVSphere] = StorageProviderS3
	r.defaultProviders["velero:storage"] = veleroStorageDefaults

	// Storage provider defaults for tempo
	tempoStorageDefaults := make(map[InfrastructureProvider]ServiceProviderType)
	tempoStorageDefaults[ProviderAWS] = StorageProviderS3
	tempoStorageDefaults[ProviderOpenStack] = StorageProviderSwift
	tempoStorageDefaults[ProviderGCP] = StorageProviderGCS
	tempoStorageDefaults[ProviderAzure] = StorageProviderAzure
	tempoStorageDefaults[ProviderBareMetal] = StorageProviderS3
	tempoStorageDefaults[ProviderVSphere] = StorageProviderS3
	r.defaultProviders["tempo:storage"] = tempoStorageDefaults
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
