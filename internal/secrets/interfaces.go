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
	"time"
)

// KeyType represents the type of encryption key.
type KeyType string

const (
	// KeyTypeAge represents an Age encryption key used by SOPS.
	KeyTypeAge KeyType = "age"
	// KeyTypeSSH represents an SSH key pair for cluster access.
	KeyTypeSSH KeyType = "ssh"
)

// String returns the string representation of the key type.
func (kt KeyType) String() string {
	return string(kt)
}

// KeyStatus represents the lifecycle status of a key.
type KeyStatus string

const (
	// KeyStatusActive indicates the key is currently in use.
	KeyStatusActive KeyStatus = "active"
	// KeyStatusArchived indicates the key has been rotated and archived.
	KeyStatusArchived KeyStatus = "archived"
	// KeyStatusRevoked indicates the key has been revoked.
	KeyStatusRevoked KeyStatus = "revoked"
)

// String returns the string representation of the key status.
func (ks KeyStatus) String() string {
	return string(ks)
}

// SecretsManager handles secrets synchronization and drift detection.
//
// This interface provides methods for synchronizing secrets from config files
// to encrypted manifests, validating that manifests match the config, and
// detecting drift between the source of truth and deployed secrets.
type SecretsManager interface {
	// SyncSecrets regenerates encrypted manifests from the config file.
	// It reads secrets from the cluster's config file and generates
	// corresponding SOPS-encrypted manifests for each service.
	//
	// Returns ErrConfigNotFound if the config file does not exist.
	// Returns ErrKeyNotFound if the cluster's Age key is not available.
	SyncSecrets(ctx context.Context, opts SyncOptions) (*SyncResult, error)

	// ValidateSecrets compares config secrets against encrypted manifests.
	// It decrypts each manifest and compares the values against the config,
	// reporting any drift, missing manifests, orphaned secrets, or security issues.
	//
	// Returns ErrConfigNotFound if the config file does not exist.
	// Returns ErrKeyNotFound if the cluster's Age key is not available.
	// Returns ErrDecryptionFailed if a manifest cannot be decrypted.
	ValidateSecrets(ctx context.Context, opts ValidateOptions) (*ValidationResult, error)

	// DetectDrift identifies differences between config and manifests.
	// This is a lower-level method that returns detailed drift information
	// without the validation context.
	DetectDrift(ctx context.Context, cluster string) (*DriftReport, error)

	// GetSecretSources returns all secret sources for a cluster.
	// This includes the config file path and all manifest paths that
	// contain secrets for the specified cluster.
	GetSecretSources(ctx context.Context, cluster string) ([]SecretSource, error)
}

// SyncOptions configures secrets synchronization behavior.
type SyncOptions struct {
	// Cluster is the target cluster name.
	Cluster string

	// Services is an optional list of specific services to sync.
	// If empty, all services are synced.
	Services []string

	// DryRun previews changes without applying them.
	DryRun bool

	// Force overwrites manifests even if no drift is detected.
	Force bool
}

// SyncResult contains the outcome of a sync operation.
type SyncResult struct {
	// Created lists files that were created.
	Created []string

	// Updated lists files that were updated.
	Updated []string

	// Unchanged lists files with no changes.
	Unchanged []string

	// Errors contains any errors encountered during sync.
	Errors []SyncError
}

// SyncError represents an error during sync for a specific file.
type SyncError struct {
	// FilePath is the path to the file that failed.
	FilePath string

	// Service is the service name associated with the file.
	Service string

	// Error is the underlying error.
	Error error
}

// ValidateOptions configures secrets validation behavior.
type ValidateOptions struct {
	// Cluster is the target cluster name.
	Cluster string

	// Fix automatically runs sync-secrets to resolve detected drift.
	Fix bool

	// OutputJSON outputs the validation result in JSON format.
	OutputJSON bool
}

// ValidationResult contains drift detection findings.
type ValidationResult struct {
	// Valid is true if no drift was detected.
	Valid bool

	// DriftItems contains secrets with drift.
	DriftItems []DriftItem

	// MissingManifests lists config secrets without corresponding manifests.
	MissingManifests []string

	// OrphanedSecrets lists manifest secrets not in the config.
	OrphanedSecrets []string

	// SecurityIssues lists unencrypted secrets found in manifests.
	SecurityIssues []SecurityIssue

	// ExitCode is 0 for valid, 1 for drift detected.
	ExitCode int
}

