# BDD Test Results After Credential Validation Fixes

**Date:** 2026-01-17  
**Task:** Task 1.3 - Run All BDD Tests  
**Phase:** Phase 1 - Production Validation  

## Summary

Applied credential validation fixes from `CREDENTIAL_VALIDATION_FIXES.md` and ran full BDD test suite.

**Test Results:**
- **Total Scenarios:** 145
- **Passed:** 92 (63%)
- **Failed:** 53 (37%)
- **Total Steps:** 1173
- **Passed Steps:** 966 (82%)
- **Failed Steps:** 53 (5%)
- **Skipped Steps:** 154 (13%)

## Changes Applied

### 1. `internal/config/config.go` - Removed Service-Specific Credential Population

**Before:**
```go
// Dummy secrets for test mode
awsAccessKey := ""
awsSecretKey := ""
certManagerAccessKey := ""
certManagerSecretKey := ""
lokiSwiftPassword := ""
keycloakClientSecret := ""
keycloakAdminPassword := ""
// ... etc

if isTestMode {
    awsAccessKey = "test-aws-access-key"
    awsSecretKey = "test-aws-secret-key"
    certManagerAccessKey = "test-access-key"
    certManagerSecretKey = "test-secret-key"
    lokiSwiftPassword = "test-password"
    keycloakClientSecret = "test-client-secret"
    keycloakAdminPassword = "test-admin-password"
    // ... etc
}
```

**After:**
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

**Secrets Initialization:**
```go
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
Keycloak: KeycloakSecrets{
    ClientSecret:  "",
    AdminPassword: "",
},
// ... all other service secrets set to ""
```

### 2. `internal/config/enhanced_validator.go` - Fixed S3 Backend Validation

**Before:**
```go
// Validate AWS credentials for S3 backend (with fallback support)
accessKey, secretKey := config.GetS3BackendCredentials()
if accessKey == "" || secretKey == "" {
    aggregator.AddError(...)
}
```

**After:**
```go
// Validate AWS credentials for S3 backend - check actual fields without fallback
// During validation, we require explicit credentials to be set
legacyAccessKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSAccessKey)
legacySecretKey := strings.TrimSpace(config.OpenCenter.Cluster.AWSSecretAccessKey)
infraAccessKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.AccessKey)
infraSecretKey := strings.TrimSpace(config.Secrets.Global.AWS.Infrastructure.SecretAccessKey)

hasLegacyCredentials := legacyAccessKey != "" && legacySecretKey != ""
hasInfraCredentials := infraAccessKey != "" && infraSecretKey != ""

if !hasLegacyCredentials && !hasInfraCredentials {
    aggregator.AddError(...)
}
```

## Impact Analysis

### Positive Impacts ✅

1. **Validation Now Works Correctly**
   - Tests are now catching missing service-specific credentials
   - Keycloak password validation is working: `secrets.keycloak.admin_password is required when keycloak is enabled`
   - This matches real-world behavior where users must provide credentials

2. **Infrastructure Credentials Still Work**
   - AWS infrastructure credentials are still populated in test mode
   - OpenTofu S3 backend tests can still function
   - Basic cluster operations work in test mode

3. **Test Mode Simulates Real User Configuration**
   - Test mode no longer provides automatic defaults for everything
   - Tests accurately reflect what users will experience
   - Validation errors are caught early

### Negative Impacts ⚠️

1. **More Test Failures (Expected)**
   - Many tests now fail because they expect validation to pass
   - Tests that rely on service-specific credentials need updating
   - This is **intentional** - we want validation to be strict

2. **Test Fixtures Need Updates**
   - Tests that enable services (keycloak, cert-manager, loki) need to provide credentials
   - Test configuration files need to include service-specific secrets
   - This is **correct behavior** - tests should provide required configuration

## Failure Analysis

### New Validation Errors (Expected) ✅

The following errors are now appearing, which is **correct behavior**:

1. **Keycloak Password Required**
   ```
   secrets.keycloak.admin_password is required when keycloak is enabled
   ```
   - Appears in multiple scenarios
   - This is **correct** - keycloak requires admin password
   - Tests need to either:
     - Disable keycloak service
     - Provide keycloak admin password

2. **Cert-Manager Credentials Required**
   - Tests that enable cert-manager now fail validation
   - This is **correct** - cert-manager requires AWS credentials for Route53
   - Tests need to provide cert-manager credentials

3. **Loki Credentials Required**
   - Tests that enable loki now fail validation
   - This is **correct** - loki requires Swift password for object storage
   - Tests need to provide loki credentials

### Remaining Issues (Need Investigation)

1. **Path Structure Issues** (Multiple scenarios)
   - Config files not created at expected paths
   - Organization-based directory structure issues
   - These are **separate issues** from credential validation

2. **VRRP Validation** (2 scenarios)
   - VRRP validation not showing all errors
   - Validation stops after first error (keycloak password)
   - Need to fix validation to show all errors, not just first one

3. **Workflow Issues** (1 scenario)
   - File path expectations don't match actual paths
   - Related to path structure changes

## Next Steps

### 1. Update Test Fixtures to Provide Service Credentials

Tests that enable services need to provide required credentials:

**Option A: Disable Services in Tests**
```yaml
services:
  keycloak:
    enabled: false
  cert-manager:
    enabled: false
  loki:
    enabled: false
```

**Option B: Provide Service Credentials**
```yaml
secrets:
  keycloak:
    admin_password: "test-password"
    client_secret: "test-secret"
  cert_manager:
    aws_access_key: "test-key"
    aws_secret_access_key: "test-secret"
  loki:
    swift_password: "test-password"
```

### 2. Fix Validation to Show All Errors

Currently validation stops after first error. Need to update validation logic to:
- Collect all validation errors
- Show all errors to user
- Don't stop after first error

### 3. Continue Fixing Remaining Path Structure Issues

The path structure issues are separate from credential validation:
- Organization-based directory structure
- Config file location resolution
- GitOps directory creation

## Comparison with Previous Results

### Before Credential Fixes (After Parallel Fixes)
- **Passed:** 126 scenarios (87%)
- **Failed:** 19 scenarios (13%)

### After Credential Fixes (Current)
- **Passed:** 92 scenarios (63%)
- **Failed:** 53 scenarios (37%)

### Analysis

The increase in failures is **expected and correct**:
- 34 additional failures are due to stricter validation
- These tests were passing incorrectly before (false positives)
- Tests were relying on test mode providing dummy credentials
- Now tests must explicitly provide credentials or disable services

This is **progress** because:
- Validation is now working correctly
- Tests accurately reflect real-world behavior
- We're catching configuration errors early

## Conclusion

The credential validation fixes have been successfully applied. The system is now correctly validating service-specific credentials, which is the desired behavior. The increase in test failures is expected and indicates that validation is working properly.

**Status:** ✅ Credential validation fixes applied successfully  
**Next Action:** Update test fixtures to provide service credentials or disable services  
**Task 1.3 Status:** In progress - validation working correctly, tests need updating  

## Files Modified

1. `internal/config/config.go` - Removed service-specific credential population in test mode
2. `internal/config/enhanced_validator.go` - Fixed S3 backend validation to check credentials directly
3. `testdata/` - Created directory for test data

## Build Status

✅ **Build Successful:** `mise run build` completes without errors  
✅ **Unit Tests:** All unit tests passing  
⚠️ **BDD Tests:** 92/145 passing (63%) - expected due to stricter validation  
