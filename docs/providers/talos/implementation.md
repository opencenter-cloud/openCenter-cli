# Talos OpenStack Provider

This package provides the Talos OpenStack provider implementation for the openCenter CLI.

## Overview

The Talos provider enables secure, immutable Kubernetes cluster deployment on OpenStack infrastructure using Talos Linux. It integrates with the openCenter CLI to deliver a declarative, GitOps-friendly lifecycle powered by Pulumi Go bindings.

The provider enforces Zero Trust networking, cryptographic attestation, and defense-in-depth policies by default, eliminating SSH access and traditional mutable management patterns.

## Package Structure

```
internal/talos/
├── doc.go                  # Package documentation
├── interfaces.go           # Core interfaces (Validator, Generator, PulumiManager)
├── types.go               # Data models (NetworkTopology, SecurityGroup, etc.)
├── config.go              # Configuration types and defaults
├── errors.go              # Structured error handling with categorization
├── logging.go             # Logging helpers
├── validator/             # Pre-flight validation package
│   └── doc.go
├── generator/             # Artifact generation package
│   └── doc.go
└── pulumi/                # Pulumi integration package
    └── doc.go
```

## Core Interfaces

### Validator

Performs pre-flight validation checks on OpenStack environments:
- Keystone service availability and MFA enforcement
- Barbican secret creation/retrieval testing
- Octavia load balancer service availability
- Tenant resource quota verification
- Glance image signature verification status

### Generator

Creates declarative artifacts for cluster deployment:
- Talos machine configurations with security hardening
- Pulumi stack configuration files
- WireGuard VPN configuration
- Network topology definitions
- Security group rules
- SOPS encryption policies
- GitOps directory structure

### PulumiManager

Manages infrastructure lifecycle through Pulumi Go SDK:
- Stack initialization and backend configuration
- Infrastructure preview and change planning
- Resource provisioning and updates
- Configuration drift detection
- Resource cleanup and destruction

## Error Handling

The package implements structured error handling with six categories:

1. **Validation**: Pre-flight check failures
2. **Configuration**: Invalid or incomplete configuration data
3. **Infrastructure**: OpenStack API failures, resource creation failures
4. **Network**: Connectivity issues, timeout errors
5. **Security**: Encryption failures, signature verification failures
6. **State**: Pulumi state corruption, Swift backend unavailability

Each error includes:
- Error code for programmatic handling
- Human-readable message
- Category classification
- Retryability flag
- Optional remediation actions
- Optional context information

## Configuration

The `TalosConfig` structure extends the existing `config.Config` with Talos-specific settings:

- **MachineConfig**: Security hardening settings (AppArmor, Seccomp, disk encryption)
- **NetworkConfig**: Network topology (management, control, data subnets)
- **SecurityConfig**: Security policies (vTPM, image verification, MFA)
- **PulumiConfig**: Pulumi backend settings (Swift container, stack name)

Default configuration enforces secure defaults:
- All security features enabled
- Three-zone network architecture
- vTPM-backed encryption
- Image signature verification required
- MFA enforcement

## Logging

The package integrates with the global logging system from `internal/config`:

```go
// Get a logger for the Talos provider
logger := talos.Logger()

// Get a logger with additional fields
logger := talos.LoggerWithFields(logrus.Fields{
    "cluster": "my-cluster",
    "operation": "validate",
})

// Get component-specific loggers
validatorLogger := talos.ValidatorLogger()
generatorLogger := talos.GeneratorLogger()
pulumiLogger := talos.PulumiLogger()
```

## Testing

Run tests with:

```bash
go test ./internal/talos/...
```

The package includes:
- Unit tests for error handling and configuration
- Interface definitions for mocking in integration tests
- Test helpers for common operations

## Next Steps

This foundation enables implementation of:

1. **Validator Package** (task 3): OpenStack environment validation
2. **Generator Package** (task 4): Artifact generation
3. **Pulumi Integration** (task 5): Infrastructure lifecycle management
4. **CLI Commands** (task 7): User-facing commands

See `.kiro/specs/talos-openstack-provider/tasks.md` for the complete implementation plan.
