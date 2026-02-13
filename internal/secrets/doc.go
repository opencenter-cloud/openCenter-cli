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

// Package secrets provides multi-cluster secrets management for openCenter-cli.
//
// This package implements comprehensive secrets lifecycle management including:
//   - Secrets synchronization between config files and encrypted manifests
//   - Drift detection and validation between source-of-truth and deployed secrets
//   - Key rotation with dual-key transition support
//   - Key expiration tracking and monitoring
//   - Key revocation for users and compromised keys
//   - Pre-commit validation hooks for Git repositories
//   - Multi-cluster operations with parallel processing
//   - Audit logging for all key operations
//
// # Architecture
//
// The package follows a layered architecture:
//
//	┌─────────────────────────────────────────────────────────────────┐
//	│                    CLI Commands (cmd/)                          │
//	│  sync-secrets, validate-secrets, rotate-keys, check-keys, etc.  │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	┌─────────────────────────────▼───────────────────────────────────┐
//	│                    Service Layer (this package)                 │
//	│  SecretsManager, KeyRegistry, KeyRotator, KeyRevoker, etc.      │
//	└─────────────────────────────────────────────────────────────────┘
//	                              │
//	┌─────────────────────────────▼───────────────────────────────────┐
//	│                    Data Layer                                   │
//	│  Config files, Key Registry, Encrypted Manifests, Audit Log     │
//	└─────────────────────────────────────────────────────────────────┘
//
// # Core Interfaces
//
// SecretsManager handles secrets synchronization and drift detection:
//
//	manager := secrets.NewSecretsManager(sopsManager, registry, logger)
//	result, err := manager.SyncSecrets(ctx, secrets.SyncOptions{
//	    Cluster:  "my-cluster",
//	    Services: []string{"harbor", "keycloak"},
//	    DryRun:   true,
//	})
//
// KeyRegistry manages key metadata and lifecycle:
//
//	registry := secrets.NewKeyRegistry(registryPath, sopsManager)
//	report, err := registry.CheckExpiration(ctx, 14) // Warn 14 days before expiry
//
// KeyRotator handles key rotation with dual-key support:
//
//	rotator := secrets.NewKeyRotator(registry, sopsManager, logger)
//	result, err := rotator.RotateAgeKey(ctx, secrets.RotateOptions{
//	    Cluster: "my-cluster",
//	    DryRun:  false,
//	})
//
// KeyRevoker handles key revocation:
//
//	revoker := secrets.NewKeyRevoker(registry, sopsManager, logger)
//	result, err := revoker.RevokeByUser(ctx, secrets.RevokeOptions{
//	    Cluster: "my-cluster",
//	    User:    "user@example.com",
//	})
//
// # Error Handling
//
// The package defines specific error types for secrets operations:
//
//   - ErrConfigNotFound: Config file does not exist
//   - ErrKeyNotFound: Age or SSH key not found
//   - ErrDecryptionFailed: Cannot decrypt manifest
//   - ErrEncryptionFailed: Cannot encrypt manifest
//   - ErrRegistryCorrupted: Key registry is invalid
//   - ErrRotationInProgress: Dual-key rotation incomplete
//   - ErrSingleKeyRevocation: Attempting to revoke only key
//
// All errors include suggestions for resolution and support the
// errors.StructuredError interface for consistent error handling.
//
// # Key Registry
//
// The key registry is stored as a SOPS-encrypted YAML file at
// secrets/key-registry.yaml and tracks:
//
//   - Key fingerprints and public keys
//   - Creation and expiration dates
//   - Key status (active, archived, revoked)
//   - Usage information (which paths use each key)
//
// # Audit Logging
//
// All key operations are logged to the audit log with:
//
//   - Timestamp and actor information
//   - Event type (key.generated, key.rotated, key.revoked, etc.)
//   - Key fingerprint and cluster
//   - HMAC signatures for tamper detection
//
// # Thread Safety
//
// All interfaces in this package are designed to be safe for concurrent use.
// The KeyRegistry uses file locking to prevent concurrent modifications.
package secrets
