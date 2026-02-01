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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/resilience"
	"github.com/rackerlabs/opencenter-cli/internal/security"
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

const (
	bootstrapStateVersion  = 1
	bootstrapStatusFailed  = "failed"
	bootstrapStatusRunning = "running"
	bootstrapStatusSkipped = "skipped"
	bootstrapStatusSuccess = "success"
)

type bootstrapStep struct {
	ID          string
	Description string
	Run         func() error
}

type bootstrapState struct {
	Version int                           `json:"version"`
	Steps   map[string]bootstrapStepState `json:"steps"`
}

type bootstrapStepState struct {
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	Error     string `json:"error,omitempty"`
}

func newClusterBootstrapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bootstrap [name]",
		Short: "Run provider-specific bootstrap actions for a cluster",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// Resolve cluster name from args or active cluster
			name, err := resolveClusterName(args, true)
			if err != nil {
				return err
			}

			// Acquire lock for bootstrap operation
			lockMgr, err := resilience.NewLockManager(resilience.DefaultLockConfig)
			if err != nil {
				return fmt.Errorf("failed to create lock manager: %w", err)
			}

			ctx := context.Background()
			lock, err := lockMgr.AcquireWithMetadata(ctx, name, 1*time.Hour, map[string]string{
				"operation": "bootstrap",
				"command":   "cluster bootstrap",
			})
			if err != nil {
				return fmt.Errorf("failed to acquire lock for cluster %q: %w\nAnother operation may be in progress. Wait for it to complete or use 'opencenter cluster info %s' to check lock status", name, err, name)
			}
			defer lockMgr.Release(lock)

			cfg, err := config.Load(name)
			if err != nil {
				return err
			}

			dryRun, _ := cmd.Flags().GetBool("dry-run")
			kubeconf, _ := cmd.Flags().GetString("kubeconfig")
			logPath, _ := cmd.Flags().GetString("log")
			runtimeFlag, _ := cmd.Flags().GetString("container-runtime")
			restart, _ := cmd.Flags().GetBool("restart")
			onlyStep, _ := cmd.Flags().GetString("step")
			fromStep, _ := cmd.Flags().GetString("from-step")

			if strings.TrimSpace(onlyStep) != "" && strings.TrimSpace(fromStep) != "" {
				return fmt.Errorf("--step and --from-step cannot be used together")
			}

			clusterDir := ""
			gitDir := strings.TrimSpace(cfg.GitOps().GitDir)
			if gitDir != "" {
				clusterDir = filepath.Join(gitDir, "infrastructure", "clusters", cfg.ClusterName())
			}
			if logPath == "" && clusterDir != "" {
				// Generate timestamped log filename: bootstrap-YYYY-MM-DD-TIMESTAMP.log
				timestamp := time.Now()
				logFilename := fmt.Sprintf("bootstrap-%s-%d.log",
					timestamp.Format("2006-01-02"),
					timestamp.Unix())
				logPath = filepath.Join(clusterDir, "logs", logFilename)
			}

			statePath := ""
			if clusterDir != "" {
				statePath = filepath.Join(clusterDir, "logs", "bootstrap-state.json")
			} else {
				clusterPath, err := config.ClusterDirectoryPath(cfg.ClusterName())
				if err == nil && strings.TrimSpace(clusterPath) != "" {
					statePath = filepath.Join(clusterPath, "bootstrap-state.json")
				}
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

			var steps []bootstrapStep
			switch provider {
			case "openstack", "aws", "gcp", "azure":
				if clusterDir == "" {
					return fmt.Errorf("gitops.git_dir must be configured for provider %q", provider)
				}
				if _, err := os.Stat(clusterDir); err != nil {
					return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
				}
				env := map[string]string{}
				if kubeconf != "" {
					env["KUBECONFIG"] = kubeconf
				}
				// Preserve PATH from current environment so tools like terraform can be found
				if path := os.Getenv("PATH"); path != "" {
					env["PATH"] = path
				}

				steps = []bootstrapStep{
					{
						ID:          "make-terraform",
						Description: "Run make terraform",
						Run: func() error {
							runner.Infof("Running make terraform in %s", clusterDir)
							if err := runner.Run(clusterDir, env, "make", "terraform"); err != nil {
								return fmt.Errorf("make terraform failed: %w", err)
							}
							return nil
						},
					},
					{
						ID:          "terraform-init",
						Description: "Initialize Terraform",
						Run: func() error {
							runner.Infof("Initializing Terraform in %s", clusterDir)
							if err := runner.Run(clusterDir, env, "terraform", "init"); err != nil {
								return fmt.Errorf("terraform init failed: %w", err)
							}
							return nil
						},
					},
					{
						ID:          "terraform-apply",
						Description: "Apply Terraform configuration",
						Run: func() error {
							runner.Infof("Applying Terraform configuration (this may take several minutes)...")
							if err := runner.RunLongRunning(clusterDir, env, "terraform", "apply", "-auto-approve"); err != nil {
								return fmt.Errorf("terraform apply failed: %w", err)
							}
							return nil
						},
					},
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

				steps = []bootstrapStep{
					{
						ID:          "kind-create",
						Description: "Create kind cluster",
						Run: func() error {
							runner.Infof("Creating kind cluster %q using %s", cfg.ClusterName(), runtime)
							if err := runner.RunWithInput("", env, kindClusterConfig, "kind", "create", "cluster", "--name", cfg.ClusterName(), "--config=-"); err != nil {
								return err
							}
							return nil
						},
					},
					{
						ID:          "kind-export-kubeconfig",
						Description: "Export kind kubeconfig",
						Run: func() error {
							if err := runner.Run("", env, "kind", "export", "kubeconfig", "--name", cfg.ClusterName()); err != nil {
								return err
							}
							return nil
						},
					},
				}
			default:
				return fmt.Errorf("unsupported provider %q", cfg.OpenCenter.Infrastructure.Provider)
			}

			state, stateEnabled, err := loadBootstrapState(statePath)
			if err != nil {
				return err
			}
			if restart && stateEnabled {
				state = newBootstrapState()
			}

			stepIndex := map[string]int{}
			for i, step := range steps {
				stepIndex[step.ID] = i
			}

			selectedSteps := steps
			ignoreState := restart
			if strings.TrimSpace(onlyStep) != "" {
				idx, ok := stepIndex[onlyStep]
				if !ok {
					return fmt.Errorf("unknown step %q", onlyStep)
				}
				selectedSteps = []bootstrapStep{steps[idx]}
				ignoreState = true
			} else if strings.TrimSpace(fromStep) != "" {
				idx, ok := stepIndex[fromStep]
				if !ok {
					return fmt.Errorf("unknown step %q", fromStep)
				}
				selectedSteps = steps[idx:]
				ignoreState = true
			}

			for _, step := range selectedSteps {
				if !ignoreState && stateEnabled && state.IsSuccess(step.ID) {
					runner.Infof("Skipping step %q (already completed)", step.ID)
					continue
				}

				runner.Infof("Step %q: %s", step.ID, step.Description)
				if stateEnabled && !dryRun {
					state.SetStatus(step.ID, bootstrapStatusRunning, "")
					if err := saveBootstrapState(statePath, state); err != nil {
						return err
					}
				}

				if err := step.Run(); err != nil {
					if stateEnabled && !dryRun {
						state.SetStatus(step.ID, bootstrapStatusFailed, err.Error())
						if saveErr := saveBootstrapState(statePath, state); saveErr != nil {
							return saveErr
						}
					}
					return err
				}

				if stateEnabled && !dryRun {
					state.SetStatus(step.ID, bootstrapStatusSuccess, "")
					if err := saveBootstrapState(statePath, state); err != nil {
						return err
					}
				}
			}

			runner.Infof("Bootstrap complete.")
			if logPath != "" && !dryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "Log written to %s\n", logPath)
			}

			// Update stage and status
			if err := config.UpdateStatus(name, config.StageBootstrap, config.StatusSuccess); err != nil {
				// Don't fail the command if status update fails, just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to update cluster status: %v\n", err)
			}

			return nil
		},
	}

	cmd.Flags().Bool("dry-run", false, "show planned actions without executing")
	cmd.Flags().String("kubeconfig", "./kubeconfig.yaml", "path to kubeconfig used by bootstrap actions")
	cmd.Flags().String("log", "", "log file path (defaults to <git_dir>/infrastructure/clusters/<name>/logs/bootstrap-YYYY-MM-DD-TIMESTAMP.log)")
	cmd.Flags().String("container-runtime", "", "container runtime for kind clusters (docker or podman)")
	cmd.Flags().Bool("restart", false, "rerun all bootstrap steps and ignore saved state")
	cmd.Flags().String("step", "", "run a single bootstrap step by ID")
	cmd.Flags().String("from-step", "", "restart bootstrap from the specified step ID")

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
	masker  security.CredentialMasker
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
		header := fmt.Sprintf(`# opencenter bootstrap log
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

	// Create credential masker
	masker := security.NewDefaultCredentialMasker()

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
		masker:  masker,
	}, nil
}

func (r *bootstrapRunner) Close() {
	if r.logFile != nil {
		_ = r.logFile.Close()
	}
}

func (r *bootstrapRunner) Infof(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	maskedMsg := r.masker.MaskString(msg)
	fmt.Fprintf(r.stdout, "%s\n", maskedMsg)
}

func (r *bootstrapRunner) Run(dir string, env map[string]string, name string, args ...string) error {
	return r.execute(dir, env, nil, name, args...)
}

func (r *bootstrapRunner) RunLongRunning(dir string, env map[string]string, name string, args ...string) error {
	return r.executeLongRunning(dir, env, nil, name, args...)
}

func (r *bootstrapRunner) executeLongRunning(dir string, env map[string]string, stdin io.Reader, name string, args ...string) error {
	printable := formatCommand(env, name, args)
	maskedPrintable := r.masker.MaskString(printable)
	fmt.Fprintf(r.stdout, "$ %s\n", maskedPrintable)

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

	// Start the command
	if err := cmd.Start(); err != nil {
		maskedErr := r.masker.MaskString(err.Error())
		return fmt.Errorf("failed to start command: %s: %s", maskedPrintable, maskedErr)
	}

	// Log progress for long-running commands
	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	// Progress ticker for long-running operations
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	startTime := time.Now()
	for {
		select {
		case err := <-done:
			elapsed := time.Since(startTime)
			if err != nil {
				fmt.Fprintf(r.stdout, "Command completed with error after %v\n", elapsed.Round(time.Second))
				maskedErr := r.masker.MaskString(err.Error())
				return fmt.Errorf("command failed: %s: %s", maskedPrintable, maskedErr)
			}
			fmt.Fprintf(r.stdout, "Command completed successfully after %v\n", elapsed.Round(time.Second))
			return nil
		case <-ticker.C:
			elapsed := time.Since(startTime)
			fmt.Fprintf(r.stdout, "Still running... (elapsed: %v)\n", elapsed.Round(time.Second))
		}
	}
}

func (r *bootstrapRunner) RunWithInput(dir string, env map[string]string, input string, name string, args ...string) error {
	return r.execute(dir, env, strings.NewReader(input), name, args...)
}

func (r *bootstrapRunner) execute(dir string, env map[string]string, stdin io.Reader, name string, args ...string) error {
	printable := formatCommand(env, name, args)
	maskedPrintable := r.masker.MaskString(printable)
	fmt.Fprintf(r.stdout, "$ %s\n", maskedPrintable)

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
		maskedErr := r.masker.MaskString(err.Error())
		return fmt.Errorf("command failed: %s: %s", maskedPrintable, maskedErr)
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

func newBootstrapState() *bootstrapState {
	return &bootstrapState{
		Version: bootstrapStateVersion,
		Steps:   map[string]bootstrapStepState{},
	}
}

func loadBootstrapState(path string) (*bootstrapState, bool, error) {
	if strings.TrimSpace(path) == "" {
		return newBootstrapState(), false, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newBootstrapState(), true, nil
		}
		return nil, true, fmt.Errorf("failed to read bootstrap state: %w", err)
	}

	var state bootstrapState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, true, fmt.Errorf("failed to parse bootstrap state: %w", err)
	}
	if state.Steps == nil {
		state.Steps = map[string]bootstrapStepState{}
	}
	if state.Version == 0 {
		state.Version = bootstrapStateVersion
	}
	return &state, true, nil
}

func saveBootstrapState(path string, state *bootstrapState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("failed to create bootstrap state directory: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize bootstrap state: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("failed to write bootstrap state: %w", err)
	}
	return nil
}

func (s *bootstrapState) SetStatus(stepID, status, message string) {
	if s.Steps == nil {
		s.Steps = map[string]bootstrapStepState{}
	}
	s.Steps[stepID] = bootstrapStepState{
		Status:    status,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Error:     message,
	}
}

func (s *bootstrapState) IsSuccess(stepID string) bool {
	step, ok := s.Steps[stepID]
	if !ok {
		return false
	}
	return step.Status == bootstrapStatusSuccess || step.Status == bootstrapStatusSkipped
}
