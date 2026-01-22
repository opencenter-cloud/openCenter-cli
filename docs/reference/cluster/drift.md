# cluster drift


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Subcommands](#subcommands)
- [Drift Severity Levels](#drift-severity-levels)
- [Drift Detection Process](#drift-detection-process)
- [Use Cases](#use-cases)
- [Implementation Status](#implementation-status)
- [See Also](#see-also)
**doc_type:** reference

Detect and reconcile infrastructure drift between desired configuration and actual state.

## Synopsis

```bash
openCenter cluster drift <subcommand> [flags]
```

## Description

The `cluster drift` command detects and reconciles differences between desired cluster configuration and actual infrastructure state. It compares configuration with cloud resources (VMs, networks, security groups, load balancers) and reports discrepancies.

**Status:** Drift detection requires cloud provider implementation (not yet available).

## Subcommands

### detect

Detect infrastructure drift for a cluster.

```bash
openCenter cluster drift detect <cluster> [flags]
```

**Flags:**
- `--output string` - Output format (text, json, yaml) (default: "text")
- `--severity string` - Filter by severity (critical, warning, info)

**Examples:**

```bash
# Detect drift for a cluster
openCenter cluster drift detect my-cluster

# Output drift report as JSON
openCenter cluster drift detect my-cluster --output=json

# Show only critical drift
openCenter cluster drift detect my-cluster --severity=critical
```

**Drift Report Contents:**

- Resource type and ID
- Field that has drifted
- Expected vs actual values
- Severity (critical, warning, info)
- Whether the drift is reconcilable

**Output Format (text):**

```
Drift Report for Cluster: my-cluster
Detected At: 2026-01-18T14:30:00Z
Overall Severity: warning
Reconcilable: true

Summary:
  Total Drifts: 3
  Critical: 0
  Warning: 2
  Info: 1
  Reconcilable: 3

Drifts:
  1. VM instance-1 (i-1234567890abcdef0)
     Field: flavor
     Expected: m1.large
     Actual: m1.medium
     Severity: warning
     Reconcilable: true

  2. Network private-net (net-abc123)
     Field: mtu
     Expected: 1500
     Actual: 1450
     Severity: info
     Reconcilable: true
```

**Output Format (json):**

```json
{
  "id": "drift-20260118-143000",
  "cluster": "my-cluster",
  "detected_at": "2026-01-18T14:30:00Z",
  "severity": "warning",
  "reconcilable": true,
  "summary": {
    "total_drifts": 3,
    "critical_count": 0,
    "warning_count": 2,
    "info_count": 1,
    "reconcilable_count": 3
  },
  "drifts": [
    {
      "resource_type": "VM",
      "resource_name": "instance-1",
      "resource_id": "i-1234567890abcdef0",
      "field": "flavor",
      "expected": "m1.large",
      "actual": "m1.medium",
      "severity": "warning",
      "reconcilable": true
    }
  ]
}
```

### reconcile

Reconcile detected infrastructure drift.

```bash
openCenter cluster drift reconcile <cluster> [flags]
```

**Flags:**
- `--dry-run` - Show what would be changed without applying
- `--confirm` - Prompt for confirmation before applying changes

**Examples:**

```bash
# Show what would be reconciled (dry-run)
openCenter cluster drift reconcile my-cluster --dry-run

# Apply reconciliation
openCenter cluster drift reconcile my-cluster

# Reconcile with confirmation prompt
openCenter cluster drift reconcile my-cluster --confirm
```

**Behavior:**

1. Detects drift
2. Identifies reconcilable drift
3. Applies changes to bring infrastructure back to desired state
4. Reports results

**Reconcilable vs Non-Reconcilable Drift:**

**Reconcilable:**
- Configuration changes (flavor, size, tags)
- Network settings (MTU, DNS)
- Security group rules

**Non-Reconcilable:**
- Deleted resources (requires recreation)
- Manual resource creation (requires removal or adoption)
- Incompatible state changes

### schedule

Schedule periodic drift detection (not yet implemented).

```bash
openCenter cluster drift schedule <cluster> [flags]
```

**Flags:**
- `--interval string` - Interval between drift checks (default: "24h")
- `--callback string` - Callback URL for drift reports

**Examples:**

```bash
# Schedule drift detection every 24 hours
openCenter cluster drift schedule my-cluster --interval=24h

# Schedule with custom callback
openCenter cluster drift schedule my-cluster --interval=12h --callback=https://example.com/drift
```

**Status:** This feature is not yet implemented and will be available in a future release.

## Drift Severity Levels

### Critical
- Resource deletion
- Security group rule removal
- Network isolation
- Data loss risk

### Warning
- Configuration mismatch
- Performance degradation
- Non-optimal settings

### Info
- Minor configuration differences
- Cosmetic changes
- Non-impactful drift

## Drift Detection Process

1. Load cluster configuration
2. Query cloud provider APIs for actual resource state
3. Compare desired vs actual state
4. Classify differences by severity
5. Determine reconcilability
6. Generate drift report

## Use Cases

### Detect Manual Changes
Identify resources modified outside of openCenter:
```bash
openCenter cluster drift detect my-cluster
```

### Validate Infrastructure
Verify infrastructure matches configuration after manual operations:
```bash
openCenter cluster drift detect my-cluster --severity=critical
```

### Automated Monitoring
Schedule periodic drift checks:
```bash
openCenter cluster drift schedule my-cluster --interval=24h
```

### Reconcile Drift
Automatically fix reconcilable drift:
```bash
openCenter cluster drift reconcile my-cluster
```

## Implementation Status

**Current Status:** Requires cloud provider implementation

**Planned Features:**
- OpenStack drift detection
- AWS drift detection
- Automated reconciliation
- Scheduled drift checks
- Webhook notifications
- Drift history tracking

## See Also

- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster status](status.md) - Show cluster status
- [cluster info](info.md) - Display cluster information
