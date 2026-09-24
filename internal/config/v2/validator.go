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

package v2

import (
	"bytes"
	"fmt"
	"net"
	"net/url"
	"sort"
	"strings"

	semver "github.com/Masterminds/semver/v3"
	"github.com/go-playground/validator/v10"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

const (
	StorageLifecycleProduction    = "production"
	StorageLifecycleNonProduction = "non-production"

	StoragePVCProviderExternal = "external"
	StoragePVCProviderLonghorn = "longhorn"

	StorageObjectProviderExternalS3 = "external-s3"
	StorageObjectProviderRustFS     = "rustfs"
)

type storagePolicyIssue struct {
	path    string
	message string
}

// EffectiveStorageProfile returns normalized profile values. Empty fields retain
// safe compatibility defaults, while new configurations materialize them in NewV2Default.
func EffectiveStorageProfile(cfg *Config) StorageProfileConfig {
	profile := StorageProfileConfig{
		Lifecycle:             StorageLifecycleNonProduction,
		PVCProvider:           StoragePVCProviderExternal,
		ObjectStorageProvider: StorageObjectProviderExternalS3,
	}
	if cfg == nil {
		return profile
	}
	configured := cfg.OpenCenter.Infrastructure.Storage.Profile
	if value := strings.ToLower(strings.TrimSpace(configured.Lifecycle)); value != "" {
		profile.Lifecycle = value
	}
	if value := strings.ToLower(strings.TrimSpace(configured.PVCProvider)); value != "" {
		profile.PVCProvider = value
	}
	if value := strings.ToLower(strings.TrimSpace(configured.ObjectStorageProvider)); value != "" {
		profile.ObjectStorageProvider = value
	}
	return profile
}

// UsesManagedObjectStorage reports whether the non-production RustFS profile is selected.
func UsesManagedObjectStorage(cfg *Config) bool {
	return EffectiveStorageProfile(cfg).ObjectStorageProvider == StorageObjectProviderRustFS
}

// ResolveObjectStorageBackend deliberately ignores infrastructure provider.
// Platform bulk data uses an S3-compatible API, supplied externally or by
// RustFS, unless a service explicitly opts out of object storage.
func ResolveObjectStorageBackend(cfg *Config, serviceName string) string {
	switch service := configuredService(cfg, serviceName).(type) {
	case *services.LokiConfig:
		if strings.EqualFold(strings.TrimSpace(service.StorageType), "none") {
			return "none"
		}
	case *services.HarborConfig:
		if strings.EqualFold(strings.TrimSpace(service.StorageType), "filesystem") {
			return "filesystem"
		}
	case *services.VeleroConfig:
		if strings.EqualFold(strings.TrimSpace(service.StorageType), "none") {
			return "none"
		}
	case *services.EtcdBackupConfig:
		if strings.EqualFold(strings.TrimSpace(service.StorageType), "none") {
			return "none"
		}
	}
	return "s3"
}

func storagePolicyIssues(cfg *Config) []storagePolicyIssue {
	if cfg == nil {
		return nil
	}
	profile := EffectiveStorageProfile(cfg)
	var issues []storagePolicyIssue
	add := func(path, message string) {
		issues = append(issues, storagePolicyIssue{path: path, message: message})
	}

	switch profile.Lifecycle {
	case StorageLifecycleProduction, StorageLifecycleNonProduction:
	default:
		add("opencenter.infrastructure.storage.profile.lifecycle", "storage profile lifecycle must be production or non-production.")
	}
	switch profile.PVCProvider {
	case StoragePVCProviderExternal, StoragePVCProviderLonghorn:
	default:
		add("opencenter.infrastructure.storage.profile.pvc_provider", "storage profile pvc_provider must be external or longhorn.")
	}
	switch profile.ObjectStorageProvider {
	case StorageObjectProviderExternalS3, StorageObjectProviderRustFS:
	default:
		add("opencenter.infrastructure.storage.profile.object_storage_provider", "storage profile object_storage_provider must be external-s3 or rustfs.")
	}

	if profile.ObjectStorageProvider == StorageObjectProviderRustFS {
		if profile.Lifecycle == StorageLifecycleProduction {
			add("opencenter.infrastructure.storage.profile.object_storage_provider", "RustFS is non-production only; production and Edge Production require externally managed S3-compatible storage.")
		}
		if profile.PVCProvider != StoragePVCProviderLonghorn {
			add("opencenter.infrastructure.storage.profile.pvc_provider", "RustFS requires pvc_provider: longhorn.")
		}
		if !isServiceEnabled(cfg, "longhorn") {
			add("opencenter.services.longhorn", "RustFS requires the openCenter-managed Longhorn service to be enabled.")
		}
	} else if profile.Lifecycle == StorageLifecycleProduction {
		for _, serviceName := range []string{"loki", "tempo", "velero", "harbor", "etcd-backup"} {
			if !isServiceEnabled(cfg, serviceName) {
				continue
			}
			if storageBackendDoesNotUseObjectStorage(cfg, serviceName) {
				continue
			}
			if err := ValidateS3Endpoint(externalS3Endpoint(cfg, serviceName)); err != nil {
				add("opencenter.services."+serviceName+".s3_endpoint", "external S3-compatible storage requires a configured absolute HTTP(S) endpoint.")
			}
		}
		if isServiceEnabled(cfg, "mimir") {
			add("opencenter.services.mimir", "Mimir cannot use the external S3 profile until its typed S3 configuration is implemented; keep Mimir disabled or use the managed RustFS profile during the migration.")
		}
	}

	for _, serviceName := range []string{"loki", "tempo", "velero", "etcd-backup"} {
		if !isServiceEnabled(cfg, serviceName) {
			continue
		}
		storageType := configuredBulkStorageType(cfg, serviceName)
		if serviceName == "loki" && storageType == "none" {
			continue
		}
		if storageType == "" || storageType == "s3" {
			continue
		}
		path := "opencenter.services." + serviceName + ".storage_type"
		if storageType == "none" && (serviceName == "loki" || serviceName == "velero" || serviceName == "etcd-backup") {
			continue
		}
		if storageType == "swift" {
			add(path, "Swift is no longer supported for platform bulk data; migrate to the S3-compatible storage profile.")
			continue
		}
		add(path, fmt.Sprintf("storage_type %q is unsupported; platform bulk data must use S3-compatible storage.", storageType))
	}
	return issues
}

func storageBackendDoesNotUseObjectStorage(cfg *Config, serviceName string) bool {
	backend := ResolveObjectStorageBackend(cfg, serviceName)
	return backend == "none" || backend == "filesystem"
}

func externalS3Endpoint(cfg *Config, serviceName string) string {
	switch service := configuredService(cfg, serviceName).(type) {
	case *services.LokiConfig:
		return service.S3Endpoint
	case *services.TempoConfig:
		return service.S3Endpoint
	case *services.VeleroConfig:
		return service.S3Endpoint
	case *services.HarborConfig:
		return service.S3Endpoint
	case *services.EtcdBackupConfig:
		return service.S3Endpoint
	default:
		return ""
	}
}

func configuredBulkStorageType(cfg *Config, serviceName string) string {
	switch service := configuredService(cfg, serviceName).(type) {
	case *services.LokiConfig:
		return strings.ToLower(strings.TrimSpace(service.StorageType))
	case *services.TempoConfig:
		return strings.ToLower(strings.TrimSpace(service.StorageType))
	case *services.VeleroConfig:
		return strings.ToLower(strings.TrimSpace(service.StorageType))
	case *services.HarborConfig:
		return strings.ToLower(strings.TrimSpace(service.StorageType))
	case *services.EtcdBackupConfig:
		return strings.ToLower(strings.TrimSpace(service.StorageType))
	default:
		return ""
	}
}

// ResolveVeleroStorageBackend returns Velero's configured backend, or the
// provider-specific default when it is omitted.
func ResolveVeleroStorageBackend(cfg *Config) string {
	if service := configuredService(cfg, "velero"); service != nil {
		if velero, ok := service.(*services.VeleroConfig); ok {
			if storageType := strings.ToLower(strings.TrimSpace(velero.StorageType)); storageType != "" {
				return storageType
			}
		}
	}
	if cfg != nil {
		switch strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)) {
		case "openstack":
			return "swift"
		case "gcp":
			return "gcs"
		case "azure":
			return "azure"
		}
	}
	return "s3"
}

