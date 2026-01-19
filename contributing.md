# Contributing to openCenter

Thank you for your interest in contributing to `openCenter`! We welcome contributions from everyone. This document provides guidelines and instructions to help you get started.

## Getting Started

Before you can contribute, you'll need to set up your local development environment.

### Prerequisites

*   [**Mise**](https://mise.jdx.dev/): For managing project-specific tool versions.
*   **Go**: The correct version is managed by Mise.
*   **Git**: For version control.

### Initial Setup

1.  **Fork and Clone the Repository**:
    ```bash
    git clone git@github.com:your-username/openCenter.git
    cd openCenter
    ```

2.  **Install Tools**: Use Mise to install the correct versions of all development tools.
    ```bash
    mise install
    ```

3.  **Build the CLI**: Compile the `openCenter` binary.
    ```bash
    mise run build
    ```

    You should now have a working `openCenter` executable in your project root.

## Development Workflow

1.  **Make Your Changes**: Implement your bug fix or new feature.
2.  **Run Tests**: Before submitting your changes, ensure that all BDD tests pass. This is a critical step to prevent regressions.
    ```bash
    mise run bdd
    ```
    For more details, see the [How-To Guide on Running Tests](./how-to/run-tests.md).
3.  **Submit a Pull Request**: Push your changes to your fork and open a pull request against the `main` branch of the upstream repository.

## Extensibility Points

`openCenter` is designed to be extensible. Here are the most common ways to add new functionality:

### Adding New Config Keys

1.  **Extend the Structs**: Add your new field to the appropriate struct in `internal/config/config.go`.
2.  **Update the Schema**: Add the corresponding entry for your new field in `internal/config/schema.go`.
3.  **Update the Default Config**: If your field should have a default value, add it to the `defaultConfig` function in `internal/config/config.go`.
4.  Ensure any new fields have sensible defaults and schema entries; `cluster init` is non-interactive and relies on defaults and explicit flags.

### Adding New Cloud Providers

1.  Create a new directory under `internal/cloud/<provider>`.
2.  Inside that directory, create a `preflight.go` file that implements the `Preflight(map[string]any) []string` function. This function should contain all the preflight checks specific to your new provider.
3.  Add a branch in the `cluster preflight` command logic to call your new preflight function when `cloud.provider` matches your new provider's name.

### Adding New Templates

1.  Add your new template files or directories under `internal/gitops/gitops-base-dir/`.
2.  If your file needs to be processed by the templating engine, give it a `.tmpl` extension. You can use any values from the `Config` struct in your template (e.g., `{{ .ClusterName }}`).
3.  If your file should be copied verbatim, do not give it a `.tmpl` extension.
4.  The `embed.FS` in `internal/gitops/embed.go` will automatically pick up your new files.

### Adding New Commands

1.  Create a new file in the `cmd/` directory (e.g., `cmd/cluster_mynewcommand.go`).
2.  In that file, create a `newClusterMyNewCommandCmd()` function that returns a `*cobra.Command`.
3.  Register your new command in `cmd/cluster.go` by adding `cmd.AddCommand(newClusterMyNewCommandCmd())` to the `newClusterCmd` function.

---

## Assumptions & Open Questions

This documentation was written based on a thorough analysis of the repository's source code, tests, and existing documentation. In the process, the following points were noted:

*   **`.mise.toml` File**: The original `README.md` and the project's workflow rely heavily on tasks defined in a `.mise.toml` file. However, this file was not visible in the initial recursive file listing. The documentation for developer tasks has been written based on the descriptions in the `README.md`. If the file exists and contains different or additional tasks, the `docs/reference/tasks.md` file should be updated.