// DriftItem represents a single drift detection.
type DriftItem struct {
	// Service is the service name (e.g., "harbor").
	Service string

	// FieldPath is the path to the differing field (e.g., "data.password").
	FieldPath string

	// ConfigHash is a hash of the config value (for comparison without exposing secrets).
	ConfigHash string

	// ManifestHash is a hash of the manifest value.
	ManifestHash string
}

// SecurityIssue represents an unencrypted secret found in a manifest.
type SecurityIssue struct {
	// FilePath is the path to the file containing the issue.
	FilePath string

	// FieldPath is the path to the unencrypted field.
	FieldPath string

	// Severity indicates the severity level ("critical", "high", "medium").
	Severity string
}

// DriftReport contains comprehensive drift analysis.
type DriftReport struct {
	// Cluster is the cluster name.
	Cluster string

	// Timestamp is when the drift detection was performed.
	Timestamp time.Time

	// ConfigPath is the path to the config file.
	ConfigPath string

	// OverlayPath is the path to the overlay directory.
	OverlayPath string

	// Services contains drift details per service.
	Services []ServiceDrift

	// TotalDriftCount is the total number of drifted fields.
	TotalDriftCount int

	// SecurityViolations is the count of unencrypted secrets.
	SecurityViolations int
}

// ServiceDrift contains drift details for a single service.
type ServiceDrift struct {
	// ServiceName is the name of the service.
	ServiceName string

	// ManifestPath is the path to the service's manifest.
	ManifestPath string

	// DriftFields contains the fields with drift.
	DriftFields []DriftField

	// Status is "synced", "drifted", or "missing".
	Status string
}

// DriftField represents a single field with drift.
type DriftField struct {
	// Path is the YAML path (e.g., "data.password").
	Path string

	// ConfigHash is a hash of the config value.
	ConfigHash string

	// ManifestHash is a hash of the manifest value.
	ManifestHash string
}

// SecretSource represents a source of secrets.
type SecretSource struct {
	// Type is "config" or "manifest".
	Type string

	// Path is the file path.
	Path string

	// Service is the service name (for manifests).
	Service string
}

// KeyRegistry manages key metadata and lifecycle.
//
// This interface provides methods for registering, retrieving, and managing
// encryption keys across clusters. The registry is stored as a SOPS-encrypted
// YAML file and tracks key fingerprints, creation dates, expiration dates,
// and lifecycle status.
type KeyRegistry interface {
	// RegisterKey adds a new key to the registry.
	// Returns an error if a key with the same fingerprint already exists.
	RegisterKey(ctx context.Context, entry KeyEntry) error

	// GetKey retrieves key metadata by cluster and type.
	// Returns ErrKeyNotFound if no matching key exists.
	GetKey(ctx context.Context, cluster string, keyType KeyType) (*KeyEntry, error)

	// UpdateKeyStatus updates the status of a key.
	// Returns ErrKeyNotFound if no matching key exists.
	UpdateKeyStatus(ctx context.Context, cluster string, keyType KeyType, status KeyStatus) error

	// ListKeys returns all keys, optionally filtered by cluster.
	// If cluster is empty, returns keys for all clusters.
	ListKeys(ctx context.Context, cluster string) ([]KeyEntry, error)

	// CheckExpiration returns keys that are expired or expiring soon.
	// The warnDays parameter specifies the warning threshold in days.
	CheckExpiration(ctx context.Context, warnDays int) (*ExpirationReport, error)

	// RebuildFromFiles reconstructs the registry from existing key files.
	// This is useful when the registry is corrupted or missing.
	RebuildFromFiles(ctx context.Context, keysDir string) error
}

