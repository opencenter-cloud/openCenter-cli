# Requirements Document

## Introduction

This document specifies requirements for multi-cluster secrets management improvements to the openCenter-cli. The feature addresses critical challenges in secrets lifecycle management: configuration drift between source-of-truth config files and encrypted manifests, manual encryption burden, plaintext keys in Git, lack of key rotation strategy, missing key expiration tracking, no access audit trail, and no key revocation process.

The solution provides CLI commands for secrets synchronization, drift detection, automated key rotation, expiration monitoring, audit logging, and access revocation across multiple Kubernetes clusters.

## Glossary

- **Config_File**: The cluster configuration file (`.k8s-<cluster>-config.yaml`) that serves as the source of truth for all secrets
- **Manifest**: Kubernetes YAML files in `applications/overlays/<cluster>/services/*/` that contain SOPS-encrypted secrets
- **SOPS**: Mozilla's Secrets OPerationS tool used for encrypting/decrypting secrets in YAML files
- **Age_Key**: An encryption key pair used by SOPS for encrypting secrets (public key for encryption, private key for decryption)
- **SSH_Key**: SSH key pair used for cluster node access and GitOps repository authentication
- **Key_Registry**: A metadata file tracking key creation dates, expiration dates, fingerprints, and usage
- **Drift**: A state where secrets in the Config_File differ from secrets in the Manifests
- **Secrets_Manager**: The CLI component responsible for secrets synchronization, validation, and key lifecycle management
- **Audit_Log**: An append-only log recording all key access, rotation, and revocation events
- **Key_Rotation**: The process of generating new keys, re-encrypting secrets, and archiving old keys
- **Revocation**: The process of removing a user's key from encryption recipients and re-encrypting all secrets

## Requirements

### Requirement 1: Secrets Synchronization

**User Story:** As a platform operator, I want to regenerate all encrypted manifests from the config file, so that I can ensure deployed secrets match my source of truth.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster sync-secrets <cluster>`, THE Secrets_Manager SHALL read all secrets from the Config_File
2. WHEN secrets are read from the Config_File, THE Secrets_Manager SHALL generate corresponding encrypted Manifests for each service
3. WHEN generating Manifests, THE Secrets_Manager SHALL use the cluster's Age_Key from the Key_Registry
4. WHEN a Manifest already exists, THE Secrets_Manager SHALL update it with the new encrypted values while preserving non-secret fields
5. WHEN the `--dry-run` flag is provided, THE Secrets_Manager SHALL display what changes would be made without modifying any files
6. WHEN the `--services` flag is provided with a comma-separated list, THE Secrets_Manager SHALL only sync secrets for the specified services
7. IF the Config_File does not exist, THEN THE Secrets_Manager SHALL return an error with the expected file path
8. IF the Age_Key is not found, THEN THE Secrets_Manager SHALL return an error instructing the user to initialize keys first
9. WHEN sync completes successfully, THE Secrets_Manager SHALL display a summary of files created, updated, and unchanged

### Requirement 2: Drift Detection and Validation

**User Story:** As a platform operator, I want to detect when config file secrets differ from deployed manifests, so that I can identify and fix configuration drift before it causes deployment issues.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster validate-secrets <cluster>`, THE Secrets_Manager SHALL compare secrets in the Config_File against encrypted Manifests
2. WHEN comparing secrets, THE Secrets_Manager SHALL decrypt each Manifest using the cluster's Age_Key
3. WHEN a secret in the Config_File differs from the corresponding Manifest, THE Secrets_Manager SHALL report the drift with the service name and field path
4. WHEN a secret exists in the Config_File but not in any Manifest, THE Secrets_Manager SHALL report it as a missing manifest
5. WHEN a secret exists in a Manifest but not in the Config_File, THE Secrets_Manager SHALL report it as an orphaned secret
6. WHEN a Manifest contains unencrypted secrets, THE Secrets_Manager SHALL report it as a security violation
7. WHEN the `--fix` flag is provided, THE Secrets_Manager SHALL automatically run sync-secrets to resolve detected drift
8. WHEN validation completes, THE Secrets_Manager SHALL return exit code 0 if no drift is detected, or exit code 1 if drift exists
9. WHEN the `--output json` flag is provided, THE Secrets_Manager SHALL output drift report in JSON format for CI/CD integration

### Requirement 3: Key Rotation

