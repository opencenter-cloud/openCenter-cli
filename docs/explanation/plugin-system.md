# Plugin System Architecture


## Table of Contents

- [Who this is for](#who-this-is-for)
- [What plugins solve](#what-plugins-solve)
- [How plugin discovery works](#how-plugin-discovery-works)
- [Plugin naming convention](#plugin-naming-convention)
- [How plugins execute](#how-plugins-execute)
- [Flag handling](#flag-handling)
- [Service plugins vs command plugins](#service-plugins-vs-command-plugins)
- [The service registry pattern](#the-service-registry-pattern)
- [Lifecycle hooks](#lifecycle-hooks)
- [Plugin manifest format](#plugin-manifest-format)
- [Dependency injection integration](#dependency-injection-integration)
- [Security considerations](#security-considerations)
- [Performance characteristics](#performance-characteristics)
- [Common patterns](#common-patterns)
- [Extending the plugin system](#extending-the-plugin-system)
- [Trade-offs and limitations](#trade-offs-and-limitations)
- [Comparison with other plugin systems](#comparison-with-other-plugin-systems)
- [See also](#see-also)
**doc_type**: explanation

## Who this is for

Developers who want to understand how openCenter's plugin system works, why it's designed the way it is, and how to extend openCenter with custom functionality.

## What plugins solve

openCenter provides a comprehensive set of commands for cluster management, but every organization has unique workflows. The plugin system lets you extend openCenter without modifying its source code.

A plugin is an executable that openCenter discovers and exposes as a subcommand. If you create `openCenter-backup`, users can run `openCenter backup` as if it were a built-in command.

## How plugin discovery works

openCenter searches three locations for plugins, in this order:

1. **`OPENCENTER_PLUGINS_DIR`** environment variable
2. **`<config-dir>/plugins`** directory (typically `~/.config/openCenter/plugins`)
3. **System `PATH`**

When openCenter starts, it scans these locations for executables matching the pattern `openCenter-*`. Each discovered plugin becomes a subcommand.

### Why this search order?

The environment variable takes precedence because it allows per-session plugin overrides—useful for testing or temporary customization. The config directory comes next because it's user-specific and doesn't require system-wide installation. The system PATH comes last as a fallback for globally installed plugins.

### Why scan at startup?

Plugin discovery happens once at startup rather than on-demand. This means:

- **Fast command execution**: No filesystem scanning when running commands
- **Predictable behavior**: The available commands don't change during execution
- **Simple caching**: Discovered plugins are cached in memory

The trade-off is that newly installed plugins require restarting openCenter to be recognized. But this is acceptable—plugin installation is rare compared to command execution.

## Plugin naming convention

Plugins must follow the naming pattern `openCenter-<name>`. The prefix is case-insensitive, so `openCenter-backup`, `OPENCENTER-backup`, and `OpenCenter-backup` all work.

The plugin name (the part after the prefix) becomes the subcommand. So `openCenter-backup` is invoked as `openCenter backup`.

### Why require a prefix?

The prefix prevents accidental plugin registration. Without it, any executable in your PATH could become an openCenter command. The prefix makes plugin registration explicit and prevents naming conflicts with other tools.

### Why case-insensitive?

Different operating systems have different conventions. Windows is case-insensitive, Unix-like systems are case-sensitive. By accepting any case, we avoid platform-specific issues.

## How plugins execute

When you run `openCenter backup --restore latest`, here's what happens:

1. **Command parsing**: Cobra parses `backup` as a subcommand
2. **Plugin lookup**: openCenter finds the `openCenter-backup` executable
3. **Argument forwarding**: All arguments after `backup` are passed to the plugin
4. **Process execution**: The plugin runs as a separate process
5. **Output streaming**: Plugin stdout/stderr stream to the terminal
6. **Exit code preservation**: The plugin's exit code becomes openCenter's exit code

### Why separate processes?

Plugins run as separate processes rather than being loaded as libraries. This design choice has significant implications:

**Isolation**: A plugin crash doesn't crash openCenter. A plugin memory leak doesn't affect openCenter. A plugin can be written in any language—Go, Python, Bash, Rust—as long as it produces an executable.

**Simplicity**: No plugin API to maintain. No version compatibility issues. No shared memory concerns. Plugins are just executables that follow Unix conventions.

**Security**: Plugins can't access openCenter's internal state. They can't modify configuration in memory or bypass validation. They interact with openCenter only through the CLI interface.

The trade-off is performance—process creation has overhead. But for typical plugin operations (which involve network calls or file I/O), this overhead is negligible.

## Flag handling

Plugins receive all flags and arguments exactly as the user provided them. openCenter doesn't parse plugin flags—it forwards them transparently.

This is implemented with Cobra's `DisableFlagParsing: true` option. When enabled, Cobra treats everything after the plugin name as raw arguments.

### Why transparent forwarding?

Each plugin defines its own flags. If openCenter tried to parse them, it would need to know every plugin's flag schema—impossible for external plugins. By forwarding transparently, plugins have complete control over their interface.

### How plugins access openCenter state

Plugins can't directly access openCenter's configuration or state. Instead, they use the CLI:

```bash
#!/bin/bash
# openCenter-backup plugin

# Get cluster configuration
config=$(openCenter config view --format json)

# Parse with jq
cluster_name=$(echo "$config" | jq -r '.opencenter.meta.name')

# Perform backup operation
echo "Backing up cluster: $cluster_name"
```

This indirection has benefits:

- **Stability**: Plugins depend on the CLI interface, which is stable and versioned
- **Testability**: Plugins can be tested independently of openCenter internals
- **Portability**: Plugins work across openCenter versions as long as the CLI interface is compatible

## Service plugins vs command plugins

openCenter has two plugin concepts that serve different purposes:

### Command plugins (external executables)

These are the plugins described above—executables that extend the CLI with new commands. They're discovered at startup and executed as separate processes.

**Use case**: Adding new workflows, integrating with external tools, custom automation.

**Example**: `openCenter-backup` for cluster backup operations.

### Service plugins (internal Go packages)

These are Go packages in `internal/services/` that define how services (like cert-manager, Prometheus, Loki) are configured and deployed.

**Use case**: Adding new Kubernetes services to the cluster, customizing service behavior.

**Example**: `CertManagerConfig` defines how cert-manager is configured and deployed.

### Why two plugin types?

They solve different problems:

- **Command plugins** extend what openCenter can do (new commands, new workflows)
- **Service plugins** extend what clusters can include (new services, new components)

Command plugins are external and language-agnostic. Service plugins are internal and Go-specific. This separation keeps the extension points focused and simple.

## The service registry pattern

Service plugins use a registry pattern for discovery and dependency management. The `ServiceRegistry` in `internal/services/registry.go` provides:

### Service registration

Services register themselves with metadata:

```go
registry.RegisterService(ServiceDefinition{
    Name:         "cert-manager",
    Type:         ServiceTypeSecurity,
    Version:      "v1.12.0",
    Dependencies: []string{"gateway-api"},
    Plugin:       certManagerPlugin,
})
```

### Dependency resolution

The registry resolves dependencies automatically:

```go
// Request cert-manager
services, err := registry.ResolveDependencies([]string{"cert-manager"})
// Returns: ["gateway-api", "cert-manager"] in correct order
```

This ensures services are deployed in the right order. If cert-manager depends on gateway-api, gateway-api is deployed first.

### Circular dependency detection

The registry detects circular dependencies at registration time:

```go
// This would fail:
// service-a depends on service-b
// service-b depends on service-a
err := registry.ValidateDependencies([]string{"service-a"})
// Returns: "circular dependency detected: [service-a, service-b, service-a]"
```

### Why a registry?

Without a registry, each command would need to know about every service and their dependencies. The registry centralizes this knowledge, making it easy to add new services without modifying existing code.

## Lifecycle hooks

Service plugins can define lifecycle hooks that run at specific points:

- **PreInstall**: Before service installation (validate prerequisites)
- **PostInstall**: After service installation (verify deployment)
- **PreUpdate**: Before service update (backup state)
- **PostUpdate**: After service update (verify upgrade)
- **PreRemove**: Before service removal (backup data)
- **PostRemove**: After service removal (cleanup resources)

### Hook execution order

When deploying multiple services, hooks execute in dependency order:

```
PreInstall: gateway-api
PreInstall: cert-manager
Install: gateway-api
PostInstall: gateway-api
Install: cert-manager
PostInstall: cert-manager
```

For removal, the order reverses (remove dependents before dependencies):

```
PreRemove: cert-manager
Remove: cert-manager
PostRemove: cert-manager
PreRemove: gateway-api
Remove: gateway-api
PostRemove: gateway-api
```

### Why lifecycle hooks?

Services often need to perform actions beyond template rendering. Cert-manager might need to validate AWS credentials before installation. Prometheus might need to create storage volumes. Lifecycle hooks provide these extension points without cluttering the core generation logic.

## Plugin manifest format

Service plugins can be defined in YAML manifests:

```yaml
name: cert-manager
version: v1.12.0
type: security
description: Certificate management for Kubernetes
dependencies:
  - gateway-api
templates:
  - name: deployment
    path: templates/cert-manager/deployment.yaml
  - name: issuer
    path: templates/cert-manager/issuer.yaml
    condition:
      enabled: "true"
config:
  schema:
    email:
      type: string
      required: true
    region:
      type: string
      default: us-east-1
  validation:
    - field: email
      type: email
      message: "Must be a valid email address"
```

This manifest-based approach allows defining services without writing Go code. The registry loads manifests from a directory and creates service definitions automatically.

### Why manifests?

Manifests lower the barrier to adding services. You can define a new service by writing YAML and templates, without understanding openCenter's internal APIs. This makes the system more accessible to operators who know Kubernetes but not Go.

## Dependency injection integration

The plugin system integrates with openCenter's dependency injection container. When a command needs a service registry, it requests it from the DI container:

```go
func newServiceEnableCmd(container di.Container) *cobra.Command {
    return &cobra.Command{
        Use: "enable <service>",
        RunE: func(cmd *cobra.Command, args []string) error {
            var registry services.ServiceRegistry
            if err := container.ResolveAs("service-registry", &registry); err != nil {
                return err
            }
            
            service, err := registry.GetService(args[0])
            // ...
        },
    }
}
```

### Why DI for plugins?

Dependency injection makes the plugin system testable. In tests, you can inject a mock registry that returns predefined services. In production, you inject the real registry that loads from manifests.

This also makes the system more flexible. You can swap registry implementations (file-based, database-based, remote) without changing command code.

## Security considerations

### Command plugin security

Command plugins run with the same permissions as openCenter. If openCenter has access to cloud credentials, plugins have access too.

**Mitigation**: Only install plugins from trusted sources. Review plugin code before installation. Use the config directory (not system PATH) for user-specific plugins.

### Service plugin security

Service plugins run in-process and have full access to openCenter's memory and state.

**Mitigation**: Service plugins are part of openCenter's codebase and go through code review. External service plugins (via manifests) are sandboxed—they can only define configuration and templates, not execute arbitrary code.

### Template security

Service templates are rendered with user-provided configuration. A malicious template could execute arbitrary code during rendering.

**Mitigation**: The template engine has a sandbox mode that disables dangerous functions (env, exec, readFile). When rendering untrusted templates, enable sandbox mode.

## Performance characteristics

### Plugin discovery

Discovery scans three directories and checks file permissions. For a typical system with 10 plugins, this takes ~5ms. Discovery happens once at startup, so the cost is amortized across all commands.

### Plugin execution

Spawning a process takes ~10ms on modern systems. For plugins that perform network operations (which take 100ms+), this overhead is negligible.

### Service registry

Service lookup is O(1) (hash map). Dependency resolution is O(n) where n is the number of services. For typical clusters with 20 services, resolution takes <1ms.

## Common patterns

### Wrapper plugins

Plugins that wrap existing tools:

```bash
#!/bin/bash
# openCenter-kubectl: Run kubectl with cluster context

cluster=$(openCenter config get opencenter.meta.name)
kubeconfig="$HOME/.config/openCenter/clusters/$cluster/kubeconfig"

kubectl --kubeconfig="$kubeconfig" "$@"
```

### Integration plugins

Plugins that integrate with external systems:

```bash
#!/bin/bash
# openCenter-slack: Send cluster events to Slack

event="$1"
message="$2"

webhook_url=$(openCenter config get integrations.slack.webhook)
curl -X POST "$webhook_url" -d "{\"text\": \"$event: $message\"}"
```

### Validation plugins

Plugins that perform custom validation:

```bash
#!/bin/bash
# openCenter-compliance: Check cluster compliance

config=$(openCenter config view --format json)

# Check for required security settings
if ! echo "$config" | jq -e '.opencenter.cluster.networking.security.os_hardening == true' > /dev/null; then
    echo "ERROR: OS hardening must be enabled for compliance"
    exit 1
fi

echo "Compliance check passed"
```

## Extending the plugin system

Future enhancements being considered:

### Plugin API

A formal Go API for plugins that need tighter integration:

```go
package main

import "github.com/rackerlabs/openCenter-cli/pkg/plugin"

func main() {
    plugin.Register(&plugin.Definition{
        Name: "backup",
        Commands: []plugin.Command{
            {
                Use: "create",
                Run: createBackup,
            },
        },
    })
}
```

This would allow plugins to use openCenter's libraries (configuration, validation, etc.) while still being separate executables.

### Plugin marketplace

A registry of community plugins with installation support:

```bash
openCenter plugin install backup
# Fetches from registry, verifies signature, installs to config directory
```

### Plugin versioning

Version constraints for plugin compatibility:

```yaml
# plugin.yaml
name: backup
version: 1.0.0
requires:
  openCenter: ">=1.0.0 <2.0.0"
```

## Trade-offs and limitations

### Process overhead

Spawning a process for each plugin invocation has overhead. For plugins that run frequently, this could be noticeable.

**Mitigation**: Cache plugin results, batch operations, or use the planned plugin API for tighter integration.

### No shared state

Plugins can't share state with openCenter or other plugins except through the filesystem or CLI.

**Mitigation**: Use the CLI interface for state access. For complex state sharing, consider contributing to openCenter core instead of using a plugin.

### Discovery limitations

Plugins must be in specific directories or PATH. There's no automatic discovery from arbitrary locations.

**Mitigation**: Use `OPENCENTER_PLUGINS_DIR` for custom locations. For team-wide plugins, install to a shared directory and add to PATH.

## Comparison with other plugin systems

### Kubectl plugins

Kubectl uses a similar executable-based plugin system. openCenter's design is inspired by kubectl but adds:

- Service plugins for internal extensibility
- Dependency resolution for service plugins
- Lifecycle hooks for complex operations

### Helm plugins

Helm plugins are also executables but use a manifest file for metadata. openCenter's command plugins don't require manifests (simpler) but service plugins do (more structured).

### Terraform providers

Terraform providers are separate binaries that communicate via gRPC. This provides tighter integration but requires more complex plugin development. openCenter's simpler approach is appropriate for its use case.

## See also

- **[Developer Guide](../dev/readme.md)**: How to develop openCenter plugins
- **[Service Configuration](../reference/services.md)**: Available services and their configuration
- **[CLI Commands](../reference/cli-commands.md)**: Built-in commands that plugins can extend
- **[Architecture](./architecture.md)**: How plugins fit into the overall system