func configuredService(cfg *Config, serviceName string) any {
	if cfg == nil {
		return nil
	}
	if service, ok := cfg.OpenCenter.Services[serviceName]; ok {
		return service
	}
	if service, ok := cfg.OpenCenter.ManagedServices[serviceName]; ok {
		return service
	}
	return nil
}

// ValidateS3Endpoint validates a concrete S3-compatible endpoint. A blank
// endpoint is never a usable S3 configuration.
func ValidateS3Endpoint(raw string) error {
	endpoint := strings.TrimSpace(raw)
	if endpoint == "" {
		return fmt.Errorf("S3 endpoint must be configured")
	}
	parsed, err := url.Parse(endpoint)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return fmt.Errorf("S3 endpoint must be an absolute HTTP(S) URL")
	}
	if strings.Contains(strings.ToUpper(parsed.Path), "/V1/AUTH_") {
		return fmt.Errorf("S3 endpoint must not be a Swift /v1/AUTH_* endpoint")
	}
	return nil
}

// ValidateHarborConfig validates the public Harbor storage contract. The YAML
// decoder supplies defaults for omitted PVC fields; explicit non-positive values
// are rejected by runtime validation and the generated schema.
func ValidateHarborConfig(config *services.HarborConfig) error {
	return validateHarborConfig(config, true)
}

