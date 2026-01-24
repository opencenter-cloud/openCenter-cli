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
)

// ServiceProviderValidator validates service provider compatibility with infrastructure
type ServiceProviderValidator struct {
	registry *ServiceProviderRegistry
}

// NewServiceProviderValidator creates a new service provider validator
func NewServiceProviderValidator() *ServiceProviderValidator {
	return &ServiceProviderValidator{
		registry: GetProviderRegistry(),
	}
}

// ValidateServiceProviders validates all service provider configurations against infrastructure
func (v *ServiceProviderValidator) ValidateServiceProviders(
	services map[string]interface{},
	infraProvider string,
	useDesignate bool,
	clusterName string,
) []error {
	var errors []error

	// Set Designate availability for OpenStack
	if infraProvider == string(ProviderOpenStack) {
		v.registry.SetDesignateAvailability(clusterName, useDesignate)
	}

	infraProv := InfrastructureProvider(infraProvider)

	// Validate cert-manager DNS provider
	if certMgr, ok := services["cert-manager"]; ok {
		if cfg, ok := certMgr.(*CertManagerConfig); ok && cfg.Enabled {
			if err := v.validateCertManagerDNSProvider(cfg, infraProv, clusterName); err != nil {
				errors = append(errors, err)
			}
		}
	}

	// Validate loki storage provider
	if loki, ok := services["loki"]; ok {
		if cfg, ok := loki.(*LokiConfig); ok && cfg.Enabled {
			if err := v.validateLokiStorageProvider(cfg, infraProv); err != nil {
				errors = append(errors, err)
			}
		}
	}

	// Validate tempo storage provider
	if tempo, ok := services["tempo"]; ok {
		if cfg, ok := tempo.(*TempoConfig); ok && cfg.Enabled {
			if err := v.validateTempoStorageProvider(cfg, infraProv); err != nil {
				errors = append(errors, err)
			}
		}
	}

	// Validate velero storage provider
	if velero, ok := services["velero"]; ok {
		if cfg, ok := velero.(*VeleroConfig); ok && cfg.Enabled {
			if err := v.validateVeleroStorageProvider(cfg, infraProv); err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

// validateCertManagerDNSProvider validates cert-manager DNS provider configuration
func (v *ServiceProviderValidator) validateCertManagerDNSProvider(
	cfg *CertManagerConfig,
	infraProvider InfrastructureProvider,
	clusterName string,
) error {
	// If DNS provider is not set, auto-select
	if cfg.DNSProvider == "" {
		provider, err := v.registry.AutoSelectProvider("cert-manager", "dns", infraProvider, clusterName)
		if err != nil {
			return fmt.Errorf("cert-manager: failed to auto-select DNS provider: %w", err)
		}
		cfg.DNSProvider = string(provider)
		return nil
	}

	// Validate explicitly configured DNS provider
	dnsProvider := ServiceProviderType(cfg.DNSProvider)
	if err := v.registry.ValidateCompatibility("cert-manager", "dns", infraProvider, dnsProvider); err != nil {
		compatible := v.registry.GetCompatibleProviders("cert-manager", "dns", infraProvider)
		return fmt.Errorf("cert-manager: %w. Compatible providers: %v", err, compatible)
	}

	return nil
}

// validateLokiStorageProvider validates loki storage provider configuration
func (v *ServiceProviderValidator) validateLokiStorageProvider(
	cfg *LokiConfig,
	infraProvider InfrastructureProvider,
) error {
	// If storage type is not set, auto-select
	if cfg.StorageType == "" {
		provider, err := v.registry.GetDefaultProvider("loki", "storage", infraProvider)
		if err != nil {
			return fmt.Errorf("loki: failed to get default storage provider: %w", err)
		}
		cfg.StorageType = string(provider)
		return nil
	}

	// Validate explicitly configured storage provider
	storageProvider := ServiceProviderType(cfg.StorageType)
	if err := v.registry.ValidateCompatibility("loki", "storage", infraProvider, storageProvider); err != nil {
		compatible := v.registry.GetCompatibleProviders("loki", "storage", infraProvider)
		return fmt.Errorf("loki: %w. Compatible providers: %v", err, compatible)
	}

	return nil
}

// validateTempoStorageProvider validates tempo storage provider configuration
func (v *ServiceProviderValidator) validateTempoStorageProvider(
	cfg *TempoConfig,
	infraProvider InfrastructureProvider,
) error {
	// If storage type is not set, auto-select
	if cfg.StorageType == "" {
		provider, err := v.registry.GetDefaultProvider("tempo", "storage", infraProvider)
		if err != nil {
			return fmt.Errorf("tempo: failed to get default storage provider: %w", err)
		}
		cfg.StorageType = string(provider)
		return nil
	}

	// Validate explicitly configured storage provider
	storageProvider := ServiceProviderType(cfg.StorageType)
	if err := v.registry.ValidateCompatibility("tempo", "storage", infraProvider, storageProvider); err != nil {
		compatible := v.registry.GetCompatibleProviders("tempo", "storage", infraProvider)
		return fmt.Errorf("tempo: %w. Compatible providers: %v", err, compatible)
	}

	return nil
}

// validateVeleroStorageProvider validates velero storage provider configuration
func (v *ServiceProviderValidator) validateVeleroStorageProvider(
	cfg *VeleroConfig,
	infraProvider InfrastructureProvider,
) error {
	// If storage type is not set, auto-select
	if cfg.StorageType == "" {
		provider, err := v.registry.GetDefaultProvider("velero", "storage", infraProvider)
		if err != nil {
			return fmt.Errorf("velero: failed to get default storage provider: %w", err)
		}
		cfg.StorageType = string(provider)
		return nil
	}

	// Validate explicitly configured storage provider
	storageProvider := ServiceProviderType(cfg.StorageType)
	if err := v.registry.ValidateCompatibility("velero", "storage", infraProvider, storageProvider); err != nil {
		compatible := v.registry.GetCompatibleProviders("velero", "storage", infraProvider)
		return fmt.Errorf("velero: %w. Compatible providers: %v", err, compatible)
	}

	return nil
}

// ApplyDefaultProviders applies default service providers based on infrastructure
func (v *ServiceProviderValidator) ApplyDefaultProviders(
	services map[string]interface{},
	infraProvider string,
	useDesignate bool,
	clusterName string,
) error {
	// Set Designate availability for OpenStack
	if infraProvider == string(ProviderOpenStack) {
		v.registry.SetDesignateAvailability(clusterName, useDesignate)
	}

	infraProv := InfrastructureProvider(infraProvider)

	// Apply cert-manager DNS provider default
	if certMgr, ok := services["cert-manager"]; ok {
		if cfg, ok := certMgr.(*CertManagerConfig); ok && cfg.Enabled && cfg.DNSProvider == "" {
			provider, err := v.registry.AutoSelectProvider("cert-manager", "dns", infraProv, clusterName)
			if err != nil {
				return fmt.Errorf("failed to auto-select cert-manager DNS provider: %w", err)
			}
			cfg.DNSProvider = string(provider)
		}
	}

	// Apply loki storage provider default
	if loki, ok := services["loki"]; ok {
		if cfg, ok := loki.(*LokiConfig); ok && cfg.Enabled && cfg.StorageType == "" {
			provider, err := v.registry.GetDefaultProvider("loki", "storage", infraProv)
			if err != nil {
				return fmt.Errorf("failed to get default loki storage provider: %w", err)
			}
			cfg.StorageType = string(provider)
		}
	}

	// Apply tempo storage provider default
	if tempo, ok := services["tempo"]; ok {
		if cfg, ok := tempo.(*TempoConfig); ok && cfg.Enabled && cfg.StorageType == "" {
			provider, err := v.registry.GetDefaultProvider("tempo", "storage", infraProv)
			if err != nil {
				return fmt.Errorf("failed to get default tempo storage provider: %w", err)
			}
			cfg.StorageType = string(provider)
		}
	}

	// Apply velero storage provider default
	if velero, ok := services["velero"]; ok {
		if cfg, ok := velero.(*VeleroConfig); ok && cfg.Enabled && cfg.StorageType == "" {
			provider, err := v.registry.GetDefaultProvider("velero", "storage", infraProv)
			if err != nil {
				return fmt.Errorf("failed to get default velero storage provider: %w", err)
			}
			cfg.StorageType = string(provider)
		}
	}

	return nil
}
