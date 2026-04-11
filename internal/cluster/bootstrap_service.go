package cluster

import (
	"bytes"
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

const (
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
	LogPath                   string
	ResumeStatePath           string
	StepsCompleted            []string
	StepsFailed               []string
}

// BootstrapService handles cluster bootstrap business logic
type BootstrapService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configurationMgr *config.ConfigurationManager
	fileSystem       fs.FileSystem
	runner           lifecycleCommandRunner
	commandRunner    security.CommandRunner
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
		runner:           newExecLifecycleCommandRunner(),
		commandRunner:    security.GetDefaultCommandRunner(),
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
	// Build the full identifier (org/cluster) for config loading when organization is known
	configIdentifier := opts.ClusterName
	if opts.Organization != "" {
		configIdentifier = opts.Organization + "/" + opts.ClusterName
	}

	var cfg v2.Config
	if s.configurationMgr != nil {
		var loadedCfg *v2.Config
		var err error

		// Use LoadWithoutValidation if validation will be skipped anyway
		if opts.SkipValidation {
			loadedCfg, err = s.configurationMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = s.configurationMgr.Load(ctx, configIdentifier)
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

		var loadedCfg *v2.Config
		if opts.SkipValidation {
			loadedCfg, err = tempMgr.LoadWithoutValidation(ctx, configIdentifier)
		} else {
			loadedCfg, err = tempMgr.Load(ctx, configIdentifier)
		}

		if err != nil {
			return nil, fmt.Errorf("loading configuration: %w", err)
		}
		cfg = *loadedCfg
	}

	// Check schema version - only v2 is supported
	if cfg.SchemaVersion != "2.0" {
		return nil, fmt.Errorf("invalid schema version for cluster %s: expected 2.0, got %q", opts.ClusterName, cfg.SchemaVersion)
	}

	result := &BootstrapResult{
		StepsCompleted: []string{},
		StepsFailed:    []string{},
	}

	var runtimePaths *bootstrapRuntimePaths
	if !opts.DryRun {
		runtimePaths, err = resolveBootstrapRuntimePaths(&cfg, opts.LogPath, startTime)
		if err != nil {
			return result, fmt.Errorf("resolving bootstrap runtime paths: %w", err)
		}
		result.LogPath = runtimePaths.LogPath

		logFile, err := openBootstrapLogFile(runtimePaths.LogPath)
		if err != nil {
			return result, err
		}
		defer logFile.Close()

		ctx = withBootstrapLogWriter(ctx, logFile)
		logBootstrapMessage(ctx, "bootstrap started for %s/%s", cfg.Organization(), cfg.ClusterName())
	}

	if !opts.SkipValidation {
		if err := s.validateBootstrapConfig(&cfg); err != nil {
			logBootstrapMessage(ctx, "bootstrap validation failed: %v", err)
			return result, fmt.Errorf("validation failed: %w", err)
		}
	}

	// Set default timeout if not specified
	if opts.Timeout == 0 {
		opts.Timeout = defaultReadyTimeout
	}
	if strings.TrimSpace(opts.KubeconfigPath) == "" {
		opts.KubeconfigPath = clusterPaths.KubeconfigPath
	}

	// Provision infrastructure
	if !opts.DryRun {
		if err := s.provisionInfrastructure(ctx, &cfg, clusterPaths, &opts, runtimePaths, result); err != nil {
			logBootstrapMessage(ctx, "bootstrap failed during infrastructure provisioning: %v", err)
			return result, fmt.Errorf("provisioning infrastructure: %w", err)
		}
		result.InfrastructureProvisioned = true
	}

	// Deploy cluster
	if !opts.DryRun {
		if err := s.deployCluster(ctx, &cfg, clusterPaths, &opts, result); err != nil {
			logBootstrapMessage(ctx, "bootstrap failed during cluster deployment: %v", err)
			return result, fmt.Errorf("deploying cluster: %w", err)
		}
		result.ClusterDeployed = true
	}

	// Wait for cluster to be ready
	if !opts.DryRun {
		endpoint, err := s.waitForReady(ctx, &cfg, opts.Timeout, opts.KubeconfigPath)
		if err != nil {
			logBootstrapMessage(ctx, "bootstrap failed while waiting for readiness: %v", err)
			return result, fmt.Errorf("waiting for cluster ready: %w", err)
		}
		result.ClusterReady = true
		result.Endpoint = endpoint
	}

	result.Duration = time.Since(startTime)
	if !opts.DryRun {
		if err := s.removeBootstrapState(runtimePaths.StatePath); err != nil {
			logBootstrapMessage(ctx, "warning: failed to remove bootstrap state %s: %v", runtimePaths.StatePath, err)
		}
		if err := s.removeBootstrapState(runtimePaths.LegacyStatePath); err != nil {
			logBootstrapMessage(ctx, "warning: failed to remove legacy bootstrap state %s: %v", runtimePaths.LegacyStatePath, err)
		}
		result.ResumeStatePath = ""
		logBootstrapMessage(ctx, "bootstrap completed in %s", result.Duration.Round(time.Second))
	}
	return result, nil
}

// provisionInfrastructure provisions the infrastructure for the cluster
func (s *BootstrapService) provisionInfrastructure(ctx context.Context, cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions, runtimePaths *bootstrapRuntimePaths, result *BootstrapResult) error {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	if provider == "" {
		provider = "openstack"
	}

	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return err
	}

	statePath := ""
	legacyStatePath := ""
	if runtimePaths != nil {
		statePath = runtimePaths.StatePath
		legacyStatePath = runtimePaths.LegacyStatePath
	}

	state := s.newBootstrapState()
	stateEnabled := strings.TrimSpace(statePath) != ""
	if opts.Restart {
		if err := s.removeBootstrapState(statePath); err != nil {
			return fmt.Errorf("clearing bootstrap state: %w", err)
		}
		if err := s.removeBootstrapState(legacyStatePath); err != nil {
			return fmt.Errorf("clearing legacy bootstrap state: %w", err)
		}
		logBootstrapMessage(ctx, "bootstrap restart requested; cleared saved state")
	} else if stateEnabled {
		loadedState, loadedPath, err := s.loadBootstrapStateWithFallback(statePath, legacyStatePath)
		if err != nil {
			return fmt.Errorf("loading bootstrap state: %w", err)
		}
		state = loadedState
		if loadedPath != "" {
			logBootstrapMessage(ctx, "resuming bootstrap from %s", loadedPath)
		}
	}

	// Build steps based on provider
	var steps []bootstrapStep

	switch provider {
	case "openstack":
		providerImpl := newOpenStackBootstrapProvider(s.runner)
		steps, err = providerImpl.BuildSteps(cfg, clusterPaths, opts)
		if err != nil {
			return err
		}

	case "aws", "gcp", "azure":
		env := buildBootstrapEnvironment(opts.KubeconfigPath)

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
		providerImpl := newKindBootstrapProvider(s.runner)
		steps, err = providerImpl.BuildSteps(cfg, clusterPaths, opts)
		if err != nil {
			return err
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
			logBootstrapMessage(ctx, "step skipped from saved state: %s", step.ID)
			continue
		}

		// Mark step as running
		if stateEnabled && !opts.DryRun {
			s.setStepStatus(state, step.ID, bootstrapStatusRunning, "")
			if err := s.saveBootstrapState(statePath, state); err != nil {
				return err
			}
		}
		logBootstrapMessage(ctx, "step started: %s - %s", step.ID, step.Description)

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
			result.ResumeStatePath = statePath
			logBootstrapMessage(ctx, "step failed: %s: %v", step.ID, err)
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
		logBootstrapMessage(ctx, "step completed: %s", step.ID)
	}

	return nil
}

