# Configuration Precedence


## Table of Contents

- [Overview](#overview)
- [Precedence Order](#precedence-order)
- [Complete Resolution Flow](#complete-resolution-flow)
- [Practical Examples](#practical-examples)
- [Best Practices](#best-practices)
- [Related Commands](#related-commands)
- [See Also](#see-also)
This document describes the order of precedence for configuration values when initializing or managing clusters with openCenter CLI.

## Overview

openCenter CLI uses a layered configuration system where values can come from multiple sources. Understanding the precedence order helps you predict which values will be used when multiple sources provide the same configuration.

## Precedence Order

Configuration values are resolved in the following order, from **highest to lowest priority**:

### 1. Command-Line Flags (Highest Priority)

Explicit command-line flags always take precedence over all other configuration sources.

**Examples:**
```bash
# Override provider
openCenter cluster init my-cluster --type aws

# Override organization
openCenter cluster init my-cluster --org production

# Override any field using dot notation
openCenter cluster init my-cluster \
  --opencenter.meta.env=prod \
  --opencenter.meta.region=us-west-2 \
  --opencenter.cluster.kubernetes.version=1.31.4
```

**Applies to:**
- All `--flag` style arguments
- All `--field.path=value` dot notation overrides
- Special flags like `--org`, `--type`, `--force`, etc.

---

### 2. Configuration File

When using the `--config` flag to load an existing configuration file, values from that file are used.

**Example:**
```bash
# Load configuration from file
openCenter cluster init --config my-cluster-config.yaml

# Load from file but override specific values
openCenter cluster init --config template.yaml \
  --opencenter.meta.env=staging
```

**Applies to:**
- All fields defined in the YAML configuration file
- Overridden by command-line flags
- Takes precedence over CLI config defaults

---

### 3. CLI Config Defaults

Global defaults set in the CLI configuration file (`~/.config/openCenter/config.yaml`) are applied to specific fields when they are empty or still set to schema defaults.

**Setting CLI Defaults:**
```bash
# Set global defaults
openCenter config set defaults.provider openstack
openCenter config set defaults.region us-west-2
openCenter config set defaults.environment production
openCenter config set defaults.ssh_authorized_keys "ssh-ed25519 AAAA... user@host"
```

**When Applied:**

CLI defaults are applied intelligently based on field state:

| Field | Applied When |
|-------|--------------|
| `provider` | Empty or `--type` flag not used |
| `region` | Empty or still set to schema default (`sjc3`) |
| `environment` | Empty or still set to schema default (`dev`) |
| `ssh_authorized_keys` | No valid SSH keys present (empty or contains only empty strings) |

**Example Behavior:**
```bash
# CLI config has: defaults.region=us-west-2, defaults.environment=production

# This cluster will use CLI defaults
openCenter cluster init my-cluster
# Result: region=us-west-2, environment=production

# This cluster overrides with flags
openCenter cluster init my-cluster --opencenter.meta.region=eu-west-1
# Result: region=eu-west-1, environment=production (CLI default)

# This cluster loads from config file
openCenter cluster init --config existing.yaml
# Result: Uses values from existing.yaml, CLI defaults fill in empty fields
```

**Applies to:**
- `defaults.provider` → `opencenter.infrastructure.provider`
- `defaults.region` → `opencenter.meta.region`
- `defaults.environment` → `opencenter.meta.env`
- `defaults.ssh_authorized_keys` → `opencenter.cluster.ssh_authorized_keys`

---

### 4. Schema Defaults (Lowest Priority)

When no other source provides a value, the JSON schema defaults are used. These provide sensible baseline values for all fields.

**Common Schema Defaults:**
- `provider`: `openstack`
- `region`: `sjc3`
- `environment`: `dev`
- `ssh_authorized_keys`: `[""]` (empty array with one empty string)
- `kubernetes.version`: `1.33.5`
- `organization`: `opencenter`

**Applies to:**
- All fields defined in the JSON schema
- Lowest priority - overridden by everything else
- Ensures no required field is left undefined

---

## Complete Resolution Flow

Here's how a configuration value is resolved:

```
┌─────────────────────────────────────┐
│ 1. Start with Schema Defaults       │
│    (Baseline values for all fields) │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 2. Load Config File (if provided)   │
│    Overrides schema defaults         │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 3. Apply CLI Config Defaults        │
│    For empty/default fields only     │
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 4. Apply Command-Line Flags         │
│    Final overrides (highest priority)│
└──────────────┬──────────────────────┘
               ↓
┌─────────────────────────────────────┐
│ 5. Write Final Configuration        │
│    All values resolved               │
└─────────────────────────────────────┘
```

## Practical Examples

### Example 1: Using Only CLI Defaults

**Setup:**
```bash
openCenter config set defaults.provider aws
openCenter config set defaults.region us-east-1
openCenter config set defaults.environment production
```

**Command:**
```bash
openCenter cluster init my-cluster
```

**Result:**
- `provider`: `aws` (from CLI defaults)
- `region`: `us-east-1` (from CLI defaults)
- `environment`: `production` (from CLI defaults)
- All other fields: schema defaults

---

### Example 2: Config File + CLI Defaults

**CLI Config:**
```yaml
defaults:
  region: us-west-2
  environment: staging
```

**Config File (`template.yaml`):**
```yaml
opencenter:
  meta:
    region: eu-west-1  # Explicitly set
    # environment not set
```

**Command:**
```bash
openCenter cluster init --config template.yaml
```

**Result:**
- `region`: `eu-west-1` (from config file, overrides CLI default)
- `environment`: `staging` (from CLI defaults, fills empty field)

---

### Example 3: Full Override Chain

**CLI Config:**
```yaml
defaults:
  provider: openstack
  region: us-west-2
  environment: dev
```

**Config File:**
```yaml
opencenter:
  infrastructure:
    provider: aws
  meta:
    region: us-east-1
```

**Command:**
```bash
openCenter cluster init --config template.yaml \
  --opencenter.meta.region=eu-central-1 \
  --opencenter.meta.env=production
```

**Result:**
- `provider`: `aws` (from config file)
- `region`: `eu-central-1` (from command-line flag, highest priority)
- `environment`: `production` (from command-line flag, overrides CLI default)

---

### Example 4: SSH Keys Precedence

**CLI Config:**
```yaml
defaults:
  ssh_authorized_keys:
    - ssh-ed25519 AAAAC3... team@company.com
```

**Scenario A - New Cluster:**
```bash
openCenter cluster init my-cluster
```
**Result:** Uses SSH key from CLI defaults

**Scenario B - Config File with Keys:**
```yaml
opencenter:
  cluster:
    ssh_authorized_keys:
      - ssh-rsa AAAAB3... user@host
```
```bash
openCenter cluster init --config existing.yaml
```
**Result:** Uses SSH key from config file (not CLI defaults)

**Scenario C - Command-Line Override:**
```bash
openCenter cluster init my-cluster \
  --opencenter.cluster.ssh_authorized_keys="ssh-ed25519 AAAA... override@host"
```
**Result:** Uses SSH key from command-line flag

---

## Best Practices

### 1. Use CLI Defaults for Organization Standards

Set organization-wide defaults in CLI config:
```bash
openCenter config set defaults.provider openstack
openCenter config set defaults.region us-west-2
openCenter config set defaults.environment production
openCenter config set defaults.ssh_authorized_keys "$(cat ~/.ssh/id_ed25519.pub)"
```

### 2. Use Config Files for Cluster Templates

Create reusable templates for different cluster types:
```bash
# Create production template
openCenter cluster init prod-template --config prod-template.yaml

# Use template for new clusters
openCenter cluster init prod-cluster-1 --config prod-template.yaml
openCenter cluster init prod-cluster-2 --config prod-template.yaml
```

### 3. Use Command-Line Flags for One-Off Overrides

Override specific values without modifying templates:
```bash
openCenter cluster init test-cluster \
  --config prod-template.yaml \
  --opencenter.meta.env=test \
  --opencenter.cluster.kubernetes.version=1.30.0
```

### 4. Check Effective Configuration

View the final resolved configuration:
```bash
# After init, check what was actually set
openCenter cluster info my-cluster

# Or view the config file directly
cat ~/.config/openCenter/clusters/<org>/.my-cluster-config.yaml
```

---

## Related Commands

- `openCenter config view` - View CLI configuration and defaults
- `openCenter config set` - Set CLI configuration defaults
- `openCenter cluster init --help` - See all available flags
- `openCenter cluster info` - View resolved cluster configuration

---

## See Also

- [CLI Configuration](./configuration.md) - CLI configuration file format
- [Cluster Init Command](./cli-commands.md#cluster-init) - Detailed command reference
- [Configuration File Format](./file-formats.md) - Cluster configuration YAML format