// KeyEntry represents metadata for a single key.
type KeyEntry struct {
	// Cluster is the cluster name this key belongs to.
	Cluster string `yaml:"cluster" json:"cluster"`

	// KeyType is "age" or "ssh".
	KeyType KeyType `yaml:"key_type" json:"key_type"`

	// Fingerprint is the unique identifier for the key.
	Fingerprint string `yaml:"fingerprint" json:"fingerprint"`

	// PublicKey is the public key string.
	PublicKey string `yaml:"public_key" json:"public_key"`

	// CreatedAt is when the key was created.
	CreatedAt time.Time `yaml:"created_at" json:"created_at"`

	// ExpiresAt is when the key expires.
	ExpiresAt time.Time `yaml:"expires_at" json:"expires_at"`

	// Status is "active", "archived", or "revoked".
	Status KeyStatus `yaml:"status" json:"status"`

	// RotatedFrom is the fingerprint of the key this was rotated from.
	RotatedFrom string `yaml:"rotated_from,omitempty" json:"rotated_from,omitempty"`

	// RevokedAt is when the key was revoked.
	RevokedAt time.Time `yaml:"revoked_at,omitempty" json:"revoked_at,omitempty"`

	// RevokedBy is who revoked the key.
	RevokedBy string `yaml:"revoked_by,omitempty" json:"revoked_by,omitempty"`

	// RevokedReason is why the key was revoked.
	RevokedReason string `yaml:"revoked_reason,omitempty" json:"revoked_reason,omitempty"`

	// UsedBy lists paths that use this key.
	UsedBy []string `yaml:"used_by,omitempty" json:"used_by,omitempty"`

	// UserEmail is the email of the user who owns this key (for multi-recipient).
	UserEmail string `yaml:"user_email,omitempty" json:"user_email,omitempty"`
}

// ExpirationReport contains key expiration status.
type ExpirationReport struct {
	// Expired lists keys that have passed their expiration date.
	Expired []KeyExpirationInfo

	// Warning lists keys expiring within the warning period.
	Warning []KeyExpirationInfo

	// Valid lists keys with time remaining.
	Valid []KeyExpirationInfo
}

// KeyExpirationInfo contains expiration details for a key.
type KeyExpirationInfo struct {
	// Cluster is the cluster name.
	Cluster string

	// KeyType is "age" or "ssh".
	KeyType KeyType

	// Fingerprint is the key fingerprint.
	Fingerprint string

	// DaysRemaining is days until expiration (negative if expired).
	DaysRemaining int

	// ExpiresAt is the expiration timestamp.
	ExpiresAt time.Time
}

// KeyRotator handles key rotation operations.
//
// This interface provides methods for rotating Age and SSH keys with
// support for dual-key transitions. During rotation, both old and new
// keys are active to allow for gradual migration.
type KeyRotator interface {
	// RotateAgeKey generates a new Age key and re-encrypts secrets.
	// The new key is added alongside the old key in dual-key mode.
	// Returns ErrKeyNotFound if the cluster has no existing Age key.
	RotateAgeKey(ctx context.Context, opts RotateOptions) (*RotationResult, error)

	// RotateSSHKey generates a new SSH key pair.
	// Updates the config file with the new key paths.
	RotateSSHKey(ctx context.Context, opts RotateOptions) (*RotationResult, error)

	// CompleteRotation removes the old key after dual-key period.
	// Re-encrypts all manifests with only the new key.
	// Returns ErrRotationInProgress if no rotation is in progress.
	CompleteRotation(ctx context.Context, cluster string, keyType KeyType) error

	// GetRotationStatus returns the current rotation state.
	GetRotationStatus(ctx context.Context, cluster string) (*RotationStatus, error)
}

// RotateOptions configures key rotation behavior.
type RotateOptions struct {
	// Cluster is the target cluster.
	Cluster string

	// KeyType is "age" or "ssh".
	KeyType KeyType

	// DryRun previews the rotation plan without making changes.
	DryRun bool

	// Complete indicates this is completing a dual-key rotation.
	Complete bool
}

// RotationResult contains the outcome of a rotation.
type RotationResult struct {
	// OldFingerprint is the previous key fingerprint.
	OldFingerprint string

	// NewFingerprint is the new key fingerprint.
	NewFingerprint string

	// ReencryptedFiles lists files that were re-encrypted.
	ReencryptedFiles []string

	// ArchivedKeyPath is the path to the archived old key.
	ArchivedKeyPath string

	// DualKeyActive indicates if dual-key mode is now active.
	DualKeyActive bool
}

// RotationStatus represents the current rotation state.
type RotationStatus struct {
	// InProgress indicates if a rotation is in progress.
	InProgress bool

	// DualKeyActive indicates if dual-key mode is active.
	DualKeyActive bool

	// OldKey is the old key entry (if in dual-key mode).
	OldKey *KeyEntry

	// NewKey is the new key entry (if in dual-key mode).
	NewKey *KeyEntry

	// PendingFiles lists files not yet re-encrypted with new key only.
	PendingFiles []string
}

