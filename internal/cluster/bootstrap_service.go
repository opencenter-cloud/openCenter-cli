package cluster

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

const (
	// kindClusterConfig is the default configuration for kind clusters
	kindClusterConfig = `kind: Cluster
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

	// Bootstrap step statuses
	bootstrapStatusSuccess = "success"
	bootstrapStatusFailed  = "failed"
	bootstrapStatusRunning = "running"
	bootstrapStatusSkipped = "skipped"

	// Bootstrap state version
	bootstrapStateVersion = 1

	// Default timeout for cluster readiness
	defaultReadyTimeout = 30 * time.Minute
)

// BootstrapOptions contains options for cluster bootstrap
type BootstrapOptions struct {
	ClusterName      string
	Organization     string
	SkipValidation   bool
	Timeout          time.Duration
	DryRun           bool
	ContainerRuntime string
	Restart          bool
	OnlyStep         string
	FromStep         string
	KubeconfigPath   string
	LogPath          string
}

// BootstrapResult contains the result of cluster bootstrap
type BootstrapResult struct {
	InfrastructureProvisioned bool
	ClusterDeployed           bool
	ClusterReady              bool
	Duration                  time.Duration
	Endpoint                  string
	StepsCompleted            []string
	StepsFailed               []string
}

// BootstrapService handles cluster bootstrap business logic
type BootstrapService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configurationMgr *config.ConfigurationManager
	fileSystem       fs.FileSystem
}

// NewBootstrapService creates a new BootstrapService
func NewBootstrapService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
) *BootstrapService {
	return NewBootstrapServiceWithConfigMgr(pathResolver, validationEngine, nil, nil)
}

// NewBootstrapServiceWithConfigMgr creates a new BootstrapService with optional ConfigurationManager
func NewBootstrapServiceWithConfigMgr(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configurationMgr *config.ConfigurationManager,
	fileSystem fs.FileSystem,
) *BootstrapService {
	// Create ConfigurationManager if not provided
	if configurationMgr == nil {
		// Try to create one, but don't fail if it doesn't work
		configurationMgr, _ = config.NewConfigurationManager()
	}

	// Create FileSystem if not provided
	if fileSystem == nil {
		errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
		fileSystem = fs.NewDefaultFileSystem(errorHandler)
	}

	return &BootstrapService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
		configurationMgr: configurationMgr,
		fileSystem:       fileSystem,
	}
}

// bootstrapStep represents a single bootstrap step
type bootstrapStep struct {
	ID          string
	Description string
	Run         func(ctx context.Context) error
}

// bootstrapState tracks the state of bootstrap steps
type bootstrapState struct {
	Version int                           `json:"version"`
	Steps   map[string]bootstrapStepState `json:"steps"`
}

// bootstrapStepState represents the state of a single step
type bootstrapStepState struct {
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
	Error     string `json:"error,omitempty"`
}

// Bootstrap performs cluster bootstrap
func (s *BootstrapService) Bootstrap(ctx context.Context, opts BootstrapOptions) (*BootstrapResult, error) {
	startTime := time.Now()

	// Resolve paths
	clusterPaths, err := s.pathResolver.Resolve(ctx, opts.ClusterName, opts.Organization)
	if err != nil {
		return nil, fmt.Errorf("resolving cluster paths: %w", err)
	}

	// Load configuration using ConfigurationManager
	var cfg config.Config
	if s.configurationMgr != nil {
		var loadedCfg *config.Config
		var err error
		
		// Use LoadWithoutValidation if validation will be skipped anyway
		if opts.SkipValidation {
			loadedCfg, err = s.configurationMgr.LoadWithoutValidation(ctx, opts.ClusterName)
		} else {
			loadedCfg, err = s.configurationMgr.Load(ctx, opts.ClusterName)
		}
		
		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	} else {
		// Fallback: create temporary manager
		tempMgr, err := config.NewConfigurationManager()
		if err != nil {
			return nil, fmt.Errorf("creating configuration manager: %w", err)
		}
		
		var loadedCfg *config.Config
		if opts.SkipValidation {
			loadedCfg, err = tempMgr.LoadWithoutValidation(ctx, opts.ClusterName)
		} else {
			loadedCfg, err = tempMgr.Load(ctx, opts.ClusterName)
		}
		
		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return nil, fmt.Errorf(`v1 configurations are not supported in v2.0.0

To upgrade to v2.0.0:
1. Install opencenter v1.x
2. Run: opencenter cluster migrate-config %s
3. Upgrade to opencenter v2.0.0

See: https://docs.opencenter.io/migration/v1-to-v2`, opts.ClusterName)
	}

	result := &BootstrapResult{
		StepsCompleted: []string{},
		StepsFailed:    []string{},
	}

	// Validate configuration unless skipped
	if !opts.SkipValidation {
		validationResult, err := s.validationEngine.Validate(ctx, "config", cfg)
		if err != nil {
			return nil, fmt.Errorf("validating config: %w", err)
		}

		if !validationResult.Valid {
			return nil, fmt.Errorf("validation failed: %v", validationResult.Errors)
		}
	}

	// Set default timeout if not specified
	if opts.Timeout == 0 {
		opts.Timeout = defaultReadyTimeout
	}

	// Provision infrastructure
	if !opts.DryRun {
		if err := s.provisionInfrastructure(ctx, &cfg, clusterPaths, &opts, result); err != nil {
			return result, fmt.Errorf("provisioning infrastructure: %w", err)
		}
		result.InfrastructureProvisioned = true
	}

	// Deploy cluster
	if !opts.DryRun {
		if err := s.deployCluster(ctx, &cfg, clusterPaths, &opts, result); err != nil {
			return result, fmt.Errorf("deploying cluster: %w", err)
		}
		result.ClusterDeployed = true
	}

	// Wait for cluster to be ready
	if !opts.DryRun {
		endpoint, err := s.waitForReady(ctx, &cfg, opts.Timeout)
		if err != nil {
			return result, fmt.Errorf("waiting for cluster ready: %w", err)
		}
		result.ClusterReady = true
		result.Endpoint = endpoint
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// provisionInfrastructure provisions the infrastructure for the cluster
func (s *BootstrapService) provisionInfrastructure(ctx context.Context, cfg *config.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions, result *BootstrapResult) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))
	if provider == "" {
		provider = "openstack"
	}

	// Get cluster directory from GitOps configuration
	gitDir := strings.TrimSpace(cfg.GitOps().GitDir)
	if gitDir == "" {
		return fmt.Errorf("gitops.git_dir must be configured for provider %q", provider)
	}

	clusterDir := filepath.Join(gitDir, "infrastructure", "clusters", cfg.ClusterName())

	// Verify cluster directory exists
	if _, err := os.Stat(clusterDir); err != nil {
		return fmt.Errorf("cluster infrastructure directory not found in GitOps repository: %s", clusterDir)
	}

	// Determine log path
	logPath := opts.LogPath
	if logPath == "" {
		timestamp := time.Now()
		logFilename := fmt.Sprintf("bootstrap-%s-%d.log",
			timestamp.Format("2006-01-02"),
			timestamp.Unix())
		logPath = filepath.Join(clusterDir, "logs", logFilename)
	}

	// Determine state path
	statePath := filepath.Join(clusterDir, "logs", "bootstrap-state.json")

	// Load or create bootstrap state
	state, stateEnabled, err := s.loadBootstrapState(statePath)
	if err != nil {
		return fmt.Errorf("loading bootstrap state: %w", err)
	}

	if opts.Restart && stateEnabled {
		state = s.newBootstrapState()
	}

	// Build steps based on provider
	var steps []bootstrapStep

	switch provider {
	case "openstack", "aws", "gcp", "azure":
		// Cloud provider bootstrap steps
		env := s.buildEnvironment(opts.KubeconfigPath)

		steps = []bootstrapStep{
			{
				ID:          "make-terraform",
				Description: "Run make terraform",
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "make", "terraform")
				},
			},
			{
				ID:          "terraform-init",
				Description: "Initialize Terraform",
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "terraform", "init")
				},
			},
			{
				ID:          "terraform-apply",
				Description: "Apply Terraform configuration",
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "terraform", "apply", "-auto-approve")
				},
			},
		}

	case "kind":
		// Kind cluster bootstrap steps
		runtime := s.resolveContainerRuntime(opts.ContainerRuntime)
		env := s.buildKindEnvironment(runtime)

		steps = []bootstrapStep{
			{
				ID:          "kind-create",
				Description: "Create kind cluster",
				Run: func(ctx context.Context) error {
					return s.runCommandWithInput(ctx, "", env, kindClusterConfig, "kind", "create", "cluster", "--name", cfg.ClusterName(), "--config=-")
				},
			},
			{
				ID:          "kind-export-kubeconfig",
				Description: "Export kind kubeconfig",
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, "", env, "kind", "export", "kubeconfig", "--name", cfg.ClusterName())
				},
			},
		}

	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}

	// Filter steps based on options
	selectedSteps, ignoreState := s.filterSteps(steps, opts)

	// Execute steps
	for _, step := range selectedSteps {
		// Skip if already completed (unless ignoring state)
		if !ignoreState && stateEnabled && s.isStepSuccess(state, step.ID) {
			continue
		}

		// Mark step as running
		if stateEnabled && !opts.DryRun {
			s.setStepStatus(state, step.ID, bootstrapStatusRunning, "")
			if err := s.saveBootstrapState(statePath, state); err != nil {
				return err
			}
		}

		// Execute step
		if err := step.Run(ctx); err != nil {
			// Mark step as failed
			if stateEnabled && !opts.DryRun {
				s.setStepStatus(state, step.ID, bootstrapStatusFailed, err.Error())
				if saveErr := s.saveBootstrapState(statePath, state); saveErr != nil {
					return saveErr
				}
			}
			result.StepsFailed = append(result.StepsFailed, step.ID)
			return fmt.Errorf("step %q failed: %w", step.ID, err)
		}

		// Mark step as successful
		if stateEnabled && !opts.DryRun {
			s.setStepStatus(state, step.ID, bootstrapStatusSuccess, "")
			if err := s.saveBootstrapState(statePath, state); err != nil {
				return err
			}
		}
		result.StepsCompleted = append(result.StepsCompleted, step.ID)
	}

	return nil
}

