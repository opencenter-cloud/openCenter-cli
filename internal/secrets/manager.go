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
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/secretartifacts"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
	"gopkg.in/yaml.v3"
)

// DefaultSecretsManager implements the SecretsManager interface.
// It provides secrets synchronization, drift detection, and validation
// by coordinating between config files, SOPS encryption, and manifest generation.
type DefaultSecretsManager struct {
	configLoader         *v2.ConfigIOHandler
	sopsManager          *sops.DefaultSOPSManager
	auditLogger          AuditLogger
	logger               *slog.Logger
	ownershipStateWriter func(string, secretartifacts.OwnershipState) error
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
	configLoader *v2.ConfigIOHandler,
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

	// Build the neutral artifact plan. The logical service is retained for
	// manifest identity while the target service controls the output route.
	artifacts, err := secretartifacts.Plan(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to plan secret artifacts: %w", err)
	}

	// Determine overlay directory path
	overlayPath, err := m.getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to determine overlay path: %w", err)
	}

	m.logger.Debug("Sync configuration",
		"config_path", configPath,
		"overlay_path", overlayPath,
		"services_count", len(artifacts))

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

	var syncLock *os.File
	if !opts.DryRun {
		syncLock, err = m.acquireSyncLock(ctx, overlayPath)
		if err != nil {
			return nil, fmt.Errorf("acquire secrets sync lock: %w", err)
		}
		defer m.releaseSyncLock(syncLock)
	}

	previousPaths, stateErr := m.loadArtifactOwnershipState(overlayPath)
	if stateErr != nil {
		return nil, fmt.Errorf("load secret artifact ownership state: %w", stateErr)
	}
	if !opts.DryRun {
		m.preflightStaleArtifactDeletions(overlayPath, previousPaths, artifacts, opts.Services, result)
	}
	successfulPaths := make(map[string]secretartifacts.OwnershipArtifact)
	journal := &secretMutationJournal{}

	// Process each planned physical artifact exactly once.
	for _, artifact := range artifacts {
		if !artifactMatchesFilter(artifact, opts.Services) {
			continue
		}
		fullPath := filepath.Join(overlayPath, filepath.FromSlash(artifact.Path))
		if _, exists := previousPaths[artifact.Path]; !exists {
			if info, err := os.Lstat(fullPath); err == nil {
				if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
					result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: fmt.Errorf("existing unowned or unsafe secret artifact")})
					continue
				}
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: fmt.Errorf("existing unowned secret artifact; refusing to adopt")})
				continue
			} else if err != nil && !os.IsNotExist(err) {
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: err})
				continue
			}
		}

		var before secretFileSnapshot
		if !opts.DryRun {
			var snapshotErr error
			before, snapshotErr = snapshotSecretFile(fullPath)
			if snapshotErr != nil {
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: snapshotErr})
				continue
			}
		}
		serviceName := artifact.TargetService
		for _, owner := range artifact.OwnerNames() {
			if owner == "grafana" {
				serviceName = "grafana"
				break
			}
		}
		outcome, err := m.syncServiceManifestOutcome(ctx, serviceName, artifact.Payload, fullPath, ageKey, opts.DryRun, opts.Force, previousPaths[artifact.Path])
		if err != nil {
			result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: err})
			continue
		}
		if !opts.DryRun {
			data, hashErr := os.ReadFile(fullPath)
			if hashErr != nil {
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: artifact.TargetService, Error: hashErr})
				continue
			}
			record := secretartifacts.OwnershipArtifact{Path: artifact.Path, Owners: artifact.OwnerNames(), Hash: secretartifacts.HashBytes(data)}
			successfulPaths[artifact.Path] = record
			if outcome == syncCreated || outcome == syncUpdated {
				journal.record(before, fullPath, record.Hash, false)
			}
		}
		switch outcome {
		case syncCreated:
			result.Created = append(result.Created, fullPath)
		case syncUpdated:
			result.Updated = append(result.Updated, fullPath)
		default:
			result.Unchanged = append(result.Unchanged, fullPath)
		}
	}

	m.reconcileArtifactStateWithRecordsAndJournal(overlayPath, previousPaths, artifacts, opts.Services, successfulPaths, result, opts.DryRun, journal)

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
		logicalService := logicalServiceForTarget(service, cfg)
		owners, targetSecrets := artifactTargetSecrets(service, cfg)
		if len(owners) > 0 {
			for _, owner := range owners {
				foundServices[owner] = true
			}
		} else {
			foundServices[logicalService] = true
		}

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
		configServiceSecrets, exists := targetSecrets, len(targetSecrets) > 0
		if len(owners) == 0 {
			configServiceSecrets, exists = configSecrets[logicalService]
		}
		if exists {
			// Check for drift
			driftItems := m.compareSecrets(logicalService, configServiceSecrets, manifestSecrets)
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
		logicalService := logicalServiceForTarget(service, cfg)
		owners, targetSecrets := artifactTargetSecrets(service, cfg)
		if len(owners) > 0 {
			for _, owner := range owners {
				foundServices[owner] = true
			}
		} else {
			foundServices[logicalService] = true
		}

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
		configServiceSecrets, exists := targetSecrets, len(targetSecrets) > 0
		if len(owners) == 0 {
			configServiceSecrets, exists = configSecrets[logicalService]
		}
		if exists {
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

	// Parse org/cluster identifier (e.g. "opencenter-dev/hamlet" → org="opencenter-dev", name="hamlet")
	clusterName := cluster
	organization := ""
	if parts := strings.SplitN(cluster, "/", 2); len(parts) == 2 {
		organization = parts[0]
		clusterName = parts[1]
	}

	var err error
	if organization != "" {
		clusterPaths, resolveErr := pathResolver.Resolve(ctx, clusterName, organization)
		if resolveErr == nil {
			return clusterPaths.ConfigPath, nil
		}
		err = resolveErr
	} else {
		clusterPaths, resolveErr := pathResolver.ResolveWithFallback(ctx, clusterName)
		if resolveErr == nil {
			return clusterPaths.ConfigPath, nil
		}
		err = resolveErr
	}

	// Fallback: search blueprints directory for the cluster config
	blueprintsDir := config.GetBlueprintsDir()
	entries, readErr := os.ReadDir(blueprintsDir)
	if readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			candidate := filepath.Join(blueprintsDir, entry.Name(), clusterName, fmt.Sprintf("%s-config.yaml", clusterName))
			if _, statErr := os.Stat(candidate); statErr == nil {
				return candidate, nil
			}
		}
	}

	// Final fallback: use organization if known, otherwise "opencenter"
	fallbackOrg := "opencenter"
	if organization != "" {
		fallbackOrg = organization
	}
	_ = err
	return filepath.Join(blueprintsDir, fallbackOrg, clusterName, fmt.Sprintf("%s-config.yaml", clusterName)), nil
}

