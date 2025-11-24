# BDD Test Fix - Quick Summary

## 📊 Current State
- **Passing**: 99/145 scenarios (68%)
- **Failing**: 46/145 scenarios (32%)

## 🎯 Fix Plan Overview

### Phase 1: Test Fixtures (2 hours) - HIGH PRIORITY
**Impact**: Fix ~5 scenarios  
**Action**: Disable cert-manager/keycloak in test YAML files or add test secrets

### Phase 2: Path Structure (4-6 hours) - HIGH PRIORITY  
**Impact**: Fix ~25 scenarios  
**Action**: Update step definitions to use organization-level config paths

### Phase 3: Output Formats (1-2 hours) - MEDIUM PRIORITY
**Impact**: Fix ~3 scenarios  
**Action**: Update CLI output expectations

### Phase 4: GitOps Setup (3-4 hours) - MEDIUM PRIORITY
**Impact**: Fix ~5 scenarios  
**Action**: Fix template rendering and force flag behavior

### Phase 5: VRRP Validation (2-3 hours) - LOW PRIORITY
**Impact**: Fix ~5 scenarios  
**Action**: Debug and fix VRRP validation logic

### Phase 6: Bootstrap/Git (2 hours) - LOW PRIORITY
**Impact**: Fix ~2 scenarios  
**Action**: Fix git remote operations

## ⏱️ Time Estimate
- **Total**: 14-19 hours (~2-3 days)
- **Quick Win** (Phase 1+2): 6-8 hours (~1 day) → 89% pass rate

## 📈 Expected Progress

| After Phase | Scenarios Fixed | Pass Rate |
|-------------|----------------|-----------|
| Phase 1 | 5 | 72% |
| Phase 2 | 30 | 89% |
| Phase 3 | 33 | 91% |
| Phase 4 | 38 | 93% |
| Phase 5 | 43 | 96% |
| Phase 6 | 46 | 100% ✅ |

## 🚀 Quick Start

```bash
# 1. Update test fixtures
vim tests/features/testdata/prosys.dev.dfw3.yaml
# Set cert-manager.enabled: false
# Set keycloak.enabled: false

# 2. Update step definitions
vim tests/features/steps/cluster_steps.go
# Change: clusters/org/infrastructure/clusters/name/.name-config.yaml
# To: clusters/org/.name-config.yaml

# 3. Verify
mise run godog
```

## 📋 Key Files to Modify

### Test Fixtures
- `tests/features/testdata/prosys.dev.dfw3.yaml`
- `tests/features/testdata/*.yaml` (any with enabled services)

### Step Definitions
- `tests/features/steps/cluster_steps.go`
- `tests/features/steps/config_steps.go`
- `tests/features/steps/organization_steps.go`

### Core Logic (if needed)
- `internal/config/validator.go` (VRRP validation)
- `internal/gitops/copy.go` (template rendering)
- `cmd/cluster_setup.go` (force flag)

## ✅ Success Criteria

**Minimum** (Phase 1+2):
- 89% pass rate (129/145)
- High-priority issues resolved
- 1 day effort

**Target** (All phases):
- 100% pass rate (145/145)
- All issues resolved
- 2-3 days effort

## 🔗 Full Details

See `/docs/bdd-test-fix-plan.md` for complete implementation guide.
