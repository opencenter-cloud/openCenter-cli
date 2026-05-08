/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package secrets

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

// DefaultSecretsManager implements the SecretsManager interface.
// It provides secrets synchronization, drift detection, and validation
// by coordinating between config files, SOPS encryption, and manifest generation.
type DefaultSecretsManager struct {
	configLoader *config.ConfigIOHandler
	sopsManager  *sops.DefaultSOPSManager
	auditLogger  AuditLogger
	logger       *slog.Logger
}

// AuditLogger defines the interface for audit logging operations.
// This interface is satisfied by security.AuditLogger.
type AuditLogger interface {
	LogSecretsSync(ctx context.Context, actor, cluster string, filesCreated, filesUpdated, filesUnchanged int) error
	LogSecretsSyncFailed(ctx context.Context, actor, cluster, reason string) error
	LogDriftDetected(ctx context.Context, actor, cluster string, driftCount, missingCount, orphanedCount int) error
	LogSecretsValidated(ctx context.Context, actor, cluster string) error
	LogKeyRotated(ctx context.Context, actor, keyType, resource string) error
	LogKeyRevoked(ctx context.Context, actor, cluster, keyFingerprint, revokedUser string, filesReencrypted int) error
	LogKeyRevocationFailed(ctx context.Context, actor, cluster, keyFingerprint, reason string) error
}

// NewDefaultSecretsManager creates a new DefaultSecretsManager with the given dependencies.
//
// Parameters:
//   - configLoader: Handler for loading and saving config files
//   - sopsManager: Manager for SOPS encryption operations
//   - auditLogger: Logger for audit events (can be nil to disable audit logging)
//   - logger: Logger for operation tracking
//
// Returns:
//   - *DefaultSecretsManager: A new secrets manager instance
func NewDefaultSecretsManager(
	configLoader *config.ConfigIOHandler,
	sopsManager *sops.DefaultSOPSManager,
	auditLogger AuditLogger,
	logger *slog.Logger,
) *DefaultSecretsManager {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultSecretsManager{
		configLoader: configLoader,
		sopsManager:  sopsManager,
		auditLogger:  auditLogger,
		logger:       logger,
	}
}

// SyncSecrets regenerates encrypted manifests from the config file.
// It reads secrets from the cluster's config file and generates
// corresponding SOPS-encrypted manifests for each service.
//
// Returns ErrConfigNotFound if the config file does not exist.
// Returns ErrKeyNotFound if the cluster's Age key is not available.
func (m *DefaultSecretsManager) SyncSecrets(ctx context.Context, opts SyncOptions) (*SyncResult, error) {
	m.logger.Info("Starting secrets sync", "cluster", opts.Cluster, "dry_run", opts.DryRun)

	// Load config file
	cfg, configPath, err := m.loadClusterConfig(ctx, opts.Cluster)
	if err != nil {
		return nil, err
	}

	// Extract secrets from config
	secretsMap, err := m.extractSecretsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract secrets from config: %w", err)
	}

	// Map secrets to service manifest paths
	manifestPaths, err := m.mapSecretsToManifests(cfg, secretsMap, opts.Services)
	if err != nil {
		return nil, fmt.Errorf("failed to map secrets to manifests: %w", err)
	}

	// Determine overlay directory path
	overlayPath, err := m.getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to determine overlay path: %w", err)
	}

	m.logger.Debug("Sync configuration",
		"config_path", configPath,
		"overlay_path", overlayPath,
		"services_count", len(manifestPaths))

	// Initialize result
	result := &SyncResult{
		Created:   []string{},
		Updated:   []string{},
		Unchanged: []string{},
		Errors:    []SyncError{},
	}

	// Get Age key for encryption
	ageKey, err := m.getAgeKey(cfg)
	if err != nil {
		return nil, err
	}

	// Process each service
	for service, relativePath := range manifestPaths {
		serviceSecrets := secretsMap[service]
		fullPath := filepath.Join(overlayPath, relativePath)

		m.logger.Debug("Processing service", "service", service, "path", fullPath)

		// Generate or update manifest
		changed, err := m.syncServiceManifest(ctx, service, serviceSecrets, fullPath, ageKey, opts.DryRun, opts.Force)
		if err != nil {
			result.Errors = append(result.Errors, SyncError{
				FilePath: fullPath,
				Service:  service,
				Error:    err,
			})
			m.logger.Error("Failed to sync service manifest", "service", service, "error", err)
			continue
		}

		// Categorize result
		if changed {
			if _, err := os.Stat(fullPath); os.IsNotExist(err) {
				result.Created = append(result.Created, fullPath)
			} else {
				result.Updated = append(result.Updated, fullPath)
			}
		} else {
			result.Unchanged = append(result.Unchanged, fullPath)
		}
	}

	m.logger.Info("Secrets sync completed",
		"cluster", opts.Cluster,
		"created", len(result.Created),
		"updated", len(result.Updated),
		"unchanged", len(result.Unchanged),
		"errors", len(result.Errors))

	// Log audit event
	if m.auditLogger != nil {
		actor := m.getActor(ctx)
		if len(result.Errors) > 0 {
			// Log failure if there were errors
			reason := fmt.Sprintf("%d files failed to sync", len(result.Errors))
			if err := m.auditLogger.LogSecretsSyncFailed(ctx, actor, opts.Cluster, reason); err != nil {
				m.logger.Warn("Failed to log audit event", "error", err)
			}
		} else {
			// Log success
			if err := m.auditLogger.LogSecretsSync(ctx, actor, opts.Cluster, len(result.Created), len(result.Updated), len(result.Unchanged)); err != nil {
				m.logger.Warn("Failed to log audit event", "error", err)
			}
		}
	}

	return result, nil
}