// KeyRevoker handles key revocation operations.
//
// This interface provides methods for revoking keys by user email or
// fingerprint, with support for emergency revocation that immediately
// generates a new primary key.
type KeyRevoker interface {
	// RevokeByUser removes all keys associated with a user.
	// Re-encrypts all manifests without the revoked keys.
	// Returns ErrSingleKeyRevocation if this would remove the only key.
	RevokeByUser(ctx context.Context, opts RevokeOptions) (*RevocationResult, error)

	// RevokeByFingerprint removes a specific key.
	// Re-encrypts all manifests without the revoked key.
	// Returns ErrKeyNotFound if the fingerprint doesn't exist.
	// Returns ErrSingleKeyRevocation if this would remove the only key.
	RevokeByFingerprint(ctx context.Context, opts RevokeOptions) (*RevocationResult, error)

	// EmergencyRevoke immediately revokes a key and generates a new primary key.
	// This is used when a key is compromised and immediate action is needed.
	EmergencyRevoke(ctx context.Context, cluster string, fingerprint string) (*RevocationResult, error)
}

// RevokeOptions configures revocation behavior.
type RevokeOptions struct {
	// Cluster is the target cluster.
	Cluster string

	// User is the user email (for RevokeByUser).
	User string

	// Fingerprint is the key fingerprint (for RevokeByFingerprint).
	Fingerprint string

	// DryRun previews the revocation without making changes.
	DryRun bool

	// Emergency performs emergency revocation with new key generation.
	Emergency bool

	// Reason is the reason for revocation (for audit logging).
	Reason string
}

// RevocationResult contains the outcome of a revocation.
type RevocationResult struct {
	// RevokedKeys lists fingerprints of revoked keys.
	RevokedKeys []string

	// ReencryptedFiles lists files re-encrypted without the revoked key.
	ReencryptedFiles []string

	// NewPrimaryKey is the new key fingerprint (emergency revocation only).
	NewPrimaryKey string
}

// HookManager handles Git hook installation and execution.
//
// This interface provides methods for installing pre-commit hooks that
// validate secrets before commits, preventing plaintext secrets and
// configuration drift from being pushed to Git.
type HookManager interface {
	// InstallHooks installs pre-commit hooks in the repository.
	// The hooks validate staged files for unencrypted secrets and drift.
	InstallHooks(ctx context.Context, repoPath string, cluster string) error

	// ValidatePreCommit runs pre-commit validation on staged files.
	// Returns a HookResult indicating whether the commit should proceed.
	ValidatePreCommit(ctx context.Context, stagedFiles []string) (*HookResult, error)

	// UninstallHooks removes installed hooks from the repository.
	UninstallHooks(ctx context.Context, repoPath string) error
}

// HookResult contains pre-commit validation results.
type HookResult struct {
	// Passed indicates whether the validation passed.
	Passed bool

	// UnencryptedFiles lists files with plaintext secrets.
	UnencryptedFiles []string

	// DriftDetected lists files with configuration drift.
	DriftDetected []string

	// PlaintextKeys lists plaintext key files that were staged.
	PlaintextKeys []string

	// Warnings contains non-blocking warnings.
	Warnings []string
}

// MultiClusterSyncer handles multi-cluster operations.
//
// This interface provides methods for performing secrets operations
// across multiple clusters in parallel.
type MultiClusterSyncer interface {
	// SyncAll syncs secrets for all clusters in the organization.
	// Processes clusters in parallel with configurable concurrency.
	SyncAll(ctx context.Context, opts MultiClusterSyncOptions) (*MultiClusterSyncResult, error)
}

// MultiClusterSyncOptions configures multi-cluster sync behavior.
type MultiClusterSyncOptions struct {
	// Organization filters to a specific organization.
	Organization string

	// Concurrency is the maximum number of parallel operations.
	Concurrency int

	// StopOnError stops processing on the first failure.
	StopOnError bool

	// DryRun previews changes without applying them.
	DryRun bool
}

// MultiClusterSyncResult contains the outcome of multi-cluster sync.
type MultiClusterSyncResult struct {
	// Results maps cluster names to their sync results.
	Results map[string]*SyncResult

	// Failures maps cluster names to their errors.
	Failures map[string]error

	// SuccessCount is the number of successful syncs.
	SuccessCount int

	// FailureCount is the number of failed syncs.
	FailureCount int
}
