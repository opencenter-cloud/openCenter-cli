---
doc_type: reference
---

# Error Code Reference


## Table of Contents

- [Error Code Format](#error-code-format)
- [E1xxx: Configuration and Validation Errors](#e1xxx-configuration-and-validation-errors)
- [E2xxx: Security Errors](#e2xxx-security-errors)
- [E3xxx: Network Errors](#e3xxx-network-errors)
- [E4xxx: File System Errors](#e4xxx-file-system-errors)
- [E5xxx: Provider Errors](#e5xxx-provider-errors)
- [E6xxx: Operational Errors](#e6xxx-operational-errors)
- [Common Error Patterns](#common-error-patterns)
- [Error Message Format](#error-message-format)
- [Reporting Bugs](#reporting-bugs)
Complete reference for openCenter CLI error codes, organized by category. Use this to diagnose and resolve issues.

## Error Code Format

Error codes follow the pattern `E[1-6]xxx`:

- **E1xxx**: Configuration and validation errors
- **E2xxx**: Security errors
- **E3xxx**: Network errors
- **E4xxx**: File system errors
- **E5xxx**: Provider errors
- **E6xxx**: Operational errors

Each error message includes:
- Error code (e.g., `E1001`)
- Short description
- Context details
- Fix suggestion with command
- Hint for troubleshooting
- Documentation link

## E1xxx: Configuration and Validation Errors

### E1001: OpenStack region not configured

**Description**: The OpenStack provider requires a region to be specified.

**Cause**: The `opencenter.infrastructure.cloud.openstack.region` field is missing or empty in your cluster configuration.

**Resolution**:
```bash
openCenter cluster update {cluster} --opencenter.infrastructure.cloud.openstack.region=RegionOne
```

**Hint**: List available regions with `openstack region list`

**Related Commands**: `cluster update`, `cluster validate`

**Example**:
```
Error: OpenStack region not configured (E1001)

The OpenStack provider requires a region to be specified.

Details: validation failed for field opencenter.infrastructure.cloud.openstack.region

Fix: Add region to your configuration:
  openCenter cluster update my-cluster --opencenter.infrastructure.cloud.openstack.region=RegionOne

Hint: List available regions: openstack region list
```

---

### E1002: SOPS key not found

**Description**: The SOPS Age encryption key for this cluster could not be found.

**Cause**: The Age key file does not exist at the expected location (`~/.config/openCenter/clusters/{org}/{cluster}/secrets/age/`).

**Resolution**:
```bash
openCenter sops keygen {cluster}
```

**Hint**: Check if the key file exists: `ls -la ~/.config/openCenter/clusters/{org}/{cluster}/secrets/age/`

**Related Commands**: `sops keygen`, `sops encrypt`, `cluster setup`

**Example**:
```
Error: SOPS key not found (E1002)

The SOPS Age encryption key for this cluster could not be found.

Fix: Generate a new SOPS key:
  openCenter sops keygen prod-cluster

Hint: Check if the key file exists: ls -la ~/.config/openCenter/clusters/myorg/prod-cluster/secrets/age/
```

---

### E1003: Invalid cluster name

**Description**: Cluster names must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 63 characters).

**Cause**: The cluster name contains invalid characters or exceeds the length limit.

**Resolution**:
```bash
openCenter cluster init my-cluster
```

**Hint**: Valid examples: `my-cluster`, `prod_cluster`, `cluster123`

**Related Commands**: `cluster init`, `cluster validate`

**Example**:
```
Error: Invalid cluster name (E1003)

Cluster names must start with a letter and contain only alphanumeric characters, hyphens, and underscores (max 63 characters).

Details: cluster name "my.cluster!" contains invalid characters

Fix: Use a valid cluster name:
  openCenter cluster init my-cluster

Hint: Valid examples: my-cluster, prod_cluster, cluster123
```

---

### E1004: Configuration validation failed

**Description**: The cluster configuration contains validation errors.

**Cause**: One or more fields in the configuration file fail schema or business rule validation.

**Resolution**:
```bash
openCenter cluster validate {cluster}
```

**Hint**: View the configuration schema: `openCenter cluster schema`

**Related Commands**: `cluster validate`, `cluster schema`, `cluster edit`

**Example**:
```
Error: Configuration validation failed (E1004)

The cluster configuration contains validation errors.

Details: 3 validation errors found

Fix: Run validation to see specific errors:
  openCenter cluster validate prod-cluster

Hint: View the configuration schema: openCenter cluster schema
```

---

### E1005: Required field missing

**Description**: A required configuration field is missing or empty.

**Cause**: A mandatory field in the cluster configuration is not set.

**Resolution**:
```bash
openCenter cluster edit {cluster}
```

**Hint**: Check the schema for required fields: `openCenter cluster schema`

**Related Commands**: `cluster edit`, `cluster validate`, `cluster schema`

**Example**:
```
Error: Required field missing (E1005)

A required configuration field is missing or empty.

Details: field opencenter.meta.organization is required

Fix: Edit the configuration and add the required field:
  openCenter cluster edit prod-cluster

Hint: Check the schema for required fields: openCenter cluster schema
```

---

## E2xxx: Security Errors

### E2001: Command injection attempt detected

**Description**: The input contains shell metacharacters that could be used for command injection.

**Cause**: User input contains characters like `;`, `|`, `&`, `<`, `>`, `$`, `` ` ``, or `\` that are blocked for security.

**Resolution**: Remove shell metacharacters from the input.

**Hint**: Avoid using characters like: `; | & < > $ ` \` in names and paths

**Related Commands**: All commands that accept user input

**Example**:
```
Error: Command injection attempt detected (E2001)

The input contains shell metacharacters that could be used for command injection.

Details: input contains forbidden character: ;

Fix: Remove shell metacharacters from the input

Hint: Avoid using characters like: ; | & < > $ ` \ in names and paths
```

---

### E2002: Template injection attempt detected

**Description**: The template contains dangerous functions that are not allowed.

**Cause**: A template uses functions that could execute arbitrary code or access sensitive data.

**Resolution**: Remove dangerous template functions.

**Hint**: Allowed functions: `upper`, `lower`, `trim`, `replace`, `split`, `join`, `printf`, `quote`

**Related Commands**: `cluster setup`, template rendering operations

**Example**:
```
Error: Template injection attempt detected (E2002)

The template contains dangerous functions that are not allowed.

Details: template uses forbidden function: exec

Fix: Remove dangerous template functions

Hint: Allowed functions: upper, lower, trim, replace, split, join, printf, quote
```

---

### E2003: Path traversal attempt detected

**Description**: The path contains sequences that could be used for path traversal attacks.

**Cause**: A file path contains `..` or absolute paths that could access files outside the allowed directory.

**Resolution**: Use a path without `..` or absolute paths.

**Hint**: Paths should be relative to the configuration directory

**Related Commands**: All commands that accept file paths

**Example**:
```
Error: Path traversal attempt detected (E2003)

The path contains sequences that could be used for path traversal attacks.

Details: path contains forbidden sequence: ../../../etc/passwd

Fix: Use a path without .. or absolute paths

Hint: Paths should be relative to the configuration directory
```

---

### E2004: Invalid EDITOR environment variable

**Description**: The EDITOR environment variable contains an unsafe value.

**Cause**: The `EDITOR` variable is set to a command that is not in the allowed list.

**Resolution**:
```bash
export EDITOR=vim
```

**Hint**: Allowed editors: `vim`, `nano`, `emacs`, `vi`, `code`, `subl`

**Related Commands**: `cluster edit`, `config edit`

**Example**:
```
Error: Invalid EDITOR environment variable (E2004)

The EDITOR environment variable contains an unsafe value.

Details: EDITOR=/usr/bin/dangerous-script

Fix: Set EDITOR to a safe editor:
  export EDITOR=vim

Hint: Allowed editors: vim, nano, emacs, vi, code, subl
```

---

## E3xxx: Network Errors

### E3001: Network timeout

**Description**: The operation timed out while waiting for a network response.

**Cause**: Network connectivity issues, firewall rules, or the remote service is not responding.

**Resolution**:
```bash
ping {host}
```

**Hint**: Verify firewall rules and network configuration

**Related Commands**: `cluster preflight`, `cluster bootstrap`, provider operations

**Example**:
```
Error: Network timeout (E3001)

The operation timed out while waiting for a network response.

Details: connection to keystone.example.com:5000 timed out after 30s

Fix: Check network connectivity and retry:
  ping keystone.example.com

Hint: Verify firewall rules and network configuration
```

---

### E3002: Connection refused

**Description**: The connection to the remote service was refused.

**Cause**: The service is not running, listening on a different port, or blocked by a firewall.

**Resolution**:
```bash
systemctl status {service}
```

**Hint**: Check if the service is listening on the expected port

**Related Commands**: `cluster preflight`, provider operations

**Example**:
```
Error: Connection refused (E3002)

The connection to the remote service was refused.

Details: connection to 192.168.1.100:6443 refused

Fix: Verify the service is running:
  systemctl status kubelet

Hint: Check if the service is listening on the expected port
```

---

## E4xxx: File System Errors

### E4001: File not found

**Description**: The specified file or directory does not exist.

**Cause**: The file path is incorrect, the file was deleted, or it was never created.

**Resolution**:
```bash
ls -la {path}
```

**Hint**: Check for typos in the file path

**Related Commands**: All commands that read files

**Example**:
```
Error: File not found (E4001)

The specified file or directory does not exist.

Details: no such file: /home/user/.config/openCenter/clusters/myorg/prod/.prod-config.yaml

Fix: Verify the path exists:
  ls -la /home/user/.config/openCenter/clusters/myorg/prod/

Hint: Check for typos in the file path
```

---

### E4002: Permission denied

**Description**: You do not have permission to access this file or directory.

**Cause**: Insufficient file permissions or ownership issues.

**Resolution**:
```bash
chmod +w {path}
```

**Hint**: Check file ownership: `ls -la {path}`

**Related Commands**: All commands that write files

**Example**:
```
Error: Permission denied (E4002)

You do not have permission to access this file or directory.

Details: cannot write to /etc/openCenter/config.yaml

Fix: Grant appropriate permissions:
  chmod +w /etc/openCenter/config.yaml

Hint: Check file ownership: ls -la /etc/openCenter/config.yaml
```

---

### E4003: Disk space exhausted

**Description**: There is not enough disk space to complete the operation.

**Cause**: The file system is full or nearly full.

**Resolution**:
```bash
df -h
```

**Hint**: Remove old logs or unused files

**Related Commands**: `cluster setup`, `cluster bootstrap`, backup operations

**Example**:
```
Error: Disk space exhausted (E4003)

There is not enough disk space to complete the operation.

Details: no space left on device: /home/user/.config/openCenter

Fix: Free up disk space:
  df -h

Hint: Remove old logs or unused files
```

---

## E5xxx: Provider Errors

### E5001: OpenStack API error

**Description**: An error occurred while communicating with the OpenStack API.

**Cause**: Invalid credentials, network issues, or OpenStack service problems.

**Resolution**:
```bash
openstack server list
```

**Hint**: Run preflight checks: `openCenter cluster preflight {cluster}`

**Related Commands**: `cluster preflight`, `cluster bootstrap`, OpenStack operations

**Example**:
```
Error: OpenStack API error (E5001)

An error occurred while communicating with the OpenStack API.

Details: authentication failed: invalid credentials

Fix: Verify OpenStack credentials and connectivity:
  openstack server list

Hint: Run preflight checks: openCenter cluster preflight prod-cluster
```

---

### E5002: AWS API error

**Description**: An error occurred while communicating with the AWS API.

**Cause**: Invalid credentials, expired tokens, or AWS service issues.

**Resolution**:
```bash
aws sts get-caller-identity
```

**Hint**: Check `AWS_ACCESS_KEY_ID` and `AWS_SECRET_ACCESS_KEY` environment variables

**Related Commands**: `cluster preflight`, `cluster bootstrap`, AWS operations

**Example**:
```
Error: AWS API error (E5002)

An error occurred while communicating with the AWS API.

Details: InvalidClientTokenId: The security token included in the request is invalid

Fix: Verify AWS credentials:
  aws sts get-caller-identity

Hint: Check AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables
```

---

### E5003: Provider authentication failed

**Description**: Authentication with the cloud provider failed.

**Cause**: Invalid credentials, expired tokens, or insufficient permissions.

**Resolution**: Verify your credentials are correct.

**Hint**: Check credential expiration and permissions

**Related Commands**: `cluster preflight`, provider operations

**Example**:
```
Error: Provider authentication failed (E5003)

Authentication with the cloud provider failed.

Details: authentication failed for provider: openstack

Fix: Verify your credentials are correct

Hint: Check credential expiration and permissions
```

---

## E6xxx: Operational Errors

### E6001: Drift detection failed

**Description**: Unable to detect configuration drift for the cluster.

**Cause**: Cluster is not accessible, provider connectivity issues, or missing state files.

**Resolution**:
```bash
openCenter cluster info {cluster}
```

**Hint**: Check cloud provider connectivity

**Related Commands**: `cluster drift`, `cluster info`

**Example**:
```
Error: Drift detection failed (E6001)

Unable to detect configuration drift for the cluster.

Details: cannot read cluster state: connection timeout

Fix: Verify cluster is accessible:
  openCenter cluster info prod-cluster

Hint: Check cloud provider connectivity
```

---

### E6002: Backup creation failed

**Description**: Unable to create a backup of the cluster configuration.

**Cause**: Insufficient disk space, permission issues, or backup directory not writable.

**Resolution**:
```bash
df -h && ls -la ~/.config/openCenter/backups/
```

**Hint**: Ensure the backup directory is writable

**Related Commands**: `cluster backup`, `cluster update`

**Example**:
```
Error: Backup creation failed (E6002)

Unable to create a backup of the cluster configuration.

Details: cannot write backup file: permission denied

Fix: Check disk space and permissions:
  df -h && ls -la ~/.config/openCenter/backups/

Hint: Ensure the backup directory is writable
```

---

### E6003: Lock acquisition failed

**Description**: Unable to acquire a lock for the cluster operation.

**Cause**: Another operation is in progress, or a stale lock exists.

**Resolution**:
```bash
openCenter cluster lock break {cluster}
```

**Hint**: Check if another operation is in progress

**Related Commands**: `cluster lock`, `cluster lock break`

**Example**:
```
Error: Lock acquisition failed (E6003)

Unable to acquire a lock for the cluster operation.

Details: lock held by process 12345 since 2024-01-15 10:30:00

Fix: Wait for the current operation to complete or break the lock:
  openCenter cluster lock break prod-cluster

Hint: Check if another operation is in progress
```

---

### E6004: Retry budget exhausted

**Description**: The operation failed after exhausting all retry attempts.

**Cause**: Persistent failures due to service issues, network problems, or invalid configuration.

**Resolution**: Check the underlying error and retry manually.

**Hint**: The service may be experiencing issues

**Related Commands**: All commands with retry logic

**Example**:
```
Error: Retry budget exhausted (E6004)

The operation failed after exhausting all retry attempts.

Details: failed after 5 attempts over 2m30s: connection timeout

Fix: Check the underlying error and retry manually

Hint: The service may be experiencing issues
```

---

## Common Error Patterns

### Configuration Syntax Errors

**Symptoms**: YAML parsing errors, unexpected characters, indentation issues

**Diagnosis**:
```bash
openCenter cluster validate {cluster}
```

**Common Causes**:
- Incorrect YAML indentation (use spaces, not tabs)
- Missing quotes around special characters
- Unclosed brackets or braces
- Invalid field names

**Resolution**: Edit the configuration file and fix syntax errors. Use a YAML validator or linter.

---

### Missing Required Fields

**Symptoms**: E1005 errors, validation failures

**Diagnosis**:
```bash
openCenter cluster schema
openCenter cluster validate {cluster}
```

**Common Causes**:
- Incomplete configuration after `cluster init`
- Fields removed during manual editing
- Schema version mismatch

**Resolution**: Add the missing required fields. Check the schema for field requirements.

---

### Invalid Values

**Symptoms**: Validation errors, type mismatches

**Diagnosis**:
```bash
openCenter cluster validate {cluster}
```

**Common Causes**:
- Wrong data type (string instead of number)
- Value outside allowed range
- Invalid enum value
- Malformed URLs or paths

**Resolution**: Correct the field value according to the schema. Use `cluster schema` to see allowed values.

---

### Provider Authentication Failures

**Symptoms**: E5001, E5002, E5003 errors

**Diagnosis**:
```bash
openCenter cluster preflight {cluster}
```

**Common Causes**:
- Expired credentials or tokens
- Missing environment variables
- Incorrect auth URL or endpoint
- Insufficient permissions

**Resolution**:
1. Verify credentials are current
2. Check environment variables are set
3. Test provider CLI tools directly
4. Review IAM/RBAC permissions

---

### Network Connectivity Issues

**Symptoms**: E3001, E3002 errors, timeouts

**Diagnosis**:
```bash
openCenter cluster preflight {cluster}
ping {host}
curl -v {url}
```

**Common Causes**:
- Firewall blocking connections
- DNS resolution failures
- Service not running
- Wrong port or endpoint

**Resolution**:
1. Check firewall rules
2. Verify DNS resolution
3. Confirm service is running
4. Test connectivity with curl or telnet

---

### SOPS Key Problems

**Symptoms**: E1002 errors, encryption/decryption failures

**Diagnosis**:
```bash
ls -la ~/.config/openCenter/clusters/{org}/{cluster}/secrets/age/
openCenter sops keygen {cluster}
```

**Common Causes**:
- Key file not generated
- Key file deleted or moved
- Wrong key file path in configuration
- Corrupted key file

**Resolution**:
1. Generate a new key with `sops keygen`
2. Verify key file location
3. Update configuration with correct key path
4. Re-encrypt secrets with new key if needed

---

## Error Message Format

Error messages follow this structure:

```
Error: {Title} ({Code})

{Description}

Details: {Context from error}

Fix: {Resolution steps}
  {Command to run}

Hint: {Additional troubleshooting tip}

Learn more: https://docs.opencenter.cloud/errors/{Code}
```

**Components**:
- **Title**: Short, descriptive error name
- **Code**: Error code (E1xxx-E6xxx)
- **Description**: What went wrong
- **Details**: Specific context from the error
- **Fix**: Actionable resolution steps
- **Command**: Exact command to run (if applicable)
- **Hint**: Additional troubleshooting guidance
- **Learn more**: Link to detailed documentation

---

## Reporting Bugs

If you encounter an error that:
- Does not have an error code
- Has incorrect or unhelpful information
- Represents a bug in openCenter

Report it at: https://github.com/rackerlabs/openCenter-cli/issues

Include:
- Full error message
- Command that triggered the error
- Configuration file (redact sensitive data)
- Output of `openCenter version`
- Steps to reproduce

For security issues, email: security@rackspace.com
