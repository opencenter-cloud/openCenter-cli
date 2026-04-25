---
id: plugin-internal-services
title: "Plugin Internal Services"
sidebar_label: Plugin Internal Services
description: How openCenter CLI internal service plugins work, using cert-manager to explain service behavior and the code required to add a new platform service.
doc_type: explanation
audience: "developers, platform engineers"
tags: [plugins, services, cert-manager, gitops, extensions]
---

# Plugin Internal Services

**Purpose:** For developers and platform engineers, explains the internal service plugin system in openCenter CLI, uses cert-manager as the worked example, and maps the practical code changes required to add a new platform service.

## What This Plugin Mechanism Is

The internal service plugin system is how openCenter models platform services that are:

- configured inside the cluster YAML
- enabled or disabled with `opencenter cluster service ...`
- validated as part of cluster configuration
- rendered into GitOps templates and manifests

Examples include:

- cert-manager
- Loki
- Harbor
- Keycloak
- Velero

This is different from the external CLI plugin mechanism. External CLI plugins add new commands such as `opencenter foo`; they do not participate in service config typing, service validation, or GitOps manifest generation.

For that separate mechanism, see [Plugin External CLI](plugin-external-cli.md).

## The Service Plugin Model

At a high level, a service such as `cert-manager` is not just a Go plugin object. It is the combination of:

1. A typed configuration struct.
2. A config-type registration entry so the CLI can instantiate that struct by service name.
3. A service plugin implementation that provides metadata, validation, status, and an optional render hook.
4. Optional validation-engine extensions.
5. GitOps templates that actually produce the Kubernetes and Flux manifests.
6. CLI wiring for `cluster service enable`, `cluster service options`, and service-specific secrets.

The core contract is the `ServicePlugin` interface with:

- `Name`
- `Type`
- `Validate`
- `Render`
- `Status`

Evidence:
- `internal/services/plugin.go`
- `internal/services/base_plugin.go`
- `internal/services/registry.go`

## Base Plugin Composition

Most built-in services use `BaseServicePlugin` rather than implementing every method from scratch.

`BaseServicePlugin` provides:

- standard metadata storage
- a validator callback
- a renderer callback
- a status callback
- default no-op behavior where needed

The common pattern is:

1. create a base plugin with metadata
2. embed it in a service-specific struct
3. inject service-specific validation/render/status functions

Evidence:
- `internal/services/base_plugin.go`
- `internal/services/plugins/cert_manager.go`

## Cert-Manager Walkthrough

### Configuration Registration

`cert-manager` starts with a typed config struct:

- `CertManagerConfig` embeds `BaseConfig`
- it adds fields such as `letsencrypt_server`, `email`, `region`, `dns_zones`, `issuers`, and `dns_provider`
- its `init()` function registers the config type under the name `cert-manager`

That registration is what lets generic CLI logic look up a service by name and instantiate the correct struct dynamically.

Evidence:
- `internal/config/services/cert_manager.go`
- `internal/config/registry/registry.go`

### Default Behavior

When a new cluster configuration is built, cert-manager is defaulted into the services map for at least the OpenStack provider path. The defaults include:

- `enabled: true`
- a default support email
- a default Route53 region
- the production Let's Encrypt ACME URL

Evidence:
- `internal/config/defaults.go`

### Enabling the Service from the CLI

When you run:

```bash
opencenter cluster service enable cert-manager --param="email=admin@example.com"
```

the CLI takes a mostly generic path:

1. Look up the service config type in the config registry.
2. Instantiate it with reflection.
3. Set `Enabled = true`.
4. Apply `--param` values by matching CLI keys to JSON tags on struct fields.
5. Apply `--secret` values by routing into the service's secret struct.
6. Run service-specific checks.
7. Save the updated cluster config.
8. Optionally render just that service with `--render`.

For cert-manager specifically, the CLI also hard-requires `email`.

One important implementation detail: the CLI help for service-specific options and secrets is manually maintained. It is not generated from the service config struct. Adding a new service usually means updating `getServiceOptions`, `getServiceSecrets`, and service-specific validation logic yourself.

Evidence:
- `cmd/cluster_service.go`

## The Cert-Manager Plugin Object

The cert-manager service plugin itself is intentionally small.

`NewCertManagerPlugin()`:

- creates a base plugin with metadata
- injects a cert-manager validator
- injects a cert-manager status function
- injects a render function that is currently just a placeholder

