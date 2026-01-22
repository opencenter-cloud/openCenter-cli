# cluster credentials


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Supported Providers](#supported-providers)
- [Configuration Sources](#configuration-sources)
- [Subcommands](#subcommands)
- [Common Workflow](#common-workflow)
- [Security Considerations](#security-considerations)
- [Migration to cluster select](#migration-to-cluster-select)
- [See Also](#see-also)
**doc_type:** reference

Manage cloud provider credentials from cluster configuration.

## Synopsis

```bash
openCenter cluster credentials <subcommand> [flags]
```

## Description

The `cluster credentials` command manages cloud provider credentials extracted from cluster configuration. It provides subcommands for exporting and unsetting credentials in various formats.

**Note:** This command is superseded by `cluster select --activate` which provides the same functionality with a simpler interface. The `credentials` command is hidden from help but maintained for backward compatibility.

## Supported Providers

- `aws` - Amazon Web Services credentials
- `openstack` - OpenStack application credentials
- `all` - All configured providers

## Configuration Sources

### AWS
- Configuration: `opencenter.infrastructure.cloud.aws`
- Secrets: `secrets.global.aws.infrastructure`

### OpenStack
- Configuration: `opencenter.infrastructure.cloud.openstack`

## Subcommands

### export

Export cloud provider credentials as environment variables.

```bash
openCenter cluster credentials export [cluster] [flags]
```

**Flags:**
- `--provider string` - Provider to export (aws, openstack, all)
- `--format string` - Output format (env, json, yaml)

**Examples:**

```bash
# Export AWS credentials for current cluster
eval $(openCenter cluster credentials export --provider aws)

# Export OpenStack credentials for specific cluster
eval $(openCenter cluster credentials export my-cluster --provider openstack)

# Export all credentials in JSON format
openCenter cluster credentials export --provider all --format json

# Export credentials in YAML format
openCenter cluster credentials export --provider all --format yaml
```

**Output Formats:**

**env (default):**
```bash
export AWS_ACCESS_KEY_ID="AKIA..."
export AWS_SECRET_ACCESS_KEY="..."
export AWS_DEFAULT_REGION="us-east-1"
```

**json:**
```json
{
  "aws": {
    "access_key_id": "AKIA...",
    "secret_access_key": "...",
    "region": "us-east-1"
  }
}
```

**yaml:**
```yaml
aws:
  access_key_id: AKIA...
  secret_access_key: ...
  region: us-east-1
```

### unset

Clear cloud provider credentials from environment.

```bash
openCenter cluster credentials unset [flags]
```

**Flags:**
- `--provider string` - Provider to unset (aws, openstack, all)

**Examples:**

```bash
# Clear AWS credentials from environment
eval $(openCenter cluster credentials unset --provider aws)

# Clear OpenStack credentials from environment
eval $(openCenter cluster credentials unset --provider openstack)

# Clear all credentials from environment
eval $(openCenter cluster credentials unset --provider all)
```

**Output:**
```bash
unset AWS_ACCESS_KEY_ID
unset AWS_SECRET_ACCESS_KEY
unset AWS_DEFAULT_REGION
```

## Common Workflow

1. Export credentials to environment variables
2. Use with other cloud tools (terraform, ansible, aws-cli, openstack-cli)
3. Unset credentials when done

```bash
# Export credentials
eval $(openCenter cluster credentials export --provider aws)

# Use AWS CLI
aws s3 ls

# Clear credentials
eval $(openCenter cluster credentials unset --provider aws)
```

## Security Considerations

- Credentials are sourced from SOPS-encrypted configuration
- Credentials are only exported to the current shell session
- Use `unset` to clear credentials when no longer needed
- Avoid logging or displaying credentials in plain text

## Migration to cluster select

The recommended approach is to use `cluster select --activate`:

```bash
# Old approach
eval $(openCenter cluster credentials export --provider aws)

# New approach (recommended)
eval $(openCenter cluster select my-cluster --activate)
```

The `select --activate` command:
- Automatically detects configured providers
- Exports all necessary credentials
- Sets the active cluster
- Provides a unified interface

## See Also

- [cluster select](../cli-commands.md#cluster-select) - Set active cluster and export credentials
- [cluster info](info.md) - Display cluster information
