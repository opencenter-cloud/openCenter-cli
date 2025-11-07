# Plugins Command Reference

The `plugins` command group manages external plugins for `openCenter`.

Plugins are executables named `openCenter-<name>` discovered at runtime and exposed as `openCenter <name>`.

## Discovery Rules
- `OPENCENTER_PLUGINS_DIR`
- `<config-dir>/plugins` (from `--config-dir` or `OPENCENTER_CONFIG_DIR`; default `~/.config/openCenter/plugins` on macOS/Linux)
- `PATH`

Built-in commands are not shadowed by plugins.

## Environment Variables
- `OPENCENTER_PLUGINS_DIR`: Absolute or relative path to a directory to search for plugin executables first. If set, it is checked before the user config directory and `PATH`.
- `OPENCENTER_CONFIG_DIR`: When set, plugins are also discovered under `$OPENCENTER_CONFIG_DIR/plugins`. This variable is also set automatically if you pass `--config-dir` on the CLI.

## `openCenter plugins list`
Lists discovered plugins and their full paths.

Usage
```bash
openCenter plugins list
```

Example output
```
kubectl	/Users/alice/.config/openCenter/plugins/openCenter-kubectl
```

## Running Plugins
After discovery, plugins are invoked like any other subcommand. For example, with a `openCenter-kubectl` plugin installed:

```bash
openCenter kubectl version
openCenter kubectl api-versions
```

See also: How-to guide at `docs/how-to/plugins.md` for installation examples.
