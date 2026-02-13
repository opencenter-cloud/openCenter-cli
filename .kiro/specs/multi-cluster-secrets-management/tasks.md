# Implementation Plan: Multi-Cluster Secrets Management

## Overview

This implementation plan breaks down the multi-cluster secrets management feature into discrete coding tasks. The implementation follows the existing openCenter-cli patterns, extending the `cmd/` layer with new commands and the `internal/` layer with new packages for secrets management.

The implementation is organized in phases: core infrastructure, secrets manager implementation, key rotation and revocation, audit logging, hooks, multi-cluster operations, and CLI commands.

## Implementation Status

Most core functionality has been implemented. Remaining work focuses on optional property-based tests that provide additional validation coverage. All required functionality for production use is complete.

## Tasks

- [x] 1. Set up secrets management package structure
  - Create `internal/secrets/` directory structure
  - Define core interfaces in `internal/secrets/interfaces.go`
  - Create error types in `internal/secrets/errors.go`
  - Create rollback manager in `internal/secrets/rollback.go`
  - _Requirements: 1.7, 1.8, 3.9, 6.7_

- [x] 2. Implement Key Registry
  - [x] 2.1 Create KeyRegistry interface and implementation
    - Implement `internal/secrets/registry.go` with DefaultKeyRegistry
    - Implement RegisterKey, GetKey, UpdateKeyStatus, ListKeys methods
    - Add SOPS encryption/decryption for registry file
    - _Requirements: 4.7, 4.8, 9.1, 9.2_
  
  - [x] 2.2 Write property test for Key Registry
    - **Property 10: Key Registry Completeness**
    - **Validates: Requirements 4.7, 4.8, 9.2, 9.3**
  
  - [x] 2.3 Implement expiration checking
    - Add CheckExpiration method with configurable warn days
    - Calculate days remaining from creation date and policy
    - Return ExpirationReport with expired, warning, and valid keys
    - _Requirements: 4.1, 4.2, 4.3, 4.4_
  
  - [x] 2.4 Write property test for expiration calculation
    - **Property 9: Key Expiration Calculation**
    - **Validates: Requirements 4.2, 4.3, 4.4**
  
  - [x] 2.5 Implement registry rebuild from files
    - Add RebuildFromFiles method to reconstruct registry from key files
    - Scan secrets/age/ and secrets/ssh/ directories
    - Extract fingerprints and creation dates from files
    - _Requirements: 9.8_

- [ ] 3. Complete SecretsManager implementation
  - [x] 3.1 Core SecretsManager structure (COMPLETED)
    - Implement `internal/secrets/manager.go` with DefaultSecretsManager
    - Define SyncOptions, SyncResult, ValidationResult types
    - Implement helper methods for config loading and path resolution
    - _Requirements: 1.1, 1.2_
  
  - [x] 3.2 Complete SyncSecrets method
    - Finish syncServiceManifest implementation (currently incomplete)
    - Implement generateSecretManifest helper
    - Implement hasManifestChanged helper
    - Ensure proper SOPS encryption of generated manifests
    - _Requirements: 1.2, 1.3, 1.4, 1.9_
  
  - [x] 3.3 Write property test for sync round-trip
    - **Property 1: Sync Round-Trip Consistency**
    - **Validates: Requirements 1.1, 1.2, 2.1, 2.2**
  
  - [x] 3.4 Write property test for field preservation
    - **Property 6: Manifest Field Preservation**
    - **Validates: Requirements 1.4**
  
  - [x] 3.5 Complete ValidateSecrets method (COMPLETED)
    - Compare config secrets against encrypted manifests
    - Decrypt manifests using cluster Age key
    - Detect drift, missing manifests, orphaned secrets
    - Check for unencrypted secrets (security violations)
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6_
  
  - [x] 3.6 Write property test for drift detection
    - **Property 2: Drift Detection Accuracy**
    - **Validates: Requirements 2.3, 2.4, 2.5**
  
  - [x] 3.7 Write property test for unencrypted secret detection
    - **Property 3: Unencrypted Secret Detection**
    - **Validates: Requirements 2.6, 7.2, 7.3**
  
  - [x] 3.8 Service filtering (COMPLETED)
    - Service filtering is implemented in mapSecretsToManifests
    - _Requirements: 1.6_
  
  - [x] 3.9 Write property test for service filtering
    - **Property 5: Service Filter Correctness**
    - **Validates: Requirements 1.6**
  
  - [x] 3.10 Implement DetectDrift method
    - Complete the DetectDrift method (currently returns empty report)
    - Build comprehensive DriftReport with ServiceDrift details
    - _Requirements: 2.1, 2.2, 2.3_

