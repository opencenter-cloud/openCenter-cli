# PathResolver Migration Guide

## Table of Contents

- [Overview](#overview)
- [Why Migrate](#why-migrate)
- [Migration Strategy](#migration-strategy)
- [Before You Start](#before-you-start)
- [Step-by-Step Migration](#step-by-step-migration)
- [Common Patterns](#common-patterns)
- [Testing Your Migration](#testing-your-migration)
- [Rollback Plan](#rollback-plan)
- [Performance Considerations](#performance-considerations)
- [Troubleshooting](#troubleshooting)
- [FAQ](#faq)

## Overview

This guide helps you migrate from direct path construction to the centralized PathResolver system. The migration eliminates duplicate path logic, improves maintainability, and provides caching and validation.

**Target audience**: Developers working on opencenter-cli codebase

**Estimated time**: 2-4 hours per package

**Difficulty**: Medium

## Why Migrate

### Current Problems

- **40+ duplicate path construction calls** across the codebase
- **Inconsistent path formats** between different modules
- **No caching** - paths reconstructed on every access
- **No validation** - unsafe paths can be constructed
- **Hard to maintain** - changes require updates in multiple places
- **Error-prone** - easy to make mistakes in path construction

### Benefits After Migration

- **Single source of truth** for all path resolution
- **97% reduction** in path construction calls (40+ → 1)
- **<1ms resolution time** with caching (<100μs cached)
- **Built-in validation** prevents unsafe paths
- **Thread-safe** operations
- **Easy to test** - no filesystem mocking needed
- **Organization-aware** - supports multi-tenant structure

## Migration Strategy

### Phased Approach

1. **Phase 1**: Migrate cmd/ package (CLI commands)
2. **Phase 2**: Migrate internal/gitops/ package
3. **Phase 3**: Migrate internal/operations/ package
4. **Phase 4**: Migrate remaining packages
5. **Phase 5**: Remove deprecated functions

### Backward Compatibility

- Old path functions remain available during migration
- Marked as deprecated with warnings
- Removed after 2 releases (6 months)
- Feature flags for gradual rollout

## Before You Start

### Prerequisites

1. Read the package documentation: `internal/core/paths/doc.go`
2. Review usage examples: `internal/core/paths/example_test.go`
3. Understand your current path construction patterns
4. Identify all path construction calls in your module

### Identify Path Construction Calls

Search for common patterns:

```bash
# Find filepath.Join calls with "clusters"
grep -r 'filepath.Join.*clusters' cmd/ internal/

# Find direct path construction
grep -r '\.config/opencenter/clusters' cmd/ internal/

# Find config path construction
grep -r '\-config\.yaml' cmd/ internal/
```

## Step-by-Step Migration

### Step 1: Add PathResolver Dependency

Import the paths package:

```go
import (
    "github.com/rackerlabs/opencenter-cli/internal/core/paths"
)
```

### Step 2: Create Resolver Instance

**Option A: Singleton Pattern (Recommended)**

Create a single resolver instance for your application:

```go
// In your main package or initialization code
var (
    pathResolver     *paths.PathResolver
    pathResolverOnce sync.Once
)

func GetPathResolver() *paths.PathResolver {
    pathResolverOnce.Do(func() {
        baseDir := filepath.Join(os.Getenv("HOME"), ".config", "opencenter", "clusters")
        pathResolver = paths.NewPathResolver(baseDir)
    })
    return pathResolver
}
```

**Option B: Dependency Injection**

Pass resolver as a dependency:

```go
type ClusterService struct {
    resolver *paths.PathResolver
}

func NewClusterService(resolver *paths.PathResolver) *ClusterService {
    return &ClusterService{
        resolver: resolver,
    }
}
```

### Step 3: Replace Path Construction

**Before (Old Pattern)**:

```go
// Direct path construction (error-prone, no caching)
configPath := filepath.Join(
    baseDir,
    organization,
    "infrastructure",
    "clusters",
    clusterName,
    "."+clusterName+"-config.yaml",
)

secretsDir := filepath.Join(
    baseDir,
    organization,
    "secrets",
)

sopsKeyPath := filepath.Join(
    baseDir,
    organization,
    "secrets",
    "age",
    "keys",
    clusterName+"-key.txt",
)
```

**After (New Pattern)**:

```go
// Centralized resolution (cached, validated)
resolver := GetPathResolver()
clusterPaths, err := resolver.Resolve(ctx, clusterName, organization)
if err != nil {
    return fmt.Errorf("failed to resolve paths: %w", err)
}

configPath := clusterPaths.ConfigPath
secretsDir := clusterPaths.SecretsDir
sopsKeyPath := clusterPaths.SOPSKeyPath
```

### Step 4: Handle Fallback Cases

When organization is unknown:

**Before**:

```go
// Search manually across organizations
entries, _ := os.ReadDir(baseDir)
for _, entry := range entries {
    configPath := filepath.Join(baseDir, entry.Name(), "infrastructure", "clusters", clusterName, "."+clusterName+"-config.yaml")
    if _, err := os.Stat(configPath); err == nil {
        // Found it
        break
    }
}
```

**After**:

```go
// Automatic fallback search
clusterPaths, err := resolver.ResolveWithFallback(ctx, clusterName)
if err != nil {
    return fmt.Errorf("cluster not found: %w", err)
}
```

### Step 5: Update Tests

**Before**:

```go
func TestClusterInit(t *testing.T) {
    // Manual path construction in tests
    baseDir := t.TempDir()
    configPath := filepath.Join(baseDir, "org", "infrastructure", "clusters", "test", ".test-config.yaml")
    // ...
}
```

**After**:

```go
func TestClusterInit(t *testing.T) {
    // Use resolver in tests
    baseDir := t.TempDir()
    resolver := paths.NewPathResolver(baseDir)
    
    // Create directories
    err := resolver.CreateClusterDirectories(context.Background(), "test", "org")
    require.NoError(t, err)
    
    // Get paths
    clusterPaths, err := resolver.Resolve(context.Background(), "test", "org")
    require.NoError(t, err)
    
    configPath := clusterPaths.ConfigPath
    // ...
}
```

## Common Patterns

### Pattern 1: Config File Path

**Before**:

```go
configPath := filepath.Join(baseDir, org, "infrastructure", "clusters", cluster, "."+cluster+"-config.yaml")
```

**After**:

```go
clusterPaths, err := resolver.Resolve(ctx, cluster, org)
configPath := clusterPaths.ConfigPath
```

### Pattern 2: Secrets Directory

**Before**:

```go
secretsDir := filepath.Join(baseDir, org, "secrets")
ageKeyDir := filepath.Join(secretsDir, "age", "keys")
sshKeyDir := filepath.Join(secretsDir, "ssh")
```

**After**:

```go
clusterPaths, err := resolver.Resolve(ctx, cluster, org)
secretsDir := clusterPaths.SecretsDir
sopsKeyPath := clusterPaths.SOPSKeyPath  // Full path to age key
sshKeyPath := clusterPaths.SSHKeyPath    // Full path to SSH key
```

### Pattern 3: GitOps Directory

**Before**:

```go
gitopsDir := filepath.Join(baseDir, org)
infraDir := filepath.Join(gitopsDir, "infrastructure", "clusters", cluster)
appsDir := filepath.Join(gitopsDir, "applications", "overlays", cluster)
```

**After**:

```go
clusterPaths, err := resolver.Resolve(ctx, cluster, org)
gitopsDir := clusterPaths.GitOpsDir
infraDir := clusterPaths.ClusterDir
appsDir := clusterPaths.ApplicationsDir
```

### Pattern 4: Directory Creation

**Before**:

```go
dirs := []string{
    filepath.Join(baseDir, org),
    filepath.Join(baseDir, org, "infrastructure"),
    filepath.Join(baseDir, org, "infrastructure", "clusters"),
    filepath.Join(baseDir, org, "infrastructure", "clusters", cluster),
    filepath.Join(baseDir, org, "secrets"),
    filepath.Join(baseDir, org, "secrets", "age"),
    // ... many more
}
for _, dir := range dirs {
    if err := os.MkdirAll(dir, 0755); err != nil {
        return err
    }
}
```

**After**:

```go
err := resolver.CreateClusterDirectories(ctx, cluster, org)
if err != nil {
    return fmt.Errorf("failed to create directories: %w", err)
}
```

### Pattern 5: Organization Detection

**Before**:

```go
var foundOrg string
entries, _ := os.ReadDir(baseDir)
for _, entry := range entries {
    clusterDir := filepath.Join(baseDir, entry.Name(), "infrastructure", "clusters", cluster)
    if _, err := os.Stat(clusterDir); err == nil {
        foundOrg = entry.Name()
        break
    }
}
```

**After**:

```go
org, err := resolver.GetOrganization(ctx, cluster)
if err != nil {
    return fmt.Errorf("failed to detect organization: %w", err)
}
```

### Pattern 6: Conditional Path Construction

**Before**:

```go
var configPath string
if org != "" {
    configPath = filepath.Join(baseDir, org, "infrastructure", "clusters", cluster, "."+cluster+"-config.yaml")
} else {
    configPath = filepath.Join(baseDir, cluster, "."+cluster+"-config.yaml")
}
```

**After**:

```go
// Resolver handles organization logic automatically
clusterPaths, err := resolver.Resolve(ctx, cluster, org)
if err != nil {
    // Try fallback if organization not specified
    clusterPaths, err = resolver.ResolveWithFallback(ctx, cluster)
    if err != nil {
        return fmt.Errorf("failed to resolve paths: %w", err)
    }
}
configPath := clusterPaths.ConfigPath
```

## Testing Your Migration

### Unit Tests

Test path resolution in isolation:

```go
func TestPathResolution(t *testing.T) {
    baseDir := t.TempDir()
    resolver := paths.NewPathResolver(baseDir)
    
    // Create directories
    err := resolver.CreateClusterDirectories(context.Background(), "test", "org")
    require.NoError(t, err)
    
    // Resolve paths
    clusterPaths, err := resolver.Resolve(context.Background(), "test", "org")
    require.NoError(t, err)
    
    // Verify paths
    assert.Contains(t, clusterPaths.ConfigPath, "test-config.yaml")
    assert.Contains(t, clusterPaths.SecretsDir, "secrets")
}
```

### Integration Tests

Test with real filesystem:

```go
func TestClusterInitWithResolver(t *testing.T) {
    // Use real config directory
    baseDir := filepath.Join(os.Getenv("HOME"), ".config", "opencenter", "clusters-test")
    defer os.RemoveAll(baseDir)
    
    resolver := paths.NewPathResolver(baseDir)
    
    // Test full workflow
    err := resolver.CreateClusterDirectories(context.Background(), "test", "org")
    require.NoError(t, err)
    
    clusterPaths, err := resolver.Resolve(context.Background(), "test", "org")
    require.NoError(t, err)
    
    // Verify directories exist
    _, err = os.Stat(clusterPaths.ClusterDir)
    assert.NoError(t, err)
}
```

### Benchmark Tests

Verify performance improvements:

```go
func BenchmarkPathResolution(b *testing.B) {
    baseDir := b.TempDir()
    resolver := paths.NewPathResolver(baseDir)
    resolver.CreateClusterDirectories(context.Background(), "test", "org")
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := resolver.Resolve(context.Background(), "test", "org")
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

### Validation Checklist

- [ ] All path construction calls replaced
- [ ] Tests pass with new resolver
- [ ] No direct filepath.Join with "clusters"
- [ ] Error handling updated
- [ ] Cache invalidation added where needed
- [ ] Performance benchmarks show improvement
- [ ] Documentation updated

## Rollback Plan

### If Issues Occur

1. **Revert to old functions**: Old path construction functions remain available
2. **Feature flag**: Disable resolver with environment variable
3. **Gradual rollback**: Revert one package at a time
4. **Monitor metrics**: Watch for performance regressions

### Rollback Steps

```go
// Add feature flag check
if os.Getenv("USE_LEGACY_PATHS") == "true" {
    // Use old path construction
    configPath = filepath.Join(baseDir, org, "infrastructure", "clusters", cluster, "."+cluster+"-config.yaml")
} else {
    // Use new resolver
    clusterPaths, err := resolver.Resolve(ctx, cluster, org)
    configPath = clusterPaths.ConfigPath
}
```

## Performance Considerations

### Expected Improvements

- **First resolution**: <1ms (uncached)
- **Cached resolution**: <100μs (10x faster)
- **Memory overhead**: ~1KB per cached cluster
- **Cache hit rate**: >90% in typical usage

### Optimization Tips

1. **Enable caching**: Default is enabled, keep it on
2. **Reuse resolver**: Create one instance, use everywhere
3. **Batch operations**: Resolve multiple clusters together
4. **Invalidate wisely**: Only invalidate when directories change
5. **Monitor cache**: Check hit rate with `GetCacheStats()`

### Performance Monitoring

```go
// Log cache statistics periodically
stats := resolver.GetCacheStats()
log.Printf("Path cache: %d entries, %.2f%% hit rate", stats.Entries, stats.HitRate*100)
```

## Troubleshooting

### Issue: "cluster not found in organization"

**Cause**: Cluster directory doesn't exist or organization is wrong

**Solution**:

```go
// Try fallback resolution
clusterPaths, err := resolver.ResolveWithFallback(ctx, cluster)
if err != nil {
    // Cluster truly doesn't exist
    return fmt.Errorf("cluster %s not found: %w", cluster, err)
}
```

### Issue: "invalid cluster name"

**Cause**: Cluster name contains invalid characters

**Solution**:

```go
// Validate cluster name before resolution
if err := validateClusterName(cluster); err != nil {
    return fmt.Errorf("invalid cluster name: %w", err)
}
```

### Issue: "permission denied"

**Cause**: Insufficient permissions on cluster directory

**Solution**:

```go
// Enable path validation to catch permission issues early
options := paths.ResolutionOptions{
    ValidatePaths: true,
}
resolver := paths.NewPathResolverWithOptions(baseDir, options)
```

### Issue: Cache not working

**Cause**: Cache disabled or invalidated too frequently

**Solution**:

```go
// Check cache statistics
stats := resolver.GetCacheStats()
if stats.HitRate < 0.5 {
    log.Printf("Low cache hit rate: %.2f%%", stats.HitRate*100)
    // Investigate cache invalidation calls
}
```

### Issue: Performance regression

**Cause**: Path validation enabled or cache disabled

**Solution**:

```go
// Disable expensive validation in production
options := paths.ResolutionOptions{
    CacheResults:  true,   // Keep caching enabled
    ValidatePaths: false,  // Disable validation
}
resolver := paths.NewPathResolverWithOptions(baseDir, options)
```

## FAQ

### Q: Do I need to migrate all at once?

**A**: No. Migrate one package at a time. Old and new code can coexist.

### Q: What if I need a custom path not in ClusterPaths?

**A**: Use the resolved paths as a base:

```go
clusterPaths, _ := resolver.Resolve(ctx, cluster, org)
customPath := filepath.Join(clusterPaths.ClusterDir, "custom", "path")
```

### Q: How do I handle errors from Resolve()?

**A**: Always check errors and provide context:

```go
clusterPaths, err := resolver.Resolve(ctx, cluster, org)
if err != nil {
    return fmt.Errorf("failed to resolve paths for cluster %s: %w", cluster, err)
}
```

### Q: Should I use Resolve() or ResolveWithFallback()?

**A**: Use `Resolve()` when you know the organization. Use `ResolveWithFallback()` when organization is unknown or user-provided.

### Q: How do I test code that uses PathResolver?

**A**: Create a resolver with a temporary directory:

```go
func TestMyFunction(t *testing.T) {
    baseDir := t.TempDir()
    resolver := paths.NewPathResolver(baseDir)
    // Use resolver in tests
}
```

### Q: What about backward compatibility?

**A**: Old path functions remain available for 2 releases (6 months) with deprecation warnings.

### Q: How do I migrate tests?

**A**: Replace manual path construction with resolver calls. See "Testing Your Migration" section.

### Q: What if performance is worse after migration?

**A**: Check that caching is enabled and path validation is disabled. See "Performance Considerations" section.

### Q: Can I use PathResolver in concurrent code?

**A**: Yes. All resolver operations are thread-safe.

### Q: How do I invalidate the cache?

**A**: Call `InvalidateCache(cluster)` after creating or deleting cluster directories.

## Next Steps

1. **Start small**: Migrate one file or function
2. **Test thoroughly**: Run unit and integration tests
3. **Monitor performance**: Check cache hit rates
4. **Iterate**: Migrate more code gradually
5. **Document**: Update package documentation
6. **Review**: Get code review from team

## Additional Resources

- Package documentation: `internal/core/paths/doc.go`
- Usage examples: `internal/core/paths/example_test.go`
- Design document: `.kiro/specs/architectural-refactoring/design.md`
- Requirements: `.kiro/specs/architectural-refactoring/requirements.md`
- Path resolver spec: `.kiro/specs/architectural-refactoring/01-path-resolver.md`

## Support

For questions or issues:

1. Check this migration guide
2. Review package documentation
3. Check existing tests for examples
4. Ask in team chat or create an issue