The current cert-manager validator checks only a few service-local rules:

- `letsencrypt_server` must use `https://`
- `email` must contain `@`

The status function reports:

- disabled state when the service is off
- config status when it is enabled
- details such as ACME server and email address

Evidence:
- `internal/services/plugins/cert_manager.go`

## Dependencies and Validation Metadata

Built-in services are also registered with dependency metadata.

For example:

- `cert-manager` has no dependencies
- `keycloak` depends on `cert-manager`
- `harbor` depends on `cert-manager`

That dependency graph can be topologically sorted by the service registry. The registry also hosts service-specific validators such as `service:cert-manager`.

Evidence:
- `internal/services/plugins/registry.go`
- `internal/services/plugins/validators.go`
- `internal/services/registry.go`

## How Rendering Actually Happens Today

This is the most important nuance in the current implementation:

The production `cluster generate` path does not call `ServicePlugin.Render()` for cert-manager. Instead, it renders by walking the embedded GitOps template tree directly.

The current setup flow is:

1. Copy the base GitOps repository structure.
2. Render the cluster application overlays from `templates/cluster-apps-base`.
3. Render the infrastructure templates.
4. Run OpenTofu provisioning when needed.

So although the service plugin has a `Render()` method, the real cert-manager behavior currently comes mostly from templates, config types, and CLI wiring.

Evidence:
- `internal/cluster/setup_service.go`
- `internal/gitops/copy.go`
- `internal/gitops/embed.go`

## Cert-Manager Template Behavior

For cert-manager, the rendered output is assembled from several template families:

1. `services/sources/opencenter-cert-manager.yaml.tpl`
2. `services/fluxcd/cert-manager.yaml.tpl`
3. `services/cert-manager/*.tpl`

These templates generate the Flux source wiring, Flux Kustomizations, overlay issuer resources, overlay secrets, and Kustomize resources.

The cert-manager templates read values directly from the cluster config, including:

- `Email`
- `LetsEncryptServer`
- `Region`
- cluster name
- cluster FQDN

Evidence:
- `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-cert-manager.yaml.tpl`
- `internal/gitops/templates/cluster-apps-base/services/fluxcd/cert-manager.yaml.tpl`
- `internal/gitops/templates/cluster-apps-base/services/cert-manager/letsencrypt-issuer.yaml.tpl`
- `internal/gitops/templates/cluster-apps-base/services/cert-manager/kustomization.yaml.tpl`

### Secrets Fallback Behavior

The AWS credentials used by the cert-manager Route53 secret follow a fallback chain:

1. service-specific cert-manager credentials
2. global application AWS credentials
3. global infrastructure AWS credentials

That logic lives on the main `Config` type so templates can call helper methods directly.

Evidence:
- `internal/config/config.go`
- `internal/gitops/templates/cluster-apps-base/services/cert-manager/opencenter-aws-credentials-secret.yaml.tpl`

### Inclusion Versus Presence

The renderer controls service behavior through two different mechanisms:

- file-level skipping for disabled service directories
- kustomization-level conditional inclusion for source and Flux wiring files

So a file may exist in the rendered overlay tree, but it only becomes active when the aggregate `kustomization.yaml` includes it.

Evidence:
- `internal/gitops/copy.go`
- `internal/gitops/templates/cluster-apps-base/services/sources/kustomization.yaml.tpl`
- `internal/gitops/templates/cluster-apps-base/services/fluxcd/kustomization.yaml.tpl`

## The Newer Registry-Based Path

The repository also contains a stage-based rendering model built around:

- a global template registry that scans embedded templates
- a service stage that resolves enabled services
- template dependency resolution
- stage-based rendering and validation

That model is conceptually cleaner and is useful to understand, but the main production setup path still uses the direct file-walk renderer described above.

Evidence:
- `internal/template/global_registry.go`
- `internal/template/embedded_registry.go`
- `internal/gitops/stages/service_stage.go`

## How To Add a New Internal Service Plugin

If you want to add a new platform service like cert-manager, the practical path today is to add a built-in service to this repository.

### Step 1: Add the Typed Config

Create `internal/config/services/<service>.go`:

- embed `BaseConfig`
- add service-specific fields
- register the config in `init()` with `registry.RegisterServiceConfig("<service>", MyServiceConfig{})`

This is required so `cluster service enable <service>` can instantiate the correct type.

