---
id: exit-codes
title: "Exit Codes"
sidebar_label: Exit Codes
description: Complete reference of CLI exit codes for error handling in scripts and CI/CD pipelines.
doc_type: reference
audience: "developers, devops engineers"
tags: [exit-codes, errors, automation, ci-cd]
---

# Exit Codes

**Purpose:** For automation users, provides complete reference of CLI exit codes for error handling in scripts and CI/CD pipelines.

This reference documents all exit codes returned by openCenter CLI commands.

## Overview

openCenter CLI uses standard Unix exit codes to indicate command success or failure. Exit codes enable automated error handling in scripts and CI/CD pipelines.

**Convention:**
- `0`: Success
- `1-255`: Error (specific error indicated by code)

**Usage in scripts:**
```bash
#!/bin/bash
opencenter cluster validate my-cluster
if [ $? -eq 0 ]; then
    echo "Validation passed"
else
    echo "Validation failed with exit code $?"
    exit 1
fi
```

## Standard Exit Codes

### 0 - Success

Command completed successfully.

**Example:**
```bash
opencenter cluster validate my-cluster
echo $?  # Output: 0
```

**When returned:**
- Configuration validation passed
- Cluster operation completed successfully
- Command executed without errors

### 1 - General Error

General error or unspecified failure.

**Example:**
```bash
opencenter cluster validate invalid-cluster
echo $?  # Output: 1
```

**When returned:**
- Command failed for unspecified reason
- Generic error condition
- Catch-all for errors without specific code

**Common causes:**
- Invalid command syntax
- Missing required arguments
- Unexpected error condition

### 2 - Configuration Error

Configuration file error or validation failure.

**Example:**
```bash
opencenter cluster validate my-cluster
echo $?  # Output: 2
```

**When returned:**
- Configuration file not found
- Configuration syntax error (invalid YAML)
- Schema validation failed
- Business rules validation failed

**Common causes:**
- Missing configuration file
- Invalid YAML syntax
- Required fields missing
- Type mismatches
- Validation rule violations

### 3 - Provider Error

Infrastructure provider error.

**Example:**
```bash
opencenter cluster deploy my-cluster
echo $?  # Output: 3
```

**When returned:**
- Provider authentication failed
- Provider API error
- Provider resource not found
- Provider quota exceeded

**Common causes:**
- Invalid credentials
- API endpoint unreachable
- Resource doesn't exist (image, flavor, network)
- Insufficient quota

### 4 - Network Error

Network connectivity error.

**Example:**
```bash
opencenter cluster validate my-cluster --check-connectivity
echo $?  # Output: 4
```

**When returned:**
- Network connection failed
- API endpoint unreachable
- DNS resolution failed
- Timeout

**Common causes:**
- No internet connection
- Firewall blocking connection
- API endpoint down
- DNS misconfiguration

### 5 - Permission Error

Permission or authentication error.

**Example:**
```bash
opencenter cluster deploy my-cluster
echo $?  # Output: 5
```

**When returned:**
- Insufficient permissions
- Authentication failed
- Authorization denied
- File permission error

**Common causes:**
- Invalid credentials
- Insufficient provider permissions
- File not readable/writable
- SSH key permission error

### 6 - Resource Error

Resource not found or unavailable.

**Example:**
```bash
opencenter cluster status non-existent-cluster
echo $?  # Output: 6
```

**When returned:**
- Cluster not found
- Configuration file not found
- Resource doesn't exist

**Common causes:**
- Cluster name incorrect
- Configuration file missing
- Resource deleted

### 7 - Dependency Error

Missing dependency or tool.

**Example:**
```bash
opencenter cluster deploy my-cluster
echo $?  # Output: 7
```

**When returned:**
- Required tool not installed
- Dependency missing
- Version incompatibility

**Common causes:**
- Terraform not installed
- Ansible not installed
- kubectl not installed
- Incompatible tool version

### 8 - Timeout Error

Operation timed out.

**Example:**
```bash
opencenter cluster deploy my-cluster --timeout 30m
echo $?  # Output: 8
```

**When returned:**
- Operation exceeded timeout
- Long-running operation failed to complete

**Common causes:**
- Cluster deploy timeout
- API request timeout
- Network timeout

### 9 - User Cancelled

User cancelled operation.

**Example:**
```bash
opencenter cluster destroy my-cluster
# User presses Ctrl+C
echo $?  # Output: 9
```

**When returned:**
- User interrupted operation (Ctrl+C)
- User declined confirmation prompt

**Common causes:**
- User pressed Ctrl+C
- User answered "no" to confirmation

### 10 - Validation Error

Specific validation error (distinct from configuration error).

**Example:**
```bash
opencenter cluster validate my-cluster
echo $?  # Output: 10
```

**When returned:**
- Validation checks failed
- Configuration invalid for deployment
- Pre-flight checks failed

**Common causes:**
- Business rule violations
- Provider validation failed
- Connectivity validation failed

## Exit Code Usage in Scripts

### Basic Error Handling

```bash
#!/bin/bash
set -e  # Exit on any error

opencenter cluster validate my-cluster
opencenter cluster generate my-cluster
opencenter cluster deploy my-cluster

echo "Cluster deployed successfully"
```

### Specific Error Handling

```bash
#!/bin/bash

opencenter cluster validate my-cluster
EXIT_CODE=$?

case $EXIT_CODE in
    0)
        echo "Validation passed"
        ;;
    2)
        echo "Configuration error - check configuration file"
        exit 1
        ;;
    3)
        echo "Provider error - check credentials and quotas"
        exit 1
        ;;
    4)
        echo "Network error - check connectivity"
        exit 1
        ;;
    *)
        echo "Unknown error (exit code: $EXIT_CODE)"
        exit 1
        ;;
esac
```

