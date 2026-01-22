# Cluster Name Resolution Pattern


## Table of Contents

- [Overview](#overview)
- [Supported Formats](#supported-formats)
- [Implementation](#implementation)
- [Validation](#validation)
- [Updated Commands](#updated-commands)
- [Usage Examples](#usage-examples)
- [Error Messages](#error-messages)
- [Benefits](#benefits)
- [Testing](#testing)
- [Migration Notes](#migration-notes)
- [Related Files](#related-files)
## Overview

All `openCenter cluster` subcommands now consistently support two methods for specifying the target cluster:

1. **Explicit cluster name** as a positional argument or flag (supports `organization/cluster` format)
2. **Active cluster** fallback when no explicit name is provided

## Supported Formats

Cluster identifiers can be specified in two formats:

- `cluster` - Just the cluster name (uses default "opencenter" organization)
- `organization/cluster` - Full organization and cluster name

## Implementation

### Helper Functions

Two helper functions in `cmd/cluster.go` provide consistent cluster name resolution:

#### `resolveClusterName(args []string, requireActive bool) (string, error)`

Used by commands that accept cluster name as a **positional argument**.

**Parameters:**
- `args`: Command arguments (first arg should be cluster name if provided)
- `requireActive`: If true and no args provided, returns error if no active cluster

**Returns:**
- `clusterName`: The resolved cluster name (may include organization prefix)
- `error`: An error if resolution fails

**Example usage:**
```go
RunE: func(cmd *cobra.Command, args []string) error {
    name, err := resolveClusterName(args, true)
    if err != nil {
        return err
    }
    // Use name...
}
```

#### `resolveClusterNameFromFlag(flagValue string, requireActive bool) (string, error)`

Used by commands that accept cluster name via a **--cluster flag**.

**Parameters:**
- `flagValue`: The value from the --cluster flag (empty string if not provided)
- `requireActive`: If true and no flag provided, returns error if no active cluster

**Returns:**
- `clusterName`: The resolved cluster name (may include organization prefix)
- `error`: An error if resolution fails

**Example usage:**
```go
var cluster string
cmd.Flags().StringVar(&cluster, "cluster", "", "Specify the cluster name")

RunE: func(cmd *cobra.Command, args []string) error {
    clusterName, err := resolveClusterNameFromFlag(cluster, true)
    if err != nil {
        return err
    }
    // Use clusterName...
}
```

## Validation

Both helper functions perform comprehensive validation:

1. **Format validation**: Ensures identifier is either `cluster` or `organization/cluster`
2. **Component validation**: Validates each part (organization and cluster name) separately
3. **Security validation**: Uses `security.InputValidator` to prevent path traversal attacks
4. **Empty check**: Ensures cluster name is not empty or whitespace-only

## Updated Commands

The following commands have been updated to use the new helpers:

### Positional Argument Commands
- `cluster validate [name]`
- `cluster preflight [name]`
- `cluster setup [name]`
- `cluster render [name]`
- `cluster info [name]`
- `cluster bootstrap [name]`
- `cluster update [name]`
- `cluster destroy [name]`
- `cluster lock [name]`
- `cluster unlock [name]`

### Flag-Based Commands
- `cluster service enable <service> --cluster <name>`
- `cluster service disable <service> --cluster <name>`
- `cluster service status --cluster <name>`

## Usage Examples

### Using Explicit Cluster Name

```bash
# Simple cluster name (uses default "opencenter" organization)
openCenter cluster validate my-cluster
openCenter cluster bootstrap my-cluster

# Organization-scoped cluster name
openCenter cluster validate myorg/my-cluster
openCenter cluster bootstrap myorg/my-cluster
```

### Using Active Cluster

```bash
# Set active cluster
openCenter cluster select my-cluster

# Commands use active cluster when no name provided
openCenter cluster validate
openCenter cluster bootstrap
openCenter cluster info
```

### Using --cluster Flag

```bash
# Service commands use --cluster flag
openCenter cluster service enable loki --cluster my-cluster
openCenter cluster service status --cluster myorg/my-cluster

# Or use active cluster
openCenter cluster select my-cluster
openCenter cluster service enable loki
```

## Error Messages

Consistent error messages across all commands:

- **No cluster specified and no active cluster:**
  ```
  no active cluster set. Use 'openCenter cluster select <cluster>' or provide cluster name as argument
  ```

- **Invalid format:**
  ```
  invalid cluster identifier format: use 'cluster' or 'organization/cluster'
  ```

- **Invalid characters:**
  ```
  invalid cluster identifier: cluster name cannot contain path separators (/ or \) for directory structure
  ```

## Benefits

1. **Consistency**: All cluster commands follow the same pattern
2. **Flexibility**: Users can choose explicit names or active cluster workflow
3. **Organization Support**: Full support for multi-organization deployments
4. **Security**: Comprehensive validation prevents path traversal attacks
5. **User Experience**: Clear error messages guide users to correct usage

## Testing

Test both resolution methods for each command:

```bash
# Build
mise run build

# Test with explicit name
./bin/openCenter cluster validate test-cluster

# Test with organization prefix
./bin/openCenter cluster validate myorg/test-cluster

# Test with active cluster
./bin/openCenter cluster select test-cluster
./bin/openCenter cluster validate

# Test error cases
./bin/openCenter cluster validate  # Should error if no active cluster
./bin/openCenter cluster validate invalid//format  # Should error on invalid format
```

## Migration Notes

For developers adding new cluster subcommands:

1. **Choose the appropriate helper** based on your command's argument style:
   - Positional argument → `resolveClusterName(args, true)`
   - Flag-based → `resolveClusterNameFromFlag(flagValue, true)`

2. **Remove manual resolution logic** - don't reimplement cluster name resolution

3. **Update command documentation** to mention both usage patterns

4. **Add examples** showing both explicit and active cluster usage

## Related Files

- `cmd/cluster.go` - Helper function implementations
- `internal/config/config.go` - `ParseClusterIdentifier()` function
- `internal/security/input_validator.go` - Cluster name validation
- `internal/config/path_resolver.go` - Organization-based path resolution
