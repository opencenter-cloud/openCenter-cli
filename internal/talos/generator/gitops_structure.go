package generator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
)

// GenerateGitOpsStructure creates directory layout, kustomization files,
// placeholder manifests, and SOPS configuration for GitOps workflows.
func (g *generator) GenerateGitOpsStructure(ctx context.Context, basePath string) error {
	if basePath == "" {
		return talos.NewConfigurationError(
			"INVALID_PATH",
			"base path cannot be empty",
			nil,
		)
	}

	// Create directory structure
	dirs := []string{
		filepath.Join(basePath, "clusters"),
		filepath.Join(basePath, "infrastructure"),
		filepath.Join(basePath, "infrastructure", "talos"),
		filepath.Join(basePath, "infrastructure", "talos", "machine-configs"),
		filepath.Join(basePath, "infrastructure", "talos", "pulumi"),
		filepath.Join(basePath, "infrastructure", "talos", "wireguard"),
		filepath.Join(basePath, "infrastructure", "networks"),
		filepath.Join(basePath, "infrastructure", "security-groups"),
		filepath.Join(basePath, "applications"),
		filepath.Join(basePath, "applications", "base"),
		filepath.Join(basePath, "applications", "overlays"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return talos.NewConfigurationError(
				"MKDIR_ERROR",
				fmt.Sprintf("failed to create directory %s", dir),
				err,
			)
		}
	}

	// Create kustomization files
	if err := g.createKustomizationFiles(basePath); err != nil {
		return err
	}

	// Create placeholder manifests
	if err := g.createPlaceholderManifests(basePath); err != nil {
		return err
	}

	// Create SOPS configuration
	if err := g.createSOPSConfig(basePath); err != nil {
		return err
	}

	// Create README files
	if err := g.createReadmeFiles(basePath); err != nil {
		return err
	}

	return nil
}

// createKustomizationFiles creates kustomization.yaml files in key directories.
func (g *generator) createKustomizationFiles(basePath string) error {
	kustomizations := map[string]string{
		filepath.Join(basePath, "infrastructure", "talos", "kustomization.yaml"): `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - machine-configs
  - pulumi
  - wireguard
`,
		filepath.Join(basePath, "infrastructure", "kustomization.yaml"): `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - talos
  - networks
  - security-groups
`,
		filepath.Join(basePath, "applications", "base", "kustomization.yaml"): `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: []
`,
	}

	for path, content := range kustomizations {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return talos.NewConfigurationError(
				"WRITE_ERROR",
				fmt.Sprintf("failed to write kustomization file %s", path),
				err,
			)
		}
	}

	return nil
}

// createPlaceholderManifests creates placeholder manifest files.
func (g *generator) createPlaceholderManifests(basePath string) error {
	placeholders := map[string]string{
		filepath.Join(basePath, "infrastructure", "talos", "machine-configs", ".gitkeep"): "",
		filepath.Join(basePath, "infrastructure", "talos", "pulumi", ".gitkeep"):          "",
		filepath.Join(basePath, "infrastructure", "talos", "wireguard", ".gitkeep"):       "",
		filepath.Join(basePath, "infrastructure", "networks", ".gitkeep"):                 "",
		filepath.Join(basePath, "infrastructure", "security-groups", ".gitkeep"):          "",
		filepath.Join(basePath, "applications", "overlays", ".gitkeep"):                   "",
	}

	for path, content := range placeholders {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return talos.NewConfigurationError(
				"WRITE_ERROR",
				fmt.Sprintf("failed to write placeholder file %s", path),
				err,
			)
		}
	}

	return nil
}

// createSOPSConfig creates .sops.yaml configuration file.
func (g *generator) createSOPSConfig(basePath string) error {
	sopsConfig := `creation_rules:
  - path_regex: .*\.(yaml|yml|json)$
    encrypted_regex: ^(data|stringData|password|token|key|secret|privateKey)$
    barbican: true
`

	sopsPath := filepath.Join(basePath, ".sops.yaml")
	if err := os.WriteFile(sopsPath, []byte(sopsConfig), 0644); err != nil {
		return talos.NewConfigurationError(
			"WRITE_ERROR",
			fmt.Sprintf("failed to write SOPS config %s", sopsPath),
			err,
		)
	}

	return nil
}

// createReadmeFiles creates README.md files in key directories.
func (g *generator) createReadmeFiles(basePath string) error {
	readmes := map[string]string{
		filepath.Join(basePath, "README.md"): `# Talos Cluster GitOps Repository

This repository contains the GitOps configuration for a Talos Linux cluster on OpenStack.

## Structure

- **clusters/**: Cluster-specific configurations
- **infrastructure/**: Infrastructure-as-code definitions
  - **talos/**: Talos machine configurations and Pulumi programs
  - **networks/**: Network topology definitions
  - **security-groups/**: Security group rules
- **applications/**: Application manifests
  - **base/**: Base application configurations
  - **overlays/**: Environment-specific overlays

## Security

All secrets are encrypted using SOPS with Barbican key management.
See .sops.yaml for encryption rules.

## Usage

1. Initialize Pulumi stack: 'cd infrastructure/talos/pulumi && pulumi stack init'
2. Apply infrastructure: 'pulumi up'
3. Bootstrap cluster: 'talosctl bootstrap'
4. Deploy applications: 'kubectl apply -k applications/overlays/<env>'
`,
		filepath.Join(basePath, "infrastructure", "talos", "README.md"): `# Talos Infrastructure

This directory contains Talos-specific infrastructure configurations:

- **machine-configs/**: Talos machine configuration files
- **pulumi/**: Pulumi programs for infrastructure provisioning
- **wireguard/**: WireGuard VPN configuration

## Machine Configs

Machine configs are generated with security hardening enabled:
- AppArmor
- Seccomp
- Hardened sysctls
- Disk encryption
- KubePrism

## Pulumi

Pulumi programs provision:
- Networks (management, control, data)
- Security groups
- Load balancers
- Compute instances
- Storage volumes

## WireGuard

WireGuard VPN provides secure access to Talos API and Kubernetes API.
All cluster management operations must go through the VPN.
`,
	}

	for path, content := range readmes {
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return talos.NewConfigurationError(
				"WRITE_ERROR",
				fmt.Sprintf("failed to write README file %s", path),
				err,
			)
		}
	}

	return nil
}