### Retry Logic

```bash
#!/bin/bash

MAX_RETRIES=3
RETRY_COUNT=0

while [ $RETRY_COUNT -lt $MAX_RETRIES ]; do
    opencenter cluster deploy my-cluster
    EXIT_CODE=$?
    
    if [ $EXIT_CODE -eq 0 ]; then
        echo "Cluster deployed successfully"
        exit 0
    elif [ $EXIT_CODE -eq 4 ]; then
        echo "Network error - retrying ($((RETRY_COUNT + 1))/$MAX_RETRIES)"
        RETRY_COUNT=$((RETRY_COUNT + 1))
        sleep 30
    else
        echo "Non-retryable error (exit code: $EXIT_CODE)"
        exit $EXIT_CODE
    fi
done

echo "Failed after $MAX_RETRIES retries"
exit 1
```

### CI/CD Integration

**GitHub Actions:**
```yaml
- name: Validate cluster
  run: opencenter cluster validate my-cluster
  continue-on-error: false

- name: Deploy cluster
  run: opencenter cluster deploy my-cluster
  if: success()
```

**GitLab CI:**
```yaml
validate:
  script:
    - opencenter cluster validate my-cluster
  allow_failure: false

deploy:
  script:
    - opencenter cluster deploy my-cluster
  needs: [validate]
  when: on_success
```

**Jenkins:**
```groovy
stage('Validate') {
    steps {
        script {
            def exitCode = sh(
                script: 'opencenter cluster validate my-cluster',
                returnStatus: true
            )
            if (exitCode != 0) {
                error("Validation failed with exit code ${exitCode}")
            }
        }
    }
}
```

## Exit Code Checking

### Check Last Exit Code

```bash
# Run command
opencenter cluster validate my-cluster

# Check exit code
echo $?

# Or save to variable
EXIT_CODE=$?
echo "Exit code: $EXIT_CODE"
```

### Check Exit Code in Conditional

```bash
# If statement
if opencenter cluster validate my-cluster; then
    echo "Validation passed"
else
    echo "Validation failed"
fi

# Or with explicit check
opencenter cluster validate my-cluster
if [ $? -eq 0 ]; then
    echo "Validation passed"
fi
```

### Check Multiple Commands

```bash
# Run multiple commands, check each
opencenter cluster validate my-cluster
VALIDATE_EXIT=$?

opencenter cluster generate my-cluster
SETUP_EXIT=$?

if [ $VALIDATE_EXIT -eq 0 ] && [ $SETUP_EXIT -eq 0 ]; then
    echo "All commands succeeded"
else
    echo "One or more commands failed"
    echo "Validate exit code: $VALIDATE_EXIT"
    echo "Setup exit code: $SETUP_EXIT"
    exit 1
fi
```

## Error Messages

Exit codes are accompanied by error messages on stderr:

```bash
# Redirect stderr to file
opencenter cluster validate my-cluster 2> error.log

# Check exit code and error message
EXIT_CODE=$?
if [ $EXIT_CODE -ne 0 ]; then
    echo "Command failed with exit code $EXIT_CODE"
    echo "Error message:"
    cat error.log
fi
```

## Best Practices

1. **Always check exit codes:** Don't assume commands succeed
2. **Use set -e in scripts:** Exit on first error
3. **Handle specific errors:** Different actions for different exit codes
4. **Log exit codes:** Record exit codes for debugging
5. **Retry transient errors:** Network errors (exit code 4) can be retried
6. **Don't retry permanent errors:** Configuration errors (exit code 2) need fixing
7. **Document expected exit codes:** In scripts and CI/CD pipelines
8. **Test error paths:** Verify error handling works correctly

## Troubleshooting

### Exit Code Not as Expected

**Symptom:** Command returns unexpected exit code

**Diagnosis:**
```bash
# Run command with verbose output
opencenter cluster validate my-cluster --verbose

# Check error message
opencenter cluster validate my-cluster 2>&1 | tee error.log

# Check exit code
echo $?
```

**Solution:**
- Read error message carefully
- Check command syntax
- Verify configuration file
- Check provider credentials

### Exit Code 0 But Command Failed

**Symptom:** Command returns 0 but didn't complete successfully

**Diagnosis:**
```bash
# Check command output
opencenter cluster validate my-cluster | tee output.log

# Verify expected output
grep "Configuration is valid" output.log
```

**Solution:**
- Check command output for warnings
- Verify expected behavior
- Report bug if exit code incorrect

### Script Exits Unexpectedly

**Symptom:** Script exits without completing

**Diagnosis:**
```bash
# Run script with set -x for debugging
bash -x script.sh

# Check which command failed
echo $?
```

**Solution:**
```bash
# Use set -e to exit on error
set -e

# Or handle errors explicitly
opencenter cluster validate my-cluster || {
    echo "Validation failed"
    exit 1
}
```

## Related Topics

- [CLI Commands](cli-commands.md) - Complete command reference
- [Integrate CI/CD](../how-to/integrate-ci-cd.md) - CI/CD integration
- [Troubleshoot Deployment](../how-to/troubleshoot-deployment.md) - Debug errors

---

## Evidence

This reference is based on:

- Exit code conventions: Unix/Linux standards
- CLI error handling: `cmd/` directory structure
- Error types: `internal/config/validator.go`, `internal/cloud/`
- CI/CD integration: Session 7 integrate-ci-cd.md
