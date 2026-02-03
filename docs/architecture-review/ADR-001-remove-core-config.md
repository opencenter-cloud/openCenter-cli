# ADR-001: Remove internal/core/config Package

**Status:** Accepted  
**Date:** 2026-02-03  
**Decision Makers:** Architecture Team  
**Related:** Phase 1 Foundation Utilities

## Context

During the Phase 1 Foundation refactoring, we discovered the `internal/core/config/` package that was created as part of an incomplete architectural migration. Analysis revealed:

- **Zero active references**: No production code imports or uses this package
- **Incomplete implementation**: The package was abandoned mid-migration
- **Duplicate functionality**: All functionality exists in `internal/config/`
- **Developer confusion**: New developers waste time understanding unused code
- **Maintenance burden**: Dead code increases cognitive load and build times

### Package Contents

The `internal/core/config/` directory contained:
- Configuration manager implementation (duplicate of `internal/config/manager.go`)
- Type aliases pointing back to `internal/config/` types
- Migration utilities (unused)
- Strategy patterns (incomplete)
- Test files for abandoned code

### Reference Analysis

Search results showed:
- **1 actual import**: `internal/plugins/loader.go` used `coreconfig.ResolveConfigDir()`
- **Multiple comment references**: Deprecation warnings in `internal/config/persistence.go`
- **No production usage**: The package was never integrated into the main codebase

## Decision

**We will remove the `internal/core/config/` package entirely.**

### Rationale

1. **Zero Production Impact**: No active code depends on this package
2. **Reduces Confusion**: Eliminates abandoned patterns that confuse developers
3. **Simplifies Architecture**: Consolidates configuration management to single location
4. **Improves Maintainability**: Reduces LOC and cognitive load
5. **Enables Clean Refactoring**: Provides clean slate for Phase 1-4 work

### Migration Path

The single reference in `internal/plugins/loader.go` was updated:

**Before:**
```go
import coreconfig "github.com/rackerlabs/opencenter-cli/internal/core/config"

if cfgDir, err := coreconfig.ResolveConfigDir(); err == nil {
    // ...
}
```

**After:**
```go
import "github.com/rackerlabs/opencenter-cli/internal/config"

if cfgDir, err := config.ResolveConfigDir(); err == nil {
    // ...
}
```

The `internal/core/config/persistence.go` file simply delegated to `internal/config.ResolveConfigDir()`, so this change has zero behavioral impact.

## Consequences

### Positive

- **Reduced Cognitive Load**: Developers no longer encounter abandoned code
- **Clearer Architecture**: Single source of truth for configuration management
- **Faster Onboarding**: New developers aren't confused by incomplete migrations
- **Smaller Codebase**: ~1,200 LOC removed
- **Faster Builds**: Less code to compile and analyze

### Negative

- **Lost History**: Some architectural exploration work is removed
  - *Mitigation*: Git history preserves all code and context
- **Potential Rework**: If the original migration goals were valid
  - *Mitigation*: Phase 3 (Configuration Consolidation) addresses the same goals with a complete plan

### Neutral

- **No Production Impact**: Zero behavioral changes to the application
- **No API Changes**: No public interfaces affected
- **No Test Changes**: Only tests for the removed package were deleted

## Alternatives Considered

### Alternative 1: Complete the Migration

**Pros:**
- Honors original architectural intent
- Potentially better separation of concerns

**Cons:**
- Requires 2-3 weeks of work to complete
- Unclear if original design was optimal
- Would delay Phase 1-4 refactoring
- No clear benefit over current `internal/config/` implementation

**Decision:** Rejected - Not worth the investment without clear benefits

### Alternative 2: Keep as Documentation

**Pros:**
- Preserves architectural exploration
- Shows what was attempted

**Cons:**
- Confuses developers (is it active or not?)
- Increases maintenance burden
- Git history already preserves this information

**Decision:** Rejected - Git history is sufficient documentation

### Alternative 3: Move to Archive Directory

**Pros:**
- Clearly marks as inactive
- Preserves code outside git history

**Cons:**
- Still increases cognitive load
- Adds complexity to project structure
- No clear use case for archived code

**Decision:** Rejected - Git history is the appropriate archive

## Implementation

### Changes Made

1. **Removed Directory**: `internal/core/config/` and all contents
2. **Updated Import**: `internal/plugins/loader.go` now uses `internal/config`
3. **Updated Documentation**:
   - `docs/architecture-review/architectural-diagram.md`
   - `docs/architecture-review/phase-1-foundation.md`
4. **Verified**: No broken imports, all tests pass

### Verification Steps

```bash
# Verify no references remain
grep -r "internal/core/config" --include="*.go" --exclude-dir="internal/core/config"

# Verify build succeeds
mise run build

# Verify tests pass
mise run test
```

## Related Decisions

- **Phase 1 Foundation**: This removal is part of the broader foundation cleanup
- **Phase 3 Configuration**: Will establish the unified configuration management approach
- **Future ADRs**: Configuration architecture decisions will be documented separately

## References

- Phase 1 Foundation Specification: `.kiro/specs/phase-1-foundation-utilities/`
- Architecture Review: `docs/architecture-review/`
- Original Migration Context: Git history of `internal/core/config/`

## Notes

This decision demonstrates the importance of completing architectural migrations or reverting them. Incomplete migrations create technical debt and confusion. The Phase 1-4 refactoring plan provides a complete, incremental approach to achieving the original goals of better separation of concerns and cleaner architecture.

---

**Approved By:** Architecture Team  
**Implementation Date:** 2026-02-03  
**Review Date:** 2026-08-03 (6 months)