func validateHarborConfig(config *services.HarborConfig, requireS3Endpoint bool) error {
	if config == nil {
		return fmt.Errorf("Harbor configuration must not be nil")
	}
	storageType := strings.ToLower(strings.TrimSpace(config.StorageType))
	if storageType != "" && storageType != "s3" && storageType != "filesystem" {
		return fmt.Errorf("Harbor storage_type %q is unsupported; only s3 or filesystem is supported", config.StorageType)
	}
	endpoint := strings.TrimSpace(config.S3Endpoint)
	if endpoint == "" {
		if config.Enabled && requireS3Endpoint && storageType != "filesystem" {
			return fmt.Errorf("Harbor s3_endpoint is required when Harbor is enabled")
		}
	} else {
		parsed, err := url.Parse(endpoint)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("Harbor s3_endpoint must be an absolute HTTP(S) URL")
		}
		if strings.Contains(strings.ToUpper(parsed.Path), "/V1/AUTH_") {
			return fmt.Errorf("Harbor s3_endpoint must not be a Swift /v1/AUTH_* endpoint")
		}
	}
	for name, size := range map[string]int{
		"registry":   config.RegistryVolumeSize,
		"jobservice": config.JobserviceVolumeSize,
		"database":   config.DatabaseVolumeSize,
		"redis":      config.RedisVolumeSize,
		"trivy":      config.TrivyVolumeSize,
	} {
		if size <= 0 {
			return fmt.Errorf("Harbor %s PVC size must be greater than zero", name)
		}
	}
	return nil
}

// Validator performs multi-layered validation of v2 configurations.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
type Validator interface {
	Validate(cfg *Config) error
	ValidateSchema(cfg *Config) error
	ValidateBusinessRules(cfg *Config) error
	ValidateProvider(cfg *Config) error
	ValidateDeployment(cfg *Config) error
	ValidateServices(cfg *Config) error
}

// defaultValidator implements the Validator interface.
type defaultValidator struct {
	schemaValidator *validator.Validate
}

// NewValidator creates a new v2 configuration validator.
func NewValidator() Validator {
	v := validator.New()

	_ = registerSchemaValidations(v)

	return &defaultValidator{
		schemaValidator: v,
	}
}

func registerSchemaValidations(v *validator.Validate) error {
	if err := v.RegisterValidation("dns1123", func(fl validator.FieldLevel) bool {
		value := fl.Field().String()
		return value == "" || isRFC1123DNSSubdomain(value)
	}); err != nil {
		return err
	}

	if err := v.RegisterValidation("semver", func(fl validator.FieldLevel) bool {
		value := strings.TrimSpace(fl.Field().String())
		if value == "" {
			return true
		}
		_, err := semver.NewVersion(value)
		return err == nil
	}); err != nil {
		return err
	}

	return nil
}

// Validate performs all validation layers.
// Requirements: 11.1, 11.2, 11.3, 11.4, 11.5, 11.6, 11.7
func (v *defaultValidator) Validate(cfg *Config) error {
	if err := v.ValidateCleanBreakRules(cfg); err != nil {
		return err
	}

	// Schema validation
	if err := v.ValidateSchema(cfg); err != nil {
		return err
	}

	// Business rules validation
	if err := v.ValidateBusinessRules(cfg); err != nil {
		return err
	}

	// Provider-specific validation
	if err := v.ValidateProvider(cfg); err != nil {
		return err
	}

	// Deployment-method validation
	if err := v.ValidateDeployment(cfg); err != nil {
		return err
	}

	// Service validation
	if err := v.ValidateServices(cfg); err != nil {
		return err
	}

	return nil
}