// ValidateSecrets compares config secrets against encrypted manifests.
// It decrypts each manifest and compares the values against the config,
// reporting any drift, missing manifests, orphaned secrets, or security issues.
//
// Returns ErrConfigNotFound if the config file does not exist.
// Returns ErrKeyNotFound if the cluster's Age key is not available.
// Returns ErrDecryptionFailed if a manifest cannot be decrypted.
func (m *DefaultSecretsManager) ValidateSecrets(ctx context.Context, opts ValidateOptions) (*ValidationResult, error) {
	m.logger.Info("Starting secrets validation", "cluster", opts.Cluster)

	// Load config file
	cfg, configPath, err := m.loadClusterConfig(ctx, opts.Cluster)
	if err != nil {
		return nil, err
	}

	// Extract secrets from config
	configSecrets, err := m.extractSecretsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract secrets from config: %w", err)
	}

	// Get overlay path
	overlayPath, err := m.getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to determine overlay path: %w", err)
	}

	// Get Age key for decryption
	ageKeyPath, err := m.getAgeKeyPath(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize result
	result := &ValidationResult{
		Valid:            true,
		DriftItems:       []DriftItem{},
		MissingManifests: []string{},
		OrphanedSecrets:  []string{},
		SecurityIssues:   []SecurityIssue{},
		ExitCode:         0,
	}

	// Track which services we've found manifests for
	foundServices := make(map[string]bool)

	// Scan overlay directory for manifest files
	manifestFiles, err := m.findManifestFiles(overlayPath)
	if err != nil {
		m.logger.Warn("Failed to scan overlay directory", "error", err)
		// Continue with validation even if scan fails
	}

	// Validate each manifest
	for _, manifestPath := range manifestFiles {
		service := m.extractServiceFromPath(manifestPath)
		if service == "" {
			continue
		}

		foundServices[service] = true

		// Check for unencrypted secrets
		isEncrypted, err := m.isManifestEncrypted(manifestPath)
		if err != nil {
			m.logger.Warn("Failed to check encryption status", "path", manifestPath, "error", err)
			continue
		}

		if !isEncrypted {
			result.SecurityIssues = append(result.SecurityIssues, SecurityIssue{
				FilePath:  manifestPath,
				FieldPath: "data",
				Severity:  "critical",
			})
			result.Valid = false
			continue
		}

		// Decrypt manifest
		manifestSecrets, err := m.decryptManifest(ctx, manifestPath, ageKeyPath)
		if err != nil {
			m.logger.Error("Failed to decrypt manifest", "path", manifestPath, "error", err)
			return nil, &ErrDecryptionFailed{
				FilePath: manifestPath,
				Cause:    err,
			}
		}

		// Compare with config secrets
		if configServiceSecrets, exists := configSecrets[service]; exists {
			// Check for drift
			driftItems := m.compareSecrets(service, configServiceSecrets, manifestSecrets)
			if len(driftItems) > 0 {
				result.DriftItems = append(result.DriftItems, driftItems...)
				result.Valid = false
			}

			// Check for orphaned secrets in manifest
			for key := range manifestSecrets {
				// Convert manifest key format (hyphens) to config format (underscores)
				configKey := strings.ReplaceAll(key, "-", "_")
				if _, exists := configServiceSecrets[configKey]; !exists {
					orphanedPath := fmt.Sprintf("%s:data.%s", manifestPath, key)
					result.OrphanedSecrets = append(result.OrphanedSecrets, orphanedPath)
					result.Valid = false
				}
			}
		} else {
			// Manifest exists but no config secrets for this service
			result.OrphanedSecrets = append(result.OrphanedSecrets, manifestPath)
			result.Valid = false
		}
	}

	// Check for missing manifests (config secrets without manifests)
	for service := range configSecrets {
		if !foundServices[service] {
			expectedPath := filepath.Join(overlayPath, m.getManifestPath(service, cfg))
			result.MissingManifests = append(result.MissingManifests, expectedPath)
			result.Valid = false
		}
	}

	// Set exit code
	if !result.Valid {
		result.ExitCode = 1
	}

	m.logger.Info("Secrets validation completed",
		"cluster", opts.Cluster,
		"valid", result.Valid,
		"drift_items", len(result.DriftItems),
		"missing_manifests", len(result.MissingManifests),
		"orphaned_secrets", len(result.OrphanedSecrets),
		"security_issues", len(result.SecurityIssues))

	// Log audit event
	if m.auditLogger != nil {
		actor := m.getActor(ctx)
		if result.Valid {
			// No drift detected
			if err := m.auditLogger.LogSecretsValidated(ctx, actor, opts.Cluster); err != nil {
				m.logger.Warn("Failed to log audit event", "error", err)
			}
		} else {
			// Drift detected
			if err := m.auditLogger.LogDriftDetected(ctx, actor, opts.Cluster, len(result.DriftItems), len(result.MissingManifests), len(result.OrphanedSecrets)); err != nil {
				m.logger.Warn("Failed to log audit event", "error", err)
			}
		}
	}

	// Auto-fix if requested
	if opts.Fix && !result.Valid {
		m.logger.Info("Auto-fixing drift by running opencenter secrets sync")
		syncOpts := SyncOptions{
			Cluster: opts.Cluster,
			DryRun:  false,
			Force:   true,
		}
		_, err := m.SyncSecrets(ctx, syncOpts)
		if err != nil {
			return result, fmt.Errorf("failed to auto-fix drift: %w", err)
		}
		m.logger.Info("Auto-fix completed successfully")
	}

	return result, nil
}

