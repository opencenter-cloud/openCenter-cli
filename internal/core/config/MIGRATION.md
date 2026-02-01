# ConfigManager Migration Guide

## Table of Contents

- [Overview](#overview)
- [Why Migrate](#why-migrate)
- [Migration Strategy](#migration-strategy)
- [Before You Start](#before-you-start)
- [Step-by-Step Migration](#step-by-step-migration)
- [Common Patterns](#common-patterns)
- [Version Migration](#version-migration)
- [Testing Your Migration](#testing-your-migration)
- [Rollback Plan](#rollback-plan)
- [Performance Considerations](#performance-considerations)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

## Overview

This guide helps you migrate from multiple overlapping configuration loaders to the unified ConfigManager system. The migration consolidates 3 loaders into 1, provides automatic version detection and migration, and improves performance through caching.

**Target audience**: Developers working on opencenter-cli codebase

**Estimated time**: 3-5 hours per package

**Difficulty**: Medium

## Why Migrate

### Current Problems

- **3 overlapping loaders** (v1, v2, legacy) with duplicate logic
- **Manual version detection** required before loading
- **No automatic migration** between versions
- **No caching** - configs re-parsed on every load
- **Inconsistent error handling** across loaders
- **Hard to maintain** - changes require updates in multiple places
- **1984-line config.go** with mixed responsibilities

### Benefits After Migration

- **Single unified loader** handles all versions
- **Automatic version detection** from YAML structure
- **Integrated migration pipeline** for seamless upgrades
- **50% faster loading** (<100ms vs ~200ms)
- **Thread-safe caching** with invalidation
- **Consistent error handling** across all versions
- **Focused modules** (<500 lines per file)
- **Easy to test** - no complex mocking needed

## Migration Strategy

### Phased Approach

1. **Phase 1**: Migrate cmd/ package (CLI commands)
2. **Phase 2**: Migrate internal/gitops/ package
3. **Phase 3**: Migrate internal/operations/ package
4. **Phase 4**: Migrate remaining packages
5. **Phase 5**: Deprecate old loaders

### Backward Compatibility

- Old loader functions remain available during migration
- Marked as deprecated with warnings
- Removed after 2 releases (6 months)
- Feature flags for gradual rollout
- All existing configs continue to work

## Before You Start

### Prerequisites

1. Read the package documentation: `internal/core/config/doc.go`
2. Review usage examples: `internal/core/config/example_test.go`
3. Understand your current config loading patterns
4. Identify all config loading calls in your module

### Identify Config Loading Calls

Search for common patterns:

```bash
# Find direct config loading
grep -r 'config\.Load' cmd/ internal/

# Find version-specific loaders
grep -r 'LoadV1\|LoadV2\|LoadLegacy' cmd/ internal/

# Find manual version detection
grep -r 'DetectVersion\|apiVersion' cmd/ internal/

# Find config saving
grep -r 'config\.Save\|WriteConfig' cmd/ internal/
```

## Step-by-Step Migration

### Step 1: Add ConfigManager Dependency

Import the config package:

```go
import (
    "github.com/rackerlabs/opencenter-cli/internal/core/config"
    "github.com/rackerlabs/opencenter-cli/internal/core/config/strategies"
)
```

### Step 2: Create Manager Instance

**Option A: Singleton Pattern (Recommended)**

Create a single manager instance for your application:

```go
// In your main package or initialization code
var (
    configManager     *config.ConfigManager
    configManagerOnce sync.Once
)

func GetConfigManager() *config.ConfigManager {
    configManagerOnce.Do(func() {
        configManager = config.NewConfigManager()
        
        // Register all supported strategies
        configManager.RegisterStrategy(strategies.NewV2Strategy())
        configManager.RegisterStrategy(strategies.NewV1Strategy())
        configManager.RegisterStrategy(strategies.NewLegacyStrategy())
    })
    return configManager
}
```

**Option B: Dependency Injection**

Pass manager as a dependency:

```go
type ClusterService struct {
    configManager *config.ConfigManager
}

func NewClusterService(manager *config.ConfigManager) *ClusterService {
    return &ClusterService{
        configManager: manager,
    }
}
```

### Step 3: Replace Config Loading

**Before (Old Pattern)**:

```go
// Manual version detection and loading
data, err := os.ReadFile(configPath)
if err != nil {
    return nil, err
}

version, err := DetectVersion(data)
if err != nil {
    return nil, err
}

var cfg *Config
switch version {
case "v2":
    cfg, err = LoadV2(data)
case "v1":
    cfg, err = LoadV1(data)
default:
    cfg, err = LoadLegacy(data)
}

if err != nil {
    return nil, err
}

// Manual migration if needed
if version != "v2" {
    cfg, err = MigrateToV2(cfg)
    if err != nil {
        return nil, err
    }
}
```

**After (New Pattern)**:

```go
// Automatic version detection, loading, and migration
manager := GetConfigManager()
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: true,
    Validate:    true,
})
if err != nil {
    return nil, fmt.Errorf("failed to load config: %w", err)
}
```

### Step 4: Replace Config Saving

**Before**:

```go
// Manual YAML marshaling and file writing
data, err := yaml.Marshal(cfg)
if err != nil {
    return err
}

err = os.WriteFile(configPath, data, 0644)
if err != nil {
    return err
}
```

**After**:

```go
// Unified save operation
manager := GetConfigManager()
err := manager.Save(configPath, cfg)
if err != nil {
    return fmt.Errorf("failed to save config: %w", err)
}
```

### Step 5: Update Tests

**Before**:

```go
func TestConfigLoad(t *testing.T) {
    // Manual config creation and loading
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    
    data := []byte(`
cluster:
  name: test
  provider: openstack
`)
    os.WriteFile(configPath, data, 0644)
    
    cfg, err := LoadLegacy(data)
    require.NoError(t, err)
    assert.Equal(t, "test", cfg.Cluster.Name)
}
```

**After**:

```go
func TestConfigLoad(t *testing.T) {
    // Use manager in tests
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    
    data := []byte(`
apiVersion: opencenter.rackspace.com/v2
kind: ClusterConfig
metadata:
  name: test
cluster:
  name: test
  provider: openstack
`)
    os.WriteFile(configPath, data, 0644)
    
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV2Strategy())
    
    cfg, err := manager.Load(configPath, config.LoadOptions{})
    require.NoError(t, err)
    assert.Equal(t, "test", cfg.Cluster.Name)
}
```

## Common Patterns

### Pattern 1: Basic Config Loading

**Before**:

```go
cfg, err := config.Load(clusterName)
if err != nil {
    return err
}
```

**After**:

```go
manager := GetConfigManager()
configPath := getConfigPath(clusterName)  // Use PathResolver
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: true,
    Validate:    true,
})
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

### Pattern 2: Loading with Version Check

**Before**:

```go
data, _ := os.ReadFile(configPath)
version, _ := DetectVersion(data)

if version != "v2" {
    return fmt.Errorf("unsupported version: %s", version)
}

cfg, err := LoadV2(data)
```

**After**:

```go
// Version detection is automatic
manager := GetConfigManager()
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: false,  // Fail if not v2
    Validate:    true,
})
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}
```

### Pattern 3: Loading with Auto-Migration

**Before**:

```go
data, _ := os.ReadFile(configPath)
version, _ := DetectVersion(data)

var cfg *Config
switch version {
case "v2":
    cfg, _ = LoadV2(data)
case "v1":
    cfg, _ = LoadV1(data)
    cfg, _ = MigrateV1ToV2(cfg)
default:
    cfg, _ = LoadLegacy(data)
    cfg, _ = MigrateLegacyToV1(cfg)
    cfg, _ = MigrateV1ToV2(cfg)
}

// Save migrated config
data, _ = yaml.Marshal(cfg)
os.WriteFile(configPath, data, 0644)
```

**After**:

```go
// Automatic migration and save
manager := GetConfigManager()
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: true,
})
if err != nil {
    return fmt.Errorf("failed to load config: %w", err)
}

// Config is automatically migrated and saved
```

### Pattern 4: Creating New Config

**Before**:

```go
cfg := &Config{
    APIVersion: "opencenter.rackspace.com/v2",
    Kind:       "ClusterConfig",
    Metadata: Metadata{
        Name:    clusterName,
        Version: "2.0",
    },
    Cluster: ClusterConfig{
        Name:     clusterName,
        Provider: "openstack",
    },
    // ... many more fields
}

data, _ := yaml.Marshal(cfg)
os.WriteFile(configPath, data, 0644)
```

**After**:

```go
// Use default generator
cfg := config.NewDefault(clusterName)
cfg.Cluster.Provider = "openstack"

manager := GetConfigManager()
err := manager.Save(configPath, cfg)
if err != nil {
    return fmt.Errorf("failed to save config: %w", err)
}
```

### Pattern 5: Loading with Cache

**Before**:

```go
// No caching - re-parse every time
cfg1, _ := config.Load(clusterName)
cfg2, _ := config.Load(clusterName)  // Re-parses YAML
```

**After**:

```go
// Automatic caching
manager := GetConfigManager()
cfg1, _ := manager.Load(configPath, config.LoadOptions{})
cfg2, _ := manager.Load(configPath, config.LoadOptions{})  // Returns cached
```

### Pattern 6: Cache Invalidation

**Before**:

```go
// No cache to invalidate
```

**After**:

```go
// Invalidate after external changes
manager := GetConfigManager()
manager.InvalidateCache(configPath)

// Next load will re-read from disk
cfg, _ := manager.Load(configPath, config.LoadOptions{})
```

### Pattern 7: Loading with Validation

**Before**:

```go
cfg, err := config.Load(clusterName)
if err != nil {
    return err
}

// Manual validation
if err := ValidateConfig(cfg); err != nil {
    return err
}
```

**After**:

```go
// Integrated validation
manager := GetConfigManager()
cfg, err := manager.Load(configPath, config.LoadOptions{
    Validate: true,
})
if err != nil {
    return fmt.Errorf("config validation failed: %w", err)
}
```

## Version Migration

### Understanding Version Detection

The ConfigManager automatically detects the configuration version by examining the YAML structure:

- **V2**: Has `apiVersion: opencenter.rackspace.com/v2` and `kind: ClusterConfig`
- **V1**: Has `cluster` section but no `apiVersion`
- **Legacy**: Flat structure with no nesting

### Migration Paths

```
Legacy → V1 → V2
  ↓      ↓
  └──────┴──→ V2 (direct)
```

### Migration Behavior

When `AutoMigrate: true`:

1. ConfigManager detects the version
2. Loads using appropriate strategy
3. Migrates to V2 if needed
4. Backs up original file
5. Saves migrated version
6. Returns V2 config

### Migration Example

**Original V1 Config**:

```yaml
cluster:
  name: my-cluster
  provider: openstack
opencenter:
  organization: myorg
```

**After Auto-Migration**:

```yaml
apiVersion: opencenter.rackspace.com/v2
kind: ClusterConfig
metadata:
  name: my-cluster
  version: "2.0"
  createdAt: "2025-01-31T12:00:00Z"
cluster:
  name: my-cluster
  provider: openstack
opencenter:
  organization: myorg
```

### Manual Migration

If you need to migrate without loading:

```go
manager := GetConfigManager()

// Load without auto-migration
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: false,
})
if err != nil {
    return err
}

// Check if migration needed
if cfg.APIVersion != "opencenter.rackspace.com/v2" {
    // Manually trigger migration
    migrator := migration.NewMigrator()
    cfg, err = migrator.Migrate(cfg, "v2")
    if err != nil {
        return err
    }
    
    // Save migrated config
    err = manager.Save(configPath, cfg)
    if err != nil {
        return err
    }
}
```

## Testing Your Migration

### Unit Tests

Test config loading in isolation:

```go
func TestConfigLoad(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    
    // Create test config
    configContent := `apiVersion: opencenter.rackspace.com/v2
kind: ClusterConfig
metadata:
  name: test
cluster:
  name: test
  provider: openstack
`
    os.WriteFile(configPath, []byte(configContent), 0644)
    
    // Load with manager
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV2Strategy())
    
    cfg, err := manager.Load(configPath, config.LoadOptions{})
    require.NoError(t, err)
    assert.Equal(t, "test", cfg.Cluster.Name)
}
```

### Migration Tests

Test version migration:

```go
func TestAutoMigration(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    
    // Create v1 config
    v1Content := `cluster:
  name: test
  provider: openstack
`
    os.WriteFile(configPath, []byte(v1Content), 0644)
    
    // Load with auto-migration
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV1Strategy())
    manager.RegisterStrategy(strategies.NewV2Strategy())
    
    cfg, err := manager.Load(configPath, config.LoadOptions{
        AutoMigrate: true,
    })
    require.NoError(t, err)
    
    // Verify migration
    assert.Equal(t, "opencenter.rackspace.com/v2", cfg.APIVersion)
    assert.Equal(t, "test", cfg.Cluster.Name)
}
```

### Cache Tests

Test caching behavior:

```go
func TestCaching(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    
    // Create config
    configContent := `apiVersion: opencenter.rackspace.com/v2
kind: ClusterConfig
metadata:
  name: test
cluster:
  name: test
`
    os.WriteFile(configPath, []byte(configContent), 0644)
    
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV2Strategy())
    
    // First load
    cfg1, err := manager.Load(configPath, config.LoadOptions{})
    require.NoError(t, err)
    
    // Modify file
    newContent := strings.Replace(configContent, "name: test", "name: modified", 1)
    os.WriteFile(configPath, []byte(newContent), 0644)
    
    // Second load (cached)
    cfg2, err := manager.Load(configPath, config.LoadOptions{})
    require.NoError(t, err)
    assert.Equal(t, "test", cfg2.Cluster.Name)  // Still cached
    
    // Invalidate and reload
    manager.InvalidateCache(configPath)
    cfg3, err := manager.Load(configPath, config.LoadOptions{})
    require.NoError(t, err)
    assert.Equal(t, "modified", cfg3.Cluster.Name)  // Fresh load
}
```

### Benchmark Tests

Verify performance improvements:

```go
func BenchmarkConfigLoad(b *testing.B) {
    configPath := filepath.Join(b.TempDir(), "config.yaml")
    
    configContent := `apiVersion: opencenter.rackspace.com/v2
kind: ClusterConfig
metadata:
  name: test
cluster:
  name: test
`
    os.WriteFile(configPath, []byte(configContent), 0644)
    
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV2Strategy())
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := manager.Load(configPath, config.LoadOptions{})
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Validation Checklist

