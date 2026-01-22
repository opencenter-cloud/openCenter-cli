---
title: Developer Commands
doc_type: reference
weight: 90
---

# Developer Commands


## Table of Contents

- [Who this is for](#who-this-is-for)
- [Overview](#overview)
- [Available Hidden Commands](#available-hidden-commands)
- [Why Commands are Hidden](#why-commands-are-hidden)
- [Discovering Hidden Commands](#discovering-hidden-commands)
- [Using Hidden Commands in Scripts](#using-hidden-commands-in-scripts)
- [Migration Guide](#migration-guide)
- [Related Documentation](#related-documentation)
- [See Also](#see-also)
openCenter includes several developer commands that are useful for development, debugging, and advanced workflows. These commands are intentionally hidden from the main help menu to keep it focused on primary user workflows, but they remain fully functional and documented here.

## Who this is for

Developers, power users, and CI/CD pipelines that need access to advanced functionality like schema generation, credential management, and debugging tools.

## Overview

Hidden commands are registered in Cobra with `Hidden: true`, which means:
- They don't appear in `openCenter --help` or `openCenter cluster --help`
- They are fully functional and can be invoked directly
- They follow the same flag and argument patterns as visible commands
- They are tested and maintained as part of the codebase

## Available Hidden Commands

### cluster template

**Purpose**: Generate a complete cluster configuration template with all available fields.

**Status**: Hidden - Development/documentation command

**Use Cases**:
- Understanding the complete configuration schema
- Creating comprehensive documentation examples
- Reference for all available configuration options
- Testing configuration validation
- Migration from older schema versions
- IDE autocomplete reference

**Usage**:

```bash
# Generate complete template to stdout
openCenter cluster template

# Save template to file
openCenter cluster template --out complete-config.yaml

# Generate template for specific provider
openCenter cluster template --provider openstack --out openstack-template.yaml

# Generate with inline comments
openCenter cluster template --comments --out documented-config.yaml

# Generate minimal template (only required fields)
openCenter cluster template --minimal --out minimal-config.yaml
```

**Flags**:
- `--out <path>`: Output file path (default: stdout)
- `--provider <name>`: Generate template for specific provider (openstack, aws, talos, kind, baremetal, all) [default: all]
- `--comments`: Include inline comments explaining each field [default: false]
- `--minimal`: Generate minimal template with only required fields [default: false]

**Difference from `cluster init`**:

| Command | Purpose | Output |
|---------|---------|--------|
| `cluster init` | Create working cluster config | Minimal config with sensible defaults, ready to use |
| `cluster template` | Generate reference template | Complete config showing all available fields |

**Provider-Specific Templates**:

```bash
# OpenStack template with all OpenStack-specific fields
openCenter cluster template --provider openstack --out openstack-full.yaml

# AWS template with all AWS-specific fields
openCenter cluster template --provider aws --out aws-full.yaml

# Talos template with Talos Linux configuration
openCenter cluster template --provider talos --out talos-full.yaml

# Kind template for local development
openCenter cluster template --provider kind --out kind-full.yaml

# Baremetal template with pre-configured nodes
openCenter cluster template --provider baremetal --out baremetal-full.yaml

# All providers (includes all provider configurations)
openCenter cluster template --provider all --out complete-full.yaml
```

**Template Contents**:

The generated template includes:
- **Metadata**: created_at, created_by, tags, annotations
- **Cluster Meta**: name, organization, env, region, status
- **Infrastructure**: All provider configurations (OpenStack, AWS, Talos)
- **Kubernetes**: Version, node counts, CNI, OIDC, Windows workers
- **Services**: All 27+ services with their specific fields
- **GitOps**: FluxCD configuration
- **Storage**: CSI, volumes, storage classes
- **OpenTofu**: Backend configuration (local, S3)
- **Secrets**: All service-specific secrets structure
- **Networking**: Proxy, NTP, DNS configuration

**Minimal Template**:

The `--minimal` flag generates a template with only required fields:

```yaml
schema_version: "1.0.0"
opencenter:
  meta:
    name: example-cluster
    organization: opencenter
  infrastructure:
    provider: openstack
  cluster:
    cluster_name: example-cluster
    kubernetes:
      version: "1.33.5"
      master_count: 3
      worker_count: 2
  gitops:
    git_dir: ./gitops-repo
opentofu:
  enabled: true
secrets: {}
```

**Use in Documentation**:

```bash
# Generate template for documentation
openCenter cluster template --comments --out docs/examples/complete-config.yaml

# Generate provider-specific examples
openCenter cluster template --provider openstack --comments --out docs/examples/openstack.yaml
openCenter cluster template --provider aws --comments --out docs/examples/aws.yaml
```

**Use in Testing**:

```bash
# Generate test fixtures
openCenter cluster template --provider openstack --out testdata/openstack-full.yaml
openCenter cluster template --minimal --out testdata/minimal-config.yaml

# Validate generated template
openCenter cluster validate --config testdata/openstack-full.yaml
```

**Related Files**:
- `cmd/cluster_template.go`: Command implementation
- `internal/config/config.go`: Default configuration generation
- `internal/config/services/*.go`: Service-specific configurations

---

### cluster schema

**Purpose**: Export the JSON schema for cluster configuration with validation rules.

**Status**: Hidden - Internal/development command

**Use Cases**:
- IDE integration (autocomplete, validation)
- Documentation generation
- Schema versioning and migration
- CI/CD validation pipelines

**Usage**:

```bash
# Print schema to stdout
openCenter cluster schema

# Save schema to file with pretty formatting
openCenter cluster schema --out schema/cluster.schema.json --pretty

# Show schema version
openCenter cluster schema --version

# Save without pretty printing (compact)
openCenter cluster schema --out schema.json --pretty=false
```

**Flags**:
- `--out <path>`: Output file path (default: stdout)
- `--pretty`: Pretty print JSON schema (default: true)
- `--version`: Show schema version only

**Output Format**:

The schema is a JSON Schema Draft 2020-12 document that includes:
- All configuration sections (opencenter, opentofu, secrets, etc.)
- Field types, constraints, and validation rules
- Default values and examples
- Descriptions for every field
- Enum values for restricted fields
- Pattern validation for strings (CIDR, UUID, email, etc.)

**Example Output** (truncated):

```json
{
  "$id": "https://opencenter.cloud/schemas/cluster-config.json",
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "version": "1.0.0",
  "properties": {
    "opencenter": {
      "properties": {
        "cluster": {
          "properties": {
            "cluster_name": {
              "type": "string",
              "pattern": "^[a-z0-9][a-z0-9-]*[a-z0-9]$",
              "minLength": 3,
              "maxLength": 63
            }
          }
        }
      }
    }
  }
}
```

**IDE Integration**:

See [IDE Integration Guide](../how-to/ide-integration.md) for instructions on configuring your editor to use the generated schema.

**Related Files**:
- `cmd/cluster_schema.go`: Command implementation
- `internal/config/schema.go`: Schema generation logic
- `schema/cluster.schema.json`: Pre-generated schema (checked into repo)

---

### cluster credentials

**Purpose**: Manage cloud provider credentials from cluster configuration.

**Status**: Hidden - Superseded by `cluster select --activate`

**Note**: This command is kept for backward compatibility but is superseded by the simpler `cluster select --activate` workflow. New users should use `cluster select --activate` instead.

**Use Cases**:
- Export credentials for use with Terraform, Ansible, or cloud CLIs
- Generate environment variable exports for shell sessions
- Convert credentials to different formats (JSON, Terraform, clouds.yaml)
- Clean up credentials from environment

**Subcommands**:
- `cluster credentials export`: Export credentials in various formats
- `cluster credentials unset`: Generate unset commands to clear credentials

---

#### cluster credentials export

Export cloud provider credentials from SOPS-encrypted cluster configuration.

**Usage**:

```bash
# Export AWS credentials for current cluster
eval $(openCenter cluster credentials export --provider aws)

# Export OpenStack credentials for specific cluster
eval $(openCenter cluster credentials export my-cluster --provider openstack)

# Export all credentials in JSON format
openCenter cluster credentials export --provider all --format json

# Export AWS credentials in Terraform format
openCenter cluster credentials export --provider aws --format terraform

# Export OpenStack credentials as clouds.yaml
openCenter cluster credentials export --provider openstack --format clouds-yaml
```

**Flags**:
- `--provider <name>`: Cloud provider to export (aws, openstack, all) [default: all]
- `--format <format>`: Output format (env, json, terraform, clouds-yaml) [default: env]

**Supported Providers**:

| Provider | Description | Credential Sources |
|----------|-------------|-------------------|
| `aws` | Amazon Web Services | `opencenter.infrastructure.cloud.aws`<br>`secrets.global.aws.infrastructure` |
| `openstack` | OpenStack | `opencenter.infrastructure.cloud.openstack`<br>Application credentials from config |
| `all` | All configured providers | Combines AWS and OpenStack |

**Output Formats**:

| Format | Description | Supported Providers | Use Case |
|--------|-------------|---------------------|----------|
| `env` | Shell environment exports | All | `eval $(...)` in shell |
| `json` | JSON object | All | Programmatic use, CI/CD |
| `terraform` | Terraform provider config | All | Terraform/OpenTofu |
| `clouds-yaml` | OpenStack clouds.yaml | OpenStack only | OpenStack CLI tools |

**Format Examples**:

**env format** (default):
```bash
export AWS_ACCESS_KEY_ID="AKIAIOSFODNN7EXAMPLE"
export AWS_SECRET_ACCESS_KEY="wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
export AWS_DEFAULT_REGION="us-east-1"
```

**json format**:
```json
{
  "access_key": "AKIAIOSFODNN7EXAMPLE",
  "secret_access_key": "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
  "region": "us-east-1",
  "profile": "default"
}
```

**terraform format**:
```hcl
provider "aws" {
  access_key = "AKIAIOSFODNN7EXAMPLE"
  secret_key = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
  region     = "us-east-1"
}
```

**clouds-yaml format** (OpenStack only):
```yaml
clouds:
  prod-cluster:
    auth:
      auth_url: https://identity.api.sjc3.rackspacecloud.com/v3
      application_credential_id: "abc123..."
      application_credential_secret: "secret123..."
    region_name: sjc3
    interface: public
    identity_api_version: 3
```

**Credential Sources**:

**AWS**:
- Infrastructure credentials: `secrets.global.aws.infrastructure`
  - `access_key`: AWS access key ID
  - `secret_access_key`: AWS secret access key
  - `region`: Default AWS region
- Configuration: `opencenter.infrastructure.cloud.aws`
  - `profile`: AWS CLI profile name
  - `vpc_id`: VPC ID for cluster
  - `private_subnets`: Private subnet IDs
  - `public_subnets`: Public subnet IDs

**OpenStack**:
- Configuration: `opencenter.infrastructure.cloud.openstack`
  - `auth_url`: Keystone authentication URL
  - `region`: OpenStack region
  - `application_credential_id`: Application credential ID
  - `application_credential_secret`: Application credential secret
  - `domain`: OpenStack domain
  - `tenant_name`: Project/tenant name

**Common Workflows**:

**1. Use with Terraform**:
```bash
# Export AWS credentials
eval $(openCenter cluster credentials export --provider aws)

# Run Terraform
cd infrastructure/
terraform plan
terraform apply
```

**2. Use with OpenStack CLI**:
```bash
# Export OpenStack credentials
eval $(openCenter cluster credentials export --provider openstack)

# Use OpenStack CLI
openstack server list
openstack network list
```

**3. Generate clouds.yaml for OpenStack tools**:
```bash
# Generate clouds.yaml
openCenter cluster credentials export \
  --provider openstack \
  --format clouds-yaml > ~/.config/openstack/clouds.yaml

# Use with OpenStack CLI
openstack --os-cloud prod-cluster server list
```

**4. CI/CD Pipeline**:
```bash
# Export credentials as JSON for parsing
CREDS=$(openCenter cluster credentials export --provider aws --format json)
AWS_KEY=$(echo $CREDS | jq -r '.access_key')
AWS_SECRET=$(echo $CREDS | jq -r '.secret_access_key')
```

**Security Considerations**:
- Credentials are read from SOPS-encrypted configuration
- Ensure SOPS Age key is available (`secrets.sops_age_key_file`)
- Never commit exported credentials to version control
- Use `credentials unset` to clear credentials after use
- Consider using `cluster select --activate` for temporary credential activation

---

#### cluster credentials unset

Generate shell commands to unset cloud provider credentials from the environment.

**Usage**:

```bash
# Unset AWS credentials
eval $(openCenter cluster credentials unset --provider aws)

# Unset OpenStack credentials
eval $(openCenter cluster credentials unset --provider openstack)

# Unset all cloud provider credentials
eval $(openCenter cluster credentials unset --provider all)

# Preview unset commands without executing
openCenter cluster credentials unset --provider aws
```

**Flags**:
- `--provider <name>`: Cloud provider to unset (aws, openstack, all) [default: all]

**Environment Variables Cleared**:

**AWS**:
- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_DEFAULT_REGION`
- `AWS_SESSION_TOKEN`

**OpenStack**:
- `OS_AUTH_URL`
- `OS_USERNAME`
- `OS_PASSWORD`
- `OS_PROJECT_NAME`
- `OS_USER_DOMAIN_NAME`
- `OS_PROJECT_DOMAIN_NAME`
- `OS_APPLICATION_CREDENTIAL_ID`
- `OS_APPLICATION_CREDENTIAL_SECRET`

**Example Output**:

```bash
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset AWS_DEFAULT_REGION
unset AWS_SESSION_TOKEN
```

**Common Workflows**:

**1. Switch between clusters**:
```bash
# Work with prod cluster
eval $(openCenter cluster credentials export prod-cluster --provider aws)
terraform apply

# Switch to dev cluster
eval $(openCenter cluster credentials unset --provider aws)
eval $(openCenter cluster credentials export dev-cluster --provider aws)
terraform apply
```

**2. Clean up after operations**:
```bash
# Export credentials
eval $(openCenter cluster credentials export --provider all)

# Perform operations
ansible-playbook deploy.yml

# Clean up
eval $(openCenter cluster credentials unset --provider all)
```

---

## Why Commands are Hidden

Commands are hidden for several reasons:

1. **Focus**: Keep the main help menu focused on primary user workflows (init → validate → setup → bootstrap)
2. **Complexity**: Advanced commands may confuse new users
3. **Deprecation**: Commands superseded by better alternatives (e.g., `credentials` → `select --activate`)
4. **Development**: Internal tools primarily used by developers and CI/CD
5. **Stability**: Commands that may change or be removed in future versions

## Discovering Hidden Commands

**Method 1: Source Code**
```bash
# Search for hidden commands
grep -r "Hidden: true" cmd/
```

**Method 2: This Documentation**

This document is the authoritative reference for all hidden commands.

**Method 3: Tab Completion**

Shell completion may still suggest hidden commands even though they don't appear in help.

## Using Hidden Commands in Scripts

Hidden commands are stable and safe to use in scripts and CI/CD pipelines. They follow semantic versioning and breaking changes will be documented in release notes.

**Example CI/CD Usage**:

```yaml
# .github/workflows/validate.yml
name: Validate Cluster Config

on: [push, pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Install openCenter
        run: |
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 -o openCenter
          chmod +x openCenter
          sudo mv openCenter /usr/local/bin/
      
      - name: Generate Schema
        run: openCenter cluster schema --out schema.json
      
      - name: Validate Against Schema
        run: |
          # Use schema for validation
          npm install -g ajv-cli
          ajv validate -s schema.json -d cluster-config.yaml
```

## Migration Guide

If you're using deprecated hidden commands, here's how to migrate:

### cluster credentials → cluster select --activate

**Old way** (still works):
```bash
eval $(openCenter cluster credentials export prod-cluster --provider aws)
terraform apply
eval $(openCenter cluster credentials unset --provider aws)
```

**New way** (recommended):
```bash
openCenter cluster select prod-cluster --activate
terraform apply
# Credentials automatically scoped to cluster context
```

**Benefits of new approach**:
- Simpler syntax
- Automatic credential scoping
- Integrated with cluster context
- No manual cleanup needed

## Related Documentation

- [CLI Commands Reference](../reference/cli-commands.md) - All visible commands
- [IDE Integration](../how-to/ide-integration.md) - Using schema for IDE features
- [Configuration Reference](../reference/configuration.md) - Cluster configuration structure
- [Secrets Management](../reference/secrets.md) - SOPS encryption and credential handling
- [Developer Guide](./README.md) - Development setup and workflows

## See Also

- [Contributing Guide](./contributing.md) - How to add new commands
- [Testing Guide](./testing/README.md) - Testing command implementations
- [Architecture](./architecture.md) - Command structure and patterns
