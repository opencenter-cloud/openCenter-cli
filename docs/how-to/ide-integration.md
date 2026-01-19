# IDE Integration Guide

This guide explains how to set up IDE integration for openCenter cluster configuration files with autocomplete, validation, and documentation support.

## Overview

openCenter provides comprehensive IDE integration through JSON Schema and YAML language server support. This enables:

- **Autocomplete**: Intelligent code completion for configuration keys and values
- **Validation**: Real-time validation against the cluster configuration schema
- **Documentation**: Inline documentation and hover information for all configuration options
- **Error Detection**: Immediate feedback on configuration errors and typos

## Supported IDEs

### Visual Studio Code

VS Code has the best support for openCenter configuration files through the YAML extension.

#### Setup

1. Install the [YAML extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml) by Red Hat:
   ```bash
   code --install-extension redhat.vscode-yaml
   ```

2. The workspace settings in `.vscode/settings.json` are pre-configured to:
   - Associate the cluster schema with configuration files
   - Enable YAML validation and completion
   - Configure formatting options
   - Support SOPS encrypted values

3. Generate the latest schema:
   ```bash
   openCenter cluster schema --out schema/cluster.schema.json
   ```

4. Open any cluster configuration file (e.g., `*-config.yaml`) and enjoy autocomplete!

#### Features

- **Autocomplete**: Press `Ctrl+Space` to see available configuration options
- **Validation**: Errors and warnings appear in real-time as you type
- **Hover Documentation**: Hover over any key to see its description and constraints
- **Go to Definition**: Jump to schema definitions with `F12`
- **Format Document**: Format YAML with `Shift+Alt+F`

### JetBrains IDEs (IntelliJ IDEA, PyCharm, WebStorm, etc.)

JetBrains IDEs have built-in JSON Schema support.

#### Setup

1. Open Settings/Preferences → Languages & Frameworks → Schemas and DTDs → JSON Schema Mappings

2. Add a new mapping:
   - **Name**: openCenter Cluster Configuration
   - **Schema file or URL**: `schema/cluster.schema.json` (relative to project root)
   - **Schema version**: JSON Schema version 7

3. Add file patterns:
   - `**/clusters/**/*.yaml`
   - `**/clusters/**/*-config.yaml`
   - `**/.opencenter.yaml`

4. Generate the latest schema:
   ```bash
   openCenter cluster schema --out schema/cluster.schema.json
   ```

#### Features

- **Autocomplete**: Press `Ctrl+Space` for code completion
- **Validation**: Real-time validation with error highlighting
- **Quick Documentation**: Press `Ctrl+Q` to view documentation
- **Reformat Code**: Press `Ctrl+Alt+L` to format YAML

### Vim/Neovim

Vim and Neovim can use the YAML language server for schema support.

#### Setup with coc.nvim

