# `openCenter` - Developer Documentation

## Overview

The openCenter CLI is built using the Cobra command framework and follows a modular architecture with clear separation between command handling, business logic, and infrastructure concerns.

## Architecture

### Command Structure

```
cmd/
├── root.go              # Root command and global configuration
├── cluster.go           # Cluster command group
├── cluster_*.go         # Cluster subcommands
├── config.go            # Config command group
├── config_*.go          # Config subcommands
├── sops.go              # SOPS command group
├── plugins.go           # Plugins command group
└── version.go           # Version command
```

### Core Components

#### Command Layer (`cmd/`)
- Cobra command definitions
- Flag parsing and validation
- User interaction and output formatting
- Command orchestration

#### Business Logic (`internal/`)
- `config/` - Configuration management
- `gitops/` - GitOps repository operations
- `sops/` - SOPS encryption management
- `cloud/` - Cloud provider integrations
- `plugins/` - Plugin system
- `util/` - Shared utilities

## Root Command Implementation

### File Location
`cmd/root.go`

### Key Components

#### Global Flags Structure

```go
type GlobalFlags struct {
    Config   string   // --config: alternative cluster configuration file path
    DryRun   bool     // --dry-run: enable dry-run mode
    LogLevel string   // --log-level: set log level explicitly
    Set      []string // --set: override configuration values
    Verbose  bool     // --verbose: enable verbose logging
}
```


#### Configuration Manager

The global configuration manager is initialized in `PersistentPreRunE`:

```go
func initializeGlobalConfig(cmd *cobra.Command) error {
    // 1. Handle legacy config-dir flag
    // 2. Parse global flags
    // 3. Initialize configuration manager
    // 4. Apply global flag overrides
    // 5. Log configuration details
}
```

#### Command Registration

Commands are registered in the `Execute` function:

```go
func Execute(version string) error {
    rootCmd.Version = version
    addGlobalFlags(rootCmd)
    
    // Register subcommands
    rootCmd.AddCommand(newClusterCmd())
    rootCmd.AddCommand(newConfigCmd())
    rootCmd.AddCommand(newSOPSCmd())
    rootCmd.AddCommand(newPluginsCmd())
    rootCmd.AddCommand(newVersionCmd())
    
    // Discover and attach external plugins
    plugins.LoadExternalPlugins(rootCmd)
    
    return rootCmd.Execute()
}
```

## Global Flag Processing

### Flag Parsing

Global flags are parsed before command execution:

```go
func parseGlobalFlags(cmd *cobra.Command) (*GlobalFlags, error) {
    config, _ := cmd.Flags().GetString("config")
    dryRun, _ := cmd.Flags().GetBool("dry-run")
    logLevel, _ := cmd.Flags().GetString("log-level")
    set, _ := cmd.Flags().GetStringArray("set")
    verbose, _ := cmd.Flags().GetBool("verbose")
    
    // Verbose overrides log level
    if verbose {
        logLevel = "debug"
    }
    
    return &GlobalFlags{...}, nil
}
```

### Configuration Overrides

The `--set` flag allows runtime configuration overrides:

```go
func applySetFlagOverrides(cliConfig *config.CLIConfig, setFlags []string) error {
    for _, setFlag := range setFlags {
        parts := strings.SplitN(setFlag, "=", 2)
        key := parts[0]
        value := parts[1]
        
        // Parse value (detect type)
        parsedValue := convertValue(value)
        
        // Apply using dot notation
        if err := tempManager.SetValue(key, parsedValue); err != nil {
            return err
        }
    }
}
```

## Command Groups

### Cluster Commands

Comprehensive cluster lifecycle management:
- Initialization and configuration
- Validation and preflight checks
- GitOps repository setup
- Bootstrap and deployment
- Migration and updates

See [cluster/readme.md](cluster/readme.md) for details.

### Config Commands

CLI configuration management:
- View current configuration
- Set/get configuration values
- Reset to defaults
- Show configuration path
- IDE integration setup

### SOPS Commands

Secrets management with SOPS:
- Age key generation and rotation
- Key backup and validation
- Secrets encryption/decryption
- Batch operations

### Plugins Commands

Plugin system management:
- List discovered plugins
- Plugin discovery from multiple sources
- Dynamic command registration

### Version Command

Build and version information:
- Semantic version
- Git commit and branch
- Build date and platform
- Short and full formats