**User Story:** As a security administrator, I want to rotate encryption keys on a schedule, so that I can limit the exposure window if keys are compromised.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster rotate-keys <cluster> --type age`, THE Secrets_Manager SHALL generate a new Age_Key pair
2. WHEN a new Age_Key is generated, THE Secrets_Manager SHALL add the new public key to the `.sops.yaml` configuration alongside the old key
3. WHEN dual-key configuration is active, THE Secrets_Manager SHALL re-encrypt all Manifests with both old and new keys
4. WHEN the `--complete` flag is provided after dual-key re-encryption, THE Secrets_Manager SHALL remove the old key from `.sops.yaml` and re-encrypt with only the new key
5. WHEN a user runs `opencenter cluster rotate-keys <cluster> --type ssh`, THE Secrets_Manager SHALL generate a new SSH_Key pair
6. WHEN a new SSH_Key is generated, THE Secrets_Manager SHALL update the Config_File with the new key paths
7. WHEN key rotation completes, THE Secrets_Manager SHALL archive the old key with a timestamp in `secrets/archive/`
8. WHEN the `--dry-run` flag is provided, THE Secrets_Manager SHALL display the rotation plan without making changes
9. IF re-encryption fails for any Manifest, THEN THE Secrets_Manager SHALL rollback all changes and report the failure
10. WHEN rotation completes successfully, THE Secrets_Manager SHALL log the rotation event to the Audit_Log

### Requirement 4: Key Expiration Tracking

**User Story:** As a security administrator, I want to track when keys were created and when they should expire, so that I can proactively rotate keys before they become a security risk.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster check-keys --all`, THE Secrets_Manager SHALL display expiration status for all clusters' keys
2. WHEN displaying key status, THE Secrets_Manager SHALL show days until expiration for each Age_Key and SSH_Key
3. WHEN a key is within 14 days of expiration, THE Secrets_Manager SHALL display a warning indicator
4. WHEN a key has expired, THE Secrets_Manager SHALL display an error indicator and recommend immediate rotation
5. WHEN the `--cluster <cluster>` flag is provided, THE Secrets_Manager SHALL only check keys for the specified cluster
6. WHEN the `--output json` flag is provided, THE Secrets_Manager SHALL output key status in JSON format
7. WHEN a new key is generated, THE Secrets_Manager SHALL record the creation timestamp and calculated expiration date in the Key_Registry
8. THE Key_Registry SHALL store key metadata including fingerprint, creation date, expiration date, and status
9. WHEN the Key_Registry does not exist, THE Secrets_Manager SHALL create it with default expiration policies (90 days for Age_Key, 180 days for SSH_Key)

### Requirement 5: Audit Logging

**User Story:** As a compliance officer, I want to track all key access and modifications, so that I can investigate security incidents and demonstrate compliance.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster audit-log <cluster>`, THE Secrets_Manager SHALL display recent key access events for the cluster
2. WHEN displaying audit events, THE Secrets_Manager SHALL show timestamp, user, event type, and affected resource
3. WHEN the `--since <duration>` flag is provided, THE Secrets_Manager SHALL filter events to the specified time window
4. WHEN the `--event-type <type>` flag is provided, THE Secrets_Manager SHALL filter events by type (key_access, key_rotation, key_revocation, decrypt_secret)
5. WHEN the `--export <file>` flag is provided, THE Secrets_Manager SHALL export the audit log to the specified file in JSON format
6. WHEN any key operation occurs (generate, rotate, revoke, access), THE Secrets_Manager SHALL append an event to the Audit_Log
7. WHEN logging an event, THE Secrets_Manager SHALL record timestamp, actor (user email or system), event type, key fingerprint, cluster, and IP address
8. THE Audit_Log SHALL be stored in an append-only format with cryptographic signatures to prevent tampering
9. WHEN the `--verify` flag is provided, THE Secrets_Manager SHALL verify the integrity of the Audit_Log signatures

### Requirement 6: Key Revocation

**User Story:** As a security administrator, I want to revoke access for departed team members or compromised keys, so that I can prevent unauthorized access to cluster secrets.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster revoke-key <cluster> --user <email>`, THE Secrets_Manager SHALL identify all keys associated with the user
2. WHEN revoking a user's key, THE Secrets_Manager SHALL remove the user's Age_Key public key from `.sops.yaml`
3. WHEN a key is removed from `.sops.yaml`, THE Secrets_Manager SHALL re-encrypt all Manifests without the revoked key
4. WHEN revocation completes, THE Secrets_Manager SHALL log the revocation event to the Audit_Log with the revoked user's identity
5. WHEN the `--key <fingerprint>` flag is provided instead of `--user`, THE Secrets_Manager SHALL revoke the specific key by fingerprint
6. WHEN the `--emergency` flag is provided, THE Secrets_Manager SHALL perform immediate revocation and generate a new primary key
7. IF the revoked key is the only encryption key, THEN THE Secrets_Manager SHALL return an error requiring a new key to be added first
8. WHEN the `--dry-run` flag is provided, THE Secrets_Manager SHALL display what would be revoked without making changes
9. WHEN revocation completes successfully, THE Secrets_Manager SHALL display confirmation with the number of re-encrypted files