// ValidateCleanBreakRules rejects legacy shapes before generic schema
// validation so users get the explicit migration-free guidance.
func (v *defaultValidator) ValidateCleanBreakRules(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if len(cfg.OpenCenter.LegacyTalos) > 0 {
		return fmt.Errorf("opencenter.talos is not supported in v2; remove the opencenter.talos section")
	}
	return nil
}

// ValidateSchema validates required fields, data types, and enum values.
// Requirements: 11.1
func (v *defaultValidator) ValidateSchema(cfg *Config) error {
	if err := v.schemaValidator.Struct(cfg); err != nil {
		return fmt.Errorf("schema validation failed: %w", err)
	}
	return nil
}

// ValidateBusinessRules validates cross-field dependencies and value ranges.
// Requirements: 11.2
func (v *defaultValidator) ValidateBusinessRules(cfg *Config) error {
	if issues := storagePolicyIssues(cfg); len(issues) > 0 {
		issue := issues[0]
		return fmt.Errorf("%s: %s", issue.path, issue.message)
	}
	if len(cfg.OpenCenter.LegacyTalos) > 0 {
		return fmt.Errorf("opencenter.talos is not supported in v2; remove the opencenter.talos section")
	}

	// Validate OpenTofu backend configuration
	if err := v.validateOpenTofuBackend(&cfg.OpenTofu); err != nil {
		return err
	}

	// Validate worker pool name uniqueness
	if err := v.validatePoolNameUniqueness(cfg); err != nil {
		return err
	}

	// Validate Windows image requirement
	if err := v.validateWindowsPoolImage(cfg); err != nil {
		return err
	}

	return nil
}

// validatePoolNameUniqueness ensures pool names are unique across Linux and Windows pools.
func (v *defaultValidator) validatePoolNameUniqueness(cfg *Config) error {
	seen := make(map[string]string)
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorker {
		if prev, exists := seen[pool.Name]; exists {
			return fmt.Errorf("duplicate pool name %q (appears in both %s and linux pools)", pool.Name, prev)
		}
		seen[pool.Name] = "linux"
	}
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if prev, exists := seen[pool.Name]; exists {
			return fmt.Errorf("duplicate pool name %q (appears in both %s and windows pools)", pool.Name, prev)
		}
		seen[pool.Name] = "windows"
	}
	return nil
}

// validateWindowsPoolImage ensures image_id_windows is set when any Windows pool has count > 0.
func (v *defaultValidator) validateWindowsPoolImage(cfg *Config) error {
	hasActiveWindowsPool := cfg.OpenCenter.Infrastructure.Compute.WorkerCountWindows > 0
	for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
		if pool.Count > 0 && pool.Image == "" {
			hasActiveWindowsPool = true
			break
		}
	}
	if !hasActiveWindowsPool {
		return nil
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageIDWindows == "" {
			// Check if all active Windows pools have per-pool images
			for _, pool := range cfg.OpenCenter.Infrastructure.Compute.AdditionalServerPoolsWorkerWindows {
				if pool.Count > 0 && pool.Image == "" {
					return fmt.Errorf("infrastructure.cloud.openstack.image_id_windows is required when Windows pools with count > 0 do not specify a per-pool image")
				}
			}
		}
	}
	return nil
}

// validateOpenTofuBackend validates that the appropriate backend configuration is present.
func (v *defaultValidator) validateOpenTofuBackend(opentofu *OpenTofuConfig) error {
	backend := &opentofu.Backend

	switch backend.Type {
	case "local":
		if backend.Local == nil {
			return fmt.Errorf("opentofu.backend.local — conditionally required based on related field: local backend requires 'local' section with 'path' field")
		}
		if backend.Local.Path == "" {
			return fmt.Errorf("opentofu.backend.local.path — required, currently empty")
		}
	case "s3":
		if backend.S3 == nil {
			return fmt.Errorf("opentofu.backend.s3 — conditionally required based on related field: S3 backend requires 's3' section with bucket, key, and region")
		}
		// The nested struct validation will handle the required fields
	case "remote":
		// Remote backend uses the Config map
		if len(backend.Config) == 0 {
			return fmt.Errorf("opentofu.backend.config — conditionally required based on related field: remote backend requires 'config' section")
		}
	}

	return nil
}

