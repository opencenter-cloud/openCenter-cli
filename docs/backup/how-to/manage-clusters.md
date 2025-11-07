# How-To: Manage Cluster Configurations

Once you have one or more cluster configurations, `openCenter` provides a set of commands to easily list, view, and switch between them. This guide covers the essential day-to-day commands for managing your cluster configurations.

## Who is this for?

*   **SREs/Operators** who need to inspect or switch between different cluster contexts.
*   **Platform engineers** managing multiple cluster definitions.

## What you'll achieve

*   List all available cluster configurations.
*   View the detailed configuration of a specific cluster.
*   Set a cluster as the "active" context for other commands.
*   Check which cluster is currently active.
*   Integrate `openCenter`'s context into your shell prompt.

---

### Listing Available Clusters

The `cluster list` command (aliased as `ls`) scans your configuration directory (`~/.config/openCenter` by default) and prints the names of all available cluster configurations.

```bash
# List all clusters
./openCenter cluster list

# Example Output:
# dev
# prod
# staging
```

For scripting or automation, you can get the output in JSON format:

```bash
./openCenter cluster ls --json

# Example Output:
# [
#   "dev",
#   "prod",
#   "staging"
# ]
```

### Viewing Cluster Details

To see the full configuration for a cluster, use the `cluster info` command.

If you provide a name, it will show the configuration for that specific cluster. If you don't provide a name, it will show the configuration for the currently **active** cluster.

```bash
# View details for a specific cluster
./openCenter cluster info dev

# Example Output:
# cluster_name: dev
# gitops:
#   git_dir: /tmp/repo-dev
#   git_url: git@github.com:example/dev.git
# ...
```

Like the `list` command, `info` also supports JSON output, which is useful for piping the configuration to other tools like `jq`.

```bash
# Get the full configuration as a JSON object
./openCenter cluster info dev --json | jq .iac.networking
```

### Selecting an Active Cluster

Many `openCenter` commands (like `setup` and `bootstrap`) operate on an "active" cluster. This avoids you having to specify the cluster name every time.

Use the `cluster select` command to set the active context:

```bash
./openCenter cluster select dev
# Output: Selected cluster: dev
```

If you run `cluster select` without a name, it will present an interactive menu where you can choose from the available clusters.

To check which cluster is currently active, use `cluster current`:

```bash
./openCenter cluster current
# Output: dev
```

### Integrating with Your Shell Prompt

`openCenter` provides a `prompt` command specifically designed to be integrated into your shell's prompt (e.g., `PS1` in Bash/Zsh).

This command is context-aware:
*   If your current working directory is the `git_dir` of the **active cluster**, it prints the active cluster's name (e.g., `(openCenter:dev)`).
*   Otherwise, it prints nothing.

This provides a subtle, helpful indicator of which cluster context you are currently working in.

**Example for Bash/Zsh:**

You can add it to your `~/.bashrc` or `~/.zshrc` file.

```bash
# Add this to your shell profile
# The space after the prompt output is intentional for proper spacing.
export PS1="\$(./path/to/openCenter prompt) $PS1"
```

After reloading your shell, your prompt will look something like this when you `cd` into an active cluster's repository:

```text
(openCenter:dev) ~/your/project/path $
```