## Configuration Management

### Configuration File

Location: `~/.config/openCenter/config.yaml`

Structure:
```yaml
logging:
  level: warn
  format: text
  output: stderr
paths:
  configDir: ~/.config/openCenter
  clustersDir: ~/.config/openCenter/clusters
behavior:
  autoConfirm: false
  dryRun: false
  verbose: false
defaults:
  provider: openstack
  region: {{ .OpenCenter.Cluster.ClusterRegion }}
  environment: dev
```

### Configuration Manager

The `ConfigManager` handles:
- Loading configuration from file
- Merging with defaults
- Environment variable expansion
- Runtime overrides
- Saving configuration

## Plugin System

### Plugin Discovery

Plugins are discovered in order:
1. `OPENCENTER_PLUGINS_DIR` environment variable
2. `<config-dir>/plugins` directory
3. System `PATH`

### Plugin Naming

Plugin binaries must follow the naming convention:
```
openCenter-<plugin-name>
```

Example: `openCenter-aws-helper`

### Plugin Registration

Plugins are dynamically registered as subcommands:

```go
func LoadExternalPlugins(rootCmd *cobra.Command) {
    discovered := Discover()
    for name, path := range discovered {
        use := strings.TrimPrefix(name, BinaryPrefix)
        cmd := createPluginCommand(use, path)
        rootCmd.AddCommand(cmd)
    }
}
```

## Error Handling

### Error Patterns

```go
// Return error from command
return fmt.Errorf("failed to load cluster: %w", err)

// Helper for formatted errors
func failf(format string, a ...interface{}) error {
    return fmt.Errorf(format, a...)
}
```

### Exit Codes

- `0` - Success
- `1` - Error occurred

## Logging

### Log Levels

- `debug` - Detailed debugging information
- `info` - Informational messages
- `warn` - Warning messages (default)
- `error` - Error messages

### Log Formats

- `text` - Human-readable text (default)
- `json` - JSON format for parsing
- `yaml` - YAML format

### Debug Mode

Enable with environment variable:
```bash
export OPENCENTER_DEBUG=1
```

## Testing

### Unit Tests

Test individual functions and components:

```go
func TestGlobalFlagParsing(t *testing.T) {
    // Test flag parsing
}

func TestConfigurationOverrides(t *testing.T) {
    // Test --set flag overrides
}
```

### Integration Tests

Test complete command workflows:

```bash
# Test cluster lifecycle
mise run godog -- features/cluster_lifecycle.feature

# Test configuration management
mise run godog -- features/config_management.feature
```

### BDD Tests

Behavior-driven tests using Godog:

```gherkin
Feature: Global Configuration
  Scenario: Override configuration with --set flag
    When I run "openCenter --set logging.level=debug cluster list"
    Then the log level should be "debug"
```

## Development Guidelines

### Adding New Commands

1. Create command file in `cmd/`
2. Implement command following Cobra patterns
3. Register in appropriate command group
4. Add tests
5. Update documentation

### Adding Global Flags

1. Add to `GlobalFlags` struct
2. Register in `addGlobalFlags()`
3. Parse in `parseGlobalFlags()`
4. Apply in `applyGlobalFlagOverrides()`
5. Document in reference docs

### Modifying Configuration Schema

1. Update `CLIConfig` struct in `internal/config/`
2. Update default values
3. Update validation
4. Update documentation
5. Test migration from old config

## Build System

### Using Mise

All build operations use Mise:

```bash
# Build
mise run build

# Test
mise run test

# Run BDD tests
mise run godog

# Format code
mise run fmt
```

### Build Variables

Set at compile time via ldflags:

```go
var (
    version   = "dev"
    gitCommit = "unknown"
    gitBranch = "unknown"
    gitTag    = ""
    buildDate = "unknown"
)
```

## Performance Considerations

- Configuration is loaded once at startup
- Plugin discovery is cached
- File operations are minimized
- Lazy loading where possible

## Security Considerations

- Configuration files: 0600 permissions
- Secrets never logged
- SOPS integration for encryption
- Input validation on all user input
- Path traversal prevention

## See Also

- [Cluster Commands](cluster/readme.md)
- [Configuration Schema](../reference/configuration.md)
- [Plugin Development](../how-to/plugin-development.md)
- [Testing Guide](../how-to/testing.md)
