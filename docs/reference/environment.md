# Environment Variables

This page summarizes environment variables respected by `openCenter`.

## Core
- `OPENCENTER_CONFIG_DIR`
  - Purpose: Overrides the directory where cluster configuration files are stored and read from.
  - Default: On macOS/Linux `~/.config/openCenter`; on Windows `%APPDATA%/openCenter` (falls back to `%LOCALAPPDATA%` or `%USERPROFILE%` if needed).
  - Precedence: Passing `--config-dir` sets this variable for the running process so dependent features (e.g., plugin discovery) see it immediately.

## Plugins
- `OPENCENTER_PLUGINS_DIR`
  - Purpose: Points to a directory that is searched first for plugin executables named `openCenter-*`.
  - Notes: Expects a single directory path. If not set, discovery falls back to `$OPENCENTER_CONFIG_DIR/plugins` and then `PATH`.

## Related
- `PATH`
  - Purpose: Standard OS environment variable used as the last location for plugin discovery.