// deployCluster deploys the Kubernetes cluster
func (s *BootstrapService) deployCluster(ctx context.Context, cfg *config.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions, result *BootstrapResult) error {
	// For most providers, deployment is handled by the infrastructure provisioning step
	// This method is a placeholder for future provider-specific deployment logic
	// that may be separate from infrastructure provisioning

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	switch provider {
	case "kind":
		// Kind cluster is already deployed in provisionInfrastructure
		return nil

	case "openstack", "aws", "gcp", "azure":
		// Cloud providers deploy via Terraform in provisionInfrastructure
		return nil

	default:
		return fmt.Errorf("unsupported provider %q", provider)
	}
}

// waitForReady waits for the cluster to be ready and returns the endpoint
func (s *BootstrapService) waitForReady(ctx context.Context, cfg *config.Config, timeout time.Duration) (string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	provider := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Infrastructure.Provider))

	switch provider {
	case "kind":
		// For kind clusters, check if the cluster is accessible
		return s.waitForKindCluster(ctx, cfg.ClusterName())

	case "openstack", "aws", "gcp", "azure":
		// For cloud providers, check if the API server is accessible
		return s.waitForCloudCluster(ctx, cfg)

	default:
		return "", fmt.Errorf("unsupported provider %q", provider)
	}
}

