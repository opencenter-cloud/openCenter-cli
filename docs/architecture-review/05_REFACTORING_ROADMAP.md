# Refactoring Roadmap

**Project**: opencenter-cli  
**Review Date**: February 4, 2026  
**Document**: Phase 5 - Implementation Guide

## Table of Contents

- [Overview](#overview)
- [Roadmap Summary](#roadmap-summary)
- [Phase 1: Foundation](#phase-1-foundation-week-1)
- [Phase 2: Core Services](#phase-2-core-services-week-2)
- [Phase 3: Integration](#phase-3-integration-week-3)
- [Phase 4: Cleanup](#phase-4-cleanup--documentation-week-4)
- [Risk Management](#risk-management)
- [Success Metrics](#success-metrics)
- [Rollback Plan](#rollback-plan)

## Overview

This document provides a step-by-step guide for refactoring the opencenter-cli codebase. The roadmap is designed to minimize breaking changes through phased implementation with comprehensive testing at each stage.

**Total Duration**: 4 weeks (10-15 working days)  
**Team Size**: 1-2 developers  
**Risk Level**: Low to Medium

## Roadmap Summary

### Timeline

```
Week 1: Foundation
├─ Day 1-2: Error Handling Consolidation
├─ Day 3: Test Utilities Consolidation
└─ Day 4-5: Validation Framework

Week 2: Core Services
├─ Day 1: Service System Analysis
├─ Day 2-3: Unified Service Implementation
└─ Day 4-5: Service Migration

Week 3: Integration
├─ Day 1-2: Crypto Utilities Consolidation
├─ Day 2-3: File I/O Abstraction
└─ Day 4-5: Configuration Optimization

Week 4: Cleanup & Documentation
├─ Day 1: Remove Deprecated Code
├─ Day 2-3: Update Documentation
└─ Day 4-5: Testing & Validation
```

### Effort Distribution

| Phase | Duration | Effort | Risk | Impact |
|-------|----------|--------|------|--------|
| Phase 1 | 5 days | High | Low | High |
| Phase 2 | 5 days | Very High | Medium | Very High |
| Phase 3 | 5 days | Medium | Low | Medium |
| Phase 4 | 5 days | Low | Low | Low |

## Phase 1: Foundation (Week 1)

**Goal**: Establish unified systems for cross-cutting concerns

**Duration**: 5 days  
**Risk**: Low  
**Impact**: High


### Day 1-2: Error Handling Consolidation

**Objective**: Create unified error handling system

**Tasks**:

1. **Create Unified Error Type** (4 hours)
   ```bash
   # Create new file
   touch internal/util/errors/structured.go
   ```
   
   Implementation:
   - Define `StructuredError` type with all necessary fields
   - Implement `Error()` method
   - Add error type constants
   - Create `ErrorContext` struct

2. **Implement Error Factory** (4 hours)
   ```bash
   touch internal/util/errors/factory.go
   ```
   
   Implementation:
   - Create `New()` function with options pattern
   - Implement error options (WithCause, WithField, WithSuggestion, etc.)
   - Add convenience functions for common error types
   - Implement error wrapping

3. **Migrate Error Creation** (6 hours)
   - Update `internal/config/errors.go` to use factory
   - Update `internal/config/flags/errors.go` to use factory
   - Update all error creation calls in codebase
   - Run tests after each package migration

4. **Update Tests** (2 hours)
   - Update error checking in tests
   - Add tests for new error system
   - Verify all tests pass

**Deliverables**:
- `internal/util/errors/structured.go` (~200 lines)
- `internal/util/errors/factory.go` (~150 lines)
- Updated error creation throughout codebase
- All tests passing

**Success Criteria**:
- ✅ All errors use unified system
- ✅ No compilation errors
- ✅ All tests pass
- ✅ Error messages are consistent

---

### Day 3: Test Utilities Consolidation

**Objective**: Centralize test utilities

**Tasks**:

1. **Merge Test Helpers** (3 hours)
   ```bash
   # Consolidate into single file
   # Keep: internal/testing/framework.go
   # Merge: internal/testing/helpers.go → framework.go
   ```
   
   Implementation:
   - Merge assertion functions
   - Remove duplicates
   - Standardize naming
   - Add missing assertions

2. **Create Centralized Setup Functions** (2 hours)
   ```bash
   touch internal/testing/fixtures.go
   ```
   
   Implementation:
   - Extract setup functions from test files
   - Create reusable fixtures
   - Add fixture factory methods
   - Document fixture usage

3. **Update Test Files** (3 hours)
   - Update imports in all test files
   - Replace scattered setup with centralized functions
   - Remove duplicate test helpers
   - Verify tests still pass

**Deliverables**:
- Consolidated `internal/testing/framework.go`
- New `internal/testing/fixtures.go`
- Updated test files
- All tests passing

**Success Criteria**:
- ✅ Single test framework
- ✅ No duplicate assertions
- ✅ All tests use centralized utilities
- ✅ Test execution time unchanged or improved

---

### Day 4-5: Validation Framework

**Objective**: Create unified validation framework

**Tasks**:

1. **Design Validation Interfaces** (3 hours)
   ```bash
   touch internal/core/validation/interfaces.go
   ```
   
   Implementation:
   - Define `Validator` interface
   - Define `ValidationResult` struct
   - Define `ValidationError` struct
   - Create validator registry

2. **Implement Validation Engine** (4 hours)
   ```bash
   touch internal/core/validation/engine.go
   ```
   
   Implementation:
   - Create `ValidationEngine` struct
   - Implement `Register()` method
   - Implement `Validate()` method
   - Add result aggregation

3. **Create Core Validators** (5 hours)
   ```bash
   mkdir -p internal/core/validation/validators
   touch internal/core/validation/validators/{cluster,service,config,dependency}.go
   ```
   
   Implementation:
   - Migrate cluster name validator
   - Migrate service validator
   - Migrate config validator
   - Migrate dependency validator

4. **Integrate with Existing Code** (4 hours)
   - Update `internal/config/services/dependency_validator.go`
   - Update `internal/services/registry.go`
   - Update validation calls throughout codebase
   - Run comprehensive tests

**Deliverables**:
- Validation framework in `internal/core/validation/`
- Core validators implemented
- Integration complete
- All tests passing

**Success Criteria**:
- ✅ Unified validation interface
- ✅ All validators use framework
- ✅ Consistent validation results
- ✅ No validation logic duplication

---

### Phase 1 Checkpoint

**Review Criteria**:
- [ ] All error handling uses unified system
- [ ] All tests use centralized utilities
- [ ] Validation framework operational
- [ ] All tests passing
- [ ] No regressions detected
- [ ] Code review completed

**Metrics**:
- Lines removed: ~400
- Lines added: ~700
- Net change: +300 (infrastructure)
- Test coverage: Maintained or improved

## Phase 2: Core Services (Week 2)

**Goal**: Consolidate service architecture into single system

**Duration**: 5 days  
**Risk**: Medium  
**Impact**: Very High

### Day 1: Service System Analysis

**Objective**: Thoroughly understand both service systems

**Tasks**:

1. **Document Current Systems** (3 hours)
   - Map all services in `internal/config/services/`
   - Map all services in `internal/services/plugins/`
   - Identify differences
   - Document dependencies

2. **Design Unified Architecture** (4 hours)
   - Define `ServiceDefinition` interface
   - Design plugin adapter pattern
   - Plan migration strategy
   - Create architecture diagram

3. **Create Migration Plan** (1 hour)
   - List all services to migrate
   - Identify high-risk areas
   - Plan testing strategy
   - Document rollback procedure

**Deliverables**:
- Architecture design document
- Migration plan
- Risk assessment

**Success Criteria**:
- ✅ Both systems fully documented
- ✅ Unified architecture designed
- ✅ Migration plan approved

---

### Day 2-3: Unified Service Implementation

**Objective**: Implement unified service system

**Tasks**:

1. **Create Service Definition** (4 hours)
   ```bash
   touch internal/services/definition.go
   ```
   
   Implementation:
   - Define `ServiceDefinition` struct
   - Implement service metadata
   - Add dependency tracking
   - Create lifecycle hooks

2. **Enhance Service Registry** (4 hours)
   ```bash
   # Update: internal/services/registry.go
   ```
   
   Implementation:
   - Add config adapter support
   - Enhance dependency resolution
   - Add validation integration
   - Improve error handling

3. **Create Configuration Adapters** (6 hours)
   ```bash
   mkdir -p internal/services/adapters
   touch internal/services/adapters/{adapter,cert_manager,cilium,harbor}.go
   ```
   
   Implementation:
   - Define `ConfigAdapter` interface
   - Implement adapters for each service
   - Add conversion logic
   - Add validation

4. **Update Service Plugins** (2 hours)
   - Update plugin interface
   - Add adapter integration
   - Update existing plugins
   - Add tests

**Deliverables**:
- `internal/services/definition.go`
- Enhanced `internal/services/registry.go`
- Configuration adapters
- Updated plugins

**Success Criteria**:
- ✅ Unified service definition
- ✅ Registry supports adapters
- ✅ All adapters implemented
- ✅ Tests passing

---

### Day 4-5: Service Migration

**Objective**: Migrate all services to unified system

**Tasks**:

1. **Migrate Service Implementations** (8 hours)
   - Migrate cert-manager
   - Migrate cilium
   - Migrate harbor
   - Migrate keycloak
   - Migrate loki
   - Migrate metallb
   - Migrate opentelemetry
   - Migrate weave-gitops
   - Migrate headlamp
   - Migrate gateway
   - Migrate kube-ovn

2. **Update Service Registration** (2 hours)
   - Update DI container
   - Update service initialization
   - Update command integration
   - Verify all services registered

3. **Update Tests** (4 hours)
   - Update service tests
   - Add integration tests
   - Verify all tests pass
   - Add regression tests

4. **Remove Old System** (2 hours)
   - Delete `internal/config/services/` (except adapters)
   - Update imports
   - Clean up references
   - Final test run

**Deliverables**:
- All services migrated
- Old system removed
- All tests passing
- Documentation updated

**Success Criteria**:
- ✅ Single service system
- ✅ All services working
- ✅ 20-25% code reduction achieved
- ✅ No regressions

---

### Phase 2 Checkpoint

**Review Criteria**:
- [ ] All services migrated to unified system
- [ ] Old service system removed
- [ ] All tests passing
- [ ] Integration tests passing
- [ ] Performance maintained
- [ ] Code review completed

**Metrics**:
- Lines removed: ~1,500
- Lines added: ~500
- Net change: -1,000 (33% reduction)
- Test coverage: Maintained or improved

## Phase 3: Integration (Week 3)

**Goal**: Integrate improvements and optimize

**Duration**: 5 days  
**Risk**: Low  
**Impact**: Medium

### Day 1-2: Crypto Utilities Consolidation

**Objective**: Merge crypto modules

**Tasks**:

1. **Merge Key Generation and Management** (4 hours)
   ```bash
   # Merge into single file
   # Keep: internal/util/crypto/keys.go
   # Merge: key_generator.go + key_manager.go → keys.go
   ```
   
   Implementation:
   - Combine generation and management functions
   - Remove delegation pattern
   - Simplify interfaces
   - Update documentation

2. **Update SOPS Integration** (3 hours)
   ```bash
   # Update: internal/sops/key_manager.go
   ```
   
   Implementation:
   - Use unified crypto module
   - Remove duplicate key generation
   - Update SOPS-specific logic
   - Add tests

3. **Update All Crypto Operations** (3 hours)
   - Update imports throughout codebase
   - Update function calls
   - Verify all crypto operations work
   - Run security tests

**Deliverables**:
- Consolidated `internal/util/crypto/keys.go`
- Updated SOPS integration
- All crypto operations working

**Success Criteria**:
- ✅ Single crypto module
- ✅ No duplicate key generation
- ✅ All crypto tests passing
- ✅ Security maintained

---

### Day 2-3: File I/O Abstraction

**Objective**: Create unified file operations

**Tasks**:

1. **Create File Operations Interface** (3 hours)
   ```bash
   touch internal/util/files/operations.go
   ```
   
   Implementation:
   - Define `FileOperations` interface
   - Implement atomic write wrapper
   - Add error handling
   - Add logging

2. **Implement File Operations** (3 hours)
   - Implement `ReadFile()`
   - Implement `WriteFile()` with atomic writes
   - Implement `CopyFile()`
   - Implement `MoveFile()`
   - Add permission handling

3. **Update File Operations** (4 hours)
   - Update `internal/cluster/init_service.go`
   - Update `internal/gitops/atomic.go`
   - Update `internal/sops/key_manager.go`
   - Update all file operations

**Deliverables**:
- `internal/util/files/operations.go`
- Updated file operations throughout codebase
- Consistent error handling

**Success Criteria**:
- ✅ Unified file I/O abstraction
- ✅ Atomic writes everywhere
- ✅ Consistent error handling
- ✅ All file operations working

---

### Day 4-5: Configuration Optimization

**Objective**: Optimize configuration loading

**Tasks**:

1. **Create Configuration Loader** (4 hours)
   ```bash
   touch internal/config/loader_optimized.go
   ```
   
   Implementation:
   - Implement caching strategy
   - Optimize YAML parsing
   - Add lazy loading
   - Add validation hooks

2. **Implement Credential Resolver** (3 hours)
   ```bash
   mkdir -p internal/config/credentials
   touch internal/config/credentials/resolver.go
   ```
   
   Implementation:
   - Extract credential logic from Config
   - Implement fallback resolution
   - Add caching
   - Add tests

3. **Update Configuration Usage** (3 hours)
   - Update config loading calls
   - Update credential access
   - Verify performance improvement
   - Run benchmarks

**Deliverables**:
- Optimized configuration loader
- Credential resolver
- Performance improvements

**Success Criteria**:
- ✅ Faster config loading
- ✅ Cleaner Config struct
- ✅ All config operations working
- ✅ Performance improved

---

### Phase 3 Checkpoint

**Review Criteria**:
- [ ] Crypto utilities consolidated
- [ ] File I/O abstraction complete
- [ ] Configuration optimized
- [ ] All tests passing
- [ ] Performance improved
- [ ] Code review completed

**Metrics**:
- Lines removed: ~400
- Lines added: ~300
- Net change: -100
- Performance: 10-20% improvement in config loading

## Phase 4: Cleanup & Documentation (Week 4)

**Goal**: Finalize refactoring and document changes

**Duration**: 5 days  
**Risk**: Low  
**Impact**: Low

### Day 1: Remove Deprecated Code

**Objective**: Clean up dead code

**Tasks**:

1. **Remove Backup Files** (1 hour)
   ```bash
   find . -name "*.bak" -delete
   git add -A
   git commit -m "chore: remove backup files"
   ```

2. **Remove Commented Code** (2 hours)
   - Review all commented code
   - Remove unnecessary comments
   - Keep only essential comments
   - Commit changes

3. **Remove Unused Exports** (2 hours)
   - Remove unused test helpers
   - Remove unused utility functions
   - Remove unused interface methods
   - Run tests

4. **Update Deprecated Functions** (3 hours)
   - Update references to deprecated functions
   - Remove deprecated functions
   - Update documentation
   - Commit changes

**Deliverables**:
- Clean codebase
- No dead code
- All tests passing

**Success Criteria**:
- ✅ No backup files
- ✅ No commented code
- ✅ No unused exports
- ✅ No deprecated functions

---

### Day 2-3: Update Documentation

**Objective**: Document all changes

**Tasks**:

1. **Update Architecture Documentation** (4 hours)
   - Update architecture diagrams
   - Document new service system
   - Document validation framework
   - Document error handling

2. **Create Migration Guides** (4 hours)
   - Write service migration guide
   - Write error handling migration guide
   - Write validation migration guide
   - Add code examples

3. **Update API Documentation** (3 hours)
   - Update package documentation
   - Update function documentation
   - Add usage examples
   - Generate godoc

4. **Update README and Contributing** (1 hour)
   - Update README with new architecture
   - Update CONTRIBUTING.md
   - Update development guide
   - Add refactoring notes

**Deliverables**:
- Updated architecture docs
- Migration guides
- Updated API docs
- Updated README

**Success Criteria**:
- ✅ All documentation current
- ✅ Migration guides complete
- ✅ API docs updated
- ✅ Examples working

---

### Day 4-5: Testing & Validation

**Objective**: Comprehensive testing

**Tasks**:

1. **Run Comprehensive Test Suite** (4 hours)
   ```bash
   # Unit tests
   mise run test
   
   # BDD tests
   mise run godog
   
   # Integration tests
   go test ./tests/integration/... -v
   
   # Property-based tests
   go test -run Property ./...
   ```

2. **Performance Benchmarking** (3 hours)
   ```bash
   # Run benchmarks
   go test -bench=. -benchmem ./...
   
   # Compare with baseline
   benchstat baseline.txt current.txt
   
   # Profile if needed
   go test -cpuprofile=cpu.prof -memprofile=mem.prof
   ```

3. **Security Audit** (2 hours)
   ```bash
   # Run security scanner
   gosec ./...
   
   # Check dependencies
   go list -m all | nancy sleuth
   
   # Verify credential masking
   # (manual testing)
   ```

4. **Final Code Review** (3 hours)
   - Review all changes
   - Check for regressions
   - Verify code quality
   - Approve merge

**Deliverables**:
- Test results
- Performance report
- Security audit report
- Code review approval

**Success Criteria**:
- ✅ All tests passing
- ✅ Performance maintained or improved
- ✅ No security issues
- ✅ Code review approved

---

### Phase 4 Checkpoint

**Review Criteria**:
- [ ] All dead code removed
- [ ] Documentation complete
- [ ] All tests passing
- [ ] Performance benchmarked
- [ ] Security audit passed
- [ ] Final code review approved

**Metrics**:
- Lines removed: ~1,785 (dead code)
- Documentation pages: +10
- Test coverage: Maintained or improved
- Performance: Maintained or improved

## Risk Management

### Risk 1: Breaking Changes

**Probability**: Medium  
**Impact**: High

**Mitigation**:
- Comprehensive test suite
- Phased rollout
- Feature flags for new code
- Rollback plan ready

**Contingency**:
- Revert to previous commit
- Fix issues incrementally
- Deploy hotfix if needed

---

### Risk 2: Performance Degradation

**Probability**: Low  
**Impact**: Medium

**Mitigation**:
- Benchmark before and after
- Profile critical paths
- Optimize hot spots
- Monitor in production

**Contingency**:
- Identify bottlenecks
- Optimize specific areas
- Consider caching strategies

---

### Risk 3: Test Failures

**Probability**: Medium  
**Impact**: Medium

**Mitigation**:
- Run tests frequently
- Fix failures immediately
- Add regression tests
- Maintain test coverage

**Contingency**:
- Debug failing tests
- Update test expectations
- Add missing tests

---

### Risk 4: Timeline Overrun

**Probability**: Medium  
**Impact**: Low

**Mitigation**:
- Prioritize high-impact items
- Track progress daily
- Adjust scope if needed
- Communicate delays early

**Contingency**:
- Extend timeline
- Reduce scope
- Add resources

## Success Metrics

### Code Quality Metrics

| Metric | Baseline | Target | Actual |
|--------|----------|--------|--------|
| Code Duplication | 15-20% | <5% | TBD |
| Test Coverage | 75% | 80% | TBD |
| Cyclomatic Complexity | Medium | Low | TBD |
| Package Coupling | Medium | Low | TBD |
| Lines of Code | ~70,000 | ~59,500 | TBD |

### Performance Metrics

| Metric | Baseline | Target | Actual |
|--------|----------|--------|--------|
| Config Load Time | TBD | -10% | TBD |
| Template Render Time | TBD | Maintained | TBD |
| Test Execution Time | TBD | Maintained | TBD |
| Build Time | TBD | Maintained | TBD |

### Developer Experience Metrics

| Metric | Baseline | Target | Actual |
|--------|----------|--------|--------|
| Onboarding Time | TBD | -30% | TBD |
| Bug Fix Time | TBD | -20% | TBD |
| Feature Dev Time | TBD | -15% | TBD |
| Code Review Time | TBD | -10% | TBD |

## Rollback Plan

### Rollback Triggers

- Critical bugs in production
- Performance degradation >20%
- Test coverage drop >10%
- Security vulnerabilities introduced

### Rollback Procedure

1. **Immediate Rollback** (if critical)
   ```bash
   git revert <commit-range>
   git push origin main
   ```

2. **Partial Rollback** (if specific feature)
   ```bash
   git revert <specific-commits>
   git push origin main
   ```

3. **Forward Fix** (if minor issues)
   - Create hotfix branch
   - Fix issues
   - Deploy fix

### Post-Rollback Actions

1. Analyze root cause
2. Document lessons learned
3. Update refactoring plan
4. Re-attempt with fixes

## Conclusion

This refactoring roadmap provides a structured approach to improving the opencenter-cli codebase. The phased implementation minimizes risk while delivering significant improvements in code quality, maintainability, and developer experience.

**Key Success Factors**:
- Comprehensive testing at each phase
- Regular code reviews
- Clear communication
- Flexibility to adjust plan
- Focus on high-impact items

**Expected Outcomes**:
- 15-20% code reduction
- 25-30% maintainability improvement
- Single source of truth for services
- Consistent error handling and validation
- Improved developer experience

**Next Steps**:
1. Review roadmap with team
2. Get approval to proceed
3. Begin Phase 1 (Foundation)
4. Track progress weekly
5. Adjust as needed

Good luck with the refactoring! 🚀