// ValidateProvider validates provider-specific requirements.
// Requirements: 11.3
func (v *defaultValidator) ValidateProvider(cfg *Config) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	switch provider {
	case "kind":
		if cfg.OpenCenter.Infrastructure.Kind == nil {
			return fmt.Errorf("opencenter.infrastructure.kind must be configured for the kind provider")
		}
	case "openstack":
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.openstack must be configured for the openstack provider")
		}
	case "aws":
		if cfg.OpenCenter.Infrastructure.Cloud.AWS == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.aws must be configured for the aws provider")
		}
	case "vmware", "vsphere":
		if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.vmware must be configured for the %s provider", provider)
		}
	case "gcp":
		if cfg.OpenCenter.Infrastructure.Cloud.GCP == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.gcp must be configured for the gcp provider")
		}
	case "azure":
		if cfg.OpenCenter.Infrastructure.Cloud.Azure == nil {
			return fmt.Errorf("opencenter.infrastructure.cloud.azure must be configured for the azure provider")
		}
	case "magnum":
		if err := validateMagnumCloudConfig(cfg.OpenCenter.Infrastructure.Cloud.Magnum); err != nil {
			return err
		}
	case "baremetal":
		// No provider-specific config block required for baremetal
	case "":
		return fmt.Errorf("opencenter.infrastructure.provider must be set")
	default:
		return fmt.Errorf("unsupported infrastructure provider: %s", provider)
	}

	return nil
}

// validateMagnumCloudConfig validates the credentials and cluster-template
// contract used by the managed Magnum provider. Magnum cluster templates own
// the VM image and network settings, so those OpenStack fields are not part of
// this validation.
func validateMagnumCloudConfig(config *MagnumCloudConfig) error {
	if config == nil {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum must be configured for the magnum provider")
	}
	if strings.TrimSpace(config.AuthURL) == "" {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.auth_url is required for Keystone authentication")
	}
	parsed, err := url.Parse(strings.TrimSpace(config.AuthURL))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.User != nil {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.auth_url must be an absolute HTTP(S) Keystone URL")
	}
	if strings.TrimSpace(config.Region) == "" {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.region is required")
	}
	if strings.TrimSpace(config.ProjectID) == "" {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.project_id is required")
	}
	if isMissingSecret(config.ApplicationCredentialID) || isMissingSecret(config.ApplicationCredentialSecret) {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.application_credential_id and application_credential_secret are required")
	}
	if strings.TrimSpace(config.ClusterTemplate) == "" {
		return fmt.Errorf("opencenter.infrastructure.cloud.magnum.cluster_template is required")
	}
	return nil
}

// ValidateDeployment validates deployment-method requirements.
// Requirements: 11.4
func (v *defaultValidator) ValidateDeployment(cfg *Config) error {
	methodName := strings.ToLower(strings.TrimSpace(cfg.Deployment.Method))
	if methodName == "" {
		return fmt.Errorf("deployment.method must be set")
	}

	deploymentMethod, err := GetDeploymentMethod(methodName)
	if err != nil {
		return err
	}
	if err := deploymentMethod.ValidateCompatibility(cfg.OpenCenter.Infrastructure.Provider); err != nil {
		return err
	}
	return deploymentMethod.ValidateConfig(cfg)
}

// ValidateServices validates service dependencies and required secrets.
// Requirements: 11.5
func (v *defaultValidator) ValidateServices(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	if service := configuredService(cfg, "velero"); service != nil {
		_, ok := service.(*services.VeleroConfig)
		if !ok {
			return fmt.Errorf("velero service has unexpected configuration type %T", service)
		}
	}
	if service, ok := cfg.OpenCenter.Services["harbor"]; ok {
		harbor, ok := service.(*services.HarborConfig)
		if !ok {
			return fmt.Errorf("harbor service has unexpected configuration type %T", service)
		}
		if err := validateHarborConfig(harbor, !UsesManagedObjectStorage(cfg)); err != nil {
			return err
		}
	}
	metallbEnabled := isServiceEnabled(cfg, "metallb")
	var metallbCfg *services.MetalLBConfig
	if metallbEnabled {
		mlb, ok := cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig)
		if !ok {
			return fmt.Errorf("metallb service has unexpected configuration type %T", cfg.OpenCenter.Services["metallb"])
		}
		metallbCfg = mlb
		if err := validateMetalLBConfig(mlb); err != nil {
			return err
		}
	}

	// OCTR-762: validate per-service MetalLB address pool selection. Any service
	// that names an address_pool must reference a pool declared in
	// services.metallb.ip_address_pools, which in turn requires metallb enabled.
	if err := validateServiceAddressPools(cfg, metallbEnabled, metallbCfg); err != nil {
		return err
	}

	return nil
}