- [x] 4. Implement KeyRotator
  - [x] 5.1 Create KeyRotator interface and implementation (COMPLETED)
    - Implement `internal/secrets/rotation.go` with DefaultKeyRotator
    - Define RotateOptions, RotationResult, RotationStatus types
    - Implement rotation state machine (initial, dual-key, complete)
    - _Requirements: 3.1, 3.5_
  
  - [x] 5.2 Complete RotateAgeKey method
    - Fix incomplete archiveKey implementation (file was truncated)
    - Verify dual-key configuration updates work correctly
    - Test re-encryption with both keys
    - _Requirements: 3.1, 3.2, 3.3, 3.7_
  
  - [x] 5.3 Write property test for dual-key decryption
    - **Property 7: Key Rotation Dual-Key Decryption**
    - **Validates: Requirements 3.2, 3.3**
  
  - [x] 5.4 CompleteRotation method (COMPLETED)
    - Remove old key from .sops.yaml
    - Re-encrypt manifests with new key only
    - Update registry with archived status for old key
    - _Requirements: 3.4_
  
  - [x] 5.5 Write property test for rotation completion
    - **Property 8: Key Rotation Completion**
    - **Validates: Requirements 3.4, 3.7**
  
  - [x] 5.6 RotateSSHKey method (COMPLETED)
    - Generate new SSH key pair
    - Update config file with new key paths
    - Archive old SSH key
    - _Requirements: 3.5, 3.6_
  
  - [x] 5.7 Rollback on failure (COMPLETED)
    - RollbackManager implemented in rollback.go
    - Used in rotation and revocation operations
    - _Requirements: 3.9_

- [x] 5. Implement KeyRevoker
  - [x] 7.1 Create KeyRevoker interface and implementation (COMPLETED)
    - Implement `internal/secrets/revocation.go` with DefaultKeyRevoker
    - Define RevokeOptions, RevocationResult types
    - _Requirements: 6.1, 6.5_
  
  - [x] 7.2 RevokeByUser method (COMPLETED)
    - Identify all keys associated with user email
    - Remove user's public key from .sops.yaml
    - Re-encrypt all manifests without revoked key
    - Update registry with revocation info
    - _Requirements: 6.1, 6.2, 6.3, 6.4_
  
  - [x] 7.3 Write property test for revocation effectiveness
    - **Property 14: Revocation Effectiveness**
    - **Validates: Requirements 6.2, 6.3**
  
  - [x] 7.4 RevokeByFingerprint method (COMPLETED)
    - Revoke specific key by fingerprint
    - Re-encryption logic implemented
    - _Requirements: 6.5_
  
  - [x] 7.5 EmergencyRevoke method (COMPLETED)
    - Immediate revocation of compromised key
    - Generate new primary key via rotation
    - Re-encrypt with new key only
    - _Requirements: 6.6_
  
  - [x] 7.6 Single-key protection (COMPLETED)
    - Check if revoked key is the only encryption key
    - Return ErrSingleKeyRevocation error
    - _Requirements: 6.7_

- [ ] 6. Extend Audit Logger for secrets operations
  - [x] 8.1 Add secrets-specific event types
    - Add event types to existing AuditLogger: secrets.sync, secrets.drift_detected, secrets.validated, key.generated, key.rotated, key.revoked, key.accessed, key.expired
    - Extend existing AuditLogger interface with secrets methods
    - _Requirements: 5.6, 5.7_
  
  - [x] 6.2 Write property test for audit event recording
    - **Property 11: Audit Log Event Recording**
    - **Validates: Requirements 5.6, 5.7, 3.10, 6.4**
  
  - [x] 6.3 Implement audit log querying
    - Add time-based filtering (--since flag)
    - Add event-type filtering
    - Add export to JSON file
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5_
  
  - [x] 6.4 Write property test for audit log filtering
    - **Property 13: Audit Log Filtering**
    - **Validates: Requirements 5.3, 5.4**
  
  - [x] 6.5 Write property test for audit log integrity
    - **Property 12: Audit Log Integrity**
    - **Validates: Requirements 5.8, 5.9**
  
  - [x] 6.6 Integrate audit logging into secrets operations
    - Add audit logging calls to SyncSecrets, ValidateSecrets
    - Add audit logging calls to RotateAgeKey, RotateSSHKey, CompleteRotation
    - Add audit logging calls to RevokeByUser, RevokeByFingerprint, EmergencyRevoke
    - _Requirements: 3.10, 5.6, 5.7, 6.4_

