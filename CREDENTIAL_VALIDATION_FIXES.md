# Credential Validation Fallback Fixes

## Problem Summary

Three BDD test scenarios are failing because validation is passing when it should fail:

1. **Missing cert-manager secrets** (`config_template_rendering.feature:491`)
2. **Missing loki secrets** (`config_template_rendering.feature:507`)  
3. **OpenTofu S3 backend requires credentials** (`validation.feature:22`)

## Root Cause

The issue is in the `defaultConfig()` function in `internal/config/config.go`. When `OPENCENTER_TEST_MODE=true` (which is set by BDD tests), the function populates service-specific secrets with dummy values:

```go
if isTestMode {
    certManagerAccessKey = "test-access-key"
    certManagerSecretKey = "test-secret-key"
    lokiSwiftPassword = "test-password"
    // ... etc
}
```

These dummy credentials are then used to initialize the `Secrets` struct, which means:
- When a config is loaded, it merges with defaults
- The test mode defaults include service-specific credentials
- Validation finds credentials (from defaults) even when user didn't provide them
- Tests expect validation to fail, but it passes

## Solution: Option 1 (Strict Validation) - IMPLEMENTED

**Do NOT populate service-specific secrets in test mode**. Only populate infrastructure-level credentials needed for basic operations.

### Changes Made

#### 1. `internal/config/config.go` - Remove Test Mode Service Secrets

Changed the test mode initialization to ONLY set infrastructure credentials:

```go
// Infrastructure credentials for test mode (OpenStack, AWS infrastructure)
// Note: Service-specific secrets (cert-manager, loki, etc.) are NOT populated
// in test mode to ensure validation tests work correctly
awsAccessKey := ""
awsSecretKey := ""

if isTestMode {
    authURL = "https://identity.example.com/v3"
    region = "RegionOne"
    tenantName = "admin"
    barbicanAuthURL = "https://identity.example.com/v3"

    // Only populate infrastructure-level AWS credentials for test mode
    // This allows OpenTofu S3 backend tests to work
    awsAccessKey = "test-aws-access-key"
    awsSecretKey = "test-aws-secret-key"
}
```

And updated the Secrets initialization to leave service-specific secrets empty:

```go
Secrets: Secrets{
    // ... SSH keys, etc ...
    Global: GlobalSecrets{
        AWS: AWSGlobalSecrets{
            Infrastructure: AWSSecrets{
                AccessKey:       awsAccessKey,
                SecretAccessKey: awsSecretKey,
                Region:          "us-east-1",
            },
            Application: AWSSecrets{
                AccessKey:       "",
                SecretAccessKey: "",
                Region:          "",
            },
        },
    },
    // Service-specific secrets - must be provided by user
    // These are intentionally left empty even in test mode to ensure
    // validation tests work correctly
    CertManager: CertManagerSecrets{
        AWSAccessKey:       "",
        AWSSecretAccessKey: "",
    },
    Loki: LokiSecrets{
        SwiftPassword: "",
    },
    // ... all other service secrets set to "" ...
}
```

#### 2. `internal/config/enhanced_validator.go` - Fix S3 Backend Validation

Changed `validateS3BackendConfiguration()` to check credentials directly without using fallback logic:

```go
func (v *EnhancedConfigValidator) validateS3BackendConfiguration(config *Config, aggregator *errors.ValidationAggregator) {
    s3 := config.OpenTofu.Backend.S3
    if s3.Bucket == "" || s3.Key == "" || s3.Region == "" {
        aggregator.AddError(errors.CreateValidationError(
            "opentofu.backend.s3",
            "S3 backend requires bucket, key, and region",
            "Set opentofu.backend.s3.bucket to S3 bucket name",
            "Set opentofu.backend.s3.key to state file path",
            "Set opentofu.backend.s3.region to AWS region",
        ))
    }

    // Validate AWS credentials for S3 backend - check actual fields without fallback
    // During validation, we require explicit credentials to be set
    legacyAccessKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSAccessKey)
    legacySecretKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSSecretAccessKey)
    infraAccessKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.AccessKey)
    infraSecretKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.SecretAccessKey)

    hasLegacyCredentials := legacyAccessKey != "" && legacySecretKey != ""
    hasInfraCredentials := infraAccessKey != "" && infraSecretKey != ""

    if !hasLegacyCredentials && !hasInfraCredentials {
        aggregator.AddError(errors.CreateCredentialError(
            "AWS",
            "opencenter.cluster.aws_access_key or secrets.global.aws.infrastructure.access_key",
            "AWS credentials required for S3/AWS backend: either set opencenter.cluster.aws_access_key/aws_secret_access_key or secrets.global.aws.infrastructure.access_key/secret_access_key",
            nil,
        ))
    }
}
```

## Validation Logic

The validation in `internal/config/validator.go` (`validateServiceSecrets()`) already checks service-specific credentials directly:

```go
// Validate cert-manager secrets
if isEnabled("cert-manager") {
    if config.Secrets.CertManager.AWSAccessKey == "" {
        result.Errors = append(result.Errors, &ConfigValidationError{
            Type:    "validation",
            Field:   "secrets.cert_manager.aws_access_key",
            Message: "cert-manager requires aws_access_key",
            // ...
        })
    }
    // ... similar for aws_secret_access_key
}
```

This validation is correct - it checks the actual field values. The problem was that test mode was populating these fields with dummy values.

## Testing

After applying these changes, run the failing tests:

```bash
mise run godog -- tests/features/config_template_rendering.feature:491
mise run godog -- tests/features/config_template_rendering.feature:507
mise run godog -- tests/features/validation.feature:22
```

Expected result: All three scenarios should now pass (validation should fail when credentials are missing).

## Impact Analysis

### Positive Impacts
- ✅ Validation correctly catches missing service-specific credentials
- ✅ Tests accurately reflect real-world validation behavior
- ✅ Users get proper error messages when credentials are missing

### Potential Concerns
- ⚠️ Some existing tests might rely on test mode providing dummy credentials
- ⚠️ Need to verify that infrastructure-level operations still work in test mode

### Files to Monitor
- Any tests that create configs in test mode and expect service secrets to be populated
- Integration tests that use test mode for full cluster operations

## Alternative Approach (Not Implemented)

**Option 2: Lenient Validation**
- Keep test mode populating all credentials
- Update test expectations to accept validation passing
- Add explicit checks in tests to verify fallback behavior

This was rejected because:
- It doesn't match real-world behavior (users won't have test mode)
- It hides configuration errors that should be caught early
- It makes tests less valuable for catching validation bugs

## Completion Checklist

- [x] Remove service-specific credential population from test mode
- [x] Update S3 backend validation to check credentials directly
- [ ] Run failing BDD tests to verify fixes
- [ ] Run full BDD test suite to check for regressions
- [ ] Run unit tests to verify no breakage
- [ ] Document changes in this file
- [ ] Create completion report

## Notes

- The `GetCertManagerAWSCredentials()`, `GetLokiS3Credentials()`, and `GetS3BackendCredentials()` functions still have fallback logic - this is correct for **runtime** use
- Validation should be **stricter** than runtime - it should catch missing configs early
- Test mode should simulate **user configuration**, not provide automatic defaults for everything
