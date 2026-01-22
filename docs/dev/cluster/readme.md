# `openCenter cluster` - Developer Documentation


## Table of Contents

- [Overview](#overview)
- [Architecture](#architecture)
- [Organization-Based Structure](#organization-based-structure)
- [Command Implementation Patterns](#command-implementation-patterns)
- [Key Implementation Details](#key-implementation-details)
- [Testing Strategy](#testing-strategy)
- [Development Guidelines](#development-guidelines)
- [Debugging](#debugging)
- [Performance Considerations](#performance-considerations)
- [Security Considerations](#security-considerations)
- [See Also](#see-also)
## Overview

The `cluster` command group provides comprehensive lifecycle management for Kubernetes cluster configurations. This document provides developer-focused information about the implementation, architecture, and internal workings of the cluster commands.

## Architecture

### Command Structure

The cluster commands follow a hierarchical structure using Cobra:

```
cmd/
├── cluster.go                  # Main cluster command
├── cluster_init.go             # Initialize cluster
├── cluster_list.go             # List clusters
├── cluster_select.go           # Select active cluster
├── cluster_current.go          # Show current cluster
├── cluster_info.go             # Show cluster info
├── cluster_edit.go             # Edit configuration
├── cluster_validate.go         # Validate configuration
├── cluster_preflight.go        # Preflight checks
├── cluster_setup.go            # Setup GitOps
├── cluster_render.go           # Render templates
├── cluster_bootstrap.go        # Bootstrap cluster
├── cluster_schema.go           # Export schema
├── cluster_destroy.go          # Destroy cluster
├── cluster_update.go           # Update configuration
├── cluster_migrate.go          # Migrate to organization structure
└── cluster_config_update.go    # Update with defaults
```

### Core Components

#### Configuration Management (`internal/config/`)
- `config.go` - Configuration struct definitions
- `path_resolver.go` - Organization-based path resolution
- `migration.go` - Legacy to organization migration
- `schema.go` - JSON schema generation
- `validation.go` - Configuration validation

#### GitOps Integration (`internal/gitops/`)
- `gitops.go` - GitOps repository management
- `templates.go` - Template rendering
- `base.go` - Base structure copying

#### SOPS Integration (`internal/sops/`)
- `key_manager.go` - Age key management
- `config.go` - SOPS configuration generation

#### Cloud Providers (`internal/cloud/`)
- `openstack/` - OpenStack provider
- `aws/` - AWS provider (future)
- `gcp/` - GCP provider (future)
- `azure/` - Azure provider (future)

## Organization-Based Structure

### Directory Layout

```
~/.config/openCenter/
└── clusters/
    └── <organization>/
        ├── .sops.yaml                    # Organization SOPS config
        ├── .<cluster>-config.yaml        # Cluster configuration
        ├── secrets/
        │   ├── age/keys/                 # SOPS keys per cluster
        │   └── ssh/                      # SSH keys per cluster
        └── gitops/                       # Shared GitOps repository
            ├── applications/
            │   └── overlays/<cluster>/   # Cluster-specific apps
            └── infrastructure/
                └── clusters/<cluster>/   # Cluster-specific infra
```

### Path Resolution

The `PathResolver` handles organization-based path resolution:

```go
type PathResolver struct {
    configManager *ConfigManager
}

type ClusterPaths struct {
    OrganizationDir string
    ClusterDir      string
    GitOpsDir       string
    SecretsDir      string
    SOPSKeyPath     string
    SOPSConfigPath  string
    // ... more paths
}
```

## Command Implementation Patterns

### Standard Command Pattern

```go
func newClusterXxxCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "xxx [name]",
        Short: "Brief description",
        Args:  cobra.MaximumNArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1. Resolve cluster name
            var name string
            if len(args) > 0 {
                name = args[0]
            } else {
                name, err = config.GetActive()
                // handle error
            }
            
            // 2. Load configuration
            cfg, err := config.Load(name)
            // handle error
            
            // 3. Perform operation
            // ...
            
            // 4. Return result
            return nil
        },
    }
    
    // Add flags
    cmd.Flags().String("flag", "default", "description")
    
    return cmd
}
```

### Dynamic Flag Parsing

For commands like `init` and `update` that support dynamic flags:

```go
FParseErrWhitelist: cobra.FParseErrWhitelist{
    UnknownFlags: true,
},
```

Then parse `os.Args` manually to capture unknown flags.

## Key Implementation Details

### Cluster Initialization

1. Generate schema-based defaults
2. Apply flag overrides using reflection
3. Determine organization (from flag, config, or cluster name)
4. Create organization structure
5. Generate SOPS and SSH keys
6. Write configuration file

### GitOps Setup

1. Create organization directory structure
2. Copy base GitOps templates
3. Render cluster-specific templates
4. Generate OpenTofu configuration
5. Setup SOPS encryption
6. Initialize git repository
7. Commit initial structure

### Configuration Validation

1. Schema validation (JSON schema)
2. Required field validation
3. Cross-field dependency validation
4. Provider-specific validation
5. Network configuration validation
6. SOPS key validation

### Migration Process

1. Detect legacy clusters
2. Create backup (tar.gz)
3. Create organization structure
4. Move configuration files
5. Move secrets (SOPS, SSH)
6. Move GitOps repository
7. Update configuration metadata
8. Update SOPS configuration
9. Validate migration

## Testing Strategy

### Unit Tests

Test individual functions and components:

```go
func TestClusterInit(t *testing.T) {
    // Test cluster initialization
}

func TestPathResolver(t *testing.T) {
    // Test path resolution
}
```

### BDD Tests

Behavior-driven tests using Godog:

```gherkin
Feature: Cluster Initialization
  Scenario: Initialize cluster with defaults
    When I run "openCenter cluster init test-cluster"
    Then the cluster configuration should exist
    And SOPS keys should be generated
    And SSH keys should be generated
```

### Integration Tests

Test complete workflows:

```bash
# Test complete cluster lifecycle
mise run godog -- features/cluster_lifecycle.feature
```

## Development Guidelines

### Adding New Commands

1. Create new file `cmd/cluster_<name>.go`
2. Implement command following standard pattern
3. Add command to `newClusterCmd()` in `cluster.go`
4. Add tests in `cmd/cluster_<name>_test.go`
5. Add BDD tests in `tests/features/cluster_<name>.feature`
6. Update documentation

### Modifying Configuration Schema

1. Update structs in `internal/config/config.go`
2. Add validation rules in `internal/config/validation.go`
3. Update schema generation in `internal/config/schema.go`
4. Run `mise run schema` to regenerate schema
5. Update tests and documentation

### Adding Provider Support

1. Create provider package in `internal/cloud/<provider>/`
2. Implement provider interface
3. Add provider-specific validation
4. Add provider-specific templates
5. Update bootstrap command
6. Add tests and documentation

## Debugging

### Enable Debug Mode

```bash
export OPENCENTER_DEBUG=1
openCenter cluster <command>
```

### Debug Output Locations

- Debug config: `<git_dir>/.openCenter.yaml`
- Bootstrap log: `<git_dir>/infrastructure/clusters/<cluster>/bootstrap.log`
- SOPS debug: Check `.sops.yaml` configuration

### Common Issues

#### Path Resolution
Check path resolver output:
```go
config.Debugf("Resolved paths: %+v", paths)
```

#### Template Rendering
Check template variables:
```go
config.Debugf("Template data: %+v", templateData)
```

#### SOPS Configuration
Verify SOPS config:
```bash
cat ~/.config/openCenter/clusters/org/.sops.yaml
```

## Performance Considerations

### File Operations
- Use buffered I/O for large files
- Minimize filesystem operations
- Cache path resolutions

### Template Rendering
- Compile templates once
- Use template caching
- Minimize template complexity

### Configuration Loading
- Lazy load configuration
- Cache loaded configurations
- Use efficient YAML parsing

## Security Considerations

### File Permissions
- Config files: 0600
- Directories: 0755
- SOPS keys: 0600
- SSH keys: 0600

### Secrets Management
- Never log secrets
- Use SOPS for encryption
- Secure key storage
- Validate key permissions

### Input Validation
- Validate cluster names
- Sanitize file paths
- Validate configuration values
- Check for path traversal

## See Also

- [Configuration Schema](../../reference/configuration.md)
- [GitOps Integration](../../explanation/gitops.md)
- [SOPS Integration](../../explanation/sops.md)
- [Testing Guide](../../how-to/testing.md)
