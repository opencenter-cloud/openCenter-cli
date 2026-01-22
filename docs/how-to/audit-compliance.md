# Implementing Audit and Compliance Workflows


## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Understanding Audit Logging](#understanding-audit-logging)
- [Task 1: Enable Audit Logging](#task-1-enable-audit-logging)
- [Task 2: Query Audit Events](#task-2-query-audit-events)
- [Task 3: Verify Audit Log Integrity](#task-3-verify-audit-log-integrity)
- [Task 4: Implement Compliance Checks](#task-4-implement-compliance-checks)
- [Task 5: Implement Secret Scanning](#task-5-implement-secret-scanning)
- [Task 6: Export Audit Logs for Compliance](#task-6-export-audit-logs-for-compliance)
- [Task 7: Implement Continuous Compliance Monitoring](#task-7-implement-continuous-compliance-monitoring)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
**doc_type**: how-to  
**priority**: 3  
**audience**: Security engineers and compliance officers  
**related_docs**:
- [Security Model](../explanation/security-model.md)
- [SOPS Integration](./sops-integration.md)
- [CI/CD Integration](./cicd-integration.md)

## Overview

This guide shows you how to implement audit logging and compliance workflows with openCenter. You'll learn how to enable audit logging, query audit events, verify integrity, and integrate compliance checks into your deployment pipelines.

## Prerequisites

- openCenter CLI installed and configured
- Understanding of security compliance requirements
- Access to audit log storage location
- Familiarity with SOPS and secrets management

## Understanding Audit Logging

openCenter provides tamper-evident audit logging for security-critical operations:

1. **Event Logging**: Records all security-relevant operations
2. **Integrity Protection**: HMAC signatures prevent tampering
3. **Credential Masking**: Automatically masks sensitive data
4. **Compliance Reporting**: Query and export audit trails

### Audit Event Types

- `key.generated`: Encryption key generation
- `key.accessed`: Key access attempts
- `key.rotated`: Key rotation events
- `validation.failed`: Configuration validation failures
- `input.rejected`: Invalid input rejection
- `template.validation.failed`: Template validation errors

## Task 1: Enable Audit Logging

### Step 1: Configure Audit Logger

Audit logging is enabled by default. Configure the log path:

```bash
# Set custom audit log path
export OPENCENTER_AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"

# Verify audit log directory exists
mkdir -p "$(dirname "$OPENCENTER_AUDIT_LOG")"
```

### Step 2: Verify Audit Logging

Run a command that generates audit events:

```bash
# Build CLI
mise run build

# Generate Age key (creates audit event)
./bin/openCenter sops generate-key my-cluster

# Check audit log
cat ~/.config/openCenter/audit/audit.log
```

Expected output:
```json
{"id":"a1b2c3d4...","timestamp":"2024-01-15T10:30:00Z","event_type":"key.generated","actor":"user@example.com","resource":"my-cluster","action":"generate","result":"success","details":{"key_type":"age"},"signature":"abc123..."}
```

### Step 3: Configure Audit Log Rotation

Audit logs automatically rotate at 100MB. Configure retention:

```bash
# Logs are retained for 30 days by default
# Old logs are automatically cleaned up during rotation

# Manually trigger rotation (for testing)
# Logs rotate automatically when size limit is reached
```

## Task 2: Query Audit Events

### Step 1: View Recent Events

```bash
# View all audit events
cat ~/.config/openCenter/audit/audit.log | jq '.'

# View last 10 events
tail -n 10 ~/.config/openCenter/audit/audit.log | jq '.'

# View events from today
cat ~/.config/openCenter/audit/audit.log | \
  jq --arg date "$(date +%Y-%m-%d)" \
  'select(.timestamp | startswith($date))'
```

### Step 2: Filter by Event Type

```bash
# Find all key generation events
cat ~/.config/openCenter/audit/audit.log | \
  jq 'select(.event_type == "key.generated")'

# Find all validation failures
cat ~/.config/openCenter/audit/audit.log | \
  jq 'select(.event_type == "validation.failed")'

# Find all failed operations
cat ~/.config/openCenter/audit/audit.log | \
  jq 'select(.result == "failure")'
```

### Step 3: Filter by Actor

```bash
# Find events by specific user
cat ~/.config/openCenter/audit/audit.log | \
  jq --arg actor "user@example.com" \
  'select(.actor == $actor)'

# Find events by resource (cluster)
cat ~/.config/openCenter/audit/audit.log | \
  jq --arg resource "production-cluster" \
  'select(.resource == $resource)'
```

### Step 4: Create Audit Report

```bash
# Create audit report script
cat > generate-audit-report.sh <<'EOF'
#!/bin/bash

AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"
REPORT_FILE="audit-report-$(date +%Y%m%d).txt"

echo "Audit Report - $(date)" > "$REPORT_FILE"
echo "================================" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Total events
echo "Total Events: $(wc -l < "$AUDIT_LOG")" >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Events by type
echo "Events by Type:" >> "$REPORT_FILE"
cat "$AUDIT_LOG" | jq -r '.event_type' | sort | uniq -c | \
  awk '{printf "  %-30s %d\n", $2, $1}' >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Failed operations
echo "Failed Operations:" >> "$REPORT_FILE"
cat "$AUDIT_LOG" | jq -r 'select(.result == "failure") | 
  "\(.timestamp) - \(.event_type) - \(.resource)"' >> "$REPORT_FILE"
echo "" >> "$REPORT_FILE"

# Key operations
echo "Key Operations:" >> "$REPORT_FILE"
cat "$AUDIT_LOG" | jq -r 'select(.event_type | startswith("key.")) | 
  "\(.timestamp) - \(.event_type) - \(.resource)"' >> "$REPORT_FILE"

echo "Report generated: $REPORT_FILE"
EOF

chmod +x generate-audit-report.sh
./generate-audit-report.sh
```

## Task 3: Verify Audit Log Integrity

### Step 1: Understand HMAC Signatures

Each audit event includes an HMAC signature for integrity verification:

```json
{
  "id": "event-id",
  "timestamp": "2024-01-15T10:30:00Z",
  "event_type": "key.generated",
  "signature": "abc123..."  // HMAC-SHA256 signature
}
```

### Step 2: Verify Signature Integrity

The audit logger automatically verifies signatures when querying events. Tampered events are detected:

```bash
# Integrity verification happens automatically
# If tampering is detected, warnings are logged to stderr

# Example: Manually verify integrity
cat ~/.config/openCenter/audit/audit.log | while read -r line; do
  # Each line is verified when parsed by the audit logger
  echo "$line" | jq -r '.id'
done 2>&1 | grep -i "signature verification failed"
```

### Step 3: Create Integrity Check Script

```bash
cat > verify-audit-integrity.sh <<'EOF'
#!/bin/bash

AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"

echo "Verifying audit log integrity..."
echo "Log file: $AUDIT_LOG"
echo ""

# Count total events
TOTAL=$(wc -l < "$AUDIT_LOG")
echo "Total events: $TOTAL"

# Check for malformed JSON
MALFORMED=$(cat "$AUDIT_LOG" | while read -r line; do
  echo "$line" | jq empty 2>/dev/null || echo "malformed"
done | grep -c "malformed")

if [ "$MALFORMED" -gt 0 ]; then
  echo "⚠️  Warning: $MALFORMED malformed entries detected"
else
  echo "✓ All entries are valid JSON"
fi

# Check for missing signatures
MISSING_SIG=$(cat "$AUDIT_LOG" | jq -r 'select(.signature == null or .signature == "") | .id' | wc -l)

if [ "$MISSING_SIG" -gt 0 ]; then
  echo "⚠️  Warning: $MISSING_SIG entries missing signatures"
else
  echo "✓ All entries have signatures"
fi

# Check for duplicate IDs
DUPLICATES=$(cat "$AUDIT_LOG" | jq -r '.id' | sort | uniq -d | wc -l)

if [ "$DUPLICATES" -gt 0 ]; then
  echo "⚠️  Warning: $DUPLICATES duplicate event IDs detected"
else
  echo "✓ No duplicate event IDs"
fi

echo ""
echo "Integrity check complete"
EOF

chmod +x verify-audit-integrity.sh
./verify-audit-integrity.sh
```

## Task 4: Implement Compliance Checks

### Step 1: Create Compliance Check Script

```bash
cat > compliance-check.sh <<'EOF'
#!/bin/bash

set -e

CLUSTER_CONFIG="$1"
CLUSTER_NAME=$(basename "$CLUSTER_CONFIG" .yaml)

echo "Running compliance checks for: $CLUSTER_NAME"
echo "=========================================="
echo ""

# Check 1: No plaintext secrets
echo "✓ Checking for plaintext secrets..."
if grep -r "password:\|secret:\|token:" "$CLUSTER_CONFIG" | grep -v "sops" | grep -v "#"; then
  echo "❌ FAIL: Plaintext secrets detected"
  exit 1
fi
echo "  PASS: No plaintext secrets found"
echo ""

# Check 2: SOPS encryption enabled
echo "✓ Checking SOPS encryption..."
if ! grep -q "sops:" "$CLUSTER_CONFIG"; then
  echo "❌ FAIL: SOPS encryption not configured"
  exit 1
fi
echo "  PASS: SOPS encryption configured"
echo ""

# Check 3: Audit logging enabled
echo "✓ Checking audit logging..."
AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"
if [ ! -f "$AUDIT_LOG" ]; then
  echo "⚠️  WARNING: Audit log not found"
else
  echo "  PASS: Audit logging enabled"
fi
echo ""

# Check 4: Configuration validation
echo "✓ Validating configuration..."
if ! openCenter config validate --config "$CLUSTER_CONFIG" > /dev/null 2>&1; then
  echo "❌ FAIL: Configuration validation failed"
  exit 1
fi
echo "  PASS: Configuration is valid"
echo ""

# Check 5: Required services enabled
echo "✓ Checking required services..."
REQUIRED_SERVICES=("cert-manager" "kube-prometheus-stack")
for service in "${REQUIRED_SERVICES[@]}"; do
  if ! grep -q "${service}:" "$CLUSTER_CONFIG"; then
    echo "⚠️  WARNING: Required service $service not found"
  else
    echo "  PASS: $service configured"
  fi
done
echo ""

echo "=========================================="
echo "Compliance check complete: PASSED"
EOF

chmod +x compliance-check.sh
```

### Step 2: Run Compliance Checks

```bash
# Run compliance check on cluster configuration
./compliance-check.sh clusters/production-cluster.yaml

# Run on all clusters
for config in clusters/*.yaml; do
  echo "Checking: $config"
  ./compliance-check.sh "$config"
  echo ""
done
```

### Step 3: Integrate with CI/CD

Add to `.github/workflows/compliance.yaml`:

```yaml
name: Compliance Checks

on:
  pull_request:
    paths:
      - 'clusters/**/*.yaml'
  schedule:
    - cron: '0 0 * * *'  # Daily

jobs:
  compliance:
    runs-on: ubuntu-latest
    
    steps:
      - name: Checkout code
        uses: actions/checkout@v4
      
      - name: Install openCenter CLI
        run: |
          curl -L https://github.com/rackerlabs/openCenter-cli/releases/latest/download/openCenter-linux-amd64 \
            -o /usr/local/bin/openCenter
          chmod +x /usr/local/bin/openCenter
      
      - name: Run compliance checks
        run: |
          for config in clusters/*.yaml; do
            echo "Checking: $config"
            ./compliance-check.sh "$config"
          done
      
      - name: Generate compliance report
        if: always()
        run: |
          echo "## Compliance Report" >> $GITHUB_STEP_SUMMARY
          echo "" >> $GITHUB_STEP_SUMMARY
          echo "- Configurations checked: $(find clusters -name '*.yaml' | wc -l)" >> $GITHUB_STEP_SUMMARY
          echo "- Status: ${{ job.status }}" >> $GITHUB_STEP_SUMMARY
```

## Task 5: Implement Secret Scanning

### Step 1: Create Secret Scanner

```bash
cat > scan-secrets.sh <<'EOF'
#!/bin/bash

set -e

TARGET_DIR="${1:-.}"

echo "Scanning for exposed secrets in: $TARGET_DIR"
echo "=============================================="
echo ""

# Patterns to detect
PATTERNS=(
  "password\s*[:=]\s*['\"]?[^'\"[:space:]]{8,}"
  "secret\s*[:=]\s*['\"]?[^'\"[:space:]]{8,}"
  "token\s*[:=]\s*['\"]?[^'\"[:space:]]{16,}"
  "api[_-]?key\s*[:=]\s*['\"]?[^'\"[:space:]]{16,}"
  "AKIA[0-9A-Z]{16}"  # AWS Access Key
  "AGE-SECRET-KEY-[A-Z0-9]{59}"  # Age secret key
)

FOUND=0

for pattern in "${PATTERNS[@]}"; do
  echo "Checking pattern: $pattern"
  
  # Search for pattern, excluding SOPS-encrypted files
  MATCHES=$(grep -rniE "$pattern" "$TARGET_DIR" \
    --include="*.yaml" \
    --include="*.yml" \
    --include="*.json" \
    --exclude-dir=".git" \
    --exclude-dir="node_modules" | \
    grep -v "sops:" || true)
  
  if [ -n "$MATCHES" ]; then
    echo "⚠️  Potential secrets found:"
    echo "$MATCHES"
    FOUND=$((FOUND + 1))
  fi
done

echo ""
echo "=============================================="

if [ $FOUND -gt 0 ]; then
  echo "❌ FAIL: $FOUND potential secret(s) detected"
  exit 1
else
  echo "✓ PASS: No exposed secrets detected"
fi
EOF

chmod +x scan-secrets.sh
```

### Step 2: Run Secret Scanner

```bash
# Scan current directory
./scan-secrets.sh

# Scan specific directory
./scan-secrets.sh clusters/

# Scan before commit
git diff --cached | ./scan-secrets.sh
```

### Step 3: Add Pre-commit Hook

```bash
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash

echo "Running secret scan..."

# Scan staged files
STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM | grep -E '\.(yaml|yml|json)$' || true)

if [ -n "$STAGED_FILES" ]; then
  for file in $STAGED_FILES; do
    # Check for plaintext secrets
    if grep -E "password:|secret:|token:" "$file" | grep -v "sops:" | grep -v "#"; then
      echo "❌ ERROR: Plaintext secrets detected in $file"
      echo "Please encrypt secrets using SOPS before committing"
      exit 1
    fi
  done
fi

echo "✓ Secret scan passed"
EOF

chmod +x .git/hooks/pre-commit
```

## Task 6: Export Audit Logs for Compliance

### Step 1: Create Export Script

```bash
cat > export-audit-logs.sh <<'EOF'
#!/bin/bash

AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"
START_DATE="${1:-$(date -d '30 days ago' +%Y-%m-%d)}"
END_DATE="${2:-$(date +%Y-%m-%d)}"
OUTPUT_FILE="audit-export-${START_DATE}-to-${END_DATE}.json"

echo "Exporting audit logs from $START_DATE to $END_DATE"

cat "$AUDIT_LOG" | jq --arg start "$START_DATE" --arg end "$END_DATE" \
  'select(.timestamp >= $start and .timestamp <= $end)' > "$OUTPUT_FILE"

echo "Exported $(wc -l < "$OUTPUT_FILE") events to $OUTPUT_FILE"

# Create CSV format for spreadsheet import
CSV_FILE="${OUTPUT_FILE%.json}.csv"
echo "timestamp,event_type,actor,resource,action,result" > "$CSV_FILE"
cat "$OUTPUT_FILE" | jq -r \
  '[.timestamp, .event_type, .actor, .resource, .action, .result] | @csv' \
  >> "$CSV_FILE"

echo "CSV export: $CSV_FILE"
EOF

chmod +x export-audit-logs.sh
```

### Step 2: Export Logs

```bash
# Export last 30 days
./export-audit-logs.sh

# Export specific date range
./export-audit-logs.sh 2024-01-01 2024-01-31

# Export for compliance audit
./export-audit-logs.sh 2024-01-01 2024-12-31
```

### Step 3: Archive Audit Logs

```bash
# Create archive script
cat > archive-audit-logs.sh <<'EOF'
#!/bin/bash

AUDIT_DIR="${HOME}/.config/openCenter/audit"
ARCHIVE_DIR="${AUDIT_DIR}/archives"
CURRENT_LOG="${AUDIT_DIR}/audit.log"

mkdir -p "$ARCHIVE_DIR"

# Archive current log
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
ARCHIVE_FILE="${ARCHIVE_DIR}/audit-${TIMESTAMP}.log.gz"

gzip -c "$CURRENT_LOG" > "$ARCHIVE_FILE"

echo "Archived to: $ARCHIVE_FILE"
echo "Archive size: $(du -h "$ARCHIVE_FILE" | cut -f1)"

# List archives
echo ""
echo "Available archives:"
ls -lh "$ARCHIVE_DIR"
EOF

chmod +x archive-audit-logs.sh
./archive-audit-logs.sh
```

## Task 7: Implement Continuous Compliance Monitoring

### Step 1: Create Monitoring Script

```bash
cat > monitor-compliance.sh <<'EOF'
#!/bin/bash

AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"
ALERT_THRESHOLD=5  # Alert if more than 5 failures in last hour

echo "Monitoring compliance status..."

# Check recent failures
RECENT_FAILURES=$(cat "$AUDIT_LOG" | \
  jq --arg since "$(date -d '1 hour ago' -Iseconds)" \
  'select(.timestamp >= $since and .result == "failure")' | \
  wc -l)

if [ "$RECENT_FAILURES" -gt "$ALERT_THRESHOLD" ]; then
  echo "⚠️  ALERT: $RECENT_FAILURES failures in last hour (threshold: $ALERT_THRESHOLD)"
  
  # Show recent failures
  echo ""
  echo "Recent failures:"
  cat "$AUDIT_LOG" | \
    jq --arg since "$(date -d '1 hour ago' -Iseconds)" \
    'select(.timestamp >= $since and .result == "failure") | 
    "\(.timestamp) - \(.event_type) - \(.resource)"'
  
  exit 1
else
  echo "✓ Compliance status: OK ($RECENT_FAILURES failures in last hour)"
fi
EOF

chmod +x monitor-compliance.sh
```

### Step 2: Schedule Monitoring

```bash
# Add to crontab for hourly monitoring
crontab -e

# Add line:
# 0 * * * * /path/to/monitor-compliance.sh >> /var/log/compliance-monitor.log 2>&1
```

## Best Practices

1. **Enable Audit Logging**: Always enable audit logging in production
2. **Protect Audit Logs**: Store audit logs in secure, tamper-evident storage
3. **Regular Reviews**: Review audit logs regularly for anomalies
4. **Automate Compliance**: Integrate compliance checks into CI/CD pipelines
5. **Encrypt Secrets**: Always use SOPS for secret encryption
6. **Scan for Leaks**: Regularly scan for exposed secrets
7. **Archive Logs**: Archive and retain audit logs per compliance requirements
8. **Monitor Continuously**: Set up continuous compliance monitoring
9. **Document Procedures**: Maintain clear compliance documentation
10. **Test Recovery**: Regularly test audit log recovery procedures

## Troubleshooting

### Audit Log Not Created

**Problem**: Audit log file doesn't exist

**Solution**: Verify audit logging is enabled and directory exists:
```bash
mkdir -p ~/.config/openCenter/audit
export OPENCENTER_AUDIT_LOG="${HOME}/.config/openCenter/audit/audit.log"
```

### Signature Verification Fails

**Problem**: Audit events show signature verification failures

**Solution**: This indicates potential tampering. Investigate immediately:
```bash
# Check for modified entries
cat ~/.config/openCenter/audit/audit.log | \
  jq 'select(.signature == null or .signature == "")'

# Restore from backup if tampering confirmed
cp ~/.config/openCenter/audit/audit.log.backup \
   ~/.config/openCenter/audit/audit.log
```

### Compliance Check Fails

**Problem**: Compliance checks fail unexpectedly

**Solution**: Review specific failure and remediate:
```bash
# Run with verbose output
./compliance-check.sh clusters/my-cluster.yaml

# Check configuration validation
mise run build
./bin/openCenter config validate --config clusters/my-cluster.yaml
```

## Next Steps

- [SOPS Integration](./sops-integration.md) - Learn secrets management
- [CI/CD Integration](./cicd-integration.md) - Automate compliance checks
- [Security Model](../explanation/security-model.md) - Understand security architecture
- [Disaster Recovery](./disaster-recovery.md) - Implement backup procedures
