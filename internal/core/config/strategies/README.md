# Configuration Loading Strategies

This package implements version-specific configuration loading strategies for opencenter-cli.

## Overview

The strategies package provides a Strategy pattern implementation for loading different versions of opencenter configuration files. Each strategy knows how to detect, load, and parse its specific configuration format.

## Architecture

```
ConfigManager
    ├── V2Strategy (schema_version: "2.0")
    ├── V1Strategy (schema_version: "1.0" or missing)
    └── LegacyStrategy (flat structure, pre-versioning)
```

## Strategies

### V2Strategy

**Purpose**: Loads v2.0 schema configurations

**Detection**: `schema_version: "2.0"`

**Features**:
- Uses `internal/config/v2` package
- Complete loading pipeline: Load → Normalize → Resolve → Hydrate → Validate → Freeze
- Provider-region defaults
- Multi-layered validation

**Status**: Implemented, conversion to v1 Config pending full v2 integration

### V1Strategy

**Purpose**: Loads v1.0 schema configurations

**Detection**: `schema_version: "1.0"` OR missing schema_version (backward compatibility)

**Features**:
- Uses `internal/config` package
- Applies defaults from `defaultConfig()`
- Resolves configuration references
- Backward compatible with pre-versioning configs

**Status**: Fully implemented and tested

### LegacyStrategy

**Purpose**: Loads pre-versioning flat configuration files

**Detection**:
- No `schema_version` field
- No `opencenter` section OR empty `opencenter` section
- Presence of legacy top-level fields (`cluster_name`, `provider`, etc.)

**Features**:
- Converts flat structure to current Config format
- Basic field mapping (provider, region, environment)
- Foundation for full legacy migration

**Status**: Basic implementation complete, full field mapping pending

## Usage

### Registration

Strategies are registered with ConfigManager:

```go
manager := config.NewConfigManager()
manager.RegisterStrategy(strategies.NewV2Strategy())
manager.RegisterStrategy(strategies.NewV1Strategy())
manager.RegisterStrategy(strategies.NewLegacyStrategy())
```

### Automatic Selection

ConfigManager automatically selects the appropriate strategy:

```go
config, err := manager.Load(path, config.LoadOptions{
    AutoMigrate: true,
    Validate: true,
})
```

### Version Detection

Each strategy implements `CanLoad()` for version detection:

```go
strategy := strategies.NewV1Strategy()
canLoad, err := strategy.CanLoad(data)
if canLoad {
    config, err := strategy.Load(data, clusterName)
}
```

## Implementation Details

### LoadStrategy Interface

```go
type LoadStrategy interface {
    CanLoad(data []byte) (bool, error)
    Load(data []byte, clusterName string) (*Config, error)
    Version() string
}
```

### Version Detection Priority

1. **V2Strategy**: Explicit `schema_version: "2.0"`
2. **V1Strategy**: Explicit `schema_version: "1.0"` OR missing version
3. **LegacyStrategy**: No version + no opencenter section + legacy fields

Note: V1 and Legacy can both match configs without schema_version. ConfigManager should try V1 first for backward compatibility.

## Testing

Comprehensive tests cover:
- Version detection for each strategy
- Loading and parsing
- Edge cases (invalid YAML, missing fields)
- Strategy selection logic

Run tests:
```bash
go test ./internal/core/config/strategies/...
```

## Future Enhancements

### V2Strategy
- Complete v2.Config to config.Config conversion
- Full integration with v2 loading pipeline

### LegacyStrategy
- Complete field mapping for all legacy fields
- Integration with migration system
- Automatic upgrade to v1/v2 format

### ConfigManager
- Strategy priority ordering
- Fallback strategies
- Migration pipeline integration

## Related Documentation

- [ConfigManager](../manager.go) - Main configuration manager
- [Design Document](../../../../.kiro/specs/architectural-refactoring/design.md) - Architecture overview
- [Requirements](../../../../.kiro/specs/architectural-refactoring/requirements.md) - Epic 1.3.2 requirements

## Phase Information

**Phase**: 1 (Foundation)  
**Epic**: 1.3.2 (Implement load strategies)  
**Status**: Complete

**Subtasks**:
- [x] 1.3.2.1 Create strategies/v2.go with V2Strategy
- [x] 1.3.2.2 Create strategies/v1.go with V1Strategy
- [x] 1.3.2.3 Create strategies/legacy.go with LegacyStrategy
- [x] 1.3.2.4 Implement CanLoad() for version detection

**Next Steps**: Epic 1.3.3 (Implement migration system)
