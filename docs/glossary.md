# Glossary

This glossary defines key terms used throughout the `openCenter` project and its documentation.

---

### Active Cluster

The cluster configuration that `openCenter` commands will operate on by default. The active cluster is set using the `openCenter cluster select` command and its name is stored in a file named `.active` within the Configuration Directory.

---

### Bootstrap

The process of pushing the local, generated GitOps Repository to its configured remote Git URL. This is the final step in the scaffolding process and is performed by the `openCenter cluster bootstrap` command.

---

### Cluster Configuration

The YAML file (e.g., `my-cluster.yaml`) that serves as the single, declarative source of truth for a cluster. It contains all the settings related to the cluster's infrastructure, Kubernetes layout, networking, and cloud provider details.

---

### Configuration Directory

The directory on the local filesystem where `openCenter` stores all Cluster Configuration files and the `.active` cluster marker. By default, this is `~/.config/openCenter`, but it can be overridden with the `--config-dir` flag or the `OPENCENTER_CONFIG_DIR` environment variable.

---

### GitOps Repository

The local Git repository that `openCenter` generates from its embedded templates. This repository contains all the manifests and infrastructure-as-code files needed to deploy the cluster. Its location is defined by the `gitops.git_dir` field in the Cluster Configuration.

---

### Materialization

The process of creating the GitOps Repository by copying and rendering the embedded templates. This is performed by the `openCenter cluster setup` command.

---

### Preflight Checks

A series of checks performed by the `openCenter cluster preflight` command to ensure the local environment is correctly configured before attempting to set up or bootstrap a cluster. These checks can include verifying that required tools (like `git` or `kubectl`) are installed and that necessary cloud provider credentials are set.

---

### Validation

The process of checking a Cluster Configuration file for logical errors, missing required fields, or incompatible settings. This is performed by the `openCenter cluster validate` command and helps prevent common misconfigurations.