- [ ] All config loading calls replaced
- [ ] Tests pass with new manager
- [ ] No direct LoadV1/LoadV2/LoadLegacy calls
- [ ] Error handling updated
- [ ] Cache invalidation added where needed
- [ ] Performance benchmarks show improvement
- [ ] Migration tests pass
- [ ] Documentation updated

## Rollback Plan

### If Issues Occur

1. **Revert to old loaders**: Old functions remain available
2. **Feature flag**: Disable manager with environment variable
3. **Gradual rollback**: Revert one package at a time
4. **Monitor metrics**: Watch for performance regressions

### Rollback Steps

```go
// Add feature flag check
if os.Getenv("USE_LEGACY_CONFIG_LOADER") == "true" {
    // Use old loader
    cfg, err = config.Load(clusterName)
} else {
    // Use new manager
    manager := GetConfigManager()
    cfg, err = manager.Load(configPath, config.LoadOptions{
        AutoMigrate: true,
        Validate:    true,
    })
}
```

## Performance Considerations

### Expected Improvements

- **First load**: <100ms (50% faster than old loaders)
- **Cached load**: <1ms (100x faster)
- **Memory overhead**: ~10KB per cached config
- **Cache hit rate**: >90% in typical usage

### Optimization Tips

1. **Enable caching**: Default is enabled, keep it on
2. **Reuse manager**: Create one instance, use everywhere
3. **Batch operations**: Load multiple configs together
4. **Invalidate wisely**: Only invalidate when config changes
5. **Monitor cache**: Check hit rate with `GetCacheStats()`

