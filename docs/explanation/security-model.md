---
title: Security Model
doc_type: explanation
description: Security architecture and design principles in openCenter
keywords: [security, encryption, sops, age, secrets, audit, compliance]
related:
  - ../reference/sops-commands.md
  - ../how-to/manage-secrets.md
  - ../operations/security-hardening.md
---

# Security Model


## Table of Contents

- [Overview](#overview)
- [Core Security Principles](#core-security-principles)
- [SOPS Integration](#sops-integration)
- [Credential Handling](#credential-handling)
- [SSH Key Management](#ssh-key-management)
- [Security Boundaries](#security-boundaries)
- [Compliance Considerations](#compliance-considerations)
- [Trade-offs](#trade-offs)
- [Best Practices](#best-practices)
- [Future Enhancements](#future-enhancements)
- [Related Documentation](#related-documentation)
- [Conclusion](#conclusion)
## Overview

openCenter implements a defense-in-depth security model designed to protect sensitive data throughout the cluster lifecycle—from initial configuration through deployment and ongoing operations. The security architecture is built on the principle that **no plaintext secrets should ever be committed to version control**, while maintaining usability for DevOps workflows.

This document explains the security principles, threat model, encryption mechanisms, and compliance considerations that guide openCenter's design.

## Core Security Principles

### 1. No Plaintext Secrets in Git

**Principle**: All sensitive data must be encrypted before being committed to version control.

**Implementation**:
- Automatic SOPS encryption for all secret files in GitOps repositories
- Age encryption with per-cluster keys
- `.sops.yaml` configuration files define encryption rules
- Pre-commit validation to prevent plaintext secret leaks

**Rationale**: Git repositories are often the weakest link in infrastructure security. Even private repositories can be compromised through credential theft, insider threats, or misconfigured access controls. By encrypting secrets at rest, we ensure that repository compromise doesn't immediately lead to credential exposure.

### 2. Encryption at Rest

**Principle**: Sensitive data is encrypted when stored on disk or in repositories.

**Implementation**:
- SOPS (Secrets OPerationS) for file-level encryption
- Age encryption algorithm (modern, secure alternative to GPG)
- Per-cluster encryption keys stored separately from encrypted data
- Support for hardware security modules (HSMs) via Barbican integration

**Key Storage Locations**:
```
~/.config/openCenter/clusters/<organization>/<cluster>/secrets/age/keys/
```

**Encrypted File Patterns**:
- `flux-system/gotk-sync.yaml` - FluxCD Git credentials
- `secrets/openstack-credentials.yaml` - Cloud provider credentials
- `secrets/vsphere-credentials.yaml` - VMware credentials
- Any file matching `encrypted_regex` in `.sops.yaml`

### 3. Least Privilege Access

**Principle**: Users and systems should have only the minimum permissions necessary to perform their functions.

**Implementation**:
- Separate Age keys per cluster (no shared keys across environments)
- Role-based access control (RBAC) for Kubernetes resources
- Service-specific credentials with limited scope
- Audit logging of all key access and secret operations

**Example**: The cert-manager service receives AWS credentials scoped only to Route53 DNS validation, not full AWS account access.

### 4. Audit Logging

**Principle**: All security-relevant events must be logged with tamper-evident integrity protection.

**Implementation**:
- HMAC-signed audit logs for integrity verification
- Structured event logging with correlation IDs
- Automatic log rotation and retention policies
- Queryable audit trail for compliance reporting

**Logged Events**:
- Key generation and rotation
- Secret access and decryption
- Validation failures
- Input rejection (potential attacks)
- Template rendering with sensitive data

**Audit Log Location**:
```
~/.config/openCenter/audit/audit.log
```

### 5. Input Validation

**Principle**: All user-controlled input must be validated and sanitized to prevent injection attacks.

**Implementation**:
- Cluster name validation (alphanumeric, hyphens, underscores, dots only)
- Path traversal prevention (reject `..` sequences)
- URL validation (require HTTPS for external URLs)
- Command sanitization (prevent shell injection)
- Editor whitelist (prevent command injection via `$EDITOR`)

**Validation Rules**:
```go
// Cluster names must match this pattern
^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$

// Rejected patterns
- Path separators: / \
- Path traversal: ..
- Shell metacharacters: ; | & $ ` < > ( ) { }
```

## SOPS Integration

### Why Age Over GPG?

openCenter uses Age encryption instead of GPG for several compelling reasons:

| Aspect | Age | GPG |
|--------|-----|-----|
| **Key Format** | Single-line text | Multi-line PEM blocks |
| **Key Generation** | Instant | Requires entropy gathering |
| **Complexity** | Minimal API surface | Large, complex codebase |
| **Modern Cryptography** | X25519 + ChaCha20-Poly1305 | Various algorithms (some legacy) |
| **Usability** | Simple CLI, no configuration | Complex trust model, keyservers |
| **Security Audits** | Modern, focused codebase | Large attack surface |

**Age Key Format**:
```
# Private key (AGE-SECRET-KEY- prefix)
AGE-SECRET-KEY-1XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX

# Public key (age1 prefix)
age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Encryption Workflow

1. **Key Generation** (per cluster):
   ```bash
   mise run cluster init <cluster-name>
   # Generates Age key pair automatically
   ```

2. **SOPS Configuration** (`.sops.yaml`):
   ```yaml
   creation_rules:
     - path_regex: .*\.(yaml|yml)$
       age: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
       encrypted_regex: '^(data|stringData|password|token|key|secret|credentials)'
   ```

3. **Automatic Encryption**:
   - openCenter encrypts files during `cluster setup`
   - Only specified fields are encrypted (preserves YAML structure)
   - Metadata remains plaintext for GitOps tooling

4. **Decryption** (runtime):
   - FluxCD uses SOPS decryption provider
   - Age private key stored as Kubernetes Secret
   - Secrets decrypted in-memory, never written to disk

### Key Rotation Procedures

**When to Rotate**:
- Suspected key compromise
- Personnel changes (employee departure)
- Compliance requirements (e.g., annual rotation)
- After security incident

**Rotation Process**:
```bash
# 1. Generate new Age key
mise run sops generate-key --cluster <cluster-name> --rotate

# 2. Re-encrypt all secrets with new key
mise run sops rotate-keys --cluster <cluster-name>

# 3. Update FluxCD secret in cluster
kubectl create secret generic sops-age \
  --from-file=age.agekey=<new-key-file> \
  --namespace=flux-system \
  --dry-run=client -o yaml | kubectl apply -f -

# 4. Verify decryption works
mise run sops decrypt <encrypted-file>
```

**Automated Rotation** (future enhancement):
- Scheduled key rotation via CronJob
- Automatic re-encryption of all secrets
- Zero-downtime rotation with dual-key support

## Credential Handling

### Credential Masking in Logs

**Problem**: Secrets can leak through log files, error messages, and command output.

**Solution**: Automatic credential masking using pattern-based detection.

**Masked Patterns**:
- AWS Access Keys: `AKIA[A-Z0-9]{16}` → `AKIA****XXXX`
- AWS Secret Keys: 40-character base64 strings → `***MASKED***`
- Age Secret Keys: `AGE-SECRET-KEY-*` → `AGE-SECRET-KEY-****`
- Passwords: `password=<value>` → `password=***MASKED***`
- Tokens: `token=<value>` → `token=***MASKED***`
- Private Keys: PEM blocks → `-----BEGIN PRIVATE KEY-----\n***MASKED***\n-----END PRIVATE KEY-----`

**Implementation** (`internal/security/credential_masker.go`):
```go
masker := security.NewDefaultCredentialMasker()
safeOutput := masker.MaskString(unsafeOutput)
fmt.Println(safeOutput) // Credentials automatically masked
```

**Masking Strategy**:
- Preserve first 4 and last 4 characters of AWS keys (for debugging)
- Completely mask passwords and tokens
- Preserve key type prefixes (e.g., `AGE-SECRET-KEY-`)

### Command Sanitization

**Problem**: User input in shell commands can lead to command injection.

**Solution**: Parameterized command execution and input validation.

**Dangerous Patterns Rejected**:
```bash
# Shell metacharacters
; | & $ ` < > ( ) { }

# Shell invocation
sh -c "user_input"
bash -c "user_input"

# Command chaining
command1 && command2
command1 || command2
command1 ; command2
```

**Safe Execution** (`internal/security/command_sanitizer.go`):
```go
sanitizer := security.NewDefaultCommandSanitizer()

// UNSAFE: Shell invocation
cmd := exec.Command("sh", "-c", userInput) // REJECTED

// SAFE: Parameterized execution
cmd, err := sanitizer.SanitizeCommand("git", []string{"clone", userURL})
```

**Editor Validation**:
```go
// Whitelist of safe editors
safeEditors := []string{
  "vim", "vi", "nvim", "nano", "emacs",
  "code", "subl", "atom", "gedit"
}

// Reject unknown editors or editors with shell metacharacters
if err := sanitizer.ValidateEditor(os.Getenv("EDITOR")); err != nil {
  return fmt.Errorf("unsafe editor: %w", err)
}
```

### Input Validation

**Validation Layers**:

1. **Cluster Name Validation**:
   ```go
   // Must start with alphanumeric, max 63 chars
   // Allowed: a-z A-Z 0-9 . - _
   // Rejected: / \ .. (path traversal)
   ```

2. **Path Validation**:
   ```go
   // Reject path traversal sequences
   if strings.Contains(path, "..") {
     return errors.New("path traversal detected")
   }
   ```

3. **URL Validation**:
   ```go
   // External URLs must use HTTPS
   // Local URLs (localhost, 127.0.0.1, RFC1918) can use HTTP
   ```

4. **Environment Variable Validation**:
   ```go
   // Special handling for EDITOR variable
   // Reject shell metacharacters in all variables
   ```

**Audit Integration**:
```go
validator := security.NewDefaultInputValidator()
validator.SetAuditLogger(auditLogger)
validator.SetActor("user@example.com")

if err := validator.ValidateClusterName(name); err != nil {
  // Rejection automatically logged to audit trail
  return err
}
```

## SSH Key Management

### Key Generation

**Supported Algorithms**:
- **Ed25519** (recommended): Modern, fast, secure
- **RSA 4096**: Traditional, widely compatible
- **ECDSA P-521**: Elliptic curve, high security

**Generation** (`internal/util/crypto/ssh_key_generator.go`):
```go
keyPair, err := crypto.GenerateSSHKey("ed25519")
// Returns passwordless SSH key pair
```

**Key Storage**:
```
<gitops-repo>/<cluster>/secrets/ssh/<cluster>-<env>-<region>
<gitops-repo>/<cluster>/secrets/ssh/<cluster>-<env>-<region>.pub
```

**Permissions**:
- Private key: `0600` (owner read/write only)
- Public key: `0644` (world-readable)

### Key Usage

**Purpose**: SSH keys are used for:
1. Git repository access (FluxCD)
2. Cluster node access (Ansible, Kubespray)
3. Bastion host authentication

**Security Considerations**:
- Keys are generated per cluster (no shared keys)
- Private keys are encrypted with SOPS before Git commit
- Keys are rotated when clusters are destroyed
- Passphrase protection is optional (not recommended for automation)

### Key Rotation

**Rotation Triggers**:
- Cluster rebuild or migration
- Security incident or suspected compromise
- Compliance requirements

**Rotation Process**:
1. Generate new SSH key pair
2. Update authorized_keys on all cluster nodes
3. Update Git repository deploy keys
4. Update FluxCD secret
5. Revoke old key from all systems
6. Delete old key files

## Security Boundaries

### What's Encrypted vs. Plaintext

| Data Type | Storage | Encryption | Rationale |
|-----------|---------|------------|-----------|
| **Cluster configuration** | Git | Plaintext | Non-sensitive metadata |
| **Cloud credentials** | Git | SOPS encrypted | Highly sensitive |
| **SSH private keys** | Git | SOPS encrypted | Authentication credentials |
| **TLS certificates** | Git | SOPS encrypted | Security credentials |
| **Kubernetes manifests** | Git | Plaintext | Non-sensitive configuration |
| **Secret data fields** | Git | SOPS encrypted | Sensitive application data |
| **Age private keys** | Local filesystem | Plaintext (0600 perms) | Encryption keys (never in Git) |
| **Audit logs** | Local filesystem | HMAC-signed | Integrity protection |

### Trust Model

**Trusted Components**:
1. **User's local machine**: Where Age keys are stored
2. **Git repository**: Stores encrypted secrets (but not keys)
3. **Kubernetes cluster**: Decrypts secrets at runtime
4. **FluxCD**: Trusted to decrypt and apply secrets

**Untrusted Components**:
1. **Git hosting provider**: Cannot decrypt secrets (no keys)
2. **CI/CD systems**: Should not have access to Age keys
3. **Backup systems**: Encrypted secrets are safe to backup
4. **Log aggregation**: Credentials are masked before logging

**Key Insight**: The Age private key is the root of trust. Compromise of this key allows decryption of all cluster secrets. Therefore:
- Age keys are never committed to Git
- Age keys are stored with restrictive permissions (0600)
- Age keys are backed up separately from encrypted data
- Age keys are rotated on suspected compromise

### Threat Model

**Threats Mitigated**:

1. **Git Repository Compromise**:
   - **Threat**: Attacker gains read access to Git repository
   - **Mitigation**: All secrets are SOPS-encrypted; attacker cannot decrypt without Age key
   - **Residual Risk**: Metadata leakage (cluster names, service names)

2. **Credential Leakage in Logs**:
   - **Threat**: Secrets appear in application logs or error messages
   - **Mitigation**: Automatic credential masking before logging
   - **Residual Risk**: Novel credential formats may not be detected

3. **Command Injection**:
   - **Threat**: Attacker injects shell commands via user input
   - **Mitigation**: Parameterized command execution, input validation
   - **Residual Risk**: Bugs in validation logic

4. **Path Traversal**:
   - **Threat**: Attacker accesses files outside intended directories
   - **Mitigation**: Path validation, rejection of `..` sequences
   - **Residual Risk**: Symlink attacks (mitigated by permission checks)

5. **Insider Threats**:
   - **Threat**: Malicious insider with Git access steals secrets
   - **Mitigation**: Encryption prevents immediate access; audit logging detects suspicious activity
   - **Residual Risk**: Insider with Age key access can decrypt secrets

6. **Supply Chain Attacks**:
   - **Threat**: Compromised dependency injects malicious code
   - **Mitigation**: Dependency pinning, Go module checksums, minimal dependencies
   - **Residual Risk**: Zero-day vulnerabilities in dependencies

**Threats NOT Mitigated**:

1. **Runtime Memory Attacks**:
   - Secrets are decrypted in memory during use
   - Memory dumps or process inspection can expose plaintext secrets
   - **Recommendation**: Use memory encryption (Intel SGX, AMD SEV) for high-security environments

2. **Kubernetes API Server Compromise**:
   - If API server is compromised, attacker can read all Secrets
   - **Recommendation**: Enable encryption at rest for etcd, use external secret stores (Vault, AWS Secrets Manager)

3. **Age Key Theft**:
   - If attacker gains access to Age private key, all secrets can be decrypted
   - **Recommendation**: Use hardware security modules (HSMs) or Barbican for key storage

4. **Social Engineering**:
   - Attacker tricks user into revealing credentials
   - **Recommendation**: Security awareness training, multi-factor authentication

## Compliance Considerations

### SOC 2 (Service Organization Control 2)

**Relevant Controls**:

- **CC6.1 - Logical and Physical Access Controls**:
  - Age keys stored with restrictive permissions (0600)
  - Audit logging of all key access
  - Least privilege access to encryption keys

- **CC6.6 - Encryption**:
  - Secrets encrypted at rest using SOPS/Age
  - TLS encryption for data in transit
  - Modern cryptographic algorithms (X25519, ChaCha20-Poly1305)

- **CC6.7 - Transmission of Data**:
  - HTTPS required for external URLs
  - SSH for Git repository access
  - TLS for Kubernetes API communication

- **CC7.2 - System Monitoring**:
  - Audit logging with HMAC integrity protection
  - Credential masking in logs
  - Queryable audit trail for incident investigation

**Evidence for Auditors**:
```bash
# Demonstrate encryption at rest
mise run sops encrypt <secret-file>

# Show audit log integrity
mise run audit verify-integrity

# Query audit events
mise run audit query --event-type key.accessed --start-date 2024-01-01
```

### HIPAA (Health Insurance Portability and Accountability Act)

**Relevant Requirements**:

- **§164.312(a)(2)(iv) - Encryption and Decryption**:
  - PHI (Protected Health Information) encrypted at rest
  - Encryption keys managed separately from encrypted data
  - Key rotation procedures documented

- **§164.312(b) - Audit Controls**:
  - Audit logging of access to PHI
  - Tamper-evident audit logs (HMAC signatures)
  - Audit log retention (30 days minimum, configurable)

- **§164.312(d) - Person or Entity Authentication**:
  - SSH key-based authentication for cluster access
  - Age key-based authentication for secret decryption
  - No shared credentials across environments

**HIPAA-Compliant Configuration**:
```yaml
# Enable audit logging
audit:
  enabled: true
  retention_days: 365  # HIPAA recommends 6 years

# Require key rotation
secrets:
  key_rotation_days: 90

# Enable encryption for all secrets
sops:
  encrypted_regex: '^(data|stringData|.*)'  # Encrypt all fields
```

### PCI-DSS (Payment Card Industry Data Security Standard)

**Relevant Requirements**:

- **Requirement 3 - Protect Stored Cardholder Data**:
  - Cardholder data encrypted using strong cryptography
  - Encryption keys stored separately from encrypted data
  - Key management procedures documented

- **Requirement 8 - Identify and Authenticate Access**:
  - Unique credentials per cluster
  - No shared or default passwords
  - Multi-factor authentication recommended (not enforced by openCenter)

- **Requirement 10 - Track and Monitor All Access**:
  - Audit logging of all access to cardholder data
  - Audit logs protected against tampering (HMAC signatures)
  - Audit log review procedures

**PCI-DSS Compliance Notes**:
- openCenter provides encryption and audit logging primitives
- Organizations must implement additional controls (network segmentation, access controls, etc.)
- Regular security assessments (QSA audits) required for PCI compliance

### GDPR (General Data Protection Regulation)

**Relevant Articles**:

- **Article 32 - Security of Processing**:
  - Encryption of personal data at rest and in transit
  - Ability to restore availability of personal data (backup/restore)
  - Regular testing of security measures

- **Article 33 - Notification of Personal Data Breach**:
  - Audit logging enables breach detection
  - Tamper-evident logs provide evidence for breach notification
  - Credential masking reduces impact of log exposure

**GDPR Compliance Features**:
- Encryption protects personal data from unauthorized access
- Audit logs provide evidence of data processing activities
- Key rotation enables "right to erasure" (delete old keys to make data unrecoverable)

## Trade-offs

### Security vs. Usability

**Encryption Overhead**:
- **Impact**: SOPS encryption/decryption adds latency to GitOps operations
- **Mitigation**: Parallel encryption for multiple files, caching of decrypted secrets
- **Acceptable**: <1 second overhead for typical secret files

**Key Management Complexity**:
- **Impact**: Users must manage Age keys separately from Git repositories
- **Mitigation**: Automatic key generation during cluster init, clear documentation
- **Acceptable**: One-time setup per cluster

**Audit Log Storage**:
- **Impact**: Audit logs consume disk space (100MB max per file, 30-day retention)
- **Mitigation**: Automatic log rotation, configurable retention policies
- **Acceptable**: ~3GB for 30 days of logs (typical usage)

### Security vs. Performance

**Credential Masking**:
- **Impact**: Regex-based masking adds CPU overhead to logging
- **Mitigation**: Compiled regex patterns, lazy evaluation
- **Acceptable**: <5% overhead on log-heavy operations

**Input Validation**:
- **Impact**: Validation checks add latency to user commands
- **Mitigation**: Fast validation (regex, string operations), fail-fast design
- **Acceptable**: <10ms overhead per command

**Audit Logging**:
- **Impact**: HMAC signature generation adds latency to security events
- **Mitigation**: Asynchronous logging, batched writes
- **Acceptable**: <50ms overhead per audit event

## Best Practices

### For Operators

1. **Protect Age Keys**:
   ```bash
   # Backup Age keys to secure location
   mise run sops backup-keys --output /secure/backup/location
   
   # Verify key permissions
   chmod 600 ~/.config/openCenter/clusters/*/secrets/age/keys/*.txt
   ```

2. **Rotate Keys Regularly**:
   ```bash
   # Rotate keys annually or after personnel changes
   mise run sops rotate-keys --cluster <cluster-name>
   ```

3. **Monitor Audit Logs**:
   ```bash
   # Review audit logs for suspicious activity
   mise run audit query --event-type validation.failed --last 7d
   
   # Verify audit log integrity
   mise run audit verify-integrity
   ```

4. **Use Separate Keys Per Environment**:
   ```yaml
   # dev-cluster uses dev-key
   # prod-cluster uses prod-key
   # Never share keys across environments
   ```

### For Developers

1. **Never Commit Plaintext Secrets**:
   ```bash
   # WRONG: Committing plaintext secret
   git add secrets/database-password.txt
   
   # RIGHT: Encrypt before committing
   mise run sops encrypt secrets/database-password.yaml
   git add secrets/database-password.yaml
   ```

2. **Use Environment Variables for Local Development**:
   ```bash
   # Store secrets in environment variables
   export DATABASE_PASSWORD="secret123"
   
   # Reference in config
   database_password: ${DATABASE_PASSWORD}
   ```

3. **Validate Input in Custom Scripts**:
   ```go
   import "github.com/rackerlabs/openCenter-cli/internal/security"
   
   validator := security.NewDefaultInputValidator()
   if err := validator.ValidateClusterName(userInput); err != nil {
     return fmt.Errorf("invalid input: %w", err)
   }
   ```

### For Security Teams

1. **Enable Audit Logging**:
   ```yaml
   # In CLI configuration
   audit:
     enabled: true
     log_path: /var/log/opencenter/audit.log
     retention_days: 365
   ```

2. **Implement Key Rotation Policies**:
   ```bash
   # Automate key rotation with cron
   0 0 1 * * mise run sops rotate-keys --all-clusters
   ```

3. **Monitor for Security Events**:
   ```bash
   # Set up alerts for validation failures
   mise run audit query --event-type validation.failed | \
     jq -r '.[] | select(.timestamp > now - 1h)'
   ```

4. **Conduct Regular Security Audits**:
   ```bash
   # Verify all secrets are encrypted
   mise run sops verify-encryption --cluster <cluster-name>
   
   # Check for weak credentials
   mise run security scan --cluster <cluster-name>
   ```

## Future Enhancements

### Planned Security Features

1. **Hardware Security Module (HSM) Integration**:
   - Store Age keys in HSMs (YubiKey, AWS CloudHSM)
   - Prevent key extraction from hardware
   - Target: Q2 2025

2. **Automated Key Rotation**:
   - Scheduled key rotation via CronJob
   - Zero-downtime rotation with dual-key support
   - Target: Q3 2025

3. **Secret Scanning**:
   - Pre-commit hooks to detect plaintext secrets
   - Integration with git-secrets, truffleHog
   - Target: Q2 2025

4. **External Secret Stores**:
   - Integration with HashiCorp Vault
   - AWS Secrets Manager support
   - Azure Key Vault support
   - Target: Q4 2025

5. **Multi-Factor Authentication**:
   - Require MFA for sensitive operations (key rotation, cluster destroy)
   - Integration with TOTP, WebAuthn
   - Target: Q4 2025

### Research Areas

1. **Homomorphic Encryption**:
   - Encrypt secrets in a way that allows GitOps operations without decryption
   - Experimental, not production-ready

2. **Zero-Knowledge Proofs**:
   - Prove secret validity without revealing secret value
   - Useful for compliance audits

3. **Confidential Computing**:
   - Run secret decryption in trusted execution environments (TEEs)
   - Intel SGX, AMD SEV, AWS Nitro Enclaves

## Related Documentation

- [SOPS Commands Reference](../reference/sops-commands.md) - CLI commands for secret management
- [Managing Secrets How-To](../how-to/manage-secrets.md) - Step-by-step secret management guide
- [Security Hardening Operations](../operations/security-hardening.md) - Production security checklist
- [Cluster Configuration Reference](../reference/cluster-config.md) - Security-related configuration options

## Conclusion

openCenter's security model balances strong cryptographic protection with operational usability. By encrypting secrets at rest, masking credentials in logs, validating all inputs, and maintaining tamper-evident audit trails, openCenter provides a solid foundation for secure Kubernetes cluster management.

However, security is a shared responsibility. Organizations must:
- Protect Age private keys with appropriate access controls
- Rotate keys regularly and after personnel changes
- Monitor audit logs for suspicious activity
- Implement additional controls for compliance requirements (MFA, network segmentation, etc.)

For questions or security concerns, please contact the openCenter security team or file a confidential security issue.
