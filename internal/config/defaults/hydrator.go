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
	"log"
	"reflect"
	"strings"
)

// DefaultSource indicates where a default value came from.
type DefaultSource string

const (
	SourceExplicit       DefaultSource = "explicit"        // User-provided value
	SourceCLIConfig      DefaultSource = "cli_config"      // CLI configuration file
	SourceProviderRegion DefaultSource = "provider_region" // Provider-region registry
	SourceProvider       DefaultSource = "provider"        // Provider-level default
	SourceGlobal         DefaultSource = "global"          // Global default
)

// Hydrator applies default values to configuration without overwriting explicit values.
type Hydrator interface {
	// Hydrate applies defaults to the configuration
	Hydrate(cfg interface{}, provider, region string) error

	// GetAppliedDefaults returns a map of field paths to their default sources
	GetAppliedDefaults() map[string]DefaultSource
}

// defaultHydrator implements the Hydrator interface.
type defaultHydrator struct {
	registry        Registry
	appliedDefaults map[string]DefaultSource
}

// NewHydrator creates a new hydrator with the given registry.
func NewHydrator(registry Registry) Hydrator {
	return &defaultHydrator{
		registry:        registry,
		appliedDefaults: make(map[string]DefaultSource),
	}
}

// Hydrate applies defaults to the configuration based on provider and region.
// It follows the precedence order: explicit > CLI > provider-region > provider > global.
// Only empty fields are populated; explicit values are never overridden.
func (h *defaultHydrator) Hydrate(cfg interface{}, provider, region string) error {
	// Get provider-region defaults from registry
	providerDefaults, err := h.registry.GetDefaults(provider, region)
	if err != nil {
		if skipsProviderRegionDefaults(provider) {
			return nil
		}

		// If region not found, log warning and continue without applying defaults
		log.Printf("[WARN] No defaults found for provider '%s' region '%s': %v", provider, region, err)
		log.Printf("[WARN] Continuing without applying provider-region defaults")
		return nil
	}

	// Apply defaults using reflection
	if err := h.applyDefaults(cfg, providerDefaults, provider, region, ""); err != nil {
		return fmt.Errorf("failed to apply defaults: %w", err)
	}

	return nil
}

func skipsProviderRegionDefaults(provider string) bool {
	return strings.EqualFold(strings.TrimSpace(provider), "kind")
}

// GetAppliedDefaults returns a map of field paths to their default sources.
func (h *defaultHydrator) GetAppliedDefaults() map[string]DefaultSource {
	return h.appliedDefaults
}

// applyDefaults recursively applies defaults to the configuration struct.
func (h *defaultHydrator) applyDefaults(cfg interface{}, providerDefaults ProviderDefaults, provider, region, fieldPath string) error {
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Build field path for tracking
		currentPath := fieldPath
		if currentPath != "" {
			currentPath += "."
		}
		currentPath += fieldType.Name

		// Handle different field types
		switch field.Kind() {
		case reflect.String:
			// Only populate if empty
			if field.String() == "" {
				if defaultValue := h.getDefaultForField(fieldType.Name, providerDefaults, provider, region); defaultValue != "" {
					field.SetString(defaultValue)
					h.appliedDefaults[currentPath] = SourceProviderRegion
					log.Printf("[DEBUG] Applied default for %s: %s (source: %s)", currentPath, defaultValue, SourceProviderRegion)
				}
			}

		case reflect.Slice:
			// Only populate if empty
			if field.Len() == 0 {
				if defaultSlice := h.getDefaultSliceForField(fieldType.Name, providerDefaults); defaultSlice != nil {
					field.Set(reflect.ValueOf(defaultSlice))
					h.appliedDefaults[currentPath] = SourceProviderRegion
					log.Printf("[DEBUG] Applied default slice for %s (source: %s)", currentPath, SourceProviderRegion)
				}
			}

		case reflect.Struct:
			// Recursively apply defaults to nested structs
			if err := h.applyDefaults(field.Addr().Interface(), providerDefaults, provider, region, currentPath); err != nil {
				return err
			}

		case reflect.Ptr:
			// Handle pointer to struct
			if field.IsNil() {
				continue
			}
			if field.Elem().Kind() == reflect.Struct {
				if err := h.applyDefaults(field.Interface(), providerDefaults, provider, region, currentPath); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// getDefaultForField returns the default value for a specific field name.
func (h *defaultHydrator) getDefaultForField(fieldName string, providerDefaults ProviderDefaults, provider, region string) string {
	switch fieldName {
	case "ImageID":
		return providerDefaults.GetImageID("24")
	case "DefaultStorageClass":
		return providerDefaults.GetDefaultStorageClass()
	case "FlavorBastion":
		return providerDefaults.GetDefaultFlavors().Bastion
	case "FlavorMaster":
		return providerDefaults.GetDefaultFlavors().Master
	case "FlavorWorker":
		return providerDefaults.GetDefaultFlavors().Worker
	case "FlavorWorkerWindows":
		return providerDefaults.GetDefaultFlavors().WorkerWindows
	default:
		return ""
	}
}

// getDefaultSliceForField returns the default slice value for a specific field name.
func (h *defaultHydrator) getDefaultSliceForField(fieldName string, providerDefaults ProviderDefaults) interface{} {
	switch fieldName {
	case "AvailabilityZones":
		return providerDefaults.GetAvailabilityZones()
	case "NTPServers":
		return providerDefaults.GetNTPServers()
	case "DNSNameservers":
		return providerDefaults.GetDNSNameservers()
	default:
		return nil
	}
}
