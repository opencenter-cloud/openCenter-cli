# Deprecated Code Removal Status

**Date**: February 9, 2026  
**Phase**: Phase 4 Cleanup & Optimization  
**Status**: In Progress (Part 1 Complete)

## Overview

Removing all deprecated code after Phase 1-4 completion. The deprecated code was kept for backward compatibility during migration but should now be removed since all phases are complete.

## Part 1: Unused Deprecated Code - ✅ COMPLETE

**Commit**: 109140f  
**Date**: February 9, 2026

### Removed

1. **`internal/config/cache.go`** (300+ lines removed)
   - ✅ `InMemoryConfigCache` type
   - ✅ `CacheEntry` type  
   - ✅ `NewInMemoryConfigCache()` function
   - ✅ All associated methods (15+ methods)

2. **`internal/config/interfaces.go`**
   - ✅ `ConfigManagerInterface` interface

3. **`cmd/config_migration_helpers.go`**
   - ✅ Updated comments to remove "migration helper" references

### Impact

- **Lines Removed**: 319 lines
- **Files Modified**: 3 files
- **Breaking Changes**: None (code was not being used)
- **Tests**: All passing

## Part 2: Deprecated Persistence Functions - ⏳ IN PROGRESS

### Functions to Remove

From `internal/config/persistence.go`:

1. **`Save(cfg Config) error`**
   - Used in: 20 locations
   - Replacement: `manager.Save(ctx, &cfg)`
   
2. **`Load(name string) (Config, error)`**
   - Used in: 20 locations
   - Replacement: `manager.Load(ctx, name)`

3. **`Validate(cfg Config) []error`**
   - Used in: 3 locations
   - Replacement: `manager.Validate(ctx, &cfg)`

4. **`ConfigPath(name string) (string, error)`**
   - Used in: 10 locations
   - Replacement: `pathResolver.ResolveClusterPaths(ctx, name, org).ConfigPath`

5. **`GenerateCompleteConfig(name string) (Config, error)`**
   - Used in: 1 test file only
   - Replacement: `manager.Load(ctx, name)` with merge options

6. **`SaveDebugConfig(clusterName, gitDir string) error`**
   - Used in: 2 test files only
   - Replacement: Manual implementation in tests

7. **`ListClusters() ([]string, error)`**
   - Used in: 0 locations
   - Can be removed immediately

8. **`SetActiveCluster(name string) error`**
   - Used in: 0 locations
   - Can be removed immediately

9. **`GetActiveCluster() (string, error)`**
   - Used in: 0 locations (GetActive() is used instead)
   - Can be removed immediately

### Files Requiring Updates

#### Test Files (4 files)
1. `internal/cluster/setup_service_test.go` - 4 uses of `config.Save()`
2. `internal/cluster/bootstrap_service_test.go` - 2 uses of `config.Save()`
3. `internal/config/migration/scanner_test.go` - Multiple uses (test data)
4. `cmd/cluster_service_test.go` - 10+ uses of `config.Load()` and `config.Save()`

#### Command Files (5 files)
1. `cmd/cluster_lock.go` - 2 uses of `config.ConfigPath()`
2. `cmd/cluster_select.go` - 1 use of `config.ConfigPath()`
3. `cmd/cluster_info.go` - 1 use of `config.ConfigPath()`
4. `cmd/cluster_config_update.go` - 1 use of `config.ConfigPath()`
5. `cmd/cluster_config.go` - 1 use of `config.ConfigPath()`

#### Config Test Files (1 file)
1. `internal/config/config_test.go` - Uses `GenerateCompleteConfig()` and `SaveDebugConfig()`

### Migration Strategy

**Option A: Bulk Update (Recommended)**
- Use automated script to replace patterns
- Handle edge cases manually
- Run tests after each file
- Estimated time: 4-6 hours

**Option B: Manual Update**
- Update each file individually
- More careful but slower
- Estimated time: 8-12 hours

**Option C: Gradual Migration**
- Keep deprecated functions as thin wrappers
- Update files over time
- Remove wrappers in v2.0.0
- Estimated time: Ongoing

## Part 3: Other Deprecated Code - 📋 TO EVALUATE

### Low Priority

1. **`internal/config/config.go`**
   - `validateServiceSecretsSimple(cfg Config) []string`
   - Only used internally
   - Should migrate to ValidationEngine

2. **`internal/util/template/interfaces.go`**
   - `TemplateValidator` interface
   - Used extensively in template engine
   - May require template engine refactoring
   - Consider separate task

## Recommendation

**Proceed with Part 2 using Option A (Bulk Update)**:

1. Create helper functions in test files
2. Use automated replacements for simple patterns
3. Handle edge cases manually
4. Run tests frequently
5. Commit after each file or small group of files

**Estimated Completion**: 4-6 hours of focused work

## Current Status

- ✅ Part 1 Complete: Unused deprecated code removed
- ⏳ Part 2 In Progress: Need to update 50+ call sites
- 📋 Part 3 To Evaluate: Template validator and internal helpers

**Next Steps**:
1. Create test helper functions for ConfigurationManager
2. Update test files to use helpers
3. Update cmd files to use PathResolver
4. Remove deprecated functions from persistence.go
5. Run full test suite
6. Commit changes

## Success Criteria

- [ ] All deprecated functions removed from persistence.go
- [ ] All test files updated to use ConfigurationManager
- [ ] All cmd files updated to use PathResolver
- [ ] All tests passing
- [ ] Build succeeds
- [ ] No deprecation warnings in code
- [ ] Documentation updated
