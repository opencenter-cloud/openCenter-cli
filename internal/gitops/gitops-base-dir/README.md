# GitOps Repository

This repository contains the GitOps configuration for your Kubernetes cluster managed by openCenter.

## Structure

- `applications/` - Application manifests and overlays
- `infrastructure/` - Infrastructure configuration including cluster-specific settings

## Usage

This repository is managed by openCenter CLI. Changes should be made through the CLI or by editing the configuration files and running `openCenter cluster render` to regenerate the manifests.

For more information, see the [openCenter documentation](https://github.com/rackerlabs/openCenter-cli).
