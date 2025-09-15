## openCenter-rmpk (Go plugin)

A Go-based `openCenter` plugin that will eventually live in its own repository. It exposes:

- `ops`: Prints available subcommands.
- `countPods`: Lists pods scheduled on a node and the images they use.

The plugin binary must be named `openCenter-rmpk` so `openCenter` discovers it as `openCenter rmpk`.

## Build

```
cd docs/examples/plugins/openCenter-rmpk
go build -o openCenter-rmpk .
```

## Install

Place the compiled binary where `openCenter` discovers plugins (pick one):

- `~/.config/openCenter/plugins`
- Directory pointed to `OPENCENTER_PLUGINS_DIR`
- Any directory on your `PATH`

Example:

```
mkdir -p ~/.config/openCenter/plugins
mv ./openCenter-rmpk ~/.config/openCenter/plugins/
chmod +x ~/.config/openCenter/plugins/openCenter-rmpk
```

## Usage

```
# List plugin subcommands
openCenter rmpk ops

# List pods and images on a specific node (requires kubectl + kubeconfig)
openCenter rmpk countPods --node <node-name>
```

Output example:

```
Pods on node worker-1:
- kube-system/coredns-558bd4d5db-xyz12
  images: coredns/coredns:1.11.1
- default/app-7c9d8c5b9-abcde
  images: ghcr.io/org/app:1.2.3, busybox:1.36

Unique images (3):
- coredns/coredns:1.11.1
- ghcr.io/org/app:1.2.3
- busybox:1.36
```

## Requirements

- `kubectl` available on `PATH` and access to your cluster (via `KUBECONFIG` or in-cluster config).

## Notes

- Uses only the Go standard library; calls `kubectl get pods -A -o json` and filters by `.spec.nodeName`.
- Designed to be extracted into its own repository later.
- Exit codes: non-zero on errors (missing node flag, `kubectl` not found, bad JSON, etc.).

