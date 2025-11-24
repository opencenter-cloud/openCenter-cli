# BDD Test Fix Plan

## Executive Summary

**Goal**: Fix 46 failing BDD scenario tests  
**Estimated Effort**: 2-3 days  
**Priority**: Medium (unit tests are passing, BDD failures don't block core functionality)  
**Approach**: Phased implementation with verification at each step

---

## Current State

- **Total Scenarios**: 145
- **Passing**: 99 (68%)
- **Failing**: 46 (32%)
- **Root Causes**: 
  - ~25 failures: Path structure changes (our improvements)
  - ~5 failures: Service secrets validation (our improvements)
  - ~16 failures: Pre-existing issues (VRRP, GitOps, CLI output)

---

## Phase 1: Update Test Fixtures (Priority: High)

### Objective
Fix test fixture YAML files that have services enabled without required secrets.

### Tasks

#### 1.1 Identify Affected Fixtures
```bash
# Find all test fixture files with cert-manager or keycloak enabled
grep -r "cert-manager:" tests/features/testdata/ -A 2
grep -r "keycloak:" tests/features/testdata/ -A 2
```

**Expected files**:
- `tests/features/testdata/prosys.dev.dfw3.yaml`
- Any other fixtures with enabled services

#### 1.2 Update Fixtures - Option A (Disable Services)
```yaml
# Before
opencenter:
  services:
    cert-manager:
      enabled: true
    keycloak:
      enabled: true

# After
opencenter:
  services:
    cert-manager:
      enabled: false  # Disabled for testing
    keycloak:
      enabled: false  # Disabled for testing
```

#### 1.3 Update Fixtures - Option B (Provide Test Secrets)
```yaml
opencenter:
  services:
    cert-manager:
      enabled: true
    keycloak:
      enabled: true

secrets:
  cert_manager:
    aws_access_key: "AKIATEST123456789"
    aws_secret_access_key: "test-secret-key-for-bdd-testing"
  keycloak:
    admin_password: "test-admin-password-123"
```

**Recommendation**: Use Option A (disable services) for simplicity unless testing service-specific functionality.

#### 1.4 Verification
```bash
# Run affected scenarios
mise run godog --tags=@validation

# Expected: 5 fewer failures
```

**Estimated Time**: 2 hours  
**Files to Modify**: ~3-5 YAML fixtures  
**Expected Impact**: Fix ~5 scenarios

---

## Phase 2: Update Step Definitions for Path Structure (Priority: High)

### Objective
Update step definitions to use new organization-based path structure.

### Tasks

#### 2.1 Locate Step Definition Files
```bash
find tests/features/steps -name "*.go" -type f
```

**Key files**:
- `tests/features/steps/cluster_steps.go`
- `tests/features/steps/config_steps.go`
- `tests/features/steps/organization_steps.go`

#### 2.2 Update Config Path Resolution

**Current pattern** (incorrect):
```go
// Looking for config in infrastructure directory
configPath := filepath.Join(
    configDir, 
    "clusters", 
    organization, 
    "infrastructure", 
    "clusters", 
    clusterName, 
    "."+clusterName+"-config.yaml",
)
```

**New pattern** (correct):
```go
// Config files are at organization level
configPath := filepath.Join(
    configDir, 
    "clusters", 
    organization, 
    "."+clusterName+"-config.yaml",
)
```

#### 2.3 Update Directory Existence Checks

**Current pattern** (incorrect):
```go
// Checking for cluster directory in infrastructure path
clusterDir := filepath.Join(
    configDir,
    "clusters",
    organization,
    "infrastructure",
    "clusters",
    clusterName,
)
```

**New pattern** (correct):
```go
// Infrastructure directory is separate from config location
infrastructureDir := filepath.Join(
    configDir,
    "clusters",
    organization,
    "infrastructure",
    "clusters",
    clusterName,
)

// Config is at organization level
configPath := filepath.Join(
    configDir,
    "clusters",
    organization,
    "."+clusterName+"-config.yaml",
)
```

#### 2.4 Update Helper Functions

Create a helper function for consistent path resolution:

```go
// Add to tests/features/steps/helpers.go
func getClusterConfigPath(configDir, organization, clusterName string) string {
    if organization == "" {
        organization = "opencenter"
    }
    return filepath.Join(
        configDir,
        "clusters",
        organization,
        "."+clusterName+"-config.yaml",
    )
}

func getClusterInfrastructurePath(configDir, organization, clusterName string) string {
    if organization == "" {
        organization = "opencenter"
    }
    return filepath.Join(
        configDir,
        "clusters",
        organization,
        "infrastructure",
        "clusters",
        clusterName,
    )
}
```

#### 2.5 Search and Replace Pattern

```bash
# Find all occurrences of old path pattern
grep -r "infrastructure/clusters" tests/features/steps/

# Review each occurrence and update appropriately
```

#### 2.6 Verification
```bash
# Run organization-related scenarios
mise run godog --tags=@organization

# Run path-related scenarios
mise run godog --tags=@paths

# Expected: 20-25 fewer failures
```

**Estimated Time**: 4-6 hours  
**Files to Modify**: ~5-8 Go step definition files  
**Expected Impact**: Fix ~25 scenarios

---

## Phase 3: Update Output Format Expectations (Priority: Medium)

### Objective
Update tests expecting old CLI output formats.

### Tasks

#### 3.1 Identify Output Format Issues

```bash
# Find scenarios checking for specific output
grep -r "stdout did not contain" tests/features/steps/
grep -r "stderr did not contain" tests/features/steps/
```

#### 3.2 Update List Command Output Expectations

**Current expectation** (old format):
```
cluster-a/cluster-a
cluster-b/cluster-b
```

**New format**:
```
opencenter/cluster-a
opencenter/cluster-b
```

**Fix in step definition**:
```go
// Before
expectedOutput := fmt.Sprintf("%s/%s", clusterName, clusterName)

// After
expectedOutput := fmt.Sprintf("%s/%s", organization, clusterName)
```

#### 3.3 Update Help Text Expectations

**Issue**: Tests checking for specific help text that may have changed.

**Fix**: Update expected strings to match current CLI output:
```go
// Before
expectedText := "openCenter cluster info"

// After - check for actual current output
expectedText := "Show configuration for a cluster"
```

#### 3.4 Verification
```bash
# Run CLI output tests
mise run godog --tags=@cli

# Expected: 2-3 fewer failures
```

**Estimated Time**: 1-2 hours  
**Files to Modify**: ~2-3 step definition files  
**Expected Impact**: Fix ~3 scenarios

---

## Phase 4: Fix GitOps Setup Issues (Priority: Medium)

### Objective
Fix template rendering and setup scenarios.

### Tasks

#### 4.1 Investigate Template Rendering

**Failing scenarios**:
- "Setup materializes GitOps template into git_dir"
- "Forced setup overwrites existing files"

**Investigation steps**:
```bash
# Run specific scenario with verbose output
mise run godog --tags=@setup --verbose

# Check if templates are being copied/rendered
ls -la testdata/*/repo-dev/

# Verify template source exists
ls -la internal/gitops/gitops-base-dir/
```

#### 4.2 Debug Force Flag Behavior

**Issue**: Forced setup not overwriting files as expected.

**Check**:
```go
// In cmd/cluster_setup.go
// Verify force flag is properly handled
if force {
    // Should remove existing files before setup
}
```

#### 4.3 Fix Template Rendering Logic

**Potential issue**: Templates not being rendered or copied correctly.

**Check**:
```go
// In internal/gitops/copy.go
// Verify template rendering is working
func RenderTemplate(src, dst string, data interface{}) error {
    // Implementation check
}
```

#### 4.4 Verification
```bash
# Run GitOps scenarios
mise run godog --tags=@gitops

# Expected: 3-5 fewer failures
```

**Estimated Time**: 3-4 hours  
**Files to Modify**: ~2-3 files in cmd/ and internal/gitops/  
**Expected Impact**: Fix ~5 scenarios

---

## Phase 5: Address VRRP Validation Issues (Priority: Low)

### Objective
Fix VRRP validation logic or test expectations.

### Tasks

#### 5.1 Understand VRRP Validation Requirements

**Failing scenarios**:
- "prosys.dev.dfw3 cluster VRRP validation fails when IP missing"
- "Initialize with org, select, validate VRRP requirement"

**Expected behavior**: Validation should fail when VRRP IP is missing but Octavia is disabled.

#### 5.2 Review VRRP Validation Logic

```bash
# Find VRRP validation code
grep -r "VRRP" internal/config/
grep -r "vrrp" internal/config/
```

**Check in validator**:
```go
// Should be in internal/config/validator.go or similar
// Verify VRRP validation is implemented
if !config.Octavia.Enabled && config.VRRP.IP == "" {
    return error("VRRP IP required when Octavia disabled")
}
```

#### 5.3 Debug Why Validation Passes

**Investigation**:
```bash
# Run failing scenario with debug output
mise run godog --tags=@vrrp --verbose

# Check what validation errors are returned
# Add debug logging to validator if needed
```

#### 5.4 Fix Validation Logic or Test

**Option A**: Fix validation logic if broken  
**Option B**: Update test expectations if validation is correct

#### 5.5 Verification
```bash
# Run VRRP scenarios
mise run godog --tags=@vrrp

# Expected: 5 fewer failures
```

**Estimated Time**: 2-3 hours  
**Files to Modify**: ~1-2 files  
**Expected Impact**: Fix ~5 scenarios

---

## Phase 6: Fix Bootstrap/Remote Git Issues (Priority: Low)

### Objective
Fix scenarios involving git remote operations.

### Tasks

#### 6.1 Investigate Bootstrap Failures

**Failing scenario**: "Bootstrap pushes the local repo to a remote"

**Issue**: Expected bare repo to have branch "main", but it did not.

#### 6.2 Check Git Initialization

```go
// In cmd/cluster_bootstrap.go or similar
// Verify git init and branch creation
git init
git checkout -b main
git add .
git commit -m "Initial commit"
```

#### 6.3 Fix Remote Push Logic

**Check**:
```go
// Verify remote is added and push works
git remote add origin <url>
git push -u origin main
```

#### 6.4 Verification
```bash
# Run bootstrap scenarios
mise run godog --tags=@bootstrap

# Expected: 1-2 fewer failures
```

**Estimated Time**: 2 hours  
**Files to Modify**: ~1 file  
**Expected Impact**: Fix ~2 scenarios

---

## Implementation Order

### Week 1: High Priority Items

**Day 1-2**: Phase 1 (Test Fixtures)
- Update YAML fixtures
- Disable services or add test secrets
- Verify 5 scenarios fixed

**Day 3-5**: Phase 2 (Path Structure)
- Update step definitions
- Create helper functions
- Verify 25 scenarios fixed

**Expected Progress**: ~30 scenarios fixed (65% of failures)

### Week 2: Medium/Low Priority Items

**Day 1**: Phase 3 (Output Formats)
- Update CLI output expectations
- Verify 3 scenarios fixed

**Day 2-3**: Phase 4 (GitOps Setup)
- Fix template rendering
- Fix force flag behavior
- Verify 5 scenarios fixed

**Day 4**: Phase 5 (VRRP Validation)
- Debug and fix VRRP logic
- Verify 5 scenarios fixed

**Day 5**: Phase 6 (Bootstrap/Git)
- Fix git remote operations
- Final verification

**Expected Progress**: All 46 scenarios fixed (100%)

---

## Verification Strategy

### After Each Phase

```bash
# Run full BDD suite
mise run godog

# Track progress
echo "Scenarios passing: X/145"
echo "Improvement: +Y scenarios"
```

### Continuous Verification

```bash
# Run specific tags during development
mise run godog --tags=@organization
mise run godog --tags=@validation
mise run godog --tags=@gitops
```

### Final Verification

```bash
# Full test suite
mise run test && mise run godog

# Should see:
# - Unit tests: 78/78 passing ✅
# - BDD scenarios: 145/145 passing ✅
```

---

## Risk Mitigation

### Potential Issues

1. **Breaking existing passing tests**
   - Mitigation: Run full suite after each change
   - Rollback strategy: Git commits per phase

2. **Uncovering new issues**
   - Mitigation: Document new findings
   - Adjust plan as needed

3. **Time overruns**
   - Mitigation: Prioritize high-impact phases
   - Can ship with some low-priority failures

### Success Criteria

**Minimum Acceptable**:
- Phase 1 & 2 complete (30+ scenarios fixed)
- 85%+ pass rate (123/145 scenarios)

**Target**:
- All phases complete
- 100% pass rate (145/145 scenarios)

**Stretch**:
- Additional test coverage
- Performance improvements
- Documentation updates

---

## Resource Requirements

### Developer Time
- **Phase 1**: 2 hours
- **Phase 2**: 4-6 hours
- **Phase 3**: 1-2 hours
- **Phase 4**: 3-4 hours
- **Phase 5**: 2-3 hours
- **Phase 6**: 2 hours

**Total**: 14-19 hours (~2-3 days)

### Tools Needed
- Go development environment
- Git
- Text editor with Go support
- Access to run BDD tests locally

### Knowledge Required
- Go programming
- Gherkin/Cucumber BDD syntax
- openCenter-CLI architecture
- Git operations

---

## Deliverables

### Code Changes
1. Updated test fixture YAML files
2. Updated step definition Go files
3. Fixed validation logic (if needed)
4. Fixed GitOps setup logic (if needed)

### Documentation
1. Updated BDD test documentation
2. Path structure migration guide
3. Test fixture examples
4. Troubleshooting guide

### Verification
1. All BDD scenarios passing
2. No regression in unit tests
3. CI/CD pipeline green

---

## Alternative Approach: Incremental Fix

If full fix is not feasible immediately, consider:

### Quick Win Strategy

**Week 1**: Fix only Phase 1 & 2 (High Priority)
- Impact: 30 scenarios fixed
- Pass rate: 89% (129/145)
- Time: 1 day

**Week 2**: Fix Phase 3 (Medium Priority)
- Impact: 3 more scenarios fixed
- Pass rate: 91% (132/145)
- Time: 2 hours

**Later**: Address remaining issues as time permits
- Phases 4, 5, 6 can be deferred
- Document known issues
- Create tickets for future work

---

## Success Metrics

### Quantitative
- BDD pass rate: 68% → 100%
- Scenarios fixed: 0 → 46
- Test execution time: <10 seconds
- Zero regressions in unit tests

### Qualitative
- Improved test maintainability
- Better alignment with new architecture
- Clearer test failure messages
- Easier onboarding for new contributors

---

## Next Steps

1. **Review and approve this plan**
2. **Assign developer resources**
3. **Create tracking tickets for each phase**
4. **Set up progress tracking dashboard**
5. **Begin Phase 1 implementation**

---

## Appendix: Quick Reference Commands

```bash
# Run all BDD tests
mise run godog

# Run specific tag
mise run godog --tags=@organization

# Run with verbose output
mise run godog --verbose

# Run specific scenario
mise run godog --name="scenario name"

# Run and save output
mise run godog 2>&1 | tee bdd_results.txt

# Count failures by type
mise run godog 2>&1 | grep "Error:" | sort | uniq -c

# Find test fixtures
find tests/features/testdata -name "*.yaml"

# Find step definitions
find tests/features/steps -name "*.go"

# Search for specific pattern in tests
grep -r "pattern" tests/features/
```

---

## Contact & Support

For questions or issues during implementation:
- Review `/docs/testing-tasks.md` for context
- Check existing test patterns in `tests/features/steps/`
- Refer to path resolver implementation in `internal/config/path_resolver.go`
- Consult configuration defaults in `internal/config/config.go`