// DetectDrift identifies differences between config and manifests.
// This is a lower-level method that returns detailed drift information
// without the validation context.
func (m *DefaultSecretsManager) DetectDrift(ctx context.Context, cluster string) (*DriftReport, error) {
	m.logger.Info("Starting drift detection", "cluster", cluster)

	// Load config file
	cfg, configPath, err := m.loadClusterConfig(ctx, cluster)
	if err != nil {
		return nil, err
	}

	// Extract secrets from config
	configSecrets, err := m.extractSecretsFromConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to extract secrets from config: %w", err)
	}

	// Get overlay path
	overlayPath, err := m.getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to determine overlay path: %w", err)
	}

	// Get Age key for decryption
	ageKeyPath, err := m.getAgeKeyPath(cfg)
	if err != nil {
		return nil, err
	}

	// Initialize report
	report := &DriftReport{
		Cluster:            cluster,
		Timestamp:          time.Now(),
		ConfigPath:         configPath,
		OverlayPath:        overlayPath,
		Services:           []ServiceDrift{},
		TotalDriftCount:    0,
		SecurityViolations: 0,
	}

	// Track which services we've found manifests for
	foundServices := make(map[string]bool)

	// Scan overlay directory for manifest files
	manifestFiles, err := m.findManifestFiles(overlayPath)
	if err != nil {
		m.logger.Warn("Failed to scan overlay directory", "error", err)
		// Continue with drift detection even if scan fails
	}

	// Analyze each manifest
	for _, manifestPath := range manifestFiles {
		service := m.extractServiceFromPath(manifestPath)
		if service == "" {
			continue
		}

		foundServices[service] = true

		serviceDrift := ServiceDrift{
			ServiceName:  service,
			ManifestPath: manifestPath,
			DriftFields:  []DriftField{},
			Status:       "synced",
		}

		// Check for unencrypted secrets (security violation)
		isEncrypted, err := m.isManifestEncrypted(manifestPath)
		if err != nil {
			m.logger.Warn("Failed to check encryption status", "path", manifestPath, "error", err)
			serviceDrift.Status = "error"
			report.Services = append(report.Services, serviceDrift)
			continue
		}

		if !isEncrypted {
			report.SecurityViolations++
			serviceDrift.Status = "unencrypted"
			report.Services = append(report.Services, serviceDrift)
			continue
		}

		// Decrypt manifest
		manifestSecrets, err := m.decryptManifest(ctx, manifestPath, ageKeyPath)
		if err != nil {
			m.logger.Error("Failed to decrypt manifest", "path", manifestPath, "error", err)
			serviceDrift.Status = "error"
			report.Services = append(report.Services, serviceDrift)
			continue
		}

		// Compare with config secrets
		if configServiceSecrets, exists := configSecrets[service]; exists {
			// Detect drift in existing secrets
			driftFields := m.detectDriftFields(configServiceSecrets, manifestSecrets)
			if len(driftFields) > 0 {
				serviceDrift.DriftFields = driftFields
				serviceDrift.Status = "drifted"
				report.TotalDriftCount += len(driftFields)
			}

			// Check for orphaned secrets in manifest (secrets not in config)
			for key := range manifestSecrets {
				// Convert manifest key format (hyphens) to config format (underscores)
				configKey := strings.ReplaceAll(key, "-", "_")
				if _, exists := configServiceSecrets[configKey]; !exists {
					// This is an orphaned secret
					serviceDrift.DriftFields = append(serviceDrift.DriftFields, DriftField{
						Path:         fmt.Sprintf("data.%s", key),
						ConfigHash:   "", // Empty indicates not in config
						ManifestHash: m.hashValue(manifestSecrets[key]),
					})
					serviceDrift.Status = "drifted"
					report.TotalDriftCount++
				}
			}
		} else {
			// Manifest exists but no config secrets for this service (orphaned manifest)
			serviceDrift.Status = "orphaned"
			// Count all secrets in the orphaned manifest as drift
			for key, value := range manifestSecrets {
				serviceDrift.DriftFields = append(serviceDrift.DriftFields, DriftField{
					Path:         fmt.Sprintf("data.%s", key),
					ConfigHash:   "",
					ManifestHash: m.hashValue(value),
				})
			}
			report.TotalDriftCount += len(serviceDrift.DriftFields)
		}

		report.Services = append(report.Services, serviceDrift)
	}

	// Check for missing manifests (config secrets without manifests)
	for service := range configSecrets {
		if !foundServices[service] {
			expectedPath := filepath.Join(overlayPath, m.getManifestPath(service, cfg))
			serviceDrift := ServiceDrift{
				ServiceName:  service,
				ManifestPath: expectedPath,
				DriftFields:  []DriftField{},
				Status:       "missing",
			}

			// Count all config secrets as drift since manifest is missing
			for key, value := range configSecrets[service] {
				manifestKey := strings.ReplaceAll(key, "_", "-")
				serviceDrift.DriftFields = append(serviceDrift.DriftFields, DriftField{
					Path:         fmt.Sprintf("data.%s", manifestKey),
					ConfigHash:   m.hashValue(value),
					ManifestHash: "", // Empty indicates missing from manifest
				})
			}
			report.TotalDriftCount += len(serviceDrift.DriftFields)
			report.Services = append(report.Services, serviceDrift)
		}
	}

	m.logger.Info("Drift detection completed",
		"cluster", cluster,
		"total_drift_count", report.TotalDriftCount,
		"security_violations", report.SecurityViolations,
		"services_analyzed", len(report.Services))

	return report, nil
}

