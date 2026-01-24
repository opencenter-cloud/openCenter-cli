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

import (
	"fmt"
	"sort"
	"sync"
)

// defaultRegistry implements the Registry interface with thread-safe access.
type defaultRegistry struct {
	mu       sync.RWMutex
	defaults map[string]map[string]ProviderDefaults // provider -> region -> defaults
}

// NewRegistry creates a new provider-region defaults registry.
func NewRegistry() Registry {
	r := &defaultRegistry{
		defaults: make(map[string]map[string]ProviderDefaults),
	}
	r.registerBuiltinDefaults()
	return r
}

// GetDefaults retrieves the defaults for a specific provider-region combination.
func (r *defaultRegistry) GetDefaults(provider, region string) (ProviderDefaults, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerDefaults, ok := r.defaults[provider]
	if !ok {
		return nil, fmt.Errorf("provider '%s' not found in registry", provider)
	}

	regionDefaults, ok := providerDefaults[region]
	if !ok {
		return nil, fmt.Errorf("region '%s' not found for provider '%s'", region, provider)
	}

	return regionDefaults, nil
}

// RegisterDefaults registers defaults for a provider-region combination.
func (r *defaultRegistry) RegisterDefaults(provider, region string, defaults ProviderDefaults) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.defaults[provider] == nil {
		r.defaults[provider] = make(map[string]ProviderDefaults)
	}
	r.defaults[provider][region] = defaults
}

// ListProviders returns all registered provider names.
func (r *defaultRegistry) ListProviders() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providers := make([]string, 0, len(r.defaults))
	for provider := range r.defaults {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	return providers
}

// ListRegions returns all registered regions for a specific provider.
func (r *defaultRegistry) ListRegions(provider string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	providerDefaults, ok := r.defaults[provider]
	if !ok {
		return []string{}
	}

	regions := make([]string, 0, len(providerDefaults))
	for region := range providerDefaults {
		regions = append(regions, region)
	}
	sort.Strings(regions)
	return regions
}

// registerBuiltinDefaults populates the registry with hardcoded provider-region defaults.
func (r *defaultRegistry) registerBuiltinDefaults() {
	// OpenStack regions
	r.RegisterDefaults("openstack", "sjc3", newOpenStackSJC3Defaults())
	r.RegisterDefaults("openstack", "dfw3", newOpenStackDFW3Defaults())
	r.RegisterDefaults("openstack", "iad3", newOpenStackIAD3Defaults())

	// AWS regions
	r.RegisterDefaults("aws", "us-east-1", newAWSUSEast1Defaults())
	r.RegisterDefaults("aws", "us-west-2", newAWSUSWest2Defaults())
	r.RegisterDefaults("aws", "eu-west-1", newAWSEUWest1Defaults())

	// GCP regions
	r.RegisterDefaults("gcp", "us-central1", newGCPUSCentral1Defaults())
	r.RegisterDefaults("gcp", "europe-west1", newGCPEuropeWest1Defaults())
}

// Global registry instance
var globalRegistry Registry
var once sync.Once

// GetGlobalRegistry returns the singleton global registry instance.
func GetGlobalRegistry() Registry {
	once.Do(func() {
		globalRegistry = NewRegistry()
	})
	return globalRegistry
}