// extractSecretsFromConfig extracts all secrets from the config file.
// It returns a map of service names to their secret values.
func (m *DefaultSecretsManager) extractSecretsFromConfig(cfg *v2.Config) (map[string]map[string]interface{}, error) {
	artifacts, err := secretartifacts.Plan(cfg)
	if err != nil {
		return nil, err
	}
	secretsMap := make(map[string]map[string]interface{})
	for _, artifact := range artifacts {
		for _, owner := range artifact.OwnerNames() {
			secretsMap[owner] = artifact.Payload
		}
	}
	return secretsMap, nil
}

// mapSecretsToManifests maps config secrets to their corresponding manifest file paths.
// It returns a map of service names to manifest paths, optionally filtered by the services list.
func (m *DefaultSecretsManager) mapSecretsToManifests(
	cfg *v2.Config,
	secretsMap map[string]map[string]interface{},
	serviceFilter []string,
) (map[string]string, error) {
	manifestPaths := make(map[string]string)

	for service := range secretsMap {
		// Filters may name either the logical source or its target route.
		if len(serviceFilter) > 0 && !serviceMatchesFilter(service, serviceFilter) {
			continue
		}
		manifestPaths[service] = m.getManifestPath(service, cfg)
	}

	return manifestPaths, nil
}

// getManifestPath returns the expected manifest path for a service.
// The path is relative to the overlay directory.
func (m *DefaultSecretsManager) getManifestPath(service string, cfg *v2.Config) string {
	if artifacts, err := secretartifacts.Plan(cfg); err == nil {
		for _, artifact := range artifacts {
			for _, owner := range artifact.OwnerNames() {
				if owner == service {
					return filepath.FromSlash(artifact.Path)
				}
			}
		}
	}
	target := service
	if service == "grafana" {
		target = "kube-prometheus-stack"
	}
	return filepath.Join("services", target, "secret.yaml")
}

