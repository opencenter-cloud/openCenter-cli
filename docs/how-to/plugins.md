# Use Plugins with openCenter

openCenter supports external plugins discovered at runtime. A plugin is any executable on your system whose filename starts with `openCenter-`. Each discovered plugin is added as a top‑level subcommand whose name is the part after the prefix.

For example, an executable named `openCenter-kubectl` becomes the `kubectl` subcommand, so you can run `openCenter kubectl ...` and all trailing arguments are forwarded to the plugin.

## Where plugins are discovered

Discovery order (later entries can override earlier ones by name):

- `OPENCENTER_PLUGINS_DIR` (if set)
- `<config-dir>/plugins` where `config-dir` is resolved from `--config-dir` or `OPENCENTER_CONFIG_DIR`, otherwise defaults to `~/.config/openCenter/plugins` on macOS/Linux
- Every directory in your `PATH`

To prevent conflicts, built‑in commands take precedence over plugins with the same name.

## List Discovered Plugins

You can list available plugins and their locations:

```bash
openCenter plugins list
```

## Example: A kubectl passthrough plugin

This simple plugin exposes `kubectl` as an openCenter subcommand. It supports two example invocations:

- `openCenter kubectl version`
- `openCenter kubectl api-versions`

Create a file named `openCenter-kubectl` with the following contents and make it executable:

```bash
#!/usr/bin/env bash
set -euo pipefail

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is not installed or not on PATH" >&2
  exit 127
fi

# Forward all args directly to kubectl
exec kubectl "$@"
```

Place it in one of the discovery locations, for example:

```bash
mkdir -p ~/.config/openCenter/plugins
mv ./openCenter-kubectl ~/.config/openCenter/plugins/
chmod +x ~/.config/openCenter/plugins/openCenter-kubectl
```

Now you can list and run:

```bash
openCenter plugins list
openCenter kubectl version
openCenter kubectl api-versions
```

Tip: You can put any logic inside the plugin — shell, Python, Go, etc. As long as it’s an executable named `openCenter-<name>`, it will appear as `openCenter <name>`.

## Example: A Go plugin that prints your public IP

A minimal Go plugin that performs an HTTPS request to `https://icanhazip.com/` and prints the result.

Source is provided at `docs/examples/plugins/openCenter-myip/`.

Build the plugin:

```bash
cd docs/examples/plugins/openCenter-myip
go build -o openCenter-myip .
```

Install it to a discovery location, e.g. your user config plugins dir:

```bash
mkdir -p ~/.config/openCenter/plugins
mv ./openCenter-myip ~/.config/openCenter/plugins/
chmod +x ~/.config/openCenter/plugins/openCenter-myip
```

Run it via openCenter:

```bash
openCenter myip
```

The command will print a single line with your public IP address.

## Example: Go plugin destined for its own repo (rmpk)

The `openCenter-rmpk` example demonstrates a slightly richer Go plugin with subcommands. It’s designed to be extracted into its own repository.

- `ops`: prints available subcommands
- `countPods --node <name>`: lists pods scheduled on a node and their images (uses `kubectl`).

Source: `docs/examples/plugins/openCenter-rmpk/`

Build and run:

```bash
cd docs/examples/plugins/openCenter-rmpk
go build -o openCenter-rmpk .
mkdir -p ~/.config/openCenter/plugins
mv ./openCenter-rmpk ~/.config/openCenter/plugins/
chmod +x ~/.config/openCenter/plugins/openCenter-rmpk

openCenter rmpk ops
openCenter rmpk countPods --node <node-name>
```
