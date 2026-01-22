# cluster edit


## Table of Contents

- [Synopsis](#synopsis)
- [Description](#description)
- [Arguments](#arguments)
- [Examples](#examples)
- [Editor Selection](#editor-selection)
- [Security Validation](#security-validation)
- [Output](#output)
- [Error Handling](#error-handling)
- [Common Editors](#common-editors)
- [Configuration File Location](#configuration-file-location)
- [Post-Edit Validation](#post-edit-validation)
- [See Also](#see-also)
**doc_type:** reference

Edit cluster configuration in your preferred editor.

## Synopsis

```bash
openCenter cluster edit [name]
```

## Description

The `cluster edit` command opens the cluster configuration file in your preferred text editor. If no cluster name is provided, it edits the currently selected cluster.

The editor is determined by checking environment variables in this order:
1. `EDITOR`
2. `VISUAL`
3. Falls back to `vi` if neither is set

## Arguments

- `name` - Cluster name (optional if active cluster is set)

## Examples

```bash
# Edit the currently selected cluster
openCenter cluster edit

# Edit a specific cluster
openCenter cluster edit my-cluster

# Edit a cluster in a specific organization
openCenter cluster edit myorg/my-cluster
```

## Editor Selection

The command uses the following precedence for editor selection:

1. **EDITOR environment variable**
   ```bash
   export EDITOR=nano
   openCenter cluster edit my-cluster
   ```

2. **VISUAL environment variable**
   ```bash
   export VISUAL=emacs
   openCenter cluster edit my-cluster
   ```

3. **Default fallback: vi**
   ```bash
   # If neither EDITOR nor VISUAL is set
   openCenter cluster edit my-cluster  # Opens in vi
   ```

## Security Validation

The command performs security validation on:
- Cluster name (alphanumeric, hyphens, underscores, forward slashes)
- Configuration file path (prevents path traversal)
- Editor command (prevents command injection)

## Output

```
Opening /path/to/clusters/org/.my-cluster-config.yaml in nano...
Configuration file saved.
```

## Error Handling

**No cluster name and no active cluster:**
```
Error: no cluster selected. Use 'openCenter cluster select' to select a cluster or provide a cluster name
```

**Invalid cluster name:**
```
Error: invalid cluster name 'my-cluster!@#': cluster name must contain only alphanumeric characters, hyphens, underscores, and forward slashes
```

**Configuration file not found:**
```
Error: cluster configuration file 'my-cluster' not found. Use 'openCenter cluster list' to see available clusters
```

**Invalid EDITOR environment variable:**
```
Error: invalid EDITOR environment variable: editor command contains invalid characters
```

**Failed to open editor:**
```
Error: failed to open editor: exec: "invalid-editor": executable file not found in $PATH
```

## Common Editors

### nano
```bash
export EDITOR=nano
openCenter cluster edit my-cluster
```

### vim
```bash
export EDITOR=vim
openCenter cluster edit my-cluster
```

### emacs
```bash
export EDITOR=emacs
openCenter cluster edit my-cluster
```

### VS Code
```bash
export EDITOR="code --wait"
openCenter cluster edit my-cluster
```

### Sublime Text
```bash
export EDITOR="subl --wait"
openCenter cluster edit my-cluster
```

## Configuration File Location

Configuration files are stored in organization-based structure:
- Organization-based: `~/.config/openCenter/clusters/<org>/.<cluster>-config.yaml`
- Legacy: `~/.config/openCenter/clusters/<cluster>/.<cluster>-config.yaml`
- Flat: `~/.config/openCenter/.<cluster>-config.yaml`

## Post-Edit Validation

After editing, validate the configuration:

```bash
# Edit configuration
openCenter cluster edit my-cluster

# Validate changes
openCenter cluster validate my-cluster
```

## See Also

- [cluster validate](../cli-commands.md#cluster-validate) - Validate cluster configuration
- [cluster info](info.md) - Display cluster information
- [cluster update](../cli-commands.md#cluster-update) - Update configuration programmatically