func serviceMatchesFilter(service string, filters []string) bool {
	target := service
	if service == "grafana" {
		target = "kube-prometheus-stack"
	}
	for _, raw := range filters {
		filter := strings.ReplaceAll(strings.TrimSpace(raw), "_", "-")
		if filter == service || filter == target || (target == "kube-prometheus-stack" && filter == "grafana") {
			return true
		}
	}
	return false
}

func artifactTargetSecrets(target string, cfg *v2.Config) ([]string, map[string]interface{}) {
	artifacts, err := secretartifacts.Plan(cfg)
	if err != nil {
		return nil, nil
	}
	for _, artifact := range artifacts {
		if artifact.TargetService == target {
			return artifact.OwnerNames(), artifact.Payload
		}
	}
	return nil, nil
}

func logicalServiceForTarget(target string, cfg *v2.Config) string {
	owners, _ := artifactTargetSecrets(target, cfg)
	if len(owners) > 0 {
		return owners[0]
	}
	return target
}

func artifactMatchesFilter(artifact secretartifacts.Artifact, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	return serviceMatchesFilter(artifact.LogicalService, filters) || serviceMatchesFilter(artifact.TargetService, filters)
}

const artifactStateFilename = secretartifacts.OwnershipStateFilename

const syncLockFilename = ".opencenter-secrets.lock"

type artifactState = secretartifacts.OwnershipState

func (m *DefaultSecretsManager) acquireSyncLock(ctx context.Context, overlayPath string) (*os.File, error) {
	if err := os.MkdirAll(overlayPath, 0o700); err != nil {
		return nil, fmt.Errorf("create overlay directory: %w", err)
	}
	lockPath := filepath.Join(overlayPath, syncLockFilename)
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open sync lock: %w", err)
	}
	for {
		if err := ctx.Err(); err != nil {
			_ = file.Close()
			return nil, err
		}
		if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err == nil {
			return file, nil
		} else if err != syscall.EWOULDBLOCK && err != syscall.EAGAIN {
			_ = file.Close()
			return nil, fmt.Errorf("lock sync transaction: %w", err)
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			_ = file.Close()
			return nil, ctx.Err()
		case <-timer.C:
		}
	}
}

func (m *DefaultSecretsManager) releaseSyncLock(file *os.File) {
	if file == nil {
		return
	}
	_ = syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	_ = file.Close()
}

func (m *DefaultSecretsManager) loadArtifactOwnershipState(overlayPath string) (map[string]secretartifacts.OwnershipArtifact, error) {
	state, _, err := secretartifacts.LoadOwnershipState(overlayPath)
	if err != nil {
		return nil, err
	}
	return state.ByPath(), nil
}

// loadArtifactState is retained for package-local legacy callers.
func (m *DefaultSecretsManager) loadArtifactState(overlayPath string) (map[string]bool, error) {
	records, err := m.loadArtifactOwnershipState(overlayPath)
	if err != nil {
		return nil, err
	}
	paths := make(map[string]bool, len(records))
	for path := range records {
		paths[path] = true
	}
	return paths, nil
}

func statePathMatchesFilter(relative string, filters []string) bool {
	if len(filters) == 0 {
		return true
	}
	parts := strings.Split(filepath.ToSlash(relative), "/")
	return len(parts) == 3 && serviceMatchesFilter(parts[1], filters)
}

type secretFileSnapshot struct {
	path   string
	exists bool
	data   []byte
	mode   os.FileMode
}