// GetSecretSources returns all secret sources for a cluster.
// This includes the config file path and all manifest paths that
// contain secrets for the specified cluster.
func (m *DefaultSecretsManager) GetSecretSources(ctx context.Context, cluster string) ([]SecretSource, error) {
	m.logger.Info("Getting secret sources", "cluster", cluster)

	// Load config to get paths
	cfg, configPath, err := m.loadClusterConfig(ctx, cluster)
	if err != nil {
		return nil, err
	}

	sources := []SecretSource{
		{
			Type:    "config",
			Path:    configPath,
			Service: "",
		},
	}

	// Get overlay path
	overlayPath, err := m.getOverlayPath(configPath, cfg)
	if err != nil {
		return sources, nil // Return config source even if overlay path fails
	}

	manifestFiles, err := m.findManifestFiles(overlayPath)
	if err != nil {
		m.logger.Warn("Failed to scan overlay directory for manifest sources", "cluster", cluster, "error", err)
		return sources, nil
	}

	for _, manifestPath := range manifestFiles {
		sources = append(sources, SecretSource{
			Type:    "manifest",
			Path:    manifestPath,
			Service: m.extractServiceFromPath(manifestPath),
		})
	}

	m.logger.Info("Found secret sources", "cluster", cluster, "count", len(sources))
	return sources, nil
}

// Helper methods

// loadClusterConfig loads the cluster configuration file.
// It searches for the config file in the standard location and returns
// both the parsed config and the file path.
//
// Returns ErrConfigNotFound if the config file does not exist.
func (m *DefaultSecretsManager) loadClusterConfig(ctx context.Context, cluster string) (*v2.Config, string, error) {
	// Determine config file path
	configPath, err := m.getConfigPath(ctx, cluster)
	if err != nil {
		return nil, "", err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return nil, "", &ErrConfigNotFound{
			Cluster:      cluster,
			ExpectedPath: configPath,
		}
	}

	// Load config file
	cfg, err := m.configLoader.LoadFromFile(ctx, configPath)
	if err != nil {
		return nil, "", fmt.Errorf("failed to load config file: %w", err)
	}

	return cfg, configPath, nil
}

// getConfigPath returns the expected path to the cluster config file.
// The config file is located at ~/.config/opencenter/clusters/blueprints/<org>/<cluster>/<cluster>-config.yaml
func (m *DefaultSecretsManager) getConfigPath(ctx context.Context, cluster string) (string, error) {
	pathResolver := config.NewPathResolverFromConfig()
	clusterPaths, err := pathResolver.ResolveWithFallback(ctx, cluster)
	if err == nil {
		return clusterPaths.ConfigPath, nil
	}

	// Fallback: search blueprints directory for the cluster config
	blueprintsDir := config.GetBlueprintsDir()
	entries, readErr := os.ReadDir(blueprintsDir)
	if readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(blueprintsDir, entry.Name(), cluster, fmt.Sprintf("%s-config.yaml", cluster))
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate, nil
			}
		}
	}

	// Final fallback: assume default "opencenter" organization
	return filepath.Join(blueprintsDir, "opencenter", cluster, fmt.Sprintf("%s-config.yaml", cluster)), nil
}

// extractSecretsFromConfig extracts all secrets from the config file.
// It returns a map of service names to their secret values.
func (m *DefaultSecretsManager) extractSecretsFromConfig(cfg *v2.Config) (map[string]map[string]interface{}, error) {
	secretsMap := make(map[string]map[string]interface{})

	serviceBlocks := []struct {
		name    string
		secrets any
	}{
		{name: "cert-manager", secrets: cfg.Secrets.CertManager},
		{name: "loki", secrets: cfg.Secrets.Loki},
		{name: "keycloak", secrets: cfg.Secrets.Keycloak},
		{name: "headlamp", secrets: cfg.Secrets.Headlamp},
		{name: "weave-gitops", secrets: cfg.Secrets.WeaveGitOps},
		{name: "grafana", secrets: cfg.Secrets.Grafana},
		{name: "tempo", secrets: cfg.Secrets.Tempo},
		{name: "alert-proxy", secrets: cfg.Secrets.AlertProxy},
		{name: "vsphere-csi", secrets: cfg.Secrets.VSphereCsi},
	}

	for _, block := range serviceBlocks {
		filtered, err := normalizeServiceSecrets(block.secrets)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize %s secrets: %w", block.name, err)
		}
		if len(filtered) == 0 {
			continue
		}

		secretsMap[block.name] = filtered
	}

	for rawService, rawSecrets := range cfg.Secrets.ServiceSecrets {
		filtered, err := normalizeServiceSecrets(rawSecrets)
		if err != nil {
			return nil, fmt.Errorf("failed to normalize %s service_secrets: %w", rawService, err)
		}

		if len(filtered) == 0 {
			continue
		}

		serviceName := strings.ReplaceAll(rawService, "_", "-")
		if existing, ok := secretsMap[serviceName]; ok {
			for key, value := range filtered {
				existing[key] = value
			}
			continue
		}
		secretsMap[serviceName] = filtered
	}

	return secretsMap, nil
}

