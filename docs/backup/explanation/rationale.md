# Explanation: Design Rationale

This document explains the reasoning behind some of the key design decisions made in `openCenter`. Understanding the "why" can provide context for how the tool works and how it is intended to be used.

## Why a Single Declarative YAML?

The core of `openCenter` revolves around a single, comprehensive YAML file for each cluster. This approach was chosen for several reasons:

*   **Simplicity**: It provides a single source of truth. Instead of managing dozens of separate configuration files or scripts, all aspects of the cluster's definition are in one place.
*   **Version Control**: A single file is easy to store in Git, providing a full, auditable history of every change made to the cluster's intended state.
*   **Clarity**: The nested, grouped structure of the YAML (e.g., `gitops`, `kubernetes`, `cloud`) makes the configuration human-readable and easier to understand at a glance.
*   **Machine-Readability**: YAML is easily parsed by both the `openCenter` CLI and other ecosystem tools. The ability to generate a JSON Schema from the configuration further enhances this.

## Why GitOps by Default?

`openCenter` does not directly apply changes to a live Kubernetes or cloud environment. Instead, its primary output is a **GitOps repository**. This is a deliberate design choice.

*   **Decoupling**: It decouples the act of *defining* a cluster from the act of *reconciling* it. `openCenter` is the scaffolding tool; a GitOps controller like FluxCD or ArgoCD is the reconciliation tool.
*   **Best Practices**: This model promotes modern GitOps best practices, where Git is the source of truth for the cluster's runtime state.
*   **Security and Control**: Changes are made via `git push`, which can be subjected to standard development workflows like pull requests, reviews, and branch protection rules. This provides a powerful control and audit layer.
*   **Disaster Recovery**: Because the entire desired state of the cluster is in Git, recreating a cluster becomes a more predictable and automated process.

## Why a Custom CLI?

While the process could be managed with a collection of scripts, a dedicated CLI was built to provide a more robust and user-friendly experience.

*   **Guided Workflows**: The CLI provides interactive prompts where appropriate (e.g., `cluster select`), while core flows like `cluster init` are non-interactive for repeatability.
*   **Validation and Guardrails**: The CLI enforces validation rules (`cluster validate`) and runs preflight checks (`cluster preflight`), catching common errors before they can cause problems.
*   **Scriptability**: For CI/CD pipelines and automation, the CLI provides non-interactive modes, flags, and JSON output, making it easy to integrate into automated workflows.
*   **Consistency**: It provides a single, consistent interface for all interactions with the system.

## Why BDD for Testing?

The project uses Behavior-Driven Development (BDD) with Godog for its primary test suite.

*   **Living Documentation**: The `.feature` files in `tests/features/` are written in plain English. They describe exactly how the CLI should behave in various scenarios, making them an invaluable source of documentation for developers and even non-technical stakeholders.
*   **Executable Specifications**: These tests are not just documentation; they are executable. This ensures that the documentation never goes out of date with the code's actual behavior.
*   **Clear Acceptance Criteria**: BDD provides a clear set of acceptance criteria for any new feature or change, improving the development process.
*   **Agent-Friendly**: As noted in the original `README.md`, the clear, structured nature of BDD tests makes them consumable by LLM agents, enabling new possibilities for automated testing and validation.

### Sources
*   `README.md`
*   `internal/`
*   `tests/features/`