// waitForKindCluster waits for a kind cluster to be ready
func (s *BootstrapService) waitForKindCluster(ctx context.Context, clusterName string) (string, error) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for kind cluster to be ready: %w", ctx.Err())

		case <-ticker.C:
			// Check if cluster is accessible
			cmd := exec.CommandContext(ctx, "kubectl", "cluster-info", "--context", fmt.Sprintf("kind-%s", clusterName))
			if err := cmd.Run(); err == nil {
				// Cluster is ready, get the endpoint
				cmd = exec.CommandContext(ctx, "kubectl", "config", "view", "-o", "jsonpath={.clusters[?(@.name==\"kind-"+clusterName+"\")].cluster.server}")
				output, err := cmd.Output()
				if err != nil {
					return "", fmt.Errorf("getting cluster endpoint: %w", err)
				}
				return strings.TrimSpace(string(output)), nil
			}
		}
	}
}

// waitForCloudCluster waits for a cloud cluster to be ready
func (s *BootstrapService) waitForCloudCluster(ctx context.Context, cfg *config.Config) (string, error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for cluster to be ready: %w", ctx.Err())

		case <-ticker.C:
			// Check if cluster API is accessible
			cmd := exec.CommandContext(ctx, "kubectl", "cluster-info")
			if err := cmd.Run(); err == nil {
				// Cluster is ready, get the endpoint
				cmd = exec.CommandContext(ctx, "kubectl", "config", "view", "-o", "jsonpath={.clusters[0].cluster.server}")
				output, err := cmd.Output()
				if err != nil {
					return "", fmt.Errorf("getting cluster endpoint: %w", err)
				}
				return strings.TrimSpace(string(output)), nil
			}
		}
	}
}