// deployCluster deploys the Kubernetes cluster
func (s *BootstrapService) deployCluster(ctx context.Context, cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions, result *BootstrapResult) error {
	// For most providers, deployment is handled by the infrastructure provisioning step
	// This method is a placeholder for future provider-specific deployment logic
	// that may be separate from infrastructure provisioning

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))

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
func (s *BootstrapService) waitForReady(ctx context.Context, cfg *v2.Config, timeout time.Duration, kubeconfigPath string) (string, error) {
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	logBootstrapMessage(ctx, "waiting for cluster readiness with timeout %s", timeout)

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))

	var (
		endpoint string
		err      error
	)

	switch provider {
	case "kind":
		endpoint, err = s.waitForKindCluster(ctx, kubeconfigPath)

	case "openstack", "aws", "gcp", "azure":
		endpoint, err = s.waitForCloudCluster(ctx, cfg, kubeconfigPath)

	default:
		err = fmt.Errorf("unsupported provider %q", provider)
	}

	if err != nil {
		logBootstrapMessage(ctx, "cluster readiness check failed: %v", err)
		return "", err
	}

	logBootstrapMessage(ctx, "cluster readiness confirmed: %s", endpoint)
	return endpoint, nil
}

// waitForKindCluster waits for a kind cluster to be ready
func (s *BootstrapService) waitForKindCluster(ctx context.Context, kubeconfigPath string) (string, error) {
	return kindprovider.NewProvider().WaitReady(ctx, kubeconfigPath)
}