### Performance Monitoring

```go
// Log cache statistics periodically
stats := manager.GetCacheStats()
log.Printf("Config cache: %d entries, %.2f%% hit rate", 
    stats.Entries, stats.HitRate*100)
```

## Troubleshooting

### Issue: "unsupported config version"

**Cause**: Config version not recognized by any strategy

**Solution**:

```go
// Ensure all strategies are registered
manager := config.NewConfigManager()
manager.RegisterStrategy(strategies.NewV2Strategy())
manager.RegisterStrategy(strategies.NewV1Strategy())
manager.RegisterStrategy(strategies.NewLegacyStrategy())
```

### Issue: "migration failed"

**Cause**: Config structure incompatible with migration

**Solution**:

```go
// Load without auto-migration to see original error
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: false,
})
if err != nil {
    log.Printf("Original error: %v", err)
}
```

### Issue: "validation failed"

**Cause**: Config doesn't meet validation requirements

**Solution**:

```go
// Load without validation to see config
cfg, err := manager.Load(configPath, config.LoadOptions{
    Validate: false,
})
if err == nil {
    // Manually validate to see specific errors
    if err := validateConfig(cfg); err != nil {
        log.Printf("Validation errors: %v", err)
    }
}
```

### Issue: Cache not working

**Cause**: Cache disabled or invalidated too frequently

