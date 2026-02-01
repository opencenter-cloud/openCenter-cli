# Configuration Migration System

This package provides a graph-based migration system for opencenter-cli configuration files, enabling automatic migration between different schema versions.

## Overview

The migration system supports:
- **Automatic path finding**: Uses BFS to find the shortest migration path between versions
- **Multi-hop migrations**: Can migrate through multiple versions (e.g., legacy → 1.0 → 2.0)
- **Validation**: Each migration step validates the configuration
- **Extensibility**: Easy to add new migrations

## Architecture

### Core Components

1. **Migrator** (`migrator.go`)
   - Main migration engine
   - Manages migration registry
   - Finds optimal migration paths using BFS
   - Executes migration chains

2. **Migration** (`migrator.go`)
   - Represents a single migration step
   - Contains source version, target version, and migration function
   - Includes description for documentation

3. **Specific Migrations**
   - `legacy_to_v1.go`: Migrates flat legacy configs to v1.0 structured format
   - `v1_to_v2.go`: Migrates v1.0 configs to v2.0 with enhanced features

4. **Registry** (`registry.go`)
   - Provides `DefaultMigrator()` with all standard migrations registered
   - Utility functions for migration graph visualization

## Usage

### Basic Usage

```go
import "github.com/rackerlabs/opencenter-cli/internal/core/config/migration"

// Get default migrator with all standard migrations
migrator, err := migration.DefaultMigrator()
if err != nil {
    log.Fatal(err)
}

// Migrate a configuration to v2.0
migrated, err := migrator.Migrate(cfg, "2.0")
if err != nil {
    log.Fatal(err)
}
```

### Check Migration Availability

```go
// Check if migration path exists
if migrator.CanMigrate("legacy", "2.0") {
    fmt.Println("Can migrate from legacy to 2.0")
}

// Find the migration path
path, err := migrator.FindPath("legacy", "2.0")
// Returns: ["legacy", "1.0", "2.0"]
```

### Custom Migrations

```go
// Create a custom migrator
migrator := migration.NewMigrator()

// Register a custom migration
customMigration := &migration.Migration{
    From: "1.5",
    To:   "2.0",
    Migrate: func(cfg *config.Config) (*config.Config, error) {
        // Perform migration logic
        cfg.SchemaVersion = "2.0"
        return cfg, nil
    },
    Description: "Custom migration from 1.5 to 2.0",
}

if err := migrator.Register(customMigration); err != nil {
    log.Fatal(err)
}
```

## Migration Graph

Current migration paths:

```
legacy -> 1.0 -> 2.0
```

To view the migration graph:

```go
migrator, _ := migration.DefaultMigrator()
fmt.Println(migration.MigrationGraph(migrator))
```

## Migration Details

### Legacy to v1.0

**Purpose**: Migrate flat configuration structure to hierarchical v1.0 format

**Changes**:
- Creates structured metadata with timestamps
- Organizes configuration into opencenter, opentofu, and secrets sections
- Migrates legacy networking fields to proper locations
- Preserves all existing configuration data

**Example**:
```yaml
# Legacy (flat)
cluster_name: my-cluster
provider: openstack
region: us-east-1

# v1.0 (structured)
schema_version: "1.0"
metadata:
  created_at: 2025-01-31T10:00:00Z
  updated_at: 2025-01-31T10:00:00Z
opencenter:
  meta:
    name: my-cluster
    region: us-east-1
  infrastructure:
    provider: openstack
```

### v1.0 to v2.0

**Purpose**: Migrate to v2.0 schema with enhanced features

**Changes**:
- Updates schema version to 2.0
- Migrates legacy networking fields to new structure
- Validates provider-specific configuration
- Clears deprecated legacy fields
- Maintains backward compatibility

**Validation**:
- OpenStack: Requires `auth_url` and `tenant_name`
- AWS: Requires `region`
- vSphere: Basic validation (extensible)

## Testing

Comprehensive test coverage includes:

1. **Unit Tests** (`migrator_test.go`)
   - Migrator creation and registration
   - Path finding (BFS algorithm)
   - Migration execution
   - Error handling

2. **Migration-Specific Tests**
   - `legacy_to_v1_test.go`: Legacy migration tests
   - `v1_to_v2_test.go`: v1 to v2 migration tests
   - Data preservation verification
   - Provider validation tests

Run tests:
```bash
go test -v ./internal/core/config/migration/...
```

## Performance

Typical performance characteristics:

- **Path finding**: <1ms (BFS on small graph)
- **Single-hop migration**: <10ms
- **Multi-hop migration**: <50ms (legacy → 1.0 → 2.0)

## Error Handling

Migrations can fail at any step:

```go
migrated, err := migrator.Migrate(cfg, "2.0")
if err != nil {
    // Error includes source and target versions
    // Original configuration is not modified
    // No partial migrations are applied
    log.Printf("Migration failed: %v", err)
}
```

## Thread Safety

The Migrator is safe for concurrent use after all migrations are registered. Registration should be done during initialization, not during concurrent use.

## Best Practices

1. **Use DefaultMigrator()** for standard migrations
2. **Register custom migrations** during initialization
3. **Validate configurations** after migration
4. **Test migrations** with real configuration files
5. **Document breaking changes** in migration descriptions

## Future Enhancements

Planned improvements:

- Rollback support for failed migrations
- Migration hooks for custom validation
- Parallel migration execution
- Migration dry-run mode
- Migration history tracking

## Integration

The migration system integrates with:

- **ConfigManager**: Automatic migration during config loading
- **Strategies**: Version detection and strategy selection
- **Validator**: Post-migration validation

## Contributing

When adding new migrations:

1. Create migration file (e.g., `v2_to_v3.go`)
2. Implement migration function
3. Add comprehensive tests
4. Register in `registry.go`
5. Update this README with migration details
6. Document breaking changes

## References

- [Design Document](../../../../../.kiro/specs/architectural-refactoring/design.md)
- [Requirements](../../../../../.kiro/specs/architectural-refactoring/requirements.md)
- [Package Documentation](doc.go)
