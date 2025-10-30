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
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/spf13/cobra"
)

const kindClusterConfig = `kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  podSubnet: "10.244.0.0/16"
  serviceSubnet: "10.96.0.0/12"
nodes:
- role: control-plane
- role: worker
- role: worker
- role: worker
`

func newClusterBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap [name]",
		Short: "Run provider-specific bootstrap actions for a cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active selection.
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
			if strings.TrimSpace(name) == "" {
				return fmt.Errorf("no active cluster; specify name or use 'select' to set it")
			}

			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			kubeconf, _ := cmd.Flags().GetString("kubeconfig")
			logPath, _ := cmd.Flags().GetString("log")
			runtimeFlag, _ := cmd.Flags().GetString("container-runtime")

			clusterDir := ""
			gitDir := strings.TrimSpace(cfg.GitOps().GitDir)
			if gitDir != "" {
				clusterDir = filepath.Join(gitDir, "infrastructure", "clusters", cfg.ClusterName())
			}
			if logPath == "" && clusterDir != "" {
				logPath = filepath.Join(clusterDir, "bootstrap.log")
			}

			runner, err := newBootstrapRunner(cmd, cfg.ClusterName(), clusterDir, logPath, dryRun)
			if err != nil {
				return err
			}
			defer runner.Close()

			provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
			if provider == "" {
				provider = "openstack"
			}

			switch provider {
			case "openstack", "aws", "gcp", "azure":
				if clusterDir == "" {
					return fmt.Errorf("gitops.git_dir must be configured for provider %q", provider)
				}
				if _, err := os.Stat(clusterDir); err != nil {
					return fmt.Errorf("cluster directory not found: %s", clusterDir)
				}
				env := map[string]string{}
				if kubeconf != "" {
					env["KUBECONFIG"] = kubeconf
				}
				runner.Infof("Running make in %s", clusterDir)
				if err := runner.Run(clusterDir, env, "make"); err != nil {
					return err
				}
			case "kind":
				runtime := resolveContainerRuntime(runtimeFlag)
				env := map[string]string{}
				switch runtime {
				case "podman":
					env["KIND_EXPERIMENTAL_PROVIDER"] = "podman"
				case "docker":
					// default, no extra env
				default:
					return fmt.Errorf("unsupported container runtime %q", runtime)
				}

				runner.Infof("Creating kind cluster %q using %s", cfg.ClusterName(), runtime)
				if err := runner.RunWithInput("", env, kindClusterConfig, "kind", "create", "cluster", "--name", cfg.ClusterName(), "--config=-"); err != nil {
					return err
				}
				if err := runner.Run("", env, "kind", "export", "kubeconfig", "--name", cfg.ClusterName()); err != nil {
					return err
				}
			default:
				return fmt.Errorf("unsupported provider %q", cfg.OpenCenter.Infrastructure.Provider)
			}

			runner.Infof("Bootstrap complete.")
			if logPath != "" && !dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Log written to %s\n", logPath)
			}
			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "show planned actions without executing")
	cmd.Flags().String("kubeconfig", "./kubeconfig.yaml", "path to kubeconfig used by bootstrap actions")
	cmd.Flags().String("log", "", "log file path (defaults to <git_dir>/infrastructure/clusters/<name>/bootstrap.log)")
	cmd.Flags().String("container-runtime", "", "container runtime for kind clusters (docker or podman)")

	return cmd
}

func resolveContainerRuntime(flagValue string) string {
	if v := strings.TrimSpace(flagValue); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("CONTAINER_RUNTIME")); v != "" {
		return strings.ToLower(v)
	}
	if v := strings.TrimSpace(os.Getenv("KIND_EXPERIMENTAL_PROVIDER")); v != "" {
		return strings.ToLower(v)
	}
	return "docker"
}

type bootstrapRunner struct {
	dryRun  bool
	logFile *os.File
	stdout  io.Writer
	stderr  io.Writer
}

func newBootstrapRunner(cmd *cobra.Command, clusterName, clusterDir, logPath string, dryRun bool) (*bootstrapRunner, error) {
	var f *os.File
	if logPath != "" && !dryRun {
		if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		header := fmt.Sprintf(`# openCenter bootstrap log
# time: %s
# cluster: %s
# dir: %s

`, time.Now().Format(time.RFC3339), clusterName, clusterDir)
		if _, err := file.WriteString(header); err != nil {
			file.Close()
			return nil, fmt.Errorf("failed to write log header: %w", err)
		}
		f = file
	}

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()
	if f != nil {
		out = io.MultiWriter(out, f)
		errOut = io.MultiWriter(errOut, f)
	}

	return &bootstrapRunner{
		dryRun:  dryRun,
		logFile: f,
		stdout:  out,
		stderr:  errOut,
	}, nil
}

func (r *bootstrapRunner) Close() {
	if r.logFile != nil {
		_ = r.logFile.Close()
	}
}

func (r *bootstrapRunner) Infof(format string, args ...interface{}) {
	fmt.Fprintf(r.stdout, format+"\n", args...)
}

func (r *bootstrapRunner) Run(dir string, env map[string]string, name string, args ...string) error {
	return r.execute(dir, env, nil, name, args...)
}

func (r *bootstrapRunner) RunWithInput(dir string, env map[string]string, input string, name string, args ...string) error {
	return r.execute(dir, env, strings.NewReader(input), name, args...)
}

func (r *bootstrapRunner) execute(dir string, env map[string]string, stdin io.Reader, name string, args ...string) error {
	printable := formatCommand(env, name, args)
	fmt.Fprintf(r.stdout, "$ %s\n", printable)

	if r.dryRun {
		return nil
	}

	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	if stdin != nil {
		cmd.Stdin = stdin
	}

	envList := os.Environ()
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("command failed: %s: %w", printable, err)
	}
	return nil
}

func formatCommand(env map[string]string, name string, args []string) string {
	var prefixes []string
	if len(env) > 0 {
		keys := make([]string, 0, len(env))
		for k := range env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			prefixes = append(prefixes, fmt.Sprintf("%s=%s", k, env[k]))
		}
	}
	parts := append(prefixes, append([]string{name}, args...)...)
	return strings.Join(parts, " ")
}