type secretFileMutation struct {
	before         secretFileSnapshot
	target         string
	expectedHash   string
	expectedAbsent bool
}

type secretMutationJournal struct {
	mutations []secretFileMutation
}

func snapshotSecretFile(path string) (secretFileSnapshot, error) {
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return secretFileSnapshot{path: path}, nil
	}
	if err != nil {
		return secretFileSnapshot{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return secretFileSnapshot{}, fmt.Errorf("unsafe secret artifact target %s", path)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return secretFileSnapshot{}, err
	}
	return secretFileSnapshot{path: path, exists: true, data: data, mode: info.Mode().Perm()}, nil
}

func (j *secretMutationJournal) record(before secretFileSnapshot, target, expectedHash string, expectedAbsent bool) {
	j.mutations = append(j.mutations, secretFileMutation{before: before, target: target, expectedHash: expectedHash, expectedAbsent: expectedAbsent})
}

func (j *secretMutationJournal) rollback(result *SyncResult) []string {
	rolledBack := make([]string, 0, len(j.mutations))
	for i := len(j.mutations) - 1; i >= 0; i-- {
		mutation := j.mutations[i]
		if err := rollbackSecretMutation(mutation); err != nil {
			result.Errors = append(result.Errors, SyncError{FilePath: mutation.target, Error: fmt.Errorf("rollback secret artifact: %w", err)})
			continue
		}
		rolledBack = append(rolledBack, mutation.target)
	}
	return rolledBack
}

func rollbackSecretMutation(mutation secretFileMutation) error {
	info, err := os.Lstat(mutation.target)
	if mutation.before.exists {
		if err != nil {
			if os.IsNotExist(err) && mutation.expectedAbsent {
				return restoreSecretSnapshot(mutation.before)
			}
			if os.IsNotExist(err) {
				return fmt.Errorf("target disappeared concurrently")
			}
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to clobber changed target")
		}
		data, err := os.ReadFile(mutation.target)
		if err != nil {
			return err
		}
		if mutation.expectedAbsent {
			return fmt.Errorf("target reappeared concurrently")
		}
		if secretartifacts.HashBytes(data) != mutation.expectedHash {
			return fmt.Errorf("target changed concurrently; expected post-mutation hash %s", mutation.expectedHash)
		}
		return restoreSecretSnapshot(mutation.before)
	}
	if err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to remove changed target")
		}
		data, readErr := os.ReadFile(mutation.target)
		if readErr != nil {
			return readErr
		}
		if secretartifacts.HashBytes(data) != mutation.expectedHash {
			return fmt.Errorf("target changed concurrently; expected post-mutation hash %s", mutation.expectedHash)
		}
		if removeErr := os.Remove(mutation.target); removeErr != nil && !os.IsNotExist(removeErr) {
			return removeErr
		}
		return nil
	}
	if !os.IsNotExist(err) {
		return err
	}
	return nil
}

func restoreSecretSnapshot(snapshot secretFileSnapshot) error {
	if !snapshot.exists {
		return nil
	}
	if info, err := os.Lstat(snapshot.path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("refusing to overwrite changed target")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(snapshot.path)
	tmp, err := os.CreateTemp(dir, ".secret-rollback-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(snapshot.mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(snapshot.data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if info, err := os.Lstat(snapshot.path); err == nil && (info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular()) {
		return fmt.Errorf("refusing to overwrite changed target")
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return os.Rename(tmpPath, snapshot.path)
}

func (m *DefaultSecretsManager) writeOwnershipState(overlayPath string, state secretartifacts.OwnershipState) error {
	if m.ownershipStateWriter != nil {
		return m.ownershipStateWriter(overlayPath, state)
	}
	return secretartifacts.WriteOwnershipStateAtomic(overlayPath, state)
}

func (m *DefaultSecretsManager) preflightStaleArtifactDeletions(overlayPath string, previous map[string]secretartifacts.OwnershipArtifact, artifacts []secretartifacts.Artifact, filters []string, result *SyncResult) {
	planned := make(map[string]bool)
	for _, artifact := range artifacts {
		if artifactMatchesFilter(artifact, filters) {
			planned[artifact.Path] = true
		}
	}
	for relative, record := range previous {
		if planned[relative] || !statePathMatchesFilter(relative, filters) {
			continue
		}
		fullPath := filepath.Join(overlayPath, filepath.FromSlash(relative))
		if err := verifyOwnedPath(overlayPath, fullPath, record.Hash); err != nil {
			result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: strings.Split(relative, "/")[1], Error: fmt.Errorf("stale secret artifact is not safely deletable: %w", err)})
		}
	}
}