func normalizeServiceSecrets(rawSecrets any) (map[string]interface{}, error) {
	if rawSecrets == nil {
		return nil, nil
	}

	if serviceSecrets, ok := rawSecrets.(map[string]any); ok {
		return filterNonEmptySecrets(serviceSecrets), nil
	}

	data, err := yaml.Marshal(rawSecrets)
	if err != nil {
		return nil, err
	}

	serviceSecrets := make(map[string]any)
	if err := yaml.Unmarshal(data, &serviceSecrets); err != nil {
		return nil, err
	}

	return filterNonEmptySecrets(serviceSecrets), nil
}

func filterNonEmptySecrets(serviceSecrets map[string]any) map[string]interface{} {
	filtered := make(map[string]interface{})
	for key, value := range serviceSecrets {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				filtered[key] = typed
			}
		case nil:
			continue
		default:
			filtered[key] = value
		}
	}

	return filtered
}

// mapSecretsToManifests maps config secrets to their corresponding manifest file paths.
// It returns a map of service names to manifest paths, optionally filtered by the services list.
func (m *DefaultSecretsManager) mapSecretsToManifests(
	cfg *v2.Config,
	secretsMap map[string]map[string]interface{},
	serviceFilter []string,
) (map[string]string, error) {
	manifestPaths := make(map[string]string)

	// Create a filter set for quick lookup
	filterSet := make(map[string]bool)
	for _, service := range serviceFilter {
		filterSet[service] = true
	}

	// Map each service to its manifest path
	for service := range secretsMap {
		// Skip if service filter is provided and service is not in filter
		if len(serviceFilter) > 0 && !filterSet[service] {
			continue
		}

		// Determine manifest path based on service
		manifestPath := m.getManifestPath(service, cfg)
		manifestPaths[service] = manifestPath
	}

	return manifestPaths, nil
}

// getManifestPath returns the expected manifest path for a service.
// The path is relative to the overlay directory.
func (m *DefaultSecretsManager) getManifestPath(service string, cfg *v2.Config) string {
	// Standard path pattern: services/<service>/secret.yaml
	return filepath.Join("services", service, "secret.yaml")
}

// getOverlayPath determines the overlay directory path for the cluster.
// The overlay directory contains the FluxCD manifests and service configurations.
func (m *DefaultSecretsManager) getOverlayPath(configPath string, cfg *v2.Config) (string, error) {
	// The overlay path is typically in the GitOps repository
	// Pattern: <repo>/applications/overlays/<cluster>/

	// For now, construct the expected path based on GitOps config
	if cfg.GitDir() == "" {
		return "", fmt.Errorf("gitops.git_dir not configured")
	}

	overlayPath := filepath.Join(
		cfg.GitDir(),
		"applications",
		"overlays",
		cfg.ClusterName(),
	)

	return overlayPath, nil
}

// getAgeKey retrieves the Age key for the cluster from the config.
// Returns ErrKeyNotFound if the Age key is not configured or not found.
func (m *DefaultSecretsManager) getAgeKey(cfg *v2.Config) (string, error) {
	if cfg.Secrets.SopsAgeKeyFile == "" {
		return "", &ErrKeyNotFound{
			Cluster: cfg.ClusterName(),
			KeyType: KeyTypeAge,
		}
	}

	// Expand home directory if needed
	keyPath := cfg.Secrets.SopsAgeKeyFile
	if strings.HasPrefix(keyPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}

	// Read the Age key file to get the public key
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return "", NewKeyNotFoundError(
			cfg.ClusterName(),
			KeyTypeAge,
			fmt.Errorf("failed to read Age key file at %s: %w", keyPath, err),
		)
	}

	// Extract the public key from the Age key file
	// Age key files contain lines like: # public key: age1...
	lines := strings.Split(string(keyData), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "# public key:") {
			publicKey := strings.TrimSpace(strings.TrimPrefix(line, "# public key:"))
			if publicKey != "" {
				return publicKey, nil
			}
		}
	}

	return "", NewKeyNotFoundError(
		cfg.ClusterName(),
		KeyTypeAge,
		fmt.Errorf("public key not found in Age key file at %s", keyPath),
	)
}

// getAgeKeyPath retrieves the Age key file path for the cluster from the config.
// Returns ErrKeyNotFound if the Age key is not configured or not found.
func (m *DefaultSecretsManager) getAgeKeyPath(cfg *v2.Config) (string, error) {
	if cfg.Secrets.SopsAgeKeyFile == "" {
		return "", &ErrKeyNotFound{
			Cluster: cfg.ClusterName(),
			KeyType: KeyTypeAge,
		}
	}

	// Expand home directory if needed
	keyPath := cfg.Secrets.SopsAgeKeyFile
	if strings.HasPrefix(keyPath, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}
		keyPath = filepath.Join(homeDir, keyPath[2:])
	}

	// Check if key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		return "", NewKeyNotFoundError(
			cfg.ClusterName(),
			KeyTypeAge,
			fmt.Errorf("Age key file not found at %s", keyPath),
		)
	}

	return keyPath, nil
}