// Helper methods

// buildEnvironment builds the environment variables for command execution
func (s *BootstrapService) buildEnvironment(kubeconfigPath string) map[string]string {
	env := make(map[string]string)

	if kubeconfigPath != "" {
		env["KUBECONFIG"] = kubeconfigPath
	}

	// Preserve PATH from current environment
	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}

// buildKindEnvironment builds the environment for kind clusters
func (s *BootstrapService) buildKindEnvironment(runtime string) map[string]string {
	env := make(map[string]string)

	if runtime == "podman" {
		env["KIND_EXPERIMENTAL_PROVIDER"] = "podman"
	}

	// Preserve PATH
	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}

// resolveContainerRuntime resolves the container runtime to use
func (s *BootstrapService) resolveContainerRuntime(flagValue string) string {
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

// filterSteps filters bootstrap steps based on options
func (s *BootstrapService) filterSteps(steps []bootstrapStep, opts *BootstrapOptions) ([]bootstrapStep, bool) {
	// Build step index
	stepIndex := make(map[string]int)
	for i, step := range steps {
		stepIndex[step.ID] = i
	}

	ignoreState := opts.Restart

	// Filter by single step
	if strings.TrimSpace(opts.OnlyStep) != "" {
		idx, ok := stepIndex[opts.OnlyStep]
		if !ok {
			return nil, true
		}
		return []bootstrapStep{steps[idx]}, true
	}

	// Filter from step onwards
	if strings.TrimSpace(opts.FromStep) != "" {
		idx, ok := stepIndex[opts.FromStep]
		if !ok {
			return nil, true
		}
		return steps[idx:], true
	}

	return steps, ignoreState
}

// runCommand executes a command in the specified directory
func (s *BootstrapService) runCommand(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	// Build environment
	envList := os.Environ()
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s %v: %w\nOutput: %s", name, args, err, string(output))
	}

	return nil
}

// runCommandWithInput executes a command with stdin input
func (s *BootstrapService) runCommandWithInput(ctx context.Context, dir string, env map[string]string, input string, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)

	// Build environment
	envList := os.Environ()
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	// Run command
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("command failed: %s %v: %w\nOutput: %s", name, args, err, string(output))
	}

	return nil
}

// Bootstrap state management

// newBootstrapState creates a new bootstrap state
func (s *BootstrapService) newBootstrapState() *bootstrapState {
	return &bootstrapState{
		Version: bootstrapStateVersion,
		Steps:   make(map[string]bootstrapStepState),
	}
}

// loadBootstrapState loads the bootstrap state from disk
func (s *BootstrapService) loadBootstrapState(path string) (*bootstrapState, bool, error) {
	if strings.TrimSpace(path) == "" {
		return s.newBootstrapState(), false, nil
	}

	data, err := s.fileSystem.ReadFile(path)
	if err != nil {
		// Unwrap to check for os.IsNotExist
		if os.IsNotExist(stderrors.Unwrap(err)) {
			return s.newBootstrapState(), true, nil
		}
		return nil, true, fmt.Errorf("reading bootstrap state: %w", err)
	}

	var state bootstrapState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, true, fmt.Errorf("parsing bootstrap state: %w", err)
	}

	if state.Steps == nil {
		state.Steps = make(map[string]bootstrapStepState)
	}
	if state.Version == 0 {
		state.Version = bootstrapStateVersion
	}

	return &state, true, nil
}

// saveBootstrapState saves the bootstrap state to disk
func (s *BootstrapService) saveBootstrapState(path string, state *bootstrapState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("creating bootstrap state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing bootstrap state: %w", err)
	}

	if err := s.fileSystem.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing bootstrap state: %w", err)
	}

	return nil
}

// setStepStatus sets the status of a bootstrap step
func (s *BootstrapService) setStepStatus(state *bootstrapState, stepID, status, message string) {
	if state.Steps == nil {
		state.Steps = make(map[string]bootstrapStepState)
	}
	state.Steps[stepID] = bootstrapStepState{
		Status:    status,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Error:     message,
	}
}

// isStepSuccess checks if a step completed successfully
func (s *BootstrapService) isStepSuccess(state *bootstrapState, stepID string) bool {
	step, ok := state.Steps[stepID]
	if !ok {
		return false
	}
	return step.Status == bootstrapStatusSuccess || step.Status == bootstrapStatusSkipped
}