func (m *DefaultSecretsManager) reconcileArtifactStateWithRecordsAndJournal(overlayPath string, previous map[string]secretartifacts.OwnershipArtifact, artifacts []secretartifacts.Artifact, filters []string, successful map[string]secretartifacts.OwnershipArtifact, result *SyncResult, dryRun bool, journal *secretMutationJournal) {
	if dryRun {
		return
	}
	hadPreexistingErrors := len(result.Errors) > 0
	planned := make(map[string]secretartifacts.Artifact)
	for _, artifact := range artifacts {
		if artifactMatchesFilter(artifact, filters) {
			planned[artifact.Path] = artifact
		}
	}
	owned := make(map[string]secretartifacts.OwnershipArtifact, len(previous)+len(successful))
	for path, record := range previous {
		owned[path] = record
	}
	for path, record := range successful {
		owned[path] = record
	}

	// A sibling error makes pruning unsafe: commit successful writes, but retain
	// every stale record so a retry can safely reconcile it.
	if !hadPreexistingErrors {
		stale := make([]struct {
			relative string
			record   secretartifacts.OwnershipArtifact
			snapshot secretFileSnapshot
		}, 0)
		preflightFailed := false
		for relative, record := range previous {
			if _, keep := planned[relative]; keep || !statePathMatchesFilter(relative, filters) {
				continue
			}
			fullPath := filepath.Join(overlayPath, filepath.FromSlash(relative))
			if err := verifyOwnedPath(overlayPath, fullPath, record.Hash); err != nil {
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: strings.Split(relative, "/")[1], Error: fmt.Errorf("stale secret artifact is not safely deletable: %w", err)})
				preflightFailed = true
				continue
			}
			snapshot, err := snapshotSecretFile(fullPath)
			if err != nil {
				result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Service: strings.Split(relative, "/")[1], Error: fmt.Errorf("snapshot stale secret artifact: %w", err)})
				preflightFailed = true
				continue
			}
			stale = append(stale, struct {
				relative string
				record   secretartifacts.OwnershipArtifact
				snapshot secretFileSnapshot
			}{relative, record, snapshot})
		}
		if !preflightFailed {
			pruneJournal := &secretMutationJournal{}
			pruneFailed := false
			for _, item := range stale {
				if err := verifyOwnedPath(overlayPath, item.snapshot.path, item.record.Hash); err != nil {
					result.Errors = append(result.Errors, SyncError{FilePath: item.snapshot.path, Service: strings.Split(item.relative, "/")[1], Error: fmt.Errorf("stale secret artifact changed before deletion: %w", err)})
					pruneFailed = true
					break
				}
				if err := os.Remove(item.snapshot.path); err != nil {
					result.Errors = append(result.Errors, SyncError{FilePath: item.snapshot.path, Service: strings.Split(item.relative, "/")[1], Error: fmt.Errorf("remove stale secret artifact: %w", err)})
					pruneFailed = true
					break
				}
				pruneJournal.record(item.snapshot, item.snapshot.path, "", true)
			}
			if pruneFailed {
				pruneJournal.rollback(result)
			} else {
				for i, item := range stale {
					delete(owned, item.relative)
					journal.mutations = append(journal.mutations, pruneJournal.mutations[i])
				}
			}
		}
	}

	state := secretartifacts.OwnershipState{Version: secretartifacts.OwnershipStateVersion}
	for _, record := range owned {
		state.Artifacts = append(state.Artifacts, record)
	}
	if err := m.writeOwnershipState(overlayPath, state); err != nil {
		result.Errors = append(result.Errors, SyncError{FilePath: filepath.Join(overlayPath, artifactStateFilename), Error: err})
		rolledBack := journal.rollback(result)
		rolledBackSet := make(map[string]struct{}, len(rolledBack))
		for _, target := range rolledBack {
			rolledBackSet[target] = struct{}{}
		}
		result.Created = removeRolledBackTargets(result.Created, rolledBackSet)
		result.Updated = removeRolledBackTargets(result.Updated, rolledBackSet)
	}
}

