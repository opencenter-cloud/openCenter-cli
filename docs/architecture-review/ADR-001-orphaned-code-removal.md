# ADR-001: Removal of Orphaned internal/core/config Package

**Status**: Accepted  
**Date**: February 9, 2026  
**Deciders**: Project maintainers  
**Related**: Phase 1 Foundation Utilities, Phase 3 Configuration Unification

## Context

During the Phase 1-4 refactoring effort, we discovered the `internal/core/config/` directory that was referenced in deprecated function comments but no longer existed in the codebase. This orphaned code reference created confusion for developers and violated the principle of keeping documentation synchronized with code.

### Background

The `internal/core/config/` package was originally planned as part of an earlier architectural design but was superseded by the current `internal/config/` package structure during the refactoring. The directory itself was removed in a previous cleanup (commit 1afa03a), but references to it remained in:

- Deprecated function comments in `internal/config/persistence.go`
- Architecture documentation describing the original design
- Spec files outlining the planned structure

### Problem Statement

The orphaned references created several issues:

1. **Developer Confusion**: Comments referenced `internal/core/config.ConfigManager` which didn't exist
2. **Broken Migration Path**: Deprecation notices pointed to non-existent code
3. **Documentation Debt**: Stale references made it unclear what the current architecture was
4. **Build Verification**: While the code compiled, the references were misleading

## Decision

We decided to **remove all code references** to the orphaned `internal/core/config/` package while **preserving historical documentation** for context.

### Actions Taken

1. **Updated Deprecated Function Comments** (10 functions in `internal/config/persistence.go`):
   - Changed: `internal/core/config.ConfigManager` → `ConfigurationManager`
   - Maintained deprecation warnings and version information
   - Preserved migration guidance with correct references

2. **Verified Directory Removal**:
   - Confirmed `internal/core/config/` does not exist
   - Verified no broken imports
   - Confirmed build succeeds

3. **Preserved Historical Context**:
   - Kept references in `.kiro/specs/` for historical context
   - Kept references in `docs/architecture-review/` to explain evolution
   - Maintained ADR (this document) to explain the decision

### Example Changes

**Before**:
```go
// Deprecated: Use internal/core/config.ConfigManager.Load() with merge options instead.
// This function will be removed in v2.0.0.
func GenerateCompleteConfig(clusterName string) (*Config, error) {
    // ...
}
```

**After**:
```go
// Deprecated: Use ConfigurationManager.Load() with merge options instead.
// This function will be removed in v2.0.0.
func GenerateCompleteConfig(clusterName string) (*Config, error) {
    // ...
}
```

## Rationale

### Why Remove References?

1. **Accuracy**: Code comments should reflect actual code structure
2. **Developer Experience**: Clear migration paths reduce confusion
3. **Maintainability**: Fewer stale references = less technical debt
4. **Consistency**: All references should point to existing code

### Why Preserve Historical Documentation?

1. **Context**: Understanding why decisions were made
2. **Learning**: Future developers can see the evolution
3. **Traceability**: Audit trail of architectural changes
4. **Completeness**: Specs document the full journey, not just the end state

### Alternative Approaches Considered

1. **Create the Missing Package**: 
   - Rejected: Would create duplicate functionality
   - The current `internal/config/` package already provides all needed functionality
   - Would increase maintenance burden

2. **Remove All References Including Documentation**:
   - Rejected: Loses historical context
   - Makes it harder to understand architectural evolution
   - Removes valuable learning material

3. **Leave References As-Is**:
   - Rejected: Perpetuates confusion
   - Violates documentation accuracy principle
   - Makes migration harder for developers

## Consequences

### Positive

- ✅ **Clearer Code**: All references point to existing code
- ✅ **Better Developer Experience**: Migration paths are accurate
- ✅ **Reduced Confusion**: No references to non-existent packages
- ✅ **Maintained History**: Documentation preserves context
- ✅ **Build Stability**: No impact on compilation or tests

### Negative

- ⚠️ **Documentation Effort**: Required updating 10 function comments
- ⚠️ **Potential Confusion**: Developers reading old commits may see old references
  - Mitigation: This ADR explains the change

### Neutral

- ℹ️ **No Functional Impact**: Code behavior unchanged
- ℹ️ **No Performance Impact**: Comments don't affect runtime
- ℹ️ **No API Changes**: Public interfaces unchanged

## Implementation

### Files Modified

1. **internal/config/persistence.go**:
   - Updated 10 deprecated function comments
   - Changed `internal/core/config.ConfigManager` → `ConfigurationManager`
   - Preserved all deprecation warnings and version info

### Verification

```bash
# Verify directory doesn't exist
ls internal/core/config/
# Result: No such file or directory ✅

# Verify no broken imports
grep -r "internal/core/config" internal/*.go
# Result: No matches ✅

# Verify build succeeds
mise run build
# Result: Success ✅

# Verify tests pass
go test ./internal/config
# Result: All tests pass ✅
```

### Time Investment

- **Estimated**: 2-3 hours (for ADR creation)
- **Actual**: 15 minutes (code cleanup) + 30 minutes (ADR creation)
- **Total**: 45 minutes

## Related Decisions

### Phase 3: Configuration Unification

The current `ConfigurationManager` in `internal/config/` is the result of Phase 3 Configuration Unification. It provides:

- Unified configuration API (Load, Save, Validate, List, Delete)
- Integration with Phase 1 FileSystem and PathResolver
- Integration with Phase 2 ValidationEngine
- Atomic operations with caching
- Fluent builder pattern

This is the **correct** target for migration from deprecated functions.

### Phase 1: Foundation Utilities

The orphaned `internal/core/config/` was likely an early attempt at creating a core configuration package before the Phase 1 foundation utilities were established. The current architecture is superior because:

- Uses FileSystem abstraction for atomic operations
- Uses ValidationEngine for consistent validation
- Uses PathResolver for unified path management
- Better separation of concerns

## Lessons Learned

1. **Keep Documentation Synchronized**: Remove code references when code is removed
2. **Preserve History**: Keep architectural documentation for context
3. **Document Decisions**: ADRs explain why changes were made
4. **Verify Thoroughly**: Check for all references, not just code
5. **Clean Up Incrementally**: Small, focused cleanups are easier to review

## Future Considerations

### Deprecation Process

Going forward, when deprecating code:

1. **Update All References**: Code, comments, and documentation
2. **Provide Clear Migration Path**: Point to existing, working code
3. **Set Removal Timeline**: Specify version when code will be removed
4. **Create ADR**: Document the decision and rationale
5. **Verify Thoroughly**: Check for all references across the codebase

### Documentation Standards

This experience reinforces the need for:

- Regular documentation audits
- Automated checks for broken references
- Clear deprecation guidelines
- ADRs for significant architectural changes

## References

- [Phase 1 Foundation Utilities Spec](.kiro/specs/phase-1-foundation-utilities/)
- [Phase 3 Configuration Unification Spec](.kiro/specs/phase-3-configuration-unification/)
- [PHASE_1_ORPHANED_CODE_CLEANUP.md](../../PHASE_1_ORPHANED_CODE_CLEANUP.md)
- [IMPLEMENTATION_STATUS.md](./IMPLEMENTATION_STATUS.md)
- Commit 1afa03a: Removed backup files and skipped tests

## Approval

**Approved By**: Project maintainers  
**Date**: February 9, 2026  
**Status**: Implemented and verified

---

**ADR Status**: Accepted  
**Implementation Status**: Complete  
**Last Updated**: February 9, 2026
