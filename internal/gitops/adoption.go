// Copyright 2025 opencenter-cloud
// Licensed under the Apache License, Version 2.0

package gitops

import (
	"reflect"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
)

// AdoptionMode mirrors config.AdoptionMode for use in gitops package.
// This avoids circular imports while maintaining type safety.
type AdoptionMode = config.AdoptionMode

// Re-export adoption mode constants for convenience.
const (
	AdoptionModeManaged  = config.AdoptionModeManaged
	AdoptionModeExternal = config.AdoptionModeExternal
	AdoptionModeSync     = config.AdoptionModeSync
	AdoptionModeDeferred = config.AdoptionModeDeferred
	AdoptionModeTakeover = config.AdoptionModeTakeover
)

// GetAdoptionMode extracts the adoption mode from a service configuration.
// It uses reflection to access the AdoptionMode field since service configs
// are stored as interface{} in the ServiceMap.
// Returns AdoptionModeManaged if the field is not found or empty (default behavior).
func GetAdoptionMode(serviceCfg any) AdoptionMode {
	if serviceCfg == nil {
		return AdoptionModeManaged
	}

	val := reflect.ValueOf(serviceCfg)
	if val.Kind() == reflect.Ptr {
		if val.IsNil() {
			return AdoptionModeManaged
		}
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return AdoptionModeManaged
	}

	// Try to find AdoptionMode field
	field := val.FieldByName("AdoptionMode")
	if !field.IsValid() {
		return AdoptionModeManaged
	}

	if field.Kind() != reflect.String {
		return AdoptionModeManaged
	}

	mode := AdoptionMode(field.String())
	if mode == "" {
		return AdoptionModeManaged
	}

	return mode
}

// IsServiceExternal returns true if the service has adoption_mode: external.
// External services are not rendered by Flux at all.
func IsServiceExternal(serviceCfg any) bool {
	return GetAdoptionMode(serviceCfg) == AdoptionModeExternal
}

// ShouldRenderService returns true if the service should have manifests rendered.
// Returns false for disabled services or services with adoption_mode: external.
func ShouldRenderService(serviceCfg any) bool {
	if IsServiceDisabled(serviceCfg) {
		return false
	}
	if IsServiceExternal(serviceCfg) {
		return false
	}
	return true
}

// AdoptionSettings contains the Flux Kustomization settings derived from adoption mode.
type AdoptionSettings struct {
	// Force determines if Flux should overwrite existing resource fields.
	// false for sync mode, true otherwise.
	Force bool

	// Suspend determines if the Kustomization should be suspended.
	// true for deferred mode, false otherwise.
	Suspend bool
}

// GetAdoptionSettings returns the Flux Kustomization settings for a given adoption mode.
func GetAdoptionSettings(mode AdoptionMode) AdoptionSettings {
	switch mode {
	case AdoptionModeSync:
		return AdoptionSettings{Force: false, Suspend: false}
	case AdoptionModeDeferred:
		return AdoptionSettings{Force: true, Suspend: true}
	case AdoptionModeManaged, AdoptionModeTakeover:
		return AdoptionSettings{Force: true, Suspend: false}
	default:
		// Default to managed behavior
		return AdoptionSettings{Force: true, Suspend: false}
	}
}

// GetServiceAdoptionSettings extracts adoption mode from a service config and returns settings.
func GetServiceAdoptionSettings(serviceCfg any) AdoptionSettings {
	return GetAdoptionSettings(GetAdoptionMode(serviceCfg))
}