// validateServiceAddressPools ensures every service's address_pool (if set)
// refers to a real MetalLB pool. Reuses the pool-membership pattern from
// validateMetalLBConfig's L2Advertisement check.
func validateServiceAddressPools(cfg *Config, metallbEnabled bool, metallbCfg *services.MetalLBConfig) error {
	poolNames := make(map[string]struct{})
	if metallbCfg != nil {
		for _, pool := range metallbCfg.IPAddressPools {
			if pool.Name != "" {
				poolNames[pool.Name] = struct{}{}
			}
		}
	}

	names := make([]string, 0, len(cfg.OpenCenter.Services))
	for name := range cfg.OpenCenter.Services {
		names = append(names, name)
	}
	sort.Strings(names)

	var problems []string
	for _, name := range names {
		svc := cfg.OpenCenter.Services[name]
		pooler, ok := svc.(interface{ GetAddressPool() string })
		if !ok {
			continue
		}
		pool := strings.TrimSpace(pooler.GetAddressPool())
		if pool == "" {
			continue
		}
		if !metallbEnabled {
			problems = append(problems, fmt.Sprintf("opencenter.services.%s.address_pool %q requires the metallb service to be enabled with matching ip_address_pools", name, pool))
			continue
		}
		if _, exists := poolNames[pool]; !exists {
			problems = append(problems, fmt.Sprintf("opencenter.services.%s.address_pool %q is not defined in services.metallb.ip_address_pools", name, pool))
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("service address pool validation failed:\n  - %s", strings.Join(problems, "\n  - "))
}

func validateMetalLBConfig(config *services.MetalLBConfig) error {
	if config == nil {
		return fmt.Errorf("metallb configuration must not be nil")
	}

	var problems []string
	poolNames := make(map[string]struct{}, len(config.IPAddressPools))
	defaultPools := make([]string, 0, 1)
	for i, pool := range config.IPAddressPools {
		path := fmt.Sprintf("services.metallb.ip_address_pools[%d]", i)
		if pool.Name == "" {
			problems = append(problems, path+".name must not be empty")
		} else if !isRFC1123DNSSubdomain(pool.Name) {
			problems = append(problems, fmt.Sprintf("%s.name %q is not a valid RFC 1123 DNS subdomain", path, pool.Name))
		} else if _, exists := poolNames[pool.Name]; exists {
			problems = append(problems, fmt.Sprintf("%s.name %q is duplicated", path, pool.Name))
		} else {
			poolNames[pool.Name] = struct{}{}
		}
		if pool.Default {
			defaultPools = append(defaultPools, pool.Name)
		}
		if len(pool.Addresses) == 0 {
			problems = append(problems, path+".addresses must contain at least one address")
		}
		for j, address := range pool.Addresses {
			if !validMetalLBAddress(address) {
				problems = append(problems, fmt.Sprintf("%s.addresses[%d] %q is not a valid CIDR or IP range", path, j, address))
			}
		}
	}
	if len(defaultPools) > 1 {
		problems = append(problems, fmt.Sprintf("services.metallb.ip_address_pools: at most one pool may set default: true, found %d (%s)", len(defaultPools), strings.Join(defaultPools, ", ")))
	}

	advertisementNames := make(map[string]struct{}, len(config.L2Advertisements))
	for i, advertisement := range config.L2Advertisements {
		path := fmt.Sprintf("services.metallb.l2_advertisements[%d]", i)
		if advertisement.Name == "" {
			problems = append(problems, path+".name must not be empty")
		} else if !isRFC1123DNSSubdomain(advertisement.Name) {
			problems = append(problems, fmt.Sprintf("%s.name %q is not a valid RFC 1123 DNS subdomain", path, advertisement.Name))
		} else if _, exists := advertisementNames[advertisement.Name]; exists {
			problems = append(problems, fmt.Sprintf("%s.name %q is duplicated", path, advertisement.Name))
		} else {
			advertisementNames[advertisement.Name] = struct{}{}
		}
		if advertisement.Type != "" && advertisement.Type != services.L2AdvertisementType {
			problems = append(problems, fmt.Sprintf("%s.type %q is not supported (only %q is supported today)", path, advertisement.Type, services.L2AdvertisementType))
		}
		seenInterfaces := make(map[string]struct{}, len(advertisement.Interfaces))
		for j, iface := range advertisement.Interfaces {
			if iface == "" {
				problems = append(problems, fmt.Sprintf("%s.interfaces[%d] must not be empty", path, j))
			} else if _, exists := seenInterfaces[iface]; exists {
				problems = append(problems, fmt.Sprintf("%s.interfaces[%d] %q is duplicated", path, j, iface))
			} else {
				seenInterfaces[iface] = struct{}{}
			}
		}
		for j, poolName := range advertisement.IPAddressPools {
			if _, exists := poolNames[poolName]; !exists {
				problems = append(problems, fmt.Sprintf("%s.ip_address_pools[%d] references unknown pool %q", path, j, poolName))
			}
		}
	}

	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("metallb validation failed:\n  - %s", strings.Join(problems, "\n  - "))
}

func isRFC1123DNSSubdomain(value string) bool {
	if len(value) == 0 || len(value) > 253 {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, c := range label {
			if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
				return false
			}
		}
	}
	return true
}

func validMetalLBAddress(address string) bool {
	if _, _, err := net.ParseCIDR(address); err == nil {
		return true
	}
	parts := strings.Split(address, "-")
	if len(parts) != 2 {
		return false
	}
	start, end := net.ParseIP(strings.TrimSpace(parts[0])), net.ParseIP(strings.TrimSpace(parts[1]))
	if start == nil || end == nil || (start.To4() == nil) != (end.To4() == nil) {
		return false
	}
	start16, end16 := start.To16(), end.To16()
	return start16 != nil && end16 != nil && bytes.Compare(start16, end16) <= 0
}

// ValidateForDeployment performs all standard validation plus deployment-readiness
// checks such as detecting placeholder secrets that must be replaced.
func ValidateForDeployment(cfg *Config) error {
	v := NewValidator().(*defaultValidator)
	if err := v.Validate(cfg); err != nil {
		return err
	}
	return v.validatePlaceholderSecrets(cfg)
}

// PlaceholderSecret is the sentinel value used in default configurations to indicate
// that a secret must be replaced before deployment.
const PlaceholderSecret = "CHANGEME"

// validatePlaceholderSecrets checks for any secrets still set to the placeholder value.
// Returns an error listing all secrets that need to be updated.
func (v *defaultValidator) validatePlaceholderSecrets(cfg *Config) error {
	var placeholders []string

	// Keycloak secrets (enabled by default)
	if isServiceEnabled(cfg, "keycloak") {
		if !oidcClientSecretsProvidedInternally(cfg) && cfg.Secrets.Keycloak.ClientSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.keycloak.client_secret")
		}
		if cfg.Secrets.Keycloak.AdminPassword == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.keycloak.admin_password")
		}
	}

	// Headlamp secrets (enabled by default)
	if isServiceEnabled(cfg, "headlamp") && !oidcClientSecretsProvidedInternally(cfg) {
		if cfg.Secrets.Headlamp.OIDCClientSecret == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.headlamp.oidc_client_secret")
		}
	}

	// Grafana secrets (kube-prometheus-stack)
	if isServiceEnabled(cfg, "kube-prometheus-stack") {
		if cfg.Secrets.Grafana.AdminPassword == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.grafana.admin_password")
		}
	}

	// Loki secrets
	if !UsesManagedObjectStorage(cfg) && isServiceEnabled(cfg, "loki") {
		switch ResolveObjectStorageBackend(cfg, "loki") {
		case "swift":
			if isMissingSecret(cfg.GetLokiSwiftApplicationCredentialSecret()) {
				placeholders = append(placeholders, "secrets.loki.swift_application_credential_secret")
			}
		case "s3":
			accessKey, secretKey := cfg.GetLokiS3Credentials()
			if isMissingSecret(accessKey) {
				placeholders = append(placeholders, "secrets.loki.s3_access_key_id")
			}
			if isMissingSecret(secretKey) {
				placeholders = append(placeholders, "secrets.loki.s3_secret_access_key")
			}
		}
	}

	// Tempo secrets
	if !UsesManagedObjectStorage(cfg) && isServiceEnabled(cfg, "tempo") {
		switch ResolveObjectStorageBackend(cfg, "tempo") {
		case "swift":
			if isMissingSecret(cfg.GetTempoSwiftApplicationCredentialSecret()) {
				placeholders = append(placeholders, "secrets.tempo.swift_application_credential_secret")
			}
		case "s3":
			accessKey, secretKey := cfg.GetTempoS3Credentials()
			if isMissingSecret(accessKey) {
				placeholders = append(placeholders, "secrets.tempo.access_key")
			}
			if isMissingSecret(secretKey) {
				placeholders = append(placeholders, "secrets.tempo.secret_key")
			}
		}
	}

	// Mimir currently has a legacy Swift secret; managed RustFS defers generated credentials.
	if !UsesManagedObjectStorage(cfg) && isServiceEnabled(cfg, "mimir") && isMissingSecret(cfg.GetMimirSwiftApplicationCredentialSecret()) {
		placeholders = append(placeholders, "secrets.mimir.swift_application_credential_secret")
	}

	// Harbor secrets
	placeholders = append(placeholders, missingHarborDeploymentSecretPaths(cfg)...)

	// Cert-manager secrets (map-based credentials)
	if isServiceEnabled(cfg, "cert-manager") {
		for name, cred := range cfg.Secrets.CertManager.AWS {
			if !cred.Enabled {
				continue
			}
			if cred.AWSAccessKey == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_access_key", name))
			}
			if cred.AWSSecretAccessKey == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.aws.%s.aws_secret_access_key", name))
			}
		}
		for name, cred := range cfg.Secrets.CertManager.Cloudflare {
			if !cred.Enabled {
				continue
			}
			if cred.APIToken == PlaceholderSecret {
				placeholders = append(placeholders, fmt.Sprintf("secrets.cert_manager.cloudflare.%s.api_token", name))
			}
		}
		// Legacy flat fields
		if cfg.Secrets.CertManager.AWSAccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.cert_manager.aws_access_key")
		}
		if cfg.Secrets.CertManager.AWSSecretAccessKey == PlaceholderSecret {
			placeholders = append(placeholders, "secrets.cert_manager.aws_secret_access_key")
		}
	}

	// Global AWS secrets
	if cfg.Secrets.Global.AWS.Infrastructure.AccessKey == PlaceholderSecret {
		placeholders = append(placeholders, "secrets.global.aws.infrastructure.access_key")
	}
	if cfg.Secrets.Global.AWS.Infrastructure.SecretAccessKey == PlaceholderSecret {
		placeholders = append(placeholders, "secrets.global.aws.infrastructure.secret_access_key")
	}

	// OpenStack application credentials
	provider := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider)
	if strings.EqualFold(provider, "openstack") && cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID == PlaceholderSecret {
			placeholders = append(placeholders, "opencenter.infrastructure.cloud.openstack.application_credential_id")
		}
		if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret == PlaceholderSecret {
			placeholders = append(placeholders, "opencenter.infrastructure.cloud.openstack.application_credential_secret")
		}
	}

	if len(placeholders) > 0 {
		return fmt.Errorf("the following secrets still have the placeholder value %q and must be updated before deployment:\n  - %s",
			PlaceholderSecret, strings.Join(placeholders, "\n  - "))
	}

	return nil
}