**Solution**:

```go
// Check cache statistics
stats := manager.GetCacheStats()
if stats.HitRate < 0.5 {
    log.Printf("Low cache hit rate: %.2f%%", stats.HitRate*100)
    // Investigate cache invalidation calls
}
```

### Issue: Performance regression

**Cause**: Validation enabled or cache disabled

**Solution**:

```go
// Disable expensive validation in production
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: true,
    Validate:    false,  // Disable validation
    SkipCache:   false,  // Keep caching enabled
})
```

## FAQ

### Q: Do I need to migrate all at once?

**A**: No. Migrate one package at a time. Old and new code can coexist.

### Q: What happens to my existing configs?

**A**: They continue to work. The manager auto-detects and loads all versions.

### Q: Will my configs be automatically migrated?

**A**: Only if you set `AutoMigrate: true`. Otherwise, they load as-is.

### Q: What if migration fails?

**A**: The original file is backed up. You can restore it and investigate the error.

### Q: How do I handle errors from Load()?

**A**: Always check errors and provide context:

```go
cfg, err := manager.Load(configPath, config.LoadOptions{
    AutoMigrate: true,
    Validate:    true,
})
if err != nil {
    return fmt.Errorf("failed to load config from %s: %w", configPath, err)
}
```

### Q: Should I enable validation?

