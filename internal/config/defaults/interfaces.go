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

package defaults

// ProviderDefaults defines the interface for provider-region specific defaults.
// Implementations supply context-aware default values based on infrastructure provider and region.
type ProviderDefaults interface {
	// GetImageID returns the default image ID for the specified OS version
	GetImageID(osVersion string) string

	// GetAvailabilityZones returns the default availability zones for the region
	GetAvailabilityZones() []string

	// GetNTPServers returns the default NTP servers for the region
	GetNTPServers() []string

	// GetDNSNameservers returns the default DNS nameservers for the region
	GetDNSNameservers() []string

	// GetDefaultStorageClass returns the default storage class for the provider
	GetDefaultStorageClass() string

	// GetDefaultFlavors returns the default instance type flavors
	GetDefaultFlavors() FlavorDefaults
}

// FlavorDefaults holds default instance type/flavor names for different node types.
type FlavorDefaults struct {
	Bastion       string
	Master        string
	Worker        string
	WorkerWindows string
}

// Registry defines the interface for managing provider-region defaults.
// It maintains a registry of ProviderDefaults implementations keyed by provider and region.
type Registry interface {
	// GetDefaults retrieves the defaults for a specific provider-region combination
	GetDefaults(provider, region string) (ProviderDefaults, error)

	// RegisterDefaults registers defaults for a provider-region combination
	RegisterDefaults(provider, region string, defaults ProviderDefaults)

	// ListProviders returns all registered provider names
	ListProviders() []string

	// ListRegions returns all registered regions for a specific provider
	ListRegions(provider string) []string
}