// ValidateHarborForDeployment performs the narrow Harbor credential gate used
// by bootstrap before any infrastructure mutation. It intentionally does not
// invoke unrelated provider or global placeholder validation.
func ValidateHarborForDeployment(cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if serviceEnabledInMap(cfg.OpenCenter.ManagedServices, "harbor") {
		return fmt.Errorf("managed Harbor is not supported; configure Harbor under opencenter.services.harbor")
	}
	missing := missingHarborDeploymentSecretPaths(cfg)
	if len(missing) == 0 {
		return nil
	}
	return fmt.Errorf("the following Harbor secrets must be set to non-placeholder values before deployment:\n  - %s", strings.Join(missing, "\n  - "))
}

func isServiceEnabled(cfg *Config, serviceName string) bool {
	if cfg == nil {
		return false
	}
	return serviceEnabledInMap(cfg.OpenCenter.Services, serviceName)
}

func missingHarborDeploymentSecretPaths(cfg *Config) []string {
	if !isServiceEnabled(cfg, "harbor") {
		return nil
	}
	var missing []string
	if isMissingSecret(cfg.Secrets.Harbor.AdminPassword) {
		missing = append(missing, "secrets.harbor.admin_password")
	}
	if isMissingSecret(cfg.Secrets.Harbor.RegistryPassword) {
		missing = append(missing, "secrets.harbor.registry_password")
	}
	if isMissingSecret(cfg.Secrets.Harbor.DatabasePassword) {
		missing = append(missing, "secrets.harbor.database_password")
	}
	if !storageBackendDoesNotUseObjectStorage(cfg, "harbor") && !UsesManagedObjectStorage(cfg) {
		if isMissingSecret(cfg.Secrets.Harbor.S3AccessKeyID) {
			missing = append(missing, "secrets.harbor.s3_access_key_id")
		}
		if isMissingSecret(cfg.Secrets.Harbor.S3SecretAccessKey) {
			missing = append(missing, "secrets.harbor.s3_secret_access_key")
		}
	}
	return missing
}
