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

	"gopkg.in/yaml.v3"
)

// ReconcileReport describes differences between a cluster's SOPS recipients
// and its active Age entries in the key registry.
type ReconcileReport struct {
	Cluster                               string
	SOPSConfigPath                        string
	InBoth                                []string
	OnlyInSOPSConfig                      []string
	OnlyInRegistry                        []string
	DuplicateFingerprints                 []string
	RecipientsRevokedButStillInSOPSConfig []string
	Imported                              []string
}

// HasDrift reports whether the registry and SOPS configuration need attention.
func (r *ReconcileReport) HasDrift() bool {
	return len(r.OnlyInSOPSConfig) > 0 ||
		len(r.OnlyInRegistry) > 0 ||
		len(r.DuplicateFingerprints) > 0 ||
		len(r.RecipientsRevokedButStillInSOPSConfig) > 0
}

// DefaultKeyReconciler reconciles SOPS Age recipients with registry metadata.
type DefaultKeyReconciler struct {
	registry       KeyRegistry
	secretsManager SecretsManager
	logger         *slog.Logger
}

// NewDefaultKeyReconciler creates a key reconciler with the given dependencies.
func NewDefaultKeyReconciler(registry KeyRegistry, secretsManager SecretsManager, logger *slog.Logger) *DefaultKeyReconciler {
	if logger == nil {
		logger = slog.Default()
	}

	return &DefaultKeyReconciler{
		registry:       registry,
		secretsManager: secretsManager,
		logger:         logger,
	}
}