func removeRolledBackTargets(paths []string, rolledBack map[string]struct{}) []string {
	remaining := paths[:0]
	for _, path := range paths {
		if _, ok := rolledBack[path]; !ok {
			remaining = append(remaining, path)
		}
	}
	return remaining
}

// reconcileArtifactState is a compatibility helper for older package tests.
// Production synchronization uses reconcileArtifactStateWithRecords.
func (m *DefaultSecretsManager) reconcileArtifactState(overlayPath string, previous map[string]bool, artifacts []secretartifacts.Artifact, filters []string, successful map[string]bool, result *SyncResult, dryRun bool) {
	if dryRun || len(result.Errors) > 0 {
		return
	}
	planned := make(map[string]bool)
	for _, artifact := range artifacts {
		if artifactMatchesFilter(artifact, filters) {
			planned[artifact.Path] = true
		}
	}
	for relative := range previous {
		if planned[relative] || !statePathMatchesFilter(relative, filters) {
			continue
		}
		fullPath := filepath.Join(overlayPath, filepath.FromSlash(relative))
		if err := os.Remove(fullPath); err != nil && !os.IsNotExist(err) {
			result.Errors = append(result.Errors, SyncError{FilePath: fullPath, Error: err})
		}
	}
}
func verifyOwnedPath(root, fullPath, expectedHash string) error {
	if rel, err := filepath.Rel(root, fullPath); err != nil || !secretartifacts.SafeArtifactPath(filepath.ToSlash(rel)) {
		return fmt.Errorf("path escapes overlay")
	}
	for current := filepath.Dir(fullPath); current != filepath.Clean(root); current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err == nil && info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink ancestor %s", current)
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	info, err := os.Lstat(fullPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink target")
	}
	data, err := os.ReadFile(fullPath)
	if err != nil {
		return err
	}
	if expectedHash == "" || secretartifacts.HashBytes(data) != expectedHash {
		return fmt.Errorf("content hash changed")
	}
	return nil
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
			fmt.Errorf("age key file not found at %s", keyPath),
		)
	}

	return keyPath, nil
}

