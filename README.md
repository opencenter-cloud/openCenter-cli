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

The fastest way to get started is to follow our getting started guide. You will need [Mise](https://mise.jdx.dev/) and [Git](https://git-scm.com/) installed.

1.  **Install project tools**:
    ```bash
    mise install
    ```
2.  **Build the binary**:
    ```bash
    mise run build
    ```
3.  **Follow the getting started guide**:
    For a complete walkthrough from installing openCenter to creating your first cluster, see our [**Getting Started Guide**](./docs/getting-started.md).

## Documentation

Comprehensive documentation is available in the `docs/` directory, organized following the [Diátaxis framework](https://diataxis.fr/).

### Quick Start
*   **[Getting Started](./docs/getting-started.md)** - Your first cluster in 10 minutes
*   **[Documentation Home](./docs/readme.md)** - Complete documentation guide

### Documentation by Type

#### 📚 Tutorials (Learning-Oriented)
Step-by-step guides to help you learn openCenter:
*   **[Getting Started](./docs/getting-started.md)** - Create your first cluster
*   **[OpenStack Quickstart](./docs/tutorials/quickstart-openstack.md)** - Deploy on OpenStack
*   **[AWS Quickstart](./docs/tutorials/quickstart-aws.md)** - Deploy on AWS
*   **[Kind Quickstart](./docs/tutorials/quickstart-kind.md)** - Local development with Kind

#### 🔧 How-To Guides (Task-Oriented)
Practical guides for specific tasks:
*   **[Troubleshooting](./docs/how-to/troubleshooting.md)** - Common issues and solutions
*   **[Adding Services](./docs/how-to/adding-services.md)** - Add services to your cluster
*   **[Managing Secrets](./docs/how-to/secrets.md)** - SOPS and secrets management
*   **[IDE Integration](./docs/how-to/ide-integration.md)** - Setup your development environment

#### 📖 Reference (Information-Oriented)
Technical specifications and detailed information:
*   **[CLI Commands](./docs/reference/cli-commands.md)** - Complete command reference
*   **[Configuration](./docs/reference/configuration.md)** - Configuration file structure
*   **[Cluster Commands](./docs/reference/cluster/readme.md)** - Cluster lifecycle commands
*   **[Shell Integration](./docs/reference/shell-integration.md)** - Shell completion and integration

#### 💡 Explanation (Understanding-Oriented)
Conceptual explanations and background:
*   **[Overview](./docs/explanation/overview.md)** - What is openCenter?
*   **[Architecture](./docs/explanation/architecture.md)** - Technical architecture and design
*   **[Current Status](./docs/explanation/current-status.md)** - Implementation status and roadmap

### For Developers
*   **[Developer Guide](./docs/dev/readme.md)** - CLI architecture and implementation
*   **[Contributing](./docs/contributing.md)** - Contribution guidelines

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
