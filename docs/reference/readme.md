# Reference Documentation

**doc_type: reference**

Complete technical specifications for openCenter. Look up commands, configuration options, APIs, and error codes.

## Who This Is For

Reference documentation is for users who need to look up specific technical details. Use this when you know what you're looking for and need exact specifications.

## Core Reference

### [CLI Commands](cli-commands.md)
Complete command reference with all flags, options, and examples.

**Includes:**
- Global flags
- Cluster commands
- SOPS commands
- Config commands
- Plugin commands
- Exit codes

### [Configuration Schema](configuration.md)
Every configuration option with types, defaults, and validation rules.

**Includes:**
- Complete YAML schema
- Field descriptions
- Default values
- Validation rules
- Examples

### [Configuration Precedence](configuration-precedence.md)
How configuration values are resolved from multiple sources.

**Includes:**
- Precedence order
- CLI defaults behavior
- Override examples
- Best practices

### [Error Codes](error-codes.md)
All error codes with descriptions and solutions.

**Includes:**
- Error code reference
- Error categories
- Diagnostic steps
- Resolution procedures

## Command Reference

### [Cluster Commands](cluster/README.md)
Detailed reference for cluster lifecycle management.

**Commands:**
- [init](cluster/init.md) - Initialize cluster configuration
- [validate](cluster/validate.md) - Validate configuration
- [setup](cluster/setup.md) - Setup GitOps repository
- [bootstrap](cluster/bootstrap.md) - Bootstrap infrastructure
- [list](cluster/list.md) - List clusters
- [select](cluster/select.md) - Select active cluster
- [current](cluster/current.md) - Show current cluster
- [info](cluster/info.md) - Display cluster information
- [edit](cluster/edit.md) - Edit configuration
- [render](cluster/render.md) - Render templates
- [schema](cluster/schema.md) - Generate JSON schema
- [update](cluster/update.md) - Update configuration
- [migrate](cluster/migrate.md) - Migrate schema version
- [preflight](cluster/preflight.md) - Run preflight checks
- [destroy](cluster/destroy.md) - Destroy cluster

## API and Integration

### [API Reference](api.md)
Go package documentation for programmatic use.

**Includes:**
- Package interfaces
- Type definitions
- Function signatures
- Usage examples

### [Environment Variables](environment-variables.md)
All environment variables that affect openCenter behavior.

**Includes:**
- Variable names
- Descriptions
- Default values
- Examples

### [Shell Integration](shell-integration.md)
Shell completion and integration features.

**Includes:**
- Bash completion
- Zsh completion
- Fish completion
- PowerShell completion

## Data Formats

### [Secrets Reference](secrets.md)
SOPS encryption format and key management.

**Includes:**
- Age key format
- SOPS configuration
- Encryption patterns
- Key rotation

### [Templates Reference](templates.md)
Template system and available templates.

**Includes:**
- Template syntax
- Available variables
- Custom templates
- Template functions

### [File Formats](file-formats.md)
File format specifications for openCenter files.

**Includes:**
- Configuration files
- GitOps structure
- Manifest formats
- Schema files

## Additional Reference

### [Glossary](glossary.md)
Definitions of terms used throughout openCenter documentation.

**Includes:**
- Technical terms
- Acronyms
- Concepts
- Related terms

## Reference by Category

### Configuration
- [Configuration Schema](configuration.md)
- [Configuration Precedence](configuration-precedence.md)
- [Environment Variables](environment-variables.md)
- [File Formats](file-formats.md)

### Commands
- [CLI Commands](cli-commands.md)
- [Cluster Commands](cluster/README.md)

### Security
- [Secrets Reference](secrets.md)
- [Error Codes](error-codes.md)

### Integration
- [API Reference](api.md)
- [Shell Integration](shell-integration.md)
- [Templates Reference](templates.md)

### General
- [Glossary](glossary.md)

## Using Reference Documentation

### Finding Information

**By Command:**
1. Go to [CLI Commands](cli-commands.md)
2. Find command in table of contents
3. Read syntax and examples

**By Configuration:**
1. Go to [Configuration Schema](configuration.md)
2. Search for field name
3. Check type and validation rules

**By Error:**
1. Go to [Error Codes](error-codes.md)
2. Look up error code
3. Follow resolution steps

**By Concept:**
1. Go to [Glossary](glossary.md)
2. Find term definition
3. Follow links to detailed docs

### Reading Reference Docs

Reference documentation is organized for quick lookup:
- **Overview**: One-paragraph summary
- **Syntax**: Command or configuration syntax
- **Parameters**: All options with types
- **Examples**: Practical usage examples
- **Notes**: Important details
- **See Also**: Related references

## Related Documentation

- **[Tutorials](../tutorials/README.md)** - Learn by doing
- **[How-To Guides](../how-to/README.md)** - Solve specific problems
- **[Explanation](../explanation/README.md)** - Understand concepts

## Contributing

Found an error in the reference docs? See our [Contributing Guide](../../contributing.md) to submit corrections.

## Version Compatibility

This reference documentation is for openCenter 1.0.0. For other versions:
- Check the version tag in GitHub
- Review the changelog for differences
- Use `openCenter version` to check your installed version
