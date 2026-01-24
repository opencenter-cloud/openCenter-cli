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
	"strings"
	"testing"
)

func TestValidateCertManagerDNSProvider_AutoSelect(t *testing.T) {
	validator := NewServiceProviderValidator()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		expected      string
	}{
		{
			name:          "AWS auto-selects Route53",
			infraProvider: ProviderAWS,
			expected:      "route53",
		},
		{
			name:          "GCP auto-selects CloudDNS",
			infraProvider: ProviderGCP,
			expected:      "clouddns",
		},
		{
			name:          "Azure auto-selects AzureDNS",
			infraProvider: ProviderAzure,
			expected:      "azuredns",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &CertManagerConfig{
				BaseConfig: BaseConfig{Enabled: true},
			}

			err := validator.validateCertManagerDNSProvider(cfg, tt.infraProvider, "test-cluster")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.DNSProvider != tt.expected {
				t.Errorf("expected DNS provider %s, got %s", tt.expected, cfg.DNSProvider)
			}
		})
	}
}

func TestValidateCertManagerDNSProvider_OpenStackWithDesignate(t *testing.T) {
	validator := NewServiceProviderValidator()

	// Set Designate as available
	validator.registry.SetDesignateAvailability("test-cluster", true)

	cfg := &CertManagerConfig{
		BaseConfig: BaseConfig{Enabled: true},
	}

	err := validator.validateCertManagerDNSProvider(cfg, ProviderOpenStack, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DNSProvider != "designate" {
		t.Errorf("expected DNS provider designate, got %s", cfg.DNSProvider)
	}
}

func TestValidateCertManagerDNSProvider_OpenStackWithoutDesignate(t *testing.T) {
	validator := NewServiceProviderValidator()

	// Set Designate as unavailable
	validator.registry.SetDesignateAvailability("test-cluster", false)

	cfg := &CertManagerConfig{
		BaseConfig: BaseConfig{Enabled: true},
	}

	err := validator.validateCertManagerDNSProvider(cfg, ProviderOpenStack, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DNSProvider != "cloudflare" {
		t.Errorf("expected DNS provider cloudflare, got %s", cfg.DNSProvider)
	}
}

func TestValidateCertManagerDNSProvider_ExplicitValid(t *testing.T) {
	validator := NewServiceProviderValidator()

	cfg := &CertManagerConfig{
		BaseConfig:  BaseConfig{Enabled: true},
		DNSProvider: "route53",
	}

	err := validator.validateCertManagerDNSProvider(cfg, ProviderAWS, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DNSProvider != "route53" {
		t.Errorf("expected DNS provider route53, got %s", cfg.DNSProvider)
	}
}

func TestValidateCertManagerDNSProvider_ExplicitInvalid(t *testing.T) {
	validator := NewServiceProviderValidator()

	cfg := &CertManagerConfig{
		BaseConfig:  BaseConfig{Enabled: true},
		DNSProvider: "route53",
	}

	err := validator.validateCertManagerDNSProvider(cfg, ProviderOpenStack, "test-cluster")
	if err == nil {
		t.Fatal("expected error for incompatible DNS provider")
	}

	if !strings.Contains(err.Error(), "not compatible") {
		t.Errorf("expected compatibility error, got: %v", err)
	}

	if !strings.Contains(err.Error(), "Compatible providers") {
		t.Errorf("expected error to suggest compatible providers, got: %v", err)
	}
}

func TestValidateLokiStorageProvider_AutoSelect(t *testing.T) {
	validator := NewServiceProviderValidator()

	tests := []struct {
		name          string
		infraProvider InfrastructureProvider
		expected      string
	}{
		{
			name:          "AWS auto-selects S3",
			infraProvider: ProviderAWS,
			expected:      "s3",
		},
		{
			name:          "OpenStack auto-selects Swift",
			infraProvider: ProviderOpenStack,
			expected:      "swift",
		},
		{
			name:          "GCP auto-selects GCS",
			infraProvider: ProviderGCP,
			expected:      "gcs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &LokiConfig{
				BaseConfig: BaseConfig{Enabled: true},
			}

			err := validator.validateLokiStorageProvider(cfg, tt.infraProvider)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if cfg.StorageType != tt.expected {
				t.Errorf("expected storage type %s, got %s", tt.expected, cfg.StorageType)
			}
		})
	}
}

func TestValidateLokiStorageProvider_ExplicitInvalid(t *testing.T) {
	validator := NewServiceProviderValidator()

	cfg := &LokiConfig{
		BaseConfig:  BaseConfig{Enabled: true},
		StorageType: "swift",
	}

	err := validator.validateLokiStorageProvider(cfg, ProviderAWS)
	if err == nil {
		t.Fatal("expected error for incompatible storage provider")
	}

	if !strings.Contains(err.Error(), "not compatible") {
		t.Errorf("expected compatibility error, got: %v", err)
	}
}

func TestValidateServiceProviders_MultipleServices(t *testing.T) {
	validator := NewServiceProviderValidator()

	services := map[string]interface{}{
		"cert-manager": &CertManagerConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			DNSProvider: "route53",
		},
		"loki": &LokiConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			StorageType: "s3",
		},
		"tempo": &TempoConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			StorageType: "s3",
		},
	}

	errors := validator.ValidateServiceProviders(services, "aws", false, "test-cluster")
	if len(errors) > 0 {
		t.Errorf("unexpected errors: %v", errors)
	}
}

