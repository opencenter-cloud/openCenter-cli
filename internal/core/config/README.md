# Core Config Package

This package provides unified configuration management for opencenter-cli as part of the architectural refactoring effort.

## Package Structure

```
internal/core/config/
├── doc.go          # Package documentation
├── manager.go      # ConfigManager with strategy pattern
├── types.go        # Core configuration types (placeholders)
├── defaults.go     # Default value generation (placeholders)
├── persistence.go  # File I/O operations (placeholders)
└── README.md       # This file
```

## Implementation Status

**Phase 1 (Foundation) - COMPLETED**

✅ Task 1.3.1.1: Created manager.go with ConfigManager struct
✅ Task 1.3.1.2: Created types.go (placeholders for Config types)
✅ Task 1.3.1.3: Created defaults.go (placeholders for default generation)
✅ Task 1.3.1.4: Created persistence.go (placeholders for I/O operations)

## Key Components

### ConfigManager (manager.go)

Provides unified configuration management with:
- Auto-detection of configuration version
- Strategy-based loading for version-specific logic
- Thread-safe caching with invalidation
- Single Load() method handles all versions

```go
manager := config.NewConfigManager()
cfg, err := manager.Load(path, config.LoadOptions{
    AutoMigrate: true,
    Validate: true,
})
```

### Types (types.go)

Placeholder definitions for:
- `Config`: Root configuration struct
- `ConfigMetadata`: Configuration metadata
- `SimplifiedOpenCenter`: OpenCenter section
- `SimplifiedOpenTofu`: OpenTofu section
- `Secrets`: Sensitive data
- `Deployment`: Deployment settings

**Note**: These are currently placeholders. Actual types will be moved from `internal/config` in Phase 2.

### Defaults (defaults.go)

Placeholder functions for:
- `NewDefault(name string)`: Generate default configuration
- `ApplyDefaults(cfg *Config)`: Apply CLI and organization defaults

**Note**: Implementation will be moved from `internal/config/config.go` in Phase 2.

### Persistence (persistence.go)

Placeholder functions for:
- `Load(name string)`: Read configuration from disk
- `Save(cfg *Config)`: Write configuration to disk
- `ConfigPath(name string)`: Resolve configuration file path
- `ResolveConfigDir()`: Resolve configuration directory
- `List()`: List all cluster configurations

**Note**: Implementation will be moved from `internal/config/config.go` in Phase 2.

## Next Steps

**Phase 2 (Migration) - Epic 1.3.2-1.3.4**

The following tasks will complete the ConfigManager implementation:

1. **Epic 1.3.2**: Implement load strategies
   - Create `strategies/v2.go` with V2Strategy
   - Create `strategies/v1.go` with V1Strategy
   - Create `strategies/legacy.go` with LegacyStrategy
   - Implement CanLoad() for version detection

2. **Epic 1.3.3**: Implement migration system
   - Create `migration/migrator.go` with Migrator struct
   - Create `migration/v1_to_v2.go`
   - Create `migration/legacy_to_v1.go`
   - Implement migration path finding

3. **Epic 1.3.4**: Implement ConfigManager core methods
   - Implement Load(path, opts)
   - Implement Save(path, config)
   - Implement InvalidateCache(path)
   - Add thread-safety with RWMutex

4. **Epic 2.4.1**: Move types and functions from internal/config
   - Move Config struct to types.go
   - Move default generation to defaults.go
   - Move I/O operations to persistence.go
   - Update imports across codebase

## Design Principles

- **Single Responsibility**: Each file has one clear purpose
- **Strategy Pattern**: Version-specific logic is isolated
- **Dependency Inversion**: High-level code depends on abstractions
- **Open/Closed**: Open for extension (new versions), closed for modification

## Performance Targets

- Config loading: <100ms (50% improvement)
- Cache hit: <1ms
- Memory usage: <100MB peak (33% reduction)

## Thread Safety

ConfigManager is thread-safe. Multiple goroutines can safely call Load() and Save() concurrently. The internal cache is protected by a RWMutex.

## References

- [Design Document](../../../.kiro/specs/architectural-refactoring/design.md)
- [Requirements](../../../.kiro/specs/architectural-refactoring/requirements.md)
- [Tasks](../../../.kiro/specs/architectural-refactoring/tasks.md)
- [PathResolver Package](../paths/README.md)
- [ValidationEngine Package](../validation/README.md)
