# `openCenter cluster edit` - Edit a Cluster Configuration

## Synopsis
```bash
openCenter cluster edit [name]
```

## Description

Open a cluster configuration file in your preferred text editor. The editor is determined by checking the `EDITOR` or `VISUAL` environment variables, falling back to `vi` if neither is set.

If no cluster name is provided, the currently selected cluster is edited.

## Arguments

### `[name]`
- **Required/Optional**: Optional
- **Description**: Name of the cluster to edit (format: `cluster` or `organization/cluster`). If not provided, edits the currently selected cluster
- **Example**: `my-cluster` or `production/my-cluster`

## Options

### `-h, --help`
- **Description**: Display help information for this subcommand

## Examples

### Edit currently selected cluster
```bash
openCenter cluster edit
```
Opens the active cluster configuration in your default editor.

### Edit specific cluster
```bash
openCenter cluster edit my-cluster
```
Opens the specified cluster configuration.

### Edit cluster in organization
```bash
openCenter cluster edit production/prod-cluster
```
Opens a cluster configuration within a specific organization.

### Use specific editor
```bash
EDITOR=nano openCenter cluster edit my-cluster
```
Opens the configuration in nano editor.

### Use VS Code
```bash
EDITOR="code --wait" openCenter cluster edit my-cluster
```
Opens the configuration in VS Code and waits for the file to be closed.

## Editor Selection

The command determines which editor to use in the following order:

1. `EDITOR` environment variable
2. `VISUAL` environment variable
3. `vi` (default fallback)

### Setting Your Preferred Editor

```bash
# In your shell profile (~/.bashrc, ~/.zshrc, etc.)
export EDITOR=vim
export VISUAL=code
```

### Common Editors

```bash
# Vim
export EDITOR=vim

# Emacs
export EDITOR=emacs

# Nano
export EDITOR=nano

# VS Code (wait for file to close)
export EDITOR="code --wait"

# Sublime Text
export EDITOR="subl --wait"

# Atom
export EDITOR="atom --wait"
```

## Output

```
Opening /home/user/.config/openCenter/clusters/myorg/.my-cluster-config.yaml in vim...
Configuration file saved.
```

## Configuration File Location

The configuration file is located at:
```
~/.config/openCenter/clusters/<organization>/.<cluster>-config.yaml
```

For example:
```
~/.config/openCenter/clusters/production/.prod-cluster-config.yaml
```

## Notes

- The command opens the actual configuration file for direct editing
- Changes are saved when you exit the editor
- No validation is performed automatically after editing
- Use `openCenter cluster validate` after editing to check for errors
- The configuration file is in YAML format
- File permissions are preserved (typically 0600 for security)
- If no cluster is selected and no name is provided, an error is returned
- The command waits for the editor to close before returning
- For GUI editors, use the `--wait` flag to ensure proper behavior

## Workflow

Typical workflow for editing a cluster configuration:

```bash
# 1. Edit the configuration
openCenter cluster edit my-cluster

# 2. Validate the changes
openCenter cluster validate my-cluster

# 3. If validation passes, update GitOps repository
openCenter cluster setup my-cluster --render
```

## Troubleshooting

### No cluster selected
**Error**: `no cluster selected. Use 'openCenter cluster select' to select a cluster or provide a cluster name`

**Solution**: Either select a cluster or provide a name:
```bash
openCenter cluster select my-cluster
openCenter cluster edit
# or
openCenter cluster edit my-cluster
```

### Cluster not found
**Error**: `cluster configuration file 'my-cluster' not found`

**Solution**: Check available clusters:
```bash
openCenter cluster list
```

### Editor not found
**Error**: `failed to open editor: exec: "myeditor": executable file not found in $PATH`

**Solution**: Set a valid editor:
```bash
export EDITOR=vim
openCenter cluster edit my-cluster
```

## See Also

- `openCenter cluster validate` - Validate cluster configuration
- `openCenter cluster info` - Show cluster information
- `openCenter cluster update` - Update specific configuration fields