### Requirement 7: Pre-Commit Validation Hook

**User Story:** As a platform operator, I want automatic validation before commits, so that I can prevent plaintext secrets and configuration drift from being pushed to Git.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster install-hooks <cluster>`, THE Secrets_Manager SHALL install a pre-commit hook in the GitOps repository
2. WHEN the pre-commit hook runs, THE Secrets_Manager SHALL scan staged files for unencrypted secrets
3. WHEN an unencrypted secret is detected in a staged file, THE Secrets_Manager SHALL block the commit and display the file path
4. WHEN the pre-commit hook runs, THE Secrets_Manager SHALL validate that staged Manifests match the Config_File
5. WHEN drift is detected during pre-commit, THE Secrets_Manager SHALL block the commit and suggest running sync-secrets
6. WHEN the pre-commit hook runs, THE Secrets_Manager SHALL check for plaintext Age_Key or SSH_Key files in staged changes
7. IF plaintext keys are staged, THEN THE Secrets_Manager SHALL block the commit and display a security warning
8. WHEN the `--skip-hooks` environment variable is set, THE Secrets_Manager SHALL bypass pre-commit validation with a warning
9. WHEN hook installation completes, THE Secrets_Manager SHALL display instructions for hook usage and bypass options

### Requirement 8: Multi-Cluster Secrets Synchronization

**User Story:** As a platform operator managing multiple clusters, I want to sync secrets across all clusters in one command, so that I can maintain consistency without repeating operations.

#### Acceptance Criteria

1. WHEN a user runs `opencenter cluster sync-secrets --all`, THE Secrets_Manager SHALL sync secrets for all clusters in the organization
2. WHEN syncing multiple clusters, THE Secrets_Manager SHALL process clusters in parallel with a configurable concurrency limit
3. WHEN the `--organization <org>` flag is provided, THE Secrets_Manager SHALL only sync clusters belonging to the specified organization
4. WHEN syncing multiple clusters, THE Secrets_Manager SHALL display progress for each cluster
5. WHEN any cluster sync fails, THE Secrets_Manager SHALL continue with remaining clusters and report failures at the end
6. WHEN the `--stop-on-error` flag is provided, THE Secrets_Manager SHALL stop processing on the first failure
7. WHEN multi-cluster sync completes, THE Secrets_Manager SHALL display a summary showing success/failure count per cluster
8. WHEN the `--dry-run` flag is provided with `--all`, THE Secrets_Manager SHALL display planned changes for all clusters without modifications

### Requirement 9: Key Registry Management

**User Story:** As a platform operator, I want to manage key metadata centrally, so that I can track key lifecycle across all clusters.

#### Acceptance Criteria

1. THE Key_Registry SHALL be stored as a SOPS-encrypted YAML file at `secrets/key-registry.yaml`
2. WHEN a new key is generated, THE Secrets_Manager SHALL add an entry to the Key_Registry with fingerprint, creation date, expiration date, and status
3. WHEN a key is rotated, THE Secrets_Manager SHALL update the Key_Registry with the new key and mark the old key as archived
4. WHEN a key is revoked, THE Secrets_Manager SHALL update the Key_Registry with revocation timestamp and reason
5. WHEN a user runs `opencenter cluster keys list`, THE Secrets_Manager SHALL display all keys from the Key_Registry with their status
6. WHEN the `--cluster <cluster>` flag is provided to keys list, THE Secrets_Manager SHALL filter to keys for the specified cluster
7. THE Key_Registry SHALL support multiple keys per cluster for multi-recipient encryption scenarios
8. WHEN the Key_Registry is corrupted or missing, THE Secrets_Manager SHALL offer to rebuild it from existing key files