**A**: Yes in development and CI. Optional in production for performance.

### Q: How do I test code that uses ConfigManager?

**A**: Create a manager with test configs:

```go
func TestMyFunction(t *testing.T) {
    configPath := filepath.Join(t.TempDir(), "config.yaml")
    // Write test config
    manager := config.NewConfigManager()
    manager.RegisterStrategy(strategies.NewV2Strategy())
    // Use manager in tests
}
```

### Q: What about backward compatibility?

**A**: Old loader functions remain available for 2 releases (6 months) with deprecation warnings.

### Q: How do I migrate tests?

**A**: Replace direct loader calls with manager calls. See "Testing Your Migration" section.

### Q: What if performance is worse after migration?

**A**: Check that caching is enabled and validation is disabled. See "Performance Considerations" section.

### Q: Can I use ConfigManager in concurrent code?

**A**: Yes. All manager operations are thread-safe.

### Q: How do I invalidate the cache?

**A**: Call `InvalidateCache(path)` after modifying config files externally.

## Next Steps

1. **Start small**: Migrate one file or function
2. **Test thoroughly**: Run unit and integration tests
3. **Monitor performance**: Check cache hit rates
4. **Iterate**: Migrate more code gradually
5. **Document**: Update package documentation
6. **Review**: Get code review from team

## Additional Resources

- Package documentation: `internal/core/config/doc.go`
- Usage examples: `internal/core/config/example_test.go`
- Design document: `.kiro/specs/architectural-refactoring/design.md`
- Requirements: `.kiro/specs/architectural-refactoring/requirements.md`
- Config manager spec: `.kiro/specs/architectural-refactoring/03-config-manager.md`
- Strategy implementations: `internal/core/config/strategies/`
- Migration implementations: `internal/core/config/migration/`

## Support

For questions or issues:

1. Check this migration guide
2. Review package documentation
3. Check existing tests for examples
4. Ask in team chat or create an issue
