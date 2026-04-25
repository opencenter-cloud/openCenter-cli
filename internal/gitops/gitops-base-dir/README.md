# GitOps Repository

This repository contains the GitOps configuration for your Kubernetes cluster managed by opencenter.

## Structure

- `applications/` - Application manifests and overlays
- `infrastructure/` - Infrastructure configuration including cluster-specific settings

## Usage

This repository is managed by opencenter CLI. Changes should be made through the CLI or by editing the configuration files and running `opencenter cluster generate --render-only` to regenerate the manifests.

For more information, see the [opencenter documentation](https://github.com/opencenter-cloud/opencenter-cli).
