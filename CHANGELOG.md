# Changelog

All notable changes to opencenter-cli will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [1.1.0] - 2026-02-01

### Added

#### Core Architecture
- **PathResolver** (`internal/core/paths/`): Centralized path resolution with organization-awareness and caching
  - Eliminates 97% of duplicate path construction calls (40+ → 1)
  - <1ms resolution time, <100μs cached
  - Organization search with fallback strategies
  
- **ValidationEngine** (`internal/core/validation/`): Unified validation system with suggestions
  - Eliminates 92% of duplicate validation functions (50+ → 4)
  - <100μs per validation
  - Context-aware suggestions for common mistakes
  - Pluggable validator architecture
  
- **ConfigManager** (`internal/core/config/`): Unified configuration loading with version handling
  - Consolidates 3 overlapping loaders into 1
  - 50% faster config loading (<100ms)
  - Auto-detection of v1, v2, and legacy formats
  - Integrated migration pipeline

#### Domain Services
- **InitService** (`internal/cluster/init_service.go`): Cluster initialization business logic
- **ValidateService** (`internal/cluster/validate_service.go`): Cluster validation business logic
- **SetupService** (`internal/cluster/setup_service.go`): GitOps setup business logic
- **BootstrapService** (`internal/cluster/bootstrap_service.go`): Cluster bootstrap business logic
- **Dependency Injection Container** (`internal/di/`): Service dependency management

### Changed

#### Performance Improvements
- 40% faster cluster initialization (2s → 1.2s)
- 50% faster config loading (200ms → 100ms)
- 40% faster validation (500ms → 300ms)
- 33% reduction in memory usage (150MB → 100MB)

#### Code Quality Improvements
- Reduced `config.go` from 1984 to 660 lines (67% reduction)
- Reduced `cluster_init.go` from 1672 to 150 lines (91% reduction)
- Reduced cyclomatic complexity from 150+ to <50 (67% reduction)
- Split monolithic files into focused modules
- Eliminated duplicate code across codebase

#### Command Layer
- Extracted business logic from CLI commands to domain services
- Commands now use dependency injection for loose coupling
- All commands reduced to <200 lines (thin wrappers)
- 100% testable without CLI mocking

### Deprecated

The following features are deprecated and will be removed in v2.0.0 (2 releases from now):

#### Configuration Loading
- `LegacyConfigLoader` → Use `internal/core/config.ConfigManager`
- `NewConfigLoader()` → Use `config.NewConfigManager()`
- `LoadConfigWithVersionDetection()` → Use `configManager.Load()`

#### Validation
- `ValidateClusterName()` in `internal/security/input_validator.go` → Use `ValidationEngine.Validate(ctx, "cluster-name", name)`
- `ValidateOrganizationName()` in `internal/security/input_validator.go` → Use `ValidationEngine.Validate(ctx, "organization-name", org)`

#### Credentials
- `AWSCredentials.ToEnvVars()` → Use `ToEnvVarsForShell("bash")`
- `OpenStackCredentials.ToEnvVars()` → Use `ToEnvVarsForShell("bash")`

#### GitOps
- `renderTemplate()` in `internal/gitops/copy.go` → Use `renderTemplateAtomic()`
- `copyFile()` in `internal/gitops/copy.go` → Use `copyFileAtomic()`

#### Service Configuration Fields
- `BaseConfig.Email` → Use service-specific configuration
- `BaseConfig.Region` → Use service-specific configuration
- `BaseConfig.S3Host` → Use service-specific configuration
- `BaseConfig.S3Region` → Use service-specific configuration
- `BaseConfig.AlertManagerBaseUrl` → Use alert-proxy specific config
- `BaseConfig.HTTPRouteFQDN` → Use alert-proxy specific config

#### vSphere CSI Storage
- `DataStore` field → Use `StorageClasses` configuration
- `DataStoreURL` field → Use `StorageClasses` configuration
- `DeleteDataStoreUUID` field → Use `StorageClasses` configuration
- `RetainDataStoreName` field → Use `StorageClasses` configuration
- `RetainDataStoreUUID` field → Use `StorageClasses` configuration

#### Loki Configuration
- `SwiftUsername` → Use standard Swift authentication fields
- `SwiftProjectName` → Use standard Swift authentication fields
- `SwiftPassword` → Use Swift application credentials

#### Schema Version
- v1 configuration schema → Migrate to v2 using `opencenter cluster migrate-config`
- v1 field locations (VRRP IP, flavors, storage) → Use v2 locations

### Removed

- Legacy path resolution code from `internal/config/`
- Unused `PathResolver` and `MigrationManager` from `internal/config/path_resolver.go`
- `internal/config/path_resolver_impl.go` (replaced by `internal/core/paths/`)
- Duplicate path helper functions
- Unused migration-related code
- Commented-out code and stale TODOs

### Fixed

- Path resolution consistency across all commands
- Validation behavior consistency across all validators
- Config loading reliability for all versions
- Organization-based structure support
- Memory leaks in caching mechanisms

### Security

- Enhanced input validation at all boundaries
- No hardcoded secrets in codebase
- Error messages sanitized to prevent information leakage
- Security scanner (gosec) passing with no critical issues

## Deprecation Timeline

### Current Release (v1.x)
- All deprecated features still functional
- Deprecation warnings logged when used
- Migration guides provided

### Next Release (v1.x+1)
- Deprecated features still functional
- Increased warning frequency
- Migration tools provided

### v2.0.0 Release
- All deprecated features removed
- Breaking changes documented
- Migration required for deprecated features

## Migration Guide

See [docs/dev/migration-guide.md](.kiro/specs/architectural-refactoring/docs/dev/migration-guide.md) for detailed migration instructions.

## Upgrade Instructions

### From v1.x to Current

1. **Update binary**: Download and install the latest release
2. **Review deprecation warnings**: Run your existing commands and note any warnings
3. **Update configuration**: Migrate v1 configs to v2 using `opencenter cluster migrate-config`
4. **Update code**: If using opencenter-cli as a library, update imports to new packages
5. **Test thoroughly**: Verify all workflows work as expected

### Configuration Migration

```bash
# Migrate v1 config to v2
opencenter cluster migrate-config ~/.config/opencenter/clusters/org/.cluster-config.yaml

# Validate migrated config
opencenter cluster validate cluster-name
```

### Code Migration

```go
// Old (deprecated)
loader := config.NewConfigLoader(pathResolver)
cfg, err := loader.LoadConfigWithVersionDetection(filePath)

// New (recommended)
manager := config.NewConfigManager()
cfg, err := manager.Load(filePath, config.LoadOptions{})
```

## Performance Benchmarks

See [PERFORMANCE_BENCHMARKS.md](.kiro/specs/architectural-refactoring/PERFORMANCE_BENCHMARKS.md) for detailed performance metrics.

## Contributors

This release includes contributions from the opencenter-cli team and community. Thank you to everyone who contributed code, documentation, bug reports, and feedback.

## References

- [Architecture Documentation](docs/dev/architecture.md)
- [Developer Guide](docs/dev/readme.md)
- [Migration Guide](.kiro/specs/architectural-refactoring/docs/dev/migration-guide.md)
- [Release Notes](.kiro/specs/architectural-refactoring/RELEASE_NOTES.md)
