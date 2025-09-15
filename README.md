# openCenter

**openCenter** is a command-line tool that streamlines cluster bootstrapping by turning a single, declarative configuration file into a ready-to-use GitOps repository.

It is designed to standardize how teams scaffold and validate Kubernetes and OpenStack-based environments, providing a configuration-first workflow that is both user-friendly and automation-ready.

---

## Key Features

*   **Configuration-First**: A single, comprehensive YAML file serves as the source of truth for your entire cluster definition.
*   **GitOps by Default**: Generates a complete, version-controlled GitOps repository from embedded templates, ready to be used with tools like FluxCD or ArgoCD.
*   **Built-in Validation**: Catches common configuration errors and logical inconsistencies before you ever apply them.
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

All our documentation lives in the `docs/` directory.

*   [**Tutorials**](./docs/tutorials/quickstart.md): Step-by-step guides to learn `openCenter`.
*   [**How-To Guides**](./docs/how-to/): Practical, goal-oriented recipes for common tasks.
*   [**Reference**](./docs/reference/): Detailed, technical information on the CLI, configuration, and developer tasks.
*   [**Explanation**](./docs/explanation/): Conceptual overviews of `openCenter`'s architecture and design.
*   Plugins:
    - [Using Plugins](./docs/how-to/plugins.md)
    - [Plugins Reference](./docs/reference/plugins.md)
    - [Environment Variables](./docs/reference/environment.md)

## Contributing

We welcome contributions! Please see our [**Contributing Guide**](./docs/contributing.md) to get started.

## License

This project is licensed under the Apache 2.0 License.
