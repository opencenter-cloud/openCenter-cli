# openCenter

**openCenter** is a command-line tool that streamlines cluster bootstrapping by turning a single, declarative configuration file into a ready-to-use GitOps repository.

It is designed to standardize how teams scaffold and validate Kubernetes and OpenStack-based environments, providing a configuration-first workflow that is both user-friendly and automation-ready.

---

## Key Features

*   **Configuration-First**: A single, comprehensive YAML file serves as the source of truth for your entire cluster definition.
*   **GitOps by Default**: Generates a complete, version-controlled GitOps repository from embedded templates, ready to be used with tools like FluxCD or ArgoCD.
*   **Built-in Validation**: Catches common configuration errors and logical inconsistencies before you ever apply them.
*   **Secrets Management**: Dedicated secrets section with SOPS integration for secure credential management.
*   **Template-Driven**: All templates use configuration references - no hardcoded values.
*   **Interactive & Scriptable**: Offers an interactive mode to guide new users and a powerful flag-based interface for CI/CD automation.
*   **Extensible**: Designed to be extended with new commands, templates, and cloud provider logic.
*   **Test-Driven**: A full suite of Behavior-Driven Development (BDD) tests ensures reliability and serves as living documentation.

## Quickstart

The fastest way to get started is to follow our end-to-end tutorial. You will need [Mise](https://mise.jdx.dev/) and [Git](https://git-scm.com/) installed.

1.  **Install project tools**:
    ```bash
    mise install
    ```
2.  **Build the binary**:
    ```bash
    mise run build
    ```
3.  **Follow the full tutorial**:
    For a complete walkthrough from initializing your first cluster to bootstrapping the GitOps repository, please see our [**Quickstart Tutorial**](./docs/tutorials/quickstart.md).

## Documentation

Comprehensive documentation is available in the `docs/` directory, organized following the [Diátaxis framework](https://diataxis.fr/).

### Quick Links
*   **[Documentation Home](./docs/readme.md)** - Complete documentation guide
*   **[Overview](./docs/overview.md)** - What is openCenter?
*   **[Current Status](./docs/current-status.md)** - Implementation status and roadmap
*   **[Architecture](./docs/architecture.md)** - Technical architecture and design
*   **[Troubleshooting](./docs/troubleshooting.md)** - Common issues and solutions

### Documentation Categories

#### Getting Started
*   **[Overview](./docs/overview.md)** - Introduction to openCenter
*   **[IDE Integration](./docs/ide-integration.md)** - Setup your IDE for openCenter
*   **[Adding Services](./docs/adding-services.md)** - Add services to your cluster

#### Reference - Information-Oriented
*   **[CLI Reference](./docs/reference/readme.md)** - Complete CLI command reference
*   **[Configuration Reference](./docs/reference/configuration.md)** - Configuration file structure
*   **[CLI Commands Overview](./docs/reference/cli-commands.md)** - Command overview

##### Cluster Commands Reference
Complete documentation for all cluster lifecycle commands:
*   **[Cluster Commands Overview](./docs/reference/cluster/readme.md)** - Cluster management guide
*   **Lifecycle Commands**
    - [init](./docs/reference/cluster/init.md) - Initialize new cluster configuration
    - [validate](./docs/reference/cluster/validate.md) - Validate cluster configuration
    - [preflight](./docs/reference/cluster/preflight.md) - Run preflight checks
    - [setup](./docs/reference/cluster/setup.md) - Setup GitOps repository
    - [render](./docs/reference/cluster/render.md) - Render templates
    - [bootstrap](./docs/reference/cluster/bootstrap.md) - Bootstrap cluster
    - [destroy](./docs/reference/cluster/destroy.md) - Destroy cluster
*   **Management Commands**
    - [list](./docs/reference/cluster/list.md) - List all clusters
    - [select](./docs/reference/cluster/select.md) - Select active cluster
    - [current](./docs/reference/cluster/current.md) - Show current cluster
    - [info](./docs/reference/cluster/info.md) - Show cluster information
    - [edit](./docs/reference/cluster/edit.md) - Edit cluster configuration
*   **Update Commands**
    - [update](./docs/reference/cluster/update.md) - Update configuration fields
    - [config-update](./docs/reference/cluster/config-update.md) - Update with defaults
    - [migrate](./docs/reference/cluster/migrate.md) - Migrate to organization structure
*   **Utility Commands**
    - [schema](./docs/reference/cluster/schema.md) - Export JSON schema

#### Developer Documentation
*   **[Developer Guide](./docs/dev/readme.md)** - CLI architecture and implementation
*   **[Cluster Commands Dev](./docs/dev/cluster/readme.md)** - Cluster command internals

### Configuration Resources
*   **[Cluster Configuration File](./docs/cluster-config.md)** - Complete cluster config reference
*   **[Configuration Schema](./CONFIG_SCHEMA_ADDITIONS.md)** - Complete field reference
*   **[Template Analysis](./TEMPLATE_ANALYSIS_REPORT.md)** - Template implementation status
*   **[Migration Guide](./MIGRATION_GUIDE.md)** - Upgrade from older configurations

## CLI Commands Quick Reference

### Cluster Management
```bash
openCenter cluster init <name>        # Initialize new cluster
openCenter cluster list                # List all clusters
openCenter cluster select <name>       # Select active cluster
openCenter cluster validate <name>     # Validate configuration
openCenter cluster setup <name>        # Setup GitOps repository
openCenter cluster bootstrap <name>    # Bootstrap cluster
```

### Configuration Management
```bash
openCenter config view                 # View current configuration
openCenter config set <key> <value>    # Set configuration value
openCenter config get <key>            # Get configuration value
```

### Secrets Management
```bash
openCenter sops generate-key           # Generate Age key pair
openCenter sops validate               # Validate SOPS configuration
openCenter sops secrets-encrypt        # Encrypt secrets
```

### Other Commands
```bash
openCenter version                     # Show version information
openCenter plugins list                # List available plugins
openCenter --help                      # Show help
```

For complete command documentation, see the [CLI Reference](./docs/reference/readme.md).

## Contributing

We welcome contributions! Please see our [**Contributing Guide**](./docs/contributing.md) to get started.

## License

This project is licensed under the Apache 2.0 License.