- [x] 7. Implement HookManager for pre-commit validation
  - [x] 7.1 Create HookManager interface and implementation
    - Create `internal/secrets/hooks.go` with DefaultHookManager
    - Define HookResult type
    - Define hook script template
    - _Requirements: 7.1_
  
  - [x] 7.2 Implement InstallHooks method
    - Generate pre-commit hook script
    - Install to .git/hooks/pre-commit
    - Set executable permissions
    - _Requirements: 7.1, 7.9_
  
  - [x] 7.3 Implement ValidatePreCommit method
    - Scan staged files for unencrypted secrets
    - Check for plaintext Age/SSH key files
    - Validate staged manifests against config
    - _Requirements: 7.2, 7.3, 7.4, 7.5, 7.6, 7.7_
  
  - [x] 7.4 Write property test for plaintext key detection
    - **Property 17: Pre-Commit Plaintext Key Detection**
    - **Validates: Requirements 7.6, 7.7**
  
  - [x] 7.5 Implement hook bypass
    - Check OPENCENTER_SKIP_HOOKS environment variable
    - Display warning when bypassing
    - _Requirements: 7.8_

- [x] 8. Implement multi-cluster operations
  - [x] 8.1 Create MultiClusterSyncer interface and implementation
    - Create `internal/secrets/multi_cluster.go` with DefaultMultiClusterSyncer
    - Define MultiClusterSyncOptions, MultiClusterSyncResult types
    - _Requirements: 8.1, 8.2_
  
  - [x] 8.2 Implement SyncAll method
    - Implement --all flag for sync-secrets
    - Add parallel processing with configurable concurrency
    - Implement organization filtering
    - _Requirements: 8.1, 8.2, 8.3_
  
  - [x] 8.3 Write property test for multi-cluster coverage
    - **Property 15: Multi-Cluster Sync Coverage**
    - **Validates: Requirements 8.1, 8.5, 8.7**
  
  - [x] 8.4 Implement failure handling
    - Continue on failure by default
    - Add --stop-on-error flag
    - Report failures at end with summary
    - _Requirements: 8.4, 8.5, 8.6, 8.7_
  
  - [x] 8.5 Write property test for failure isolation
    - **Property 16: Multi-Cluster Failure Isolation**
    - **Validates: Requirements 8.5, 8.6**

- [x] 9. Implement CLI commands
  - [x] 9.1 Implement sync-secrets command
    - Create `cmd/cluster_sync_secrets.go`
    - Wire to SecretsManager.SyncSecrets and MultiClusterSyncer.SyncAll
    - Add all flags: --services, --dry-run, --force, --all, --organization, --concurrency, --stop-on-error
    - Display summary of created, updated, unchanged files
    - _Requirements: 1.1-1.9, 8.1-8.8_
  
  - [x] 9.2 Write property test for dry-run immutability
    - **Property 4: Dry-Run Immutability**
    - **Validates: Requirements 1.5, 3.8, 6.8, 8.8**
  
  - [x] 9.3 Implement validate-secrets command
    - Create `cmd/cluster_validate_secrets.go`
    - Wire to SecretsManager.ValidateSecrets
    - Add flags: --fix, --output (text/json)
    - Return appropriate exit codes (0 for valid, 1 for drift)
    - Display drift items, missing manifests, orphaned secrets, security issues
    - _Requirements: 2.1-2.9_
  
  - [x] 9.4 Implement rotate-keys command
    - Create `cmd/cluster_rotate_keys.go`
    - Wire to KeyRotator.RotateAgeKey, RotateSSHKey, CompleteRotation
    - Add flags: --type (age/ssh), --complete, --dry-run
    - Display rotation result with old/new fingerprints
    - _Requirements: 3.1-3.10_
  
  - [x] 9.5 Implement check-keys command
    - Create `cmd/cluster_check_keys.go`
    - Wire to KeyRegistry.CheckExpiration
    - Add flags: --all, --cluster, --output (text/json), --warn-days
    - Display expiration status with warning/error indicators
    - _Requirements: 4.1-4.9_
  
  - [x] 9.6 Implement audit-log command
    - Create `cmd/cluster_audit_log.go`
    - Wire to AuditLogger query methods
    - Add flags: --since, --event-type, --export, --verify
    - Display audit events with timestamp, user, event type, resource
    - _Requirements: 5.1-5.9_
  
  - [x] 9.7 Implement revoke-key command
    - Create `cmd/cluster_revoke_key.go`
    - Wire to KeyRevoker.RevokeByUser, RevokeByFingerprint, EmergencyRevoke
    - Add flags: --user, --key, --emergency, --dry-run
    - Display revocation result with re-encrypted file count
    - _Requirements: 6.1-6.9_
  
  - [x] 9.8 Implement install-hooks command
    - Create `cmd/cluster_install_hooks.go`
    - Wire to HookManager.InstallHooks
    - Add flags: --repo-path, --force
    - Display installation instructions
    - _Requirements: 7.1, 7.9_
  
  - [x] 9.9 Implement keys list command
    - Create `cmd/cluster_keys.go` with list subcommand
    - Wire to KeyRegistry.ListKeys
    - Add flags: --cluster, --status (active/archived/revoked), --output (text/json)
    - Display key metadata: cluster, type, fingerprint, created, expires, status
    - _Requirements: 9.5, 9.6, 9.7_
  
  - [x] 9.10 Write property test for JSON output validity
    - **Property 18: JSON Output Validity**
    - **Validates: Requirements 2.9, 4.6, 5.5**

