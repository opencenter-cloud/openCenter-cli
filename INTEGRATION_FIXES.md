# Integration and Edge Case Fixes - Phase 1

## Overview
Fixed remaining integration issues and edge cases in BDD test scenarios for feature flag cleanup Phase 1.

## Issues Fixed

### 1. Directory Creation Issue (cluster_commands_integration.feature:6)
**Problem**: Infrastructure directory not created during cluster init
- Test expected: `clusters/integration-test/infrastructure/clusters/integration-test`
- Actual: Directory not created

**Root Cause**: `CreateClusterDirectories` was trying to create `inventoryPath` as a directory, but it's actually a file path (`infrastructure/clusters/<cluster>/inventory`)

**Fix**: Changed directory creation to create parent directory of inventory file
```go
// Before:
dirs := []string{
    paths.ClusterDir,
    paths.ApplicationsDir,
    paths.InventoryPath,  // This is a file path, not a directory!
    paths.BinPath,
    filepath.Dir(paths.SOPSKeyPath),
}

// After:
dirs := []string{
    paths.ClusterDir,
    paths.ApplicationsDir,
    filepath.Dir(paths.InventoryPath),  // Create parent directory
    paths.BinPath,
    filepath.Dir(paths.SOPSKeyPath),
}
```

**File**: `internal/config/path_resolver_impl.go:145`

### 2. VRRP Validation Logic (validation.feature:422)
**Problem**: Validation only showed VRRP error, missing other validation errors (OpenStack auth_url, region, etc.)
- Expected: Multiple validation errors
- Actual: Only VRRP error shown

**Root Cause**: VRRP validation had early return when checking top-level `networking` field, preventing it from checking legacy location. When top-level fields were all defaults (false/empty), it would return early without checking the legacy `opencenter.cluster.kubernetes.networking` location.

**Fix**: Updated validation logic to check if top-level networking has any configuration before returning early
```go
// Check if top-level Networking field has any configuration
hasTopLevelNetworking := config.Networking.VRRPEnabled || config.Networking.UseOctavia || config.Networking.VRRPIP != ""

// Only return early if top-level networking is actually configured
if hasTopLevelNetworking {
    if !config.Networking.UseOctavia && config.Networking.VRRPEnabled {
        if config.Networking.VRRPIP == "" {
            result.Errors = append(result.Errors, ...)
        }
    }
    return  // Only return if we found top-level config
}

// Check legacy location for backward compatibility
if !config.OpenCenter.Cluster.Kubernetes.Networking.UseOctavia && config.OpenCenter.Cluster.Kubernetes.Networking.VRRPEnabled {
    if config.OpenCenter.Cluster.Kubernetes.Networking.VRRPIP == "" {
        result.Errors = append(result.Errors, ...)
    }
}
```

**File**: `internal/config/validator.go:1145-1185`

### 3. Workflow Test Field Names (workflow.feature:17)
**Problem**: Test was using legacy `iac.networking` field names that don't exist in current schema
- Test set: `iac.networking.use_octavia`, `iac.networking.vrrp_enabled`, `iac.networking.vrrp_ip`
- Actual schema: `networking.use_octavia`, `networking.vrrp_enabled`, `networking.vrrp_ip`

**Root Cause**: Test was using outdated field names from before the schema refactoring

**Fix**: Updated test to use correct top-level `networking` field
```yaml
# Before:
iac:
  counts: {}
  flavors: {}
  networking:
    use_octavia: false
    vrrp_enabled: true
    vrrp_ip: ""

# After:
opencenter:
  gitops:
    git_dir: tmp/repo-demo
    git_url: tmp/remote.git
networking:
  use_octavia: false
  vrrp_enabled: true
  vrrp_ip: ""
```

**File**: `tests/features/workflow.feature:28-56`

## Testing

### Build
```bash
mise run clean
mise run build
```

### Run Specific Tests
```bash
mise run godog -- tests/features/cluster_commands_integration.feature:6
mise run godog -- tests/features/validation.feature:422
mise run godog -- tests/features/workflow.feature:17
```

## Impact

### Files Modified
1. `internal/config/path_resolver_impl.go` - Fixed directory creation logic
2. `internal/config/validator.go` - Fixed VRRP validation to check both locations properly
3. `tests/features/workflow.feature` - Updated test to use correct field names

### Validation Improvements
- VRRP validation now properly checks both top-level `networking` and legacy `opencenter.cluster.kubernetes.networking` locations
- Validation collects ALL errors, not just the first batch
- Directory creation now correctly creates parent directories for file paths

## Success Criteria
- ✅ Infrastructure directories created correctly during init
- ✅ VRRP validation checks both top-level and legacy locations
- ✅ Workflow test uses correct schema field names
- ✅ All validation errors collected and displayed

## Related Issues
- Part of Feature Flag Cleanup Phase 1
- Addresses miscellaneous integration and edge case issues
- Completes removal of legacy feature flag code