### Step 2: Add Defaults

Add the service to the default services map in `internal/config/defaults.go`.

Without this, new cluster configs will not consistently include or initialize the service.

### Step 3: Add the Service Plugin

Create `internal/services/plugins/<service>.go` and model it after cert-manager, Loki, Harbor, or Velero:

```go
type MyServiceConfig struct {
    BaseConfig `yaml:",inline"`
    Endpoint   string `yaml:"endpoint" json:"endpoint,omitempty"`
}

func init() {
    registry.RegisterServiceConfig("my-service", MyServiceConfig{})
}

type MyServicePlugin struct {
    *svc.BaseServicePlugin
}

func NewMyServicePlugin() svc.ServicePlugin {
    base := svc.NewBasePlugin(svc.PluginMetadata{
        Name:        "my-service",
        Version:     "1.0.0",
        Description: "My custom platform service",
        Type:        svc.ServiceTypeCore,
        Author:      "opencenter",
        License:     "Apache-2.0",
    })

    p := &MyServicePlugin{BaseServicePlugin: base}
    base.SetValidator(p.validate)
    base.SetStatusFunc(p.status)
    return p
}
```

### Step 4: Register the Service and Dependencies

Update `internal/services/plugins/registry.go`:

- add `NewMyServicePlugin()` to the built-in service list
- declare dependencies such as `cert-manager` if your service requires TLS or issuers

If you want validation-engine integration, also add a validator in `internal/services/plugins/validators.go`.

### Step 5: Wire the CLI Management Path

Update `cmd/cluster_service.go`:

- `getServiceOptions`
- `getServiceSecrets`
- `validateService`
- `processSecrets` service-to-field mapping if the service has secrets

This is necessary because the current CLI service-management path is only partially generic.

### Step 6: Add Secret Types and Fallback Helpers

If your templates need secrets:

- add the service secret struct in the main config types
- add helper methods on `Config` if templates need fallback logic

Cert-manager is the best example because its templates call config helper methods directly.

### Step 7: Add GitOps Templates

At minimum, add:

- `internal/gitops/templates/cluster-apps-base/services/<service>/...`
- `internal/gitops/templates/cluster-apps-base/services/sources/opencenter-<service>.yaml.tpl`
- `internal/gitops/templates/cluster-apps-base/services/fluxcd/<service>.yaml.tpl`

Those exact file names matter. `RenderSingleService()` looks for:

- a service directory named after the service
- a source file named `opencenter-<service>.yaml` or `.yaml.tpl`
- a Flux file named `<service>.yaml` or `.yaml.tpl`

If the naming does not match, `opencenter cluster service enable <service> --render` will miss the files.

### Step 8: Update Aggregate Kustomizations

If the new service should be selectable through the normal overlay structure, update the aggregate kustomization templates:

- `services/sources/kustomization.yaml.tpl`
- `services/fluxcd/kustomization.yaml.tpl`

Those files decide whether the rendered source and Flux files are actually included.

### Step 9: Update Template Registry Inference If Needed

If you want the stage-based template registry path to recognize the new service automatically, add the service name to the hard-coded list in `internal/template/embedded_registry.go`.

The direct render path does not need this, but the template-registry path does.

### Step 10: Add Tests

At minimum, add:

- plugin unit tests
- integration tests for dependencies and status
- config and service-enable tests
- template rendering tests

## Related Reading

- [Plugin External CLI](plugin-external-cli.md)
- [Service Templates](services-templates.md)
- [Adding Services](../dev/adding-services.md)
- [Code Structure](../dev/code-structure.md)
- [GitOps Workflow](gitops-workflow.md)

## Evidence

Primary code paths referenced in this explanation:

- `cmd/cluster_service.go`
- `internal/config/services/cert_manager.go`
- `internal/config/registry/registry.go`
- `internal/config/defaults.go`
- `internal/config/config.go`
- `internal/services/plugin.go`
- `internal/services/base_plugin.go`
- `internal/services/registry.go`
- `internal/services/plugins/cert_manager.go`
- `internal/services/plugins/registry.go`
- `internal/services/plugins/validators.go`
- `internal/cluster/setup_service.go`
- `internal/gitops/copy.go`
- `internal/gitops/embed.go`
- `internal/gitops/stages/service_stage.go`
- `internal/template/global_registry.go`
- `internal/template/embedded_registry.go`