// waitForCloudCluster waits for a cloud cluster to be ready
func (s *BootstrapService) waitForCloudCluster(ctx context.Context, cfg *v2.Config, kubeconfigPath string) (string, error) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for cluster to be ready: %w", ctx.Err())

		case <-ticker.C:
			// Check if cluster API is accessible
			args := []string{}
			if strings.TrimSpace(kubeconfigPath) != "" {
				args = append(args, "--kubeconfig", kubeconfigPath)
			}
			args = append(args, "cluster-info")
			cmd, err := s.commandRunner.PrepareCommandContext(ctx, "kubectl", args...)
			if err != nil {
				return "", fmt.Errorf("preparing kubectl cluster-info: %w", err)
			}
			if err := cmd.Run(); err == nil {
				args = []string{}
				if strings.TrimSpace(kubeconfigPath) != "" {
					args = append(args, "--kubeconfig", kubeconfigPath)
				}
				args = append(args, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
				cmd, err = s.commandRunner.PrepareCommandContext(ctx, "kubectl", args...)
				if err != nil {
					return "", fmt.Errorf("preparing kubectl endpoint lookup: %w", err)
				}
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
func (s *BootstrapService) validateBootstrapConfig(cfg *v2.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if strings.TrimSpace(cfg.ClusterName()) == "" {
		return fmt.Errorf("cluster name must be set")
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	if provider == "" {
		return fmt.Errorf("opencenter.infrastructure.provider must be set")
	}
	if provider == "kind" && cfg.OpenCenter.Infrastructure.Kind == nil {
		return fmt.Errorf("opencenter.infrastructure.kind must be configured for the kind provider")
	}

	return nil
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
	_, err := s.runner.Run(ctx, dir, env, name, args...)
	return err
}

// runCommandWithInput executes a command with stdin input
func (s *BootstrapService) runCommandWithInput(ctx context.Context, dir string, env map[string]string, input string, name string, args ...string) error {
	cmd, err := s.commandRunner.PrepareCommandContext(ctx, name, args...)
	if err != nil {
		return fmt.Errorf("preparing command %s: %w", name, err)
	}
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(input)

	// Build environment
	envList := os.Environ()
	for k, v := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = envList

	// Run command
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if writer := bootstrapLogWriter(ctx); writer != nil {
		cmd.Stdout = io.MultiWriter(&stdout, writer)
		cmd.Stderr = io.MultiWriter(&stderr, writer)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}
	err = cmd.Run()
	output := append(stdout.Bytes(), stderr.Bytes()...)
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

func (s *BootstrapService) loadBootstrapStateWithFallback(path, fallbackPath string) (*bootstrapState, string, error) {
	state, enabled, err := s.loadBootstrapState(path)
	if err != nil {
		return nil, "", err
	}
	if enabled && s.fileSystem.Exists(path) {
		return state, path, nil
	}

	if strings.TrimSpace(fallbackPath) == "" {
		return state, "", nil
	}

	legacyState, enabled, err := s.loadBootstrapState(fallbackPath)
	if err != nil {
		return nil, "", err
	}
	if enabled && s.fileSystem.Exists(fallbackPath) {
		return legacyState, fallbackPath, nil
	}

	return state, "", nil
}

// saveBootstrapState saves the bootstrap state to disk
func (s *BootstrapService) saveBootstrapState(path string, state *bootstrapState) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	if err := s.fileSystem.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("creating bootstrap state directory: %w", err)
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("serializing bootstrap state: %w", err)
	}

	if err := s.fileSystem.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing bootstrap state: %w", err)
	}

	return nil
}

func (s *BootstrapService) removeBootstrapState(path string) error {
	if strings.TrimSpace(path) == "" {
		return nil
	}

	if !s.fileSystem.Exists(path) {
		return nil
	}

	if err := s.fileSystem.Remove(path); err != nil && !os.IsNotExist(stderrors.Unwrap(err)) {
		return fmt.Errorf("removing bootstrap state: %w", err)
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