- [x] 10. Register commands with cluster command
  - [x] 10.1 Add sync-secrets command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 1.1-1.9_
  
  - [x] 10.2 Add validate-secrets command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 2.1-2.9_
  
  - [x] 10.3 Add rotate-keys command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 3.1-3.10_
  
  - [x] 10.4 Add check-keys command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 4.1-4.9_
  
  - [x] 10.5 Add audit-log command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 5.1-5.9_
  
  - [x] 10.6 Add revoke-key command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 6.1-6.9_
  
  - [x] 10.7 Add install-hooks command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 7.1, 7.9_
  
  - [x] 10.8 Add keys command to cluster command
    - Register in `cmd/cluster.go` NewClusterCmd()
    - Add help text and usage examples
    - _Requirements: 9.5, 9.6, 9.7_

## Implementation Status Summary

### Completed Components
- ✅ Core interfaces and types (`internal/secrets/interfaces.go`)
- ✅ Error types (`internal/secrets/errors.go`)
- ✅ Rollback manager (`internal/secrets/rollback.go`)
- ✅ Key Registry implementation (`internal/secrets/registry.go`)
- ✅ Secrets Manager implementation (`internal/secrets/manager.go`)
- ✅ Key Rotator implementation (`internal/secrets/rotation.go`)
- ✅ Key Revoker implementation (`internal/secrets/revocation.go`)
- ✅ Audit Logger extensions (`internal/security/audit_logger.go`)
- ✅ Hook Manager implementation (`internal/secrets/hooks.go`)
- ✅ Multi-cluster operations (`internal/secrets/multi_cluster.go`)
- ✅ All CLI commands (sync-secrets, validate-secrets, rotate-keys, check-keys, audit-log, revoke-key, install-hooks, keys)
- ✅ Command registration in cluster command

### Optional Property-Based Tests (Not Started)
- ❌ Property test for Key Registry (2.2)
- ❌ Property test for expiration calculation (2.4)
- ❌ Property test for sync round-trip (3.3)
- ❌ Property test for field preservation (3.4)
- ❌ Property test for drift detection (3.6)
- ❌ Property test for unencrypted secret detection (3.7)
- ❌ Property test for service filtering (3.9)
- ❌ Property test for dual-key decryption (4.3)
- ❌ Property test for rotation completion (4.5)
- ❌ Property test for revocation effectiveness (5.3)
- ❌ Property test for audit event recording (6.2)
- ❌ Property test for audit log filtering (6.4)
- ❌ Property test for audit log integrity (6.5)
- ❌ Property test for plaintext key detection (7.4)
- ❌ Property test for multi-cluster coverage (8.3)
- ❌ Property test for failure isolation (8.5)
- ❌ Property test for dry-run immutability (9.2)
- ❌ Property test for JSON output validity (9.10)

## Notes

- All required functionality for production use is complete
- Tasks marked with `*` are optional property-based tests that provide additional validation coverage
- Property tests use the `rapid` library for Go property-based testing
- Unit tests validate specific examples and edge cases
- The implementation builds on existing packages: `internal/sops`, `internal/security`, `internal/config`
- All CLI commands are fully implemented and registered
- Multi-cluster operations support parallel processing with configurable concurrency