// findManifestFiles scans the overlay directory for secret manifest files.
// Returns a list of absolute paths to manifest files.
func (m *DefaultSecretsManager) findManifestFiles(overlayPath string) ([]string, error) {
	var manifestFiles []string

	// Check if overlay directory exists
	if _, err := os.Stat(overlayPath); os.IsNotExist(err) {
		return manifestFiles, nil // Return empty list if directory doesn't exist
	}

	// Walk the services directory looking for secret.yaml files
	servicesPath := filepath.Join(overlayPath, "services")
	if _, err := os.Stat(servicesPath); os.IsNotExist(err) {
		return manifestFiles, nil // Return empty list if services directory doesn't exist
	}

	err := filepath.Walk(servicesPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip directories with errors
		}

		if !info.IsDir() && info.Name() == "secret.yaml" {
			manifestFiles = append(manifestFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk services directory: %w", err)
	}

	return manifestFiles, nil
}

// extractServiceFromPath extracts the service name from a manifest file path.
// For example: /path/to/services/cert-manager/secret.yaml -> cert-manager
func (m *DefaultSecretsManager) extractServiceFromPath(manifestPath string) string {
	// Get the directory containing the secret.yaml file
	dir := filepath.Dir(manifestPath)
	// The service name is the last directory component
	return filepath.Base(dir)
}

// isManifestEncrypted checks if a manifest file is SOPS-encrypted.
// Returns true if the file contains SOPS metadata, false otherwise.
func (m *DefaultSecretsManager) isManifestEncrypted(manifestPath string) (bool, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return false, fmt.Errorf("failed to read manifest: %w", err)
	}

	// Check for SOPS metadata in the file
	// SOPS-encrypted files contain a "sops:" section with metadata
	content := string(data)
	return strings.Contains(content, "sops:") && strings.Contains(content, "mac:"), nil
}