1. Install [coc.nvim](https://github.com/neoclide/coc.nvim)

2. Install the YAML language server:
   ```vim
   :CocInstall coc-yaml
   ```

3. Configure coc-settings.json:
   ```json
   {
     "yaml.schemas": {
       "./schema/cluster.schema.json": [
         "**/clusters/**/*.yaml",
         "**/clusters/**/*-config.yaml",
         "**/.opencenter.yaml"
       ]
     },
     "yaml.validate": true,
     "yaml.completion": true
   }
   ```

4. Generate the latest schema:
   ```bash
   openCenter cluster schema --out schema/cluster.schema.json
   ```

#### Setup with nvim-lspconfig

1. Install [nvim-lspconfig](https://github.com/neovim/nvim-lspconfig)

2. Install yaml-language-server:
   ```bash
   npm install -g yaml-language-server
   ```

3. Configure in your `init.lua`:
   ```lua
   require'lspconfig'.yamlls.setup{
     settings = {
       yaml = {
         schemas = {
           ["./schema/cluster.schema.json"] = {
             "**/clusters/**/*.yaml",
             "**/clusters/**/*-config.yaml",
             "**/.opencenter.yaml"
           }
         },
         validate = true,
         completion = true
       }
     }
   }
   ```

### Emacs

Emacs can use the YAML language server through lsp-mode.

#### Setup

1. Install [lsp-mode](https://github.com/emacs-lsp/lsp-mode)

2. Install yaml-language-server:
   ```bash
   npm install -g yaml-language-server
   ```

3. Configure in your init.el:
   ```elisp
   (use-package lsp-mode
     :hook (yaml-mode . lsp)
     :config
     (setq lsp-yaml-schemas
           '(:cluster "./schema/cluster.schema.json")))
   ```

4. Add file associations:
   ```elisp
   (add-to-list 'auto-mode-alist '("\\*-config\\.yaml\\'" . yaml-mode))
   (add-to-list 'auto-mode-alist '("\\.opencenter\\.yaml\\'" . yaml-mode))
   ```

## Schema Generation

The JSON schema is generated from the Go struct definitions and includes:

- **Type validation**: Ensures correct data types for all fields
- **Pattern validation**: Validates formats like CIDR blocks, UUIDs, and hostnames
- **Range validation**: Enforces minimum/maximum values for numeric fields
- **Enum validation**: Restricts values to predefined options
- **Required fields**: Identifies mandatory configuration options
- **Descriptions**: Provides inline documentation for all fields

### Generating the Schema

Generate the latest schema with:

```bash
# Generate to default location
openCenter cluster schema --out schema/cluster.schema.json

# Generate with pretty formatting (default)
openCenter cluster schema --out schema/cluster.schema.json --pretty

# View schema version
openCenter cluster schema --version
```

### Schema Versioning

The schema includes version information for backward compatibility tracking:

```bash
openCenter cluster schema --version
# Output: Schema version: 1.0.0
```

## YAML Linting

The project includes a `.yamllint` configuration file for consistent YAML formatting.

### Using yamllint

Install yamllint:

```bash
# macOS
brew install yamllint

# Linux
pip install yamllint

# Verify installation
yamllint --version
```

Lint your configuration files:

```bash
# Lint a specific file
yamllint ~/.config/openCenter/clusters/myorg/my-cluster/.my-cluster-config.yaml

# Lint all cluster configurations
yamllint ~/.config/openCenter/clusters/

# Auto-fix issues (if supported by your editor)
yamllint --format auto ~/.config/openCenter/clusters/
```

## Troubleshooting

### Schema Not Loading

**Problem**: IDE doesn't show autocomplete or validation

**Solutions**:
1. Verify the schema file exists: `ls -la schema/cluster.schema.json`
2. Regenerate the schema: `openCenter cluster schema --out schema/cluster.schema.json`
3. Restart your IDE or reload the window
4. Check IDE logs for schema loading errors

### Validation Errors

**Problem**: IDE shows validation errors for valid configuration

**Solutions**:
1. Ensure you're using the latest schema version
2. Check if the file matches the schema file patterns
3. Verify the schema version matches your openCenter version
4. Review the error message for specific validation failures

### Autocomplete Not Working

**Problem**: No autocomplete suggestions appear

**Solutions**:
1. Verify the YAML language server is installed and running
2. Check that the file extension is `.yaml` or `.yml`
3. Ensure the file path matches the schema file patterns
4. Try triggering autocomplete manually (Ctrl+Space in most IDEs)

### Performance Issues

**Problem**: IDE becomes slow when editing large configuration files

**Solutions**:
1. Disable validation for large files temporarily
2. Split large configurations into multiple files
3. Increase IDE memory allocation
4. Use a more lightweight editor for quick edits

## Best Practices

### Configuration Organization

1. **Use organization-based structure**: Store configurations in `~/.config/openCenter/clusters/<org>/<cluster>/`
2. **Version control**: Keep configurations in Git for history and collaboration
3. **Encrypt secrets**: Use SOPS for sensitive values
4. **Validate before commit**: Run `openCenter cluster validate` before committing changes

### Schema Updates

1. **Regenerate after updates**: Run schema generation after updating openCenter
2. **Commit schema changes**: Include schema updates in version control
3. **Document breaking changes**: Note schema version changes in commit messages
4. **Test configurations**: Validate existing configurations after schema updates

### IDE Configuration

1. **Enable format on save**: Automatically format YAML files on save
2. **Configure auto-indent**: Use 2-space indentation for YAML
3. **Enable validation**: Keep real-time validation enabled
4. **Use snippets**: Create custom snippets for common configuration patterns

## Additional Resources

- [JSON Schema Documentation](https://json-schema.org/)
- [YAML Language Server](https://github.com/redhat-developer/yaml-language-server)
- [VS Code YAML Extension](https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml)
- [openCenter Documentation](https://docs.opencenter.cloud)
- [openCenter GitHub Repository](https://github.com/rackerlabs/openCenter-cli)

## Support

For issues or questions about IDE integration:

1. Check the [troubleshooting section](#troubleshooting) above
2. Review the [openCenter documentation](https://docs.opencenter.cloud)
3. Open an issue on [GitHub](https://github.com/rackerlabs/openCenter-cli/issues)
4. Join the community discussions

## Contributing

To improve IDE integration:

1. Enhance the JSON schema with additional validation rules
2. Add support for more IDEs and editors
3. Create IDE-specific plugins or extensions
4. Improve documentation and examples
5. Report bugs and suggest features

See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines.
