# Troubleshooting Guide


## Table of Contents

- [General Troubleshooting Steps](#general-troubleshooting-steps)
- [Common Issues](#common-issues)
- [Debugging Techniques](#debugging-techniques)
- [Performance Issues](#performance-issues)
- [Getting More Help](#getting-more-help)
- [Additional Resources](#additional-resources)
- [Common Error Messages](#common-error-messages)
**doc_type: how-to**

This guide helps you diagnose and resolve common issues with openCenter.

## General Troubleshooting Steps

### 1. Enable Verbose Logging

```bash
openCenter --verbose <command>
```

This provides detailed debug information about what openCenter is doing.

### 2. Generate Debug Configuration

```bash
openCenter cluster validate my-cluster --generate-debug-config
```

This creates a complete configuration file (`.openCenter.yaml`) in your GitOps directory showing all resolved values.

### 3. Check Environment Variables

```bash
env | grep OPENCENTER
```

Verify that environment variables are set correctly:
- `OPENCENTER_CONFIG_DIR`
- `OPENCENTER_DEBUG`
- `OPENCENTER_PLUGINS_DIR`

### 4. Verify Tool Versions

```bash
mise list
```

Ensure all required tools are installed and at the correct versions.

## Common Issues

### Configuration Issues

#### Issue: "cluster name cannot be empty"

**Symptom:** Error when running cluster commands.

**Cause:** No cluster name provided and no active cluster set.

**Solution:**
```bash
# Either provide cluster name explicitly
openCenter cluster validate my-cluster

# Or set active cluster
openCenter cluster select my-cluster
openCenter cluster validate
```

#### Issue: "invalid cluster name"

**Symptom:** Error during cluster initialization.

**Cause:** Cluster name doesn't meet requirements (3-63 characters, lowercase, alphanumeric with hyphens).

**Solution:**
```bash
# Use valid cluster name
openCenter cluster init my-valid-cluster-name

# Invalid examples:
# - "My Cluster" (spaces not allowed)
# - "a" (too short)
# - "CLUSTER" (uppercase not allowed)
# - "-cluster" (cannot start with hyphen)
```

#### Issue: "validation failed" with multiple errors

**Symptom:** Configuration validation fails with multiple error messages.

**Cause:** Configuration doesn't meet schema or business rule requirements.

**Solution:**
1. Read each error message carefully
2. Fix issues one at a time
3. Use `--verbose` for more details
4. Generate debug config to see resolved values

```bash
openCenter --verbose cluster validate my-cluster --generate-debug-config
```

#### Issue: "only one network plugin can be enabled"

**Symptom:** Validation fails with network plugin conflict.

**Cause:** Multiple network plugins (Calico, Cilium, Kube-OVN) are enabled.

**Solution:**
```bash
# Disable all but one network plugin in your configuration
# Edit your cluster config file and ensure only one is enabled:

opencenter:
  cluster:
    kubernetes:
      network_plugin:
        calico:
          enabled: true    # Only this one enabled
        cilium:
          enabled: false
        kube-ovn:
          enabled: false
```

### SOPS and Secrets Issues

#### Issue: "failed to generate Age key pair"

**Symptom:** Error during cluster initialization or SOPS key generation.

**Cause:** SOPS or Age not installed, or insufficient permissions.

**Solution:**
```bash
# Check if SOPS is installed
which sops

# Check if Age is installed
which age

# Install if missing (macOS)
brew install sops age

# Install if missing (Linux)
# Download from https://github.com/mozilla/sops/releases
# Download from https://github.com/FiloSottile/age/releases

# Check directory permissions
ls -la ~/.config/openCenter/clusters/
```

#### Issue: "Age key file not found"

**Symptom:** Error when trying to encrypt/decrypt secrets.

**Cause:** SOPS key file doesn't exist at specified path.

**Solution:**
```bash
# Generate new key
openCenter sops generate-key

# Or specify correct path in configuration
secrets:
  sops_age_key_file: /correct/path/to/key.txt

# Or regenerate for cluster
openCenter cluster init my-cluster --force
```

#### Issue: "failed to encrypt file"

**Symptom:** Error when running `sops secrets-encrypt`.

**Cause:** SOPS configuration missing or incorrect, or Age key not accessible.

**Solution:**
```bash
# Validate SOPS setup
openCenter sops validate

# Check .sops.yaml exists
ls -la .sops.yaml

# Verify Age key is readable
cat ~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/secrets/age/keys/my-cluster-key.txt

# Regenerate SOPS configuration
openCenter sops generate-key --update-sops-config
```

### GitOps Issues

#### Issue: "gitops.git_dir must be set"

**Symptom:** Validation fails with missing git_dir.

**Cause:** GitOps directory not configured.

**Solution:**
```bash
# Update configuration with git_dir
openCenter cluster update my-cluster \
  --opencenter.gitops.git_dir=./gitops-my-cluster

# Or edit configuration file directly
opencenter:
  gitops:
    git_dir: ./gitops-my-cluster
```

#### Issue: "failed to initialize git repository"

**Symptom:** Error during cluster setup.

**Cause:** Git not installed, or directory permissions issue.

**Solution:**
```bash
# Check if git is installed
which git
git --version

# Install git if missing (macOS)
brew install git

# Install git if missing (Linux)
sudo apt-get install git  # Debian/Ubuntu
sudo yum install git      # RHEL/CentOS

# Check directory permissions
ls -la $(dirname ./gitops-my-cluster)

# Try with force flag
openCenter cluster setup my-cluster --force
```

#### Issue: "template rendering failed"

**Symptom:** Error during cluster setup or render.

**Cause:** Invalid template syntax or missing configuration values.

**Solution:**
```bash
# Validate configuration first
openCenter cluster validate my-cluster

# Generate debug config to see all values
openCenter cluster validate my-cluster --generate-debug-config

# Check for missing required fields
openCenter --verbose cluster setup my-cluster
```

### Provider Issues

#### Issue: "openstack_auth_url must be set"

**Symptom:** Validation fails for OpenStack provider.

**Cause:** Required OpenStack configuration missing.

**Solution:**
```bash
# Update configuration with OpenStack details
openCenter cluster update my-cluster \
  --opencenter.infrastructure.cloud.openstack.auth_url=https://keystone.example.com/v3/ \
  --opencenter.infrastructure.cloud.openstack.region=RegionOne \
  --opencenter.infrastructure.cloud.openstack.tenant_name=my-project
```

#### Issue: "AWS credentials required for S3 backend"

**Symptom:** Validation fails when using S3 backend.

**Cause:** AWS credentials not configured.

**Solution:**
```bash
# Add AWS credentials to configuration
openCenter cluster update my-cluster \
  --opencenter.cluster.aws_access_key=AKIAIOSFODNN7EXAMPLE \
  --opencenter.cluster.aws_secret_access_key=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
```

#### Issue: "failed to create kind cluster"

**Symptom:** Error during Kind cluster bootstrap.

**Cause:** Docker/Podman not running, or Kind not installed.

**Solution:**
```bash
# Check if Docker is running
docker ps

# Or check if Podman is running
podman ps

# Start Docker (macOS)
open -a Docker

# Start Podman (Linux)
sudo systemctl start podman

# Check if Kind is installed
which kind
kind version

# Install Kind if missing
mise install kind
```

### Build and Test Issues

#### Issue: "mise: command not found"

**Symptom:** Cannot run mise commands.

**Cause:** Mise not installed.

**Solution:**
```bash
# Install Mise (macOS)
brew install mise

# Install Mise (Linux)
curl https://mise.run | sh

# Add to shell profile
echo 'eval "$(mise activate bash)"' >> ~/.bashrc  # Bash
echo 'eval "$(mise activate zsh)"' >> ~/.zshrc    # Zsh
```

#### Issue: "go: command not found"

**Symptom:** Cannot build openCenter.

**Cause:** Go not installed or not in PATH.

**Solution:**
```bash
# Install tools via Mise
mise install

# Verify Go is available
mise exec -- go version

# Or install Go directly
# Download from https://go.dev/dl/
```

#### Issue: "BDD tests failing"

**Symptom:** `mise run godog` fails.

**Cause:** Test environment not properly configured.

**Solution:**
```bash
# Clean test artifacts
mise run clean

# Rebuild
mise run build

# Run tests with verbose output
go test ./tests/... -v

# Run specific test
go test ./tests/... -v -run TestFeatureName
```

### Path and Directory Issues

#### Issue: "failed to create cluster directory"

**Symptom:** Error during cluster initialization.

**Cause:** Insufficient permissions or invalid path.

**Solution:**
```bash
# Check permissions on config directory
ls -la ~/.config/openCenter/

# Create directory manually if needed
mkdir -p ~/.config/openCenter/clusters

# Set correct permissions
chmod 755 ~/.config/openCenter/clusters

# Try with different config directory
OPENCENTER_CONFIG_DIR=/tmp/opencenter openCenter cluster init my-cluster
```

#### Issue: "cluster configuration file not found"

**Symptom:** Cannot load cluster configuration.

**Cause:** Cluster not initialized or wrong directory.

**Solution:**
```bash
# List available clusters
openCenter cluster list

# Check if cluster exists
ls -la ~/.config/openCenter/clusters/

# Initialize cluster if missing
openCenter cluster init my-cluster

# Specify custom config directory
openCenter --config-dir=/path/to/configs cluster list
```

## Debugging Techniques

### 1. Inspect Configuration Files

```bash
# View cluster configuration
cat ~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/.my-cluster-config.yaml

# View generated debug configuration
cat ./gitops-my-cluster/.openCenter.yaml

# View SOPS configuration
cat ~/.config/openCenter/clusters/myorg/.sops.yaml
```

### 2. Check File Permissions

```bash
# Check config directory permissions
ls -la ~/.config/openCenter/

# Check cluster directory permissions
ls -la ~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/

# Check SOPS key permissions (should be 0600)
ls -la ~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/secrets/age/keys/
```

### 3. Validate JSON Schema

```bash
# Generate schema
openCenter cluster schema --pretty --out schema.json

# Use online validator
# Upload schema.json and your config to https://www.jsonschemavalidator.net/
```

### 4. Test SOPS Manually

```bash
# Export Age key
export SOPS_AGE_KEY_FILE=~/.config/openCenter/clusters/myorg/infrastructure/clusters/my-cluster/secrets/age/keys/my-cluster-key.txt

# Test encryption
echo "test: secret" > test.yaml
sops -e test.yaml > test.enc.yaml

# Test decryption
sops -d test.enc.yaml

# Clean up
rm test.yaml test.enc.yaml
```

### 5. Check Provider Connectivity

```bash
# OpenStack
openstack --os-auth-url https://keystone.example.com/v3/ \
  --os-application-credential-id <id> \
  --os-application-credential-secret <secret> \
  server list

# AWS
aws --profile myprofile ec2 describe-instances

# Kind
kind get clusters
```

## Performance Issues

### Issue: Slow configuration validation

**Symptom:** Validation takes more than a few seconds.

**Cause:** Large configuration file or complex validation rules.

**Solution:**
```bash
# Use dry-run mode for faster feedback
openCenter --dry-run cluster validate my-cluster

# Disable connectivity checks (if implemented)
# Check only schema and business rules
```

### Issue: Slow template rendering

**Symptom:** Cluster setup takes a long time.

**Cause:** Large number of services or complex templates.

**Solution:**
```bash
# Disable unnecessary services in configuration
# Render only required templates

# Use copy mode instead of render mode
openCenter cluster setup my-cluster  # Without --render flag
```

## Getting More Help

### Enable Debug Mode

```bash
export OPENCENTER_DEBUG=true
openCenter cluster validate my-cluster
```

This creates additional debug artifacts and enables detailed logging.

### Collect Diagnostic Information

```bash
# System information
uname -a
go version
mise list

# openCenter version
./bin/openCenter --version

# Configuration
openCenter cluster list
openCenter cluster current
openCenter --verbose cluster validate my-cluster

# Environment
env | grep OPENCENTER
```

### Report Issues

When reporting issues, include:

1. **openCenter version:** `./bin/openCenter --version`
2. **Go version:** `go version`
3. **Operating system:** `uname -a`
4. **Command executed:** Full command with flags
5. **Error message:** Complete error output
6. **Configuration:** Sanitized configuration file (remove secrets)
7. **Debug output:** Output with `--verbose` flag

Submit issues at: https://github.com/rackerlabs/openCenter-cli/issues

## Additional Resources

- [CLI Commands Reference](reference/cli-commands.md)
- [Configuration Reference](reference/configuration.md)
- [Architecture Documentation](architecture.md)
- [Current Status](current-status.md)

## Common Error Messages

### Error Code Reference

openCenter uses structured error codes to help you quickly identify and resolve issues. Each error code follows the format `EXXX` where XXX is a unique identifier.

| Error Code | Category | Description | Quick Fix |
|------------|----------|-------------|-----------|
| E1001 | Configuration | OpenStack region not configured | Add `--opencenter.infrastructure.cloud.openstack.region=RegionOne` |
| E1002 | Secrets | SOPS key not found | Run `openCenter sops generate-key <cluster>` |
| E1003 | Validation | Invalid cluster name format | Use lowercase alphanumeric with hyphens, 3-63 chars |
| E1004 | Network | Network plugin conflict | Ensure only one network plugin is enabled |
| E1005 | GitOps | GitOps directory not set | Add `--opencenter.gitops.git_dir=./gitops-<cluster>` |
| E1006 | Provider | AWS credentials missing | Configure AWS credentials or IAM role |
| E1007 | Provider | Insufficient OpenStack quota | Request quota increase from admin |
| E1008 | Template | Template rendering failed | Check configuration for missing required fields |
| E1009 | Validation | VRRP IP required | Set `vrrp_ip` or enable Octavia |
| E1010 | Validation | Windows workers misconfigured | Match `windows_workers.enabled` with `worker_count_windows` |

**Note:** Error codes E1001-E1999 are configuration errors, E2001-E2999 are validation errors, E3001-E3999 are provider errors, E4001-E4999 are secrets/security errors, E5001-E5999 are GitOps errors, and E6001-E6999 are runtime errors.

### "field not found: 'X' in struct 'Y'"

**Meaning:** Configuration field doesn't exist in schema.

**Fix:** Check [Configuration Reference](reference/configuration.md) for correct field names.

### "failed to parse YAML configuration"

**Meaning:** YAML syntax error in configuration file.

**Fix:** Validate YAML syntax using online validator or `yamllint`.

### "cluster name must match meta.name"

**Meaning:** Inconsistency between cluster.cluster_name and meta.name.

**Fix:** Ensure both fields have the same value.

### "vrrp_ip must be set when use_octavia is false"

**Meaning:** VRRP IP required for non-Octavia load balancing.

**Fix:** Either enable Octavia or provide VRRP IP address.

### "windows_workers.enabled must be false when worker_count_windows is 0"

**Meaning:** Windows workers enabled but count is zero.

**Fix:** Either disable Windows workers or set count > 0.

---

If you encounter an issue not covered here, please check the [documentation index](INDEX.md) or [report it](https://github.com/rackerlabs/openCenter-cli/issues).