// decryptManifest decrypts a SOPS-encrypted manifest and extracts the secret data.
// Returns a map of secret keys to their values.
func (m *DefaultSecretsManager) decryptManifest(ctx context.Context, manifestPath string, ageKeyPath string) (map[string]interface{}, error) {
	// Create a temporary file for decrypted output
	tmpFile, err := os.CreateTemp("", "decrypted-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath) // Clean up temp file

	// Set the Age key file environment variable for SOPS
	oldEnv := os.Getenv("SOPS_AGE_KEY_FILE")
	os.Setenv("SOPS_AGE_KEY_FILE", ageKeyPath)
	defer func() {
		if oldEnv != "" {
			os.Setenv("SOPS_AGE_KEY_FILE", oldEnv)
		} else {
			os.Unsetenv("SOPS_AGE_KEY_FILE")
		}
	}()

	// Decrypt the manifest using the encryptor
	encryptor := m.sopsManager.GetEncryptor()
	if err := encryptor.DecryptFile(ctx, manifestPath, tmpPath); err != nil {
		return nil, fmt.Errorf("failed to decrypt manifest: %w", err)
	}

	// Read the decrypted content
	decryptedData, err := os.ReadFile(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read decrypted file: %w", err)
	}

	// Parse the decrypted YAML
	var manifest map[string]interface{}
	if err := yaml.Unmarshal(decryptedData, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse decrypted manifest: %w", err)
	}

	// Extract the data section
	data, ok := manifest["data"].(map[string]interface{})
	if !ok {
		return make(map[string]interface{}), nil // Return empty map if no data section
	}

	return data, nil
}

// compareSecrets compares config secrets against manifest secrets and returns drift items.
// Returns a list of DriftItem for any differences found.
func (m *DefaultSecretsManager) compareSecrets(service string, configSecrets map[string]interface{}, manifestSecrets map[string]interface{}) []DriftItem {
	var driftItems []DriftItem

	// Compare each config secret against manifest
	for configKey, configValue := range configSecrets {
		// Convert config key format (underscores) to manifest format (hyphens)
		manifestKey := strings.ReplaceAll(configKey, "_", "-")

		manifestValue, exists := manifestSecrets[manifestKey]
		if !exists {
			// Secret in config but not in manifest
			driftItems = append(driftItems, DriftItem{
				Service:      service,
				FieldPath:    fmt.Sprintf("data.%s", manifestKey),
				ConfigHash:   m.hashValue(configValue),
				ManifestHash: "", // Empty hash indicates missing
			})
			continue
		}

		// Compare values using hashes (to avoid exposing secrets in logs)
		configHash := m.hashValue(configValue)
		manifestHash := m.hashValue(manifestValue)

		if configHash != manifestHash {
			driftItems = append(driftItems, DriftItem{
				Service:      service,
				FieldPath:    fmt.Sprintf("data.%s", manifestKey),
				ConfigHash:   configHash,
				ManifestHash: manifestHash,
			})
		}
	}

	return driftItems
}

// detectDriftFields compares config secrets against manifest secrets and returns drift fields.
// This is similar to compareSecrets but returns DriftField instead of DriftItem.
// Returns a list of DriftField for any differences found.
func (m *DefaultSecretsManager) detectDriftFields(configSecrets map[string]interface{}, manifestSecrets map[string]interface{}) []DriftField {
	var driftFields []DriftField

	// Compare each config secret against manifest
	for configKey, configValue := range configSecrets {
		// Convert config key format (underscores) to manifest format (hyphens)
		manifestKey := strings.ReplaceAll(configKey, "_", "-")

		manifestValue, exists := manifestSecrets[manifestKey]
		if !exists {
			// Secret in config but not in manifest
			driftFields = append(driftFields, DriftField{
				Path:         fmt.Sprintf("data.%s", manifestKey),
				ConfigHash:   m.hashValue(configValue),
				ManifestHash: "", // Empty hash indicates missing
			})
			continue
		}

		// Compare values using hashes (to avoid exposing secrets in logs)
		configHash := m.hashValue(configValue)
		manifestHash := m.hashValue(manifestValue)

		if configHash != manifestHash {
			driftFields = append(driftFields, DriftField{
				Path:         fmt.Sprintf("data.%s", manifestKey),
				ConfigHash:   configHash,
				ManifestHash: manifestHash,
			})
		}
	}

	return driftFields
}

// hashValue creates a hash of a value for comparison without exposing the actual value.
// Uses SHA-256 to create a consistent hash.
func (m *DefaultSecretsManager) hashValue(value interface{}) string {
	// Convert value to string
	var strValue string
	switch v := value.(type) {
	case string:
		strValue = v
	case []byte:
		strValue = string(v)
	default:
		strValue = fmt.Sprintf("%v", v)
	}

	// Create SHA-256 hash
	hash := fmt.Sprintf("%x", []byte(strValue))
	// Return first 16 characters for brevity
	if len(hash) > 16 {
		return hash[:16]
	}
	return hash
}

// syncServiceManifest generates or updates a service's secret manifest.
// Returns true if the manifest was changed, false if unchanged.
func (m *DefaultSecretsManager) syncServiceManifest(
	ctx context.Context,
	service string,
	secrets map[string]interface{},
	manifestPath string,
	ageKey string,
	dryRun bool,
	force bool,
) (bool, error) {
	// Check if manifest already exists
	manifestExists := false
	if _, err := os.Stat(manifestPath); err == nil {
		manifestExists = true
	}

	// If manifest doesn't exist, we need to create it
	if !manifestExists {
		if dryRun {
			m.logger.Info("Would create manifest (dry-run)", "service", service, "path", manifestPath)
			return true, nil
		}
		return m.writeEncryptedManifest(ctx, service, secrets, manifestPath, ageKey, nil)
	}

	// Manifest exists - check if it needs updating
	if !force {
		// Get Age key path for decryption
		ageKeyPath, err := m.getAgeKeyPathFromPublicKey(ageKey)
		if err != nil {
			// If we can't get the key path, we can't decrypt to compare
			// In this case, we'll update if force is set or skip if not
			m.logger.Warn("Cannot decrypt existing manifest for comparison", "error", err)
			if dryRun {
				m.logger.Info("Would update manifest (dry-run, cannot verify changes)", "service", service, "path", manifestPath)
				return true, nil
			}
			// Skip update since we can't verify changes
			return false, nil
		}

		// Decrypt existing manifest to compare
		existingSecrets, err := m.decryptManifest(ctx, manifestPath, ageKeyPath)
		if err != nil {
			m.logger.Warn("Failed to decrypt existing manifest for comparison", "error", err)
			// If we can't decrypt, assume it needs updating
			if dryRun {
				m.logger.Info("Would update manifest (dry-run, cannot decrypt existing)", "service", service, "path", manifestPath)
				return true, nil
			}
		} else {
			// Compare secrets to detect changes
			changed := m.hasSecretsChanged(secrets, existingSecrets)
			if !changed {
				m.logger.Debug("Manifest unchanged", "service", service)
				return false, nil
			}
		}
	}

	// Manifest needs updating
	if dryRun {
		m.logger.Info("Would update manifest (dry-run)", "service", service, "path", manifestPath)
		return true, nil
	}

	// Load existing manifest to preserve metadata
	existingManifest, err := m.loadExistingManifest(manifestPath)
	if err != nil {
		m.logger.Warn("Failed to load existing manifest metadata", "error", err)
		existingManifest = nil
	}

	return m.writeEncryptedManifest(ctx, service, secrets, manifestPath, ageKey, existingManifest)
}

// writeEncryptedManifest writes an encrypted secret manifest to disk.
// Returns true on success, false on failure.
func (m *DefaultSecretsManager) writeEncryptedManifest(
	ctx context.Context,
	service string,
	secrets map[string]interface{},
	manifestPath string,
	ageKey string,
	existingManifest map[string]interface{},
) (bool, error) {
	// Generate new manifest
	newManifest := m.generateSecretManifest(service, secrets, existingManifest)

	// Create directory if it doesn't exist
	dir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return false, fmt.Errorf("failed to create directory: %w", err)
	}

	// Write unencrypted manifest to temporary file
	tmpFile, err := os.CreateTemp(dir, "secret-*.yaml")
	if err != nil {
		return false, fmt.Errorf("failed to create temporary file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath) // Clean up temp file

	// Marshal manifest to YAML
	yamlData, err := yaml.Marshal(newManifest)
	if err != nil {
		return false, fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if _, err := tmpFile.Write(yamlData); err != nil {
		tmpFile.Close()
		return false, fmt.Errorf("failed to write temporary file: %w", err)
	}
	tmpFile.Close()

	// Encrypt the manifest using SOPS
	encryptor := m.sopsManager.GetEncryptor()
	if encryptor == nil {
		return false, fmt.Errorf("SOPS encryptor not available")
	}

	encryptConfig := sops.EncryptionConfig{
		AgeKeys: []string{ageKey},
		InPlace: true,
	}

	if err := encryptor.EncryptFile(ctx, tmpPath, encryptConfig); err != nil {
		return false, fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// Read encrypted content
	encryptedData, err := os.ReadFile(tmpPath)
	if err != nil {
		return false, fmt.Errorf("failed to read encrypted file: %w", err)
	}

	// Write encrypted content to final location
	if err := os.WriteFile(manifestPath, encryptedData, 0644); err != nil {
		return false, fmt.Errorf("failed to write manifest: %w", err)
	}

	m.logger.Info("Manifest updated", "service", service, "path", manifestPath)
	return true, nil
}

// loadExistingManifest loads an existing manifest file if it exists.
func (m *DefaultSecretsManager) loadExistingManifest(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var manifest map[string]interface{}
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	return manifest, nil
}

// generateSecretManifest generates a Kubernetes Secret manifest from secrets.
// If existingManifest is provided, non-secret fields are preserved.
func (m *DefaultSecretsManager) generateSecretManifest(
	service string,
	secrets map[string]interface{},
	existingManifest map[string]interface{},
) map[string]interface{} {
	manifest := make(map[string]interface{})

	// Preserve or set apiVersion and kind
	if existingManifest != nil {
		if apiVersion, ok := existingManifest["apiVersion"]; ok {
			manifest["apiVersion"] = apiVersion
		}
		if kind, ok := existingManifest["kind"]; ok {
			manifest["kind"] = kind
		}
	}
	if manifest["apiVersion"] == nil {
		manifest["apiVersion"] = "v1"
	}
	if manifest["kind"] == nil {
		manifest["kind"] = "Secret"
	}

	// Preserve or generate metadata
	metadata := make(map[string]interface{})
	if existingManifest != nil {
		if existingMeta, ok := existingManifest["metadata"].(map[string]interface{}); ok {
			// Preserve all metadata fields
			for k, v := range existingMeta {
				metadata[k] = v
			}
		}
	}
	// Ensure name is set
	if metadata["name"] == nil {
		metadata["name"] = m.generateSecretName(service)
	}
	manifest["metadata"] = metadata

	// Generate data section with secrets
	data := make(map[string]interface{})
	for key, value := range secrets {
		// Convert key to Kubernetes-friendly format (replace underscores with hyphens)
		k8sKey := strings.ReplaceAll(key, "_", "-")
		data[k8sKey] = value
	}
	manifest["data"] = data

	return manifest
}

// generateSecretName generates a Kubernetes Secret name from a service name.
func (m *DefaultSecretsManager) generateSecretName(service string) string {
	// Standard naming pattern: opencenter-<service>-secret
	return fmt.Sprintf("opencenter-%s-secret", service)
}

// hasSecretsChanged compares new secrets against existing decrypted secrets.
// Returns true if any secret value has changed, false if all are identical.
func (m *DefaultSecretsManager) hasSecretsChanged(
	newSecrets map[string]interface{},
	existingSecrets map[string]interface{},
) bool {
	// Check if number of secrets differs
	if len(newSecrets) != len(existingSecrets) {
		return true
	}

	// Compare each secret value
	for key, newValue := range newSecrets {
		// Convert key to manifest format (underscores to hyphens)
		manifestKey := strings.ReplaceAll(key, "_", "-")

		existingValue, exists := existingSecrets[manifestKey]
		if !exists {
			// New secret added
			return true
		}

		// Compare values as strings
		newStr := fmt.Sprintf("%v", newValue)
		existingStr := fmt.Sprintf("%v", existingValue)

		if newStr != existingStr {
			// Secret value changed
			return true
		}
	}

	return false
}

// getAgeKeyPathFromPublicKey attempts to find the Age key file path from a public key.
// This is a helper for decryption when we only have the public key.
func (m *DefaultSecretsManager) getAgeKeyPathFromPublicKey(publicKey string) (string, error) {
	// Try to find the key file in standard locations
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	// Common Age key locations
	possiblePaths := []string{
		filepath.Join(homeDir, ".config", "sops", "age", "keys.txt"),
		filepath.Join(homeDir, ".config", "opencenter", "secrets", "age", "keys.txt"),
	}

	// Also check for cluster-specific keys
	clustersDir := filepath.Join(homeDir, ".config", "opencenter", "clusters")
	if _, err := os.Stat(clustersDir); err == nil {
		// Walk clusters directory to find key files
		filepath.Walk(clustersDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if !info.IsDir() && strings.HasSuffix(path, "_keys.txt") {
				possiblePaths = append(possiblePaths, path)
			}
			return nil
		})
	}

	// Try each path and check if it contains the public key
	for _, keyPath := range possiblePaths {
		if _, err := os.Stat(keyPath); err == nil {
			// Read the key file
			data, err := os.ReadFile(keyPath)
			if err != nil {
				continue
			}

			// Check if this file contains the public key
			if strings.Contains(string(data), publicKey) {
				return keyPath, nil
			}
		}
	}

	return "", fmt.Errorf("Age key file not found for public key: %s", publicKey)
}

// getActor retrieves the actor (user) from context or returns a default value.
func (m *DefaultSecretsManager) getActor(ctx context.Context) string {
	if actor, ok := ctx.Value("actor").(string); ok && actor != "" {
		return actor
	}
	// Try to get current user
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return "system"
}