// Reconcile compares a cluster's .sops.yaml recipients with its registry.
// When apply is true, only recipients missing from the active registry are
// imported. The SOPS file and active registry entries are never removed or
// rewritten by reconciliation.
func (r *DefaultKeyReconciler) Reconcile(ctx context.Context, cluster string, apply bool) (*ReconcileReport, error) {
	manager, ok := r.secretsManager.(*DefaultSecretsManager)
	if !ok || manager == nil {
		return nil, fmt.Errorf("key reconciliation requires a default secrets manager")
	}

	cfg, configPath, err := manager.loadClusterConfig(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to load cluster config: %w", err)
	}
	overlayPath, err := manager.getOverlayPath(configPath, cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to get overlay path: %w", err)
	}

	sopsConfigPath := filepath.Join(overlayPath, ".sops.yaml")
	data, err := os.ReadFile(sopsConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read .sops.yaml: %w", err)
	}
	sopsRecipients, err := collectSOPSAgeRecipients(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse .sops.yaml: %w", err)
	}

	keys, err := r.registry.ListKeys(ctx, cluster)
	if err != nil {
		return nil, fmt.Errorf("failed to list keys: %w", err)
	}

	report := &ReconcileReport{
		Cluster:                               cluster,
		SOPSConfigPath:                        sopsConfigPath,
		InBoth:                                []string{},
		OnlyInSOPSConfig:                      []string{},
		OnlyInRegistry:                        []string{},
		DuplicateFingerprints:                 []string{},
		RecipientsRevokedButStillInSOPSConfig: []string{},
		Imported:                              []string{},
	}

	activeRecipients := make(map[string]struct{})
	inactiveRecipients := make(map[string]struct{})
	fingerprintCounts := make(map[string]int)
	for _, key := range keys {
		if key.KeyType != KeyTypeAge {
			continue
		}
		if key.Fingerprint != "" {
			fingerprintCounts[key.Fingerprint]++
		}
		recipient, err := canonicalAgeRecipient(key)
		if err != nil {
			return report, fmt.Errorf("invalid Age key in registry: %w", err)
		}
		switch key.Status {
		case KeyStatusActive:
			activeRecipients[recipient] = struct{}{}
		case KeyStatusRevoked, KeyStatusArchived:
			inactiveRecipients[recipient] = struct{}{}
		}
	}

	for _, key := range keys {
		if key.KeyType != KeyTypeAge || key.Fingerprint == "" {
			continue
		}
		if fingerprintCounts[key.Fingerprint] > 1 && !reconcileContainsString(report.DuplicateFingerprints, key.Fingerprint) {
			report.DuplicateFingerprints = append(report.DuplicateFingerprints, key.Fingerprint)
		}
	}

	sopsSet := make(map[string]struct{}, len(sopsRecipients))
	for _, recipient := range sopsRecipients {
		sopsSet[recipient] = struct{}{}
		if _, ok := activeRecipients[recipient]; ok {
			report.InBoth = append(report.InBoth, recipient)
			continue
		}
		report.OnlyInSOPSConfig = append(report.OnlyInSOPSConfig, recipient)
		if _, ok := inactiveRecipients[recipient]; ok {
			report.RecipientsRevokedButStillInSOPSConfig = append(report.RecipientsRevokedButStillInSOPSConfig, recipient)
		}
	}

	for _, key := range keys {
		if key.KeyType != KeyTypeAge || key.Status != KeyStatusActive {
			continue
		}
		recipient, err := canonicalAgeRecipient(key)
		if err != nil {
			return report, fmt.Errorf("invalid active Age key in registry: %w", err)
		}
		if _, ok := sopsSet[recipient]; !ok && !reconcileContainsString(report.OnlyInRegistry, recipient) {
			report.OnlyInRegistry = append(report.OnlyInRegistry, recipient)
		}
	}

	if apply {
		// Reconciliation deliberately imports only missing registry metadata. It
		// does not delete registry entries or rewrite .sops.yaml.
		for _, recipient := range report.OnlyInSOPSConfig {
			if err := r.registry.RegisterKey(ctx, KeyEntry{
				Cluster:     cluster,
				KeyType:     KeyTypeAge,
				Fingerprint: recipient,
				PublicKey:   recipient,
				Status:      KeyStatusActive,
				Primary:     false,
			}); err != nil {
				return report, fmt.Errorf("failed to import SOPS recipient %s: %w", recipient, err)
			}
			report.Imported = append(report.Imported, recipient)
		}
		if len(report.Imported) > 0 {
			keys, err = r.registry.ListKeys(ctx, cluster)
			if err != nil {
				return report, fmt.Errorf("failed to reload keys after imports: %w", err)
			}
		}
		activeCount := 0
		primaryCount := 0
		var soleCandidate KeyEntry
		for _, key := range keys {
			if key.Cluster != cluster || key.KeyType != KeyTypeAge || key.Status != KeyStatusActive {
				continue
			}
			activeCount++
			if key.Primary {
				primaryCount++
			}
			soleCandidate = key
		}
		if activeCount == 1 && primaryCount == 0 {
			if err := r.registry.SetPrimaryKey(ctx, cluster, KeyTypeAge, soleCandidate.Fingerprint); err != nil {
				return report, fmt.Errorf("failed to select sole active Age key as primary: %w", err)
			}
		}
	}

	r.logger.Info("Key registry reconciliation completed",
		"cluster", cluster,
		"in_both", len(report.InBoth),
		"only_in_sops", len(report.OnlyInSOPSConfig),
		"only_in_registry", len(report.OnlyInRegistry),
		"duplicates", len(report.DuplicateFingerprints),
		"revoked_in_sops", len(report.RecipientsRevokedButStillInSOPSConfig),
		"imported", len(report.Imported))

	return report, nil
}

func collectSOPSAgeRecipients(data []byte) ([]string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return []string{}, nil
	}

	recipients := []string{}
	seen := make(map[string]struct{})
	for _, rule := range findSOPSCreationRules(document.Content[0]) {
		if rule.Kind != yaml.MappingNode {
			continue
		}
		for i := 0; i+1 < len(rule.Content); i += 2 {
			if rule.Content[i].Value != "age" {
				continue
			}
			value := rule.Content[i+1]
			if value.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("creation rule age value is not a scalar")
			}
			for _, recipient := range splitSOPSRecipients(value.Value) {
				if _, ok := seen[recipient]; ok {
					continue
				}
				seen[recipient] = struct{}{}
				recipients = append(recipients, recipient)
			}
			break
		}
	}
	return recipients, nil
}

func reconcileContainsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
