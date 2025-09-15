// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
    "bytes"
    "fmt"
    "io"
    "os"
    "os/exec"
    "path/filepath"
    "strings"
    "time"

    "github.com/rackerlabs/openCenter/internal/config"
    "github.com/spf13/cobra"
)

func newClusterBootstrapCmd() *cobra.Command {
    cmd := &cobra.Command{
        Use:   "bootstrap [name]",
        Short: "Provision and configure the cluster (terraform, kubectl, helm, ansible)",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Resolve cluster name
            var name string
            var err error
            if len(args) > 0 {
                name = args[0]
            } else {
                name, err = config.GetActive()
                if err != nil {
                    return err
                }
            }
            if name == "" {
                return fmt.Errorf("no active cluster; specify name or use 'select' to set it")
            }
            cfg, err := config.Load(name)
            if err != nil {
                return err
            }

            // Flags
            dryRun, _ := cmd.Flags().GetBool("dry-run")
            kubeconf, _ := cmd.Flags().GetString("kubeconfig")
            logPath, _ := cmd.Flags().GetString("log")
            if logPath == "" {
                logPath = filepath.Join(cfg.GitOps.GitDir, "infrastructure", "clusters", cfg.ClusterName, "bootstrap.log")
            }

            clusterDir := filepath.Join(cfg.GitOps.GitDir, "infrastructure", "clusters", cfg.ClusterName)
            if _, statErr := os.Stat(clusterDir); statErr != nil {
                return fmt.Errorf("cluster directory not found: %s", clusterDir)
            }

            // Open log file
            var logFile *os.File
            if !dryRun {
                if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
                    return fmt.Errorf("failed to create log directory: %w", err)
                }
                f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
                if err != nil {
                    return fmt.Errorf("failed to open log file: %w", err)
                }
                logFile = f
                defer logFile.Close()
                // Write header
                fmt.Fprintf(logFile, "# openCenter bootstrap log\n# time: %s\n# cluster: %s\n# dir: %s\n\n", time.Now().Format(time.RFC3339), cfg.ClusterName, clusterDir)
            }

            // Helper to run a command with logging and optional env
            run := func(dir string, env map[string]string, name string, args ...string) error {
                // Print the command
                printable := name + " " + strings.Join(args, " ")
                if len(env) > 0 {
                    kv := make([]string, 0, len(env))
                    for k, v := range env {
                        kv = append(kv, fmt.Sprintf("%s=%s", k, v))
                    }
                    printable = strings.Join(kv, " ") + " " + printable
                }
                fmt.Fprintln(cmd.OutOrStdout(), "$ ", printable)
                if logFile != nil {
                    fmt.Fprintln(logFile, "$ ", printable)
                }
                if dryRun {
                    return nil
                }
                c := exec.Command(name, args...)
                c.Dir = dir
                // Build environment
                envList := os.Environ()
                for k, v := range env {
                    envList = append(envList, fmt.Sprintf("%s=%s", k, v))
                }
                c.Env = envList
                // Wire outputs to both stdout and log file
                var outW io.Writer = cmd.OutOrStdout()
                var errW io.Writer = cmd.ErrOrStderr()
                if logFile != nil {
                    outW = io.MultiWriter(cmd.OutOrStdout(), logFile)
                    errW = io.MultiWriter(cmd.ErrOrStderr(), logFile)
                }
                c.Stdout = outW
                c.Stderr = errW
                if err := c.Run(); err != nil {
                    return fmt.Errorf("command failed: %s: %w", printable, err)
                }
                return nil
            }

            // Build override values path using the current cluster name
            overrideValues := filepath.Join(cfg.GitOps.GitDir, "applications", "overlays", cfg.ClusterName, "services", "calico", "helm-values", "override_values.yaml")
            // But the original relative path in request used ../../../ from clusterDir, compute that too for logs
            relOverride, _ := filepath.Rel(clusterDir, overrideValues)
            if relOverride == "" {
                relOverride = "../../../applications/overlays/" + cfg.ClusterName + "/services/calico/helm-values/override_values.yaml"
            }

            // Step 1: cd to clusterDir (implicit by setting dir in run())
            // Step 2: terraform init
            if err := run(clusterDir, nil, "terraform", "init"); err != nil {
                return err
            }
            // Step 3: terraform apply (non-interactive)
            if err := run(clusterDir, nil, "terraform", "apply", "-auto-approve"); err != nil {
                return err
            }
            // Step 4: kubectl get nodes with KUBECONFIG
            if kubeconf == "" {
                kubeconf = "./kubeconfig.yaml"
            }
            if err := run(clusterDir, map[string]string{"KUBECONFIG": kubeconf}, "kubectl", "get", "nodes"); err != nil {
                return err
            }
            // Step 5: helm repo add
            if err := run(clusterDir, nil, "helm", "repo", "add", "projectcalico", "https://docs.tigera.io/calico/charts"); err != nil {
                return err
            }
            // Step 6: helm upgrade --install calico
            if err := run(clusterDir, nil, "helm", "upgrade", "--install", "calico", "projectcalico/tigera-operator",
                "--namespace", "tigera-operator", "-f", relOverride, "--create-namespace"); err != nil {
                return err
            }
            // Step 7: terraform apply again
            if err := run(clusterDir, nil, "terraform", "apply", "-auto-approve"); err != nil {
                return err
            }
            // Step 8: export ANSIBLE_INVENTORY env for subsequent playbook
            inventory := filepath.Join(clusterDir, "inventory", "inventory.yaml")
            // Step 9: ansible-playbook using venv if present, from kubespray subdir
            venvBin := filepath.Join(clusterDir, "venv", "bin")
            envMap := map[string]string{"ANSIBLE_INVENTORY": inventory}
            // Prepend venv bin to PATH if it exists
            if st, err := os.Stat(venvBin); err == nil && st.IsDir() {
                envMap["PATH"] = venvBin + string(os.PathListSeparator) + os.Getenv("PATH")
            }
            ksDir := filepath.Join(clusterDir, "kubespray")
            if err := run(ksDir, envMap, "ansible-playbook", "-f", "10", "-b", "upgrade-cluster.yml", "-e", "@../inventory/k8s_hardening.yml"); err != nil {
                return err
            }

            fmt.Fprintln(cmd.OutOrStdout(), "Bootstrap complete.")
            if logPath != "" {
                fmt.Fprintf(cmd.OutOrStdout(), "Log written to %s\n", logPath)
            }
            return nil
        },
    }
    cmd.Flags().Bool("dry-run", false, "show planned actions without executing")
    cmd.Flags().String("kubeconfig", "./kubeconfig.yaml", "path to kubeconfig for kubectl commands (relative to cluster dir)")
    cmd.Flags().String("log", "", "log file path (defaults to <git_dir>/infrastructure/clusters/<name>/bootstrap.log)")
    return cmd
}

// hasOrigin returns true if the git repository at `dir` has a remote
// named `origin`.
func hasOrigin(dir string) (bool, error) {
	remotes := exec.Command("git", "remote")
	remotes.Dir = dir
	out, err := remotes.Output()
	if err != nil {
		return false, err
	}
	//
	for _, line := range bytes.Split(out, []byte("\n")) {
		if string(line) == "origin" {
			return true, nil
		}
	}
	return false, nil
}