// findManifestFiles scans the overlay directory for secret manifest files.
// Returns a list of absolute paths to manifest files.
func (m *DefaultSecretsManager) findManifestFiles(overlayPath string) ([]string, error) {
	overlayInfo, err := os.Lstat(overlayPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to stat overlay directory %q: %w", overlayPath, err)
	}
	if overlayInfo.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing symlinked overlay directory %q", overlayPath)
	}
	if !overlayInfo.IsDir() {
		return nil, fmt.Errorf("overlay path is not a directory: %q", overlayPath)
	}

	var manifestFiles []string
	for _, rootName := range []string{"services", "managed-services"} {
		rootPath := filepath.Join(overlayPath, rootName)
		rootInfo, err := os.Lstat(rootPath)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("failed to stat manifest root %q: %w", rootPath, err)
		}
		if rootInfo.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("refusing symlinked manifest root %q", rootPath)
		}
		if !rootInfo.IsDir() {
			return nil, fmt.Errorf("manifest root is not a directory: %q", rootPath)
		}

		err = filepath.Walk(rootPath, func(path string, info os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if info == nil {
				return fmt.Errorf("missing file information for %q", path)
			}
			if info.Mode()&os.ModeSymlink != 0 {
				return fmt.Errorf("refusing symlinked path %q", path)
			}
			if !info.IsDir() && info.Name() == "secret.yaml" {
				manifestFiles = append(manifestFiles, path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("failed to walk manifest root %q: %w", rootPath, err)
		}
	}

	sort.Strings(manifestFiles)
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

	// Extract the secret payload. Manifests are generated with `stringData`
	// (raw values); `data` is only present in manifests written by older
	// versions, so it is read as a fallback to keep drift detection working
	// against a repo that has not been re-synced yet.
	if data, ok := manifest["stringData"].(map[string]interface{}); ok {
		return data, nil
	}
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

type syncOutcome int

const (
	syncUnchanged syncOutcome = iota
	syncCreated
	syncUpdated
)

func (m *DefaultSecretsManager) syncServiceManifestOutcome(ctx context.Context, service string, secrets map[string]interface{}, manifestPath, ageKey string, dryRun, force bool, record secretartifacts.OwnershipArtifact) (syncOutcome, error) {
	info, err := os.Lstat(manifestPath)
	existed := err == nil
	if err != nil && !os.IsNotExist(err) {
		return syncUnchanged, err
	}
	if existed {
		if info.Mode()&os.ModeSymlink != 0 || info.IsDir() {
			return syncUnchanged, fmt.Errorf("refusing symlink or directory target")
		}
		if record.Hash == "" {
			return syncUnchanged, fmt.Errorf("existing secret artifact has no ownership hash")
		}
		if err := verifyOwnedPath(filepath.Dir(filepath.Dir(filepath.Dir(manifestPath))), manifestPath, record.Hash); err != nil {
			return syncUnchanged, err
		}
	}
	changed, err := m.syncServiceManifest(ctx, service, secrets, manifestPath, ageKey, dryRun, force)
	if err != nil {
		return syncUnchanged, err
	}
	if !changed {
		return syncUnchanged, nil
	}
	if dryRun {
		if existed {
			return syncUpdated, nil
		}
		return syncCreated, nil
	}
	if existed {
		return syncUpdated, nil
	}
	return syncCreated, nil
}

// syncServiceManifest generates or updates a service's secret manifest.
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

	dir := filepath.Dir(manifestPath)
	// Verify every existing ancestor and the final target before creating anything.
	root := filepath.Dir(filepath.Dir(dir))
	for current := dir; ; current = filepath.Dir(current) {
		if info, statErr := os.Lstat(current); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
			return false, fmt.Errorf("refusing to write through symlink %s", current)
		} else if statErr != nil && !os.IsNotExist(statErr) {
			return false, statErr
		}
		if current == root || filepath.Dir(current) == current {
			break
		}
	}
	if info, statErr := os.Lstat(manifestPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to overwrite symlink %s", manifestPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return false, statErr
	}
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
		AgeKeys:          []string{ageKey},
		EncryptedRegex:   "^(data|stringData)$",
		FilenameOverride: manifestPath,
		InPlace:          true,
	}

	if err := encryptor.EncryptFile(ctx, tmpPath, encryptConfig); err != nil {
		return false, fmt.Errorf("failed to encrypt manifest: %w", err)
	}

	// The encrypted temporary file is already in the verified destination parent;
	// atomically rename it into place rather than writing through the target.
	if info, statErr := os.Lstat(manifestPath); statErr == nil && info.Mode()&os.ModeSymlink != 0 {
		return false, fmt.Errorf("refusing to overwrite symlink %s", manifestPath)
	} else if statErr != nil && !os.IsNotExist(statErr) {
		return false, statErr
	}
	if err := os.Rename(tmpPath, manifestPath); err != nil {
		return false, fmt.Errorf("failed to replace manifest: %w", err)
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
	manifest["stringData"] = data

	return manifest
}

// generateSecretName generates a Kubernetes Secret name from a service name.
func (m *DefaultSecretsManager) generateSecretName(service string) string {
	// Service-specific secret name overrides where the Helm chart expects
	// a particular secret name that doesn't match the standard pattern.
	overrides := map[string]string{
		"grafana":     "grafana-admin-password",
		"etcd-backup": "etcd-backup-secrets",
		"velero":      "velero-cloud-credentials",
	}
	if name, ok := overrides[service]; ok {
		return name
	}
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

	return "", fmt.Errorf("age key file not found for public key: %s (Age key file not found)", publicKey)
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