func TestValidateServiceProviders_MultipleErrors(t *testing.T) {
	validator := NewServiceProviderValidator()

	services := map[string]interface{}{
		"cert-manager": &CertManagerConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			DNSProvider: "route53", // Invalid for OpenStack
		},
		"loki": &LokiConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			StorageType: "swift", // Invalid for AWS (but we're using OpenStack)
		},
	}

	errors := validator.ValidateServiceProviders(services, "openstack", false, "test-cluster")
	if len(errors) != 1 {
		t.Errorf("expected 1 error, got %d: %v", len(errors), errors)
	}
}

func TestApplyDefaultProviders(t *testing.T) {
	validator := NewServiceProviderValidator()

	services := map[string]interface{}{
		"cert-manager": &CertManagerConfig{
			BaseConfig: BaseConfig{Enabled: true},
		},
		"loki": &LokiConfig{
			BaseConfig: BaseConfig{Enabled: true},
		},
		"tempo": &TempoConfig{
			BaseConfig: BaseConfig{Enabled: true},
		},
		"velero": &VeleroConfig{
			BaseConfig: BaseConfig{Enabled: true},
		},
	}

	err := validator.ApplyDefaultProviders(services, "aws", false, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify defaults were applied
	certMgr := services["cert-manager"].(*CertManagerConfig)
	if certMgr.DNSProvider != "route53" {
		t.Errorf("expected cert-manager DNS provider route53, got %s", certMgr.DNSProvider)
	}

	loki := services["loki"].(*LokiConfig)
	if loki.StorageType != "s3" {
		t.Errorf("expected loki storage type s3, got %s", loki.StorageType)
	}

	tempo := services["tempo"].(*TempoConfig)
	if tempo.StorageType != "s3" {
		t.Errorf("expected tempo storage type s3, got %s", tempo.StorageType)
	}

	velero := services["velero"].(*VeleroConfig)
	if velero.StorageType != "s3" {
		t.Errorf("expected velero storage type s3, got %s", velero.StorageType)
	}
}

func TestApplyDefaultProviders_DoesNotOverrideExplicit(t *testing.T) {
	validator := NewServiceProviderValidator()

	services := map[string]interface{}{
		"cert-manager": &CertManagerConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			DNSProvider: "cloudflare", // Explicit value
		},
		"loki": &LokiConfig{
			BaseConfig:  BaseConfig{Enabled: true},
			StorageType: "s3", // Explicit value
		},
	}

	err := validator.ApplyDefaultProviders(services, "openstack", true, "test-cluster")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify explicit values were not overridden
	certMgr := services["cert-manager"].(*CertManagerConfig)
	if certMgr.DNSProvider != "cloudflare" {
		t.Errorf("expected cert-manager DNS provider cloudflare (explicit), got %s", certMgr.DNSProvider)
	}

	loki := services["loki"].(*LokiConfig)
	if loki.StorageType != "s3" {
		t.Errorf("expected loki storage type s3 (explicit), got %s", loki.StorageType)
	}
}
