package cluster

import (
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
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
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
	Debug            bool
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
	Plan                      *BootstrapPlan
}

// BootstrapService handles cluster bootstrap business logic
type BootstrapService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configurationMgr *config.ConfigurationManager
	fileSystem       fs.FileSystem
	runner           lifecycleCommandRunner
	commandRunner    security.CommandRunner
	output           io.Writer
}

// SetOutput sets the writer used for user-facing progress messages.
// When nil, progress messages are suppressed.
func (s *BootstrapService) SetOutput(w io.Writer) {
	s.output = w
}

// progress writes a user-facing progress line to the configured output writer.
func (s *BootstrapService) progress(format string, args ...interface{}) {
	if s.output == nil {
		return
	}
	fmt.Fprintf(s.output, format+"\n", args...)
}

func (s *BootstrapService) printStepDebug(step bootstrapStep) {
	if s.output == nil {
		return
	}

	fmt.Fprintln(s.output, "-----")
	fmt.Fprintf(s.output, "Step: %q\n", debugStepName(step))
	if len(step.Plan.Environment) == 0 {
		fmt.Fprintln(s.output, "Environment: (none)")
	} else {
		fmt.Fprintln(s.output, "Environment:")
		for _, env := range step.Plan.Environment {
			fmt.Fprintf(s.output, "  %s\n", formatDebugEnv(env))
		}
	}
	fmt.Fprintf(s.output, "PATH: %s\n", debugWorkingDir(step.Plan.WorkingDir))
	if len(step.Plan.Commands) == 0 {
		fmt.Fprintln(s.output, "Command: (none)")
		return
	}
	for _, command := range step.Plan.Commands {
		fmt.Fprintf(s.output, "Command: %s\n", formatDebugCommand(command))
	}
}

func debugStepName(step bootstrapStep) string {
	if strings.TrimSpace(step.Description) != "" {
		return step.Description
	}
	if strings.TrimSpace(step.Plan.Action) != "" {
		return step.Plan.Action
	}
	if strings.TrimSpace(step.Plan.ID) != "" {
		return step.Plan.ID
	}
	return step.ID
}

func debugWorkingDir(workingDir string) string {
	if strings.TrimSpace(workingDir) == "" {
		return "(not set)"
	}
	return workingDir
}

func formatDebugCommand(command BootstrapPlanCommand) string {
	parts := []string{command.Name}
	parts = append(parts, command.Args...)
	return strings.Join(parts, " ")
}

func formatDebugEnv(env BootstrapPlanEnv) string {
	if env.Redacted {
		return env.Name + "=<redacted>"
	}
	if strings.TrimSpace(env.Value) == "" {
		return env.Name
	}
	return env.Name + "=" + env.Value
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
	Plan        BootstrapPlanStep
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

	runtimePaths, err := resolveBootstrapRuntimePaths(&cfg, opts.LogPath, startTime)
	if err != nil {
		return result, fmt.Errorf("resolving bootstrap runtime paths: %w", err)
	}
	result.LogPath = runtimePaths.LogPath

	if !opts.DryRun {
		logFile, err := openBootstrapLogFile(runtimePaths.LogPath)
		if err != nil {
			return result, err
		}
		defer logFile.Close()

		ctx = withBootstrapLogWriter(ctx, logFile)
		s.progress("Bootstrap started for %s", opts.ClusterName)
		s.progress("Log file: %s", runtimePaths.LogPath)
		logging.Debugf("bootstrap: log file at %s", runtimePaths.LogPath)
		logging.Debugf("bootstrap: state file at %s", runtimePaths.StatePath)
		logBootstrapMessage(ctx, "bootstrap started for %s/%s", cfg.Organization(), cfg.ClusterName())
	}

	if !opts.SkipValidation {
		if !opts.DryRun {
			s.progress("Validating bootstrap configuration...")
		}
		logging.Debug("bootstrap: running configuration validation")
		if err := s.validateBootstrapConfig(&cfg); err != nil {
			logBootstrapMessage(ctx, "bootstrap validation failed: %v", err)
			return result, fmt.Errorf("validation failed: %w", err)
		}
		if !opts.DryRun {
			s.progress("✓ Configuration valid")
		}
		logging.Debug("bootstrap: configuration validation passed")
	}

	// Set default timeout if not specified
	if opts.Timeout == 0 {
		opts.Timeout = defaultReadyTimeout
	}
	if strings.TrimSpace(opts.KubeconfigPath) == "" {
		opts.KubeconfigPath = clusterPaths.KubeconfigPath
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	logging.Debugf("bootstrap: provider=%s cluster=%s org=%s", provider, opts.ClusterName, opts.Organization)
	logging.Debugf("bootstrap: kubeconfig=%s timeout=%s", opts.KubeconfigPath, opts.Timeout)

	if opts.DryRun {
		steps, err := s.buildBootstrapSteps(&cfg, clusterPaths, &opts)
		if err != nil {
			return result, err
		}
		selectedSteps, ignoreState, err := s.filterSteps(steps, &opts)
		if err != nil {
			return result, err
		}
		result.Plan = s.buildDryRunPlan(&cfg, clusterPaths, runtimePaths, &opts, selectedSteps, filterDescription(&opts, ignoreState))
		result.ResumeStatePath = runtimePaths.StatePath
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Provision infrastructure
	s.progress("\nProvisioning infrastructure (%s)...", provider)
	logging.Debugf("bootstrap: starting infrastructure provisioning for provider %s", provider)
	if err := s.provisionInfrastructure(ctx, &cfg, clusterPaths, &opts, runtimePaths, result); err != nil {
		logBootstrapMessage(ctx, "bootstrap failed during infrastructure provisioning: %v", err)
		return result, fmt.Errorf("provisioning infrastructure: %w", err)
	}
	result.InfrastructureProvisioned = true
	s.progress("✓ Infrastructure provisioned")

	// Deploy cluster
	s.progress("\nDeploying cluster...")
	logging.Debug("bootstrap: starting cluster deployment")
	if err := s.deployCluster(ctx, &cfg, clusterPaths, &opts, result); err != nil {
		logBootstrapMessage(ctx, "bootstrap failed during cluster deployment: %v", err)
		return result, fmt.Errorf("deploying cluster: %w", err)
	}
	result.ClusterDeployed = true
	s.progress("✓ Cluster deployed")

	// Wait for cluster to be ready
	s.progress("\nWaiting for cluster readiness (timeout: %s)...", opts.Timeout)
	logging.Debugf("bootstrap: waiting for cluster readiness, timeout=%s", opts.Timeout)
	endpoint, err := s.waitForReady(ctx, &cfg, opts.Timeout, opts.KubeconfigPath)
	if err != nil {
		logBootstrapMessage(ctx, "bootstrap failed while waiting for readiness: %v", err)
		return result, fmt.Errorf("waiting for cluster ready: %w", err)
	}
	result.ClusterReady = true
	result.Endpoint = endpoint
	s.progress("✓ Cluster ready at %s", endpoint)

	result.Duration = time.Since(startTime)
	if err := s.removeBootstrapState(runtimePaths.StatePath); err != nil {
		logBootstrapMessage(ctx, "warning: failed to remove bootstrap state %s: %v", runtimePaths.StatePath, err)
	}
	if err := s.removeBootstrapState(runtimePaths.LegacyStatePath); err != nil {
		logBootstrapMessage(ctx, "warning: failed to remove legacy bootstrap state %s: %v", runtimePaths.LegacyStatePath, err)
	}
	result.ResumeStatePath = ""
	logBootstrapMessage(ctx, "bootstrap completed in %s", result.Duration.Round(time.Second))
	return result, nil
}

// provisionInfrastructure provisions the infrastructure for the cluster
func (s *BootstrapService) provisionInfrastructure(ctx context.Context, cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions, runtimePaths *bootstrapRuntimePaths, result *BootstrapResult) error {
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

	steps, err := s.buildBootstrapSteps(cfg, clusterPaths, opts)
	if err != nil {
		return err
	}

	// Filter steps based on options
	selectedSteps, ignoreState, err := s.filterSteps(steps, opts)
	if err != nil {
		return err
	}

	return s.executeBootstrapSteps(ctx, selectedSteps, ignoreState, stateEnabled, statePath, state, result, opts)
}

func (s *BootstrapService) buildBootstrapSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	if provider == "" {
		provider = "openstack"
	}

	switch provider {
	case "openstack":
		providerImpl := newOpenStackBootstrapProvider(s.runner)
		return providerImpl.BuildSteps(cfg, clusterPaths, opts)

	case "aws", "gcp", "azure":
		clusterDir, err := infrastructureClusterDir(cfg)
		if err != nil {
			return nil, err
		}
		env := buildBootstrapEnvironment(opts.KubeconfigPath)

		return []bootstrapStep{
			{
				ID:          "make-terraform",
				Description: "Run make terraform",
				Plan: BootstrapPlanStep{
					ID:         "make-terraform",
					Action:     "Run make terraform",
					WorkingDir: clusterDir,
					Commands:   []BootstrapPlanCommand{commandPlan("make", "terraform")},
					Environment: envPlanFromMap(map[string]string{
						"KUBECONFIG": opts.KubeconfigPath,
						"PATH":       "<current PATH>",
					}, nil),
				},
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "make", "terraform")
				},
			},
			{
				ID:          "terraform-init",
				Description: "Initialize Terraform",
				Plan: BootstrapPlanStep{
					ID:          "terraform-init",
					Action:      "Initialize Terraform",
					WorkingDir:  clusterDir,
					Commands:    []BootstrapPlanCommand{commandPlan("terraform", "init")},
					Environment: envPlanFromMap(map[string]string{"KUBECONFIG": opts.KubeconfigPath, "PATH": "<current PATH>"}, nil),
				},
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "terraform", "init")
				},
			},
			{
				ID:          "terraform-apply",
				Description: "Apply Terraform configuration",
				Plan: BootstrapPlanStep{
					ID:          "terraform-apply",
					Action:      "Apply Terraform configuration",
					WorkingDir:  clusterDir,
					Commands:    []BootstrapPlanCommand{commandPlan("terraform", "apply", "-auto-approve")},
					Environment: envPlanFromMap(map[string]string{"KUBECONFIG": opts.KubeconfigPath, "PATH": "<current PATH>"}, nil),
				},
				Run: func(ctx context.Context) error {
					return s.runCommand(ctx, clusterDir, env, "terraform", "apply", "-auto-approve")
				},
			},
		}, nil

	case "kind":
		providerImpl := newKindBootstrapProvider(s.runner)
		return providerImpl.BuildSteps(cfg, clusterPaths, opts)

	case "vmware", "vsphere", "baremetal":
		providerImpl := newOpenStackBootstrapProvider(s.runner)
		return providerImpl.BuildSteps(cfg, clusterPaths, opts)

	default:
		if opts.DryRun {
			return nil, fmt.Errorf("deploy planning is not available for provider %q", provider)
		}
		return nil, fmt.Errorf("unsupported provider %q", provider)
	}
}

func (s *BootstrapService) executeBootstrapSteps(ctx context.Context, selectedSteps []bootstrapStep, ignoreState bool, stateEnabled bool, statePath string, state *bootstrapState, result *BootstrapResult, opts *BootstrapOptions) error {
	totalSteps := len(selectedSteps)
	logging.Debugf("bootstrap: executing %d step(s) (ignoreState=%v)", totalSteps, ignoreState)

	// Execute steps
	for i, step := range selectedSteps {
		// Skip if already completed (unless ignoring state)
		if !ignoreState && stateEnabled && s.isStepSuccess(state, step.ID) {
			s.progress("  [%d/%d] ⏭ %s (%s) (already completed)", i+1, totalSteps, step.Description, step.ID)
			logging.Debugf("bootstrap: step %s skipped (already completed in saved state)", step.ID)
			logBootstrapMessage(ctx, "step skipped from saved state: %s", step.ID)
			continue
		}

		// Mark step as running
		if stateEnabled {
			s.setStepStatus(state, step.ID, bootstrapStatusRunning, "")
			if err := s.saveBootstrapState(statePath, state); err != nil {
				return err
			}
		}
		if opts != nil && opts.Debug {
			s.printStepDebug(step)
		}
		s.progress("  [%d/%d] → %s (%s)...", i+1, totalSteps, step.Description, step.ID)
		logging.Debugf("bootstrap: step %s started - %s", step.ID, step.Description)
		logBootstrapMessage(ctx, "step started: %s - %s", step.ID, step.Description)

		stepStart := time.Now()

		// Execute step
		if err := step.Run(ctx); err != nil {
			stepDuration := time.Since(stepStart).Round(time.Millisecond)
			// Mark step as failed
			if stateEnabled {
				s.setStepStatus(state, step.ID, bootstrapStatusFailed, err.Error())
				if saveErr := s.saveBootstrapState(statePath, state); saveErr != nil {
					return saveErr
				}
			}
			result.StepsFailed = append(result.StepsFailed, step.ID)
			result.ResumeStatePath = statePath
			s.progress("  [%d/%d] ✗ %s (%s) failed after %s: %v", i+1, totalSteps, step.Description, step.ID, stepDuration, err)
			logging.Debugf("bootstrap: step %s failed after %s: %v", step.ID, stepDuration, err)
			logBootstrapMessage(ctx, "step failed: %s: %v", step.ID, err)
			return fmt.Errorf("step %q failed: %w", step.ID, err)
		}

		stepDuration := time.Since(stepStart).Round(time.Millisecond)

		// Mark step as successful
		if stateEnabled {
			s.setStepStatus(state, step.ID, bootstrapStatusSuccess, "")
			if err := s.saveBootstrapState(statePath, state); err != nil {
				return err
			}
		}
		result.StepsCompleted = append(result.StepsCompleted, step.ID)
		s.progress("  [%d/%d] ✓ %s (%s) (%s)", i+1, totalSteps, step.Description, step.ID, stepDuration)
		logging.Debugf("bootstrap: step %s completed in %s", step.ID, stepDuration)
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

	case "openstack", "aws", "gcp", "azure", "vmware", "vsphere", "baremetal":
		// Cloud/static providers deploy via Terraform in provisionInfrastructure
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

	case "openstack", "aws", "gcp", "azure", "vmware", "vsphere", "baremetal":
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

	attempt := 0
	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for cluster to be ready: %w", ctx.Err())

		case <-ticker.C:
			attempt++
			logging.Debugf("bootstrap: cluster readiness check attempt %d", attempt)
			s.progress("  Checking cluster readiness (attempt %d)...", attempt)
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
func (s *BootstrapService) filterSteps(steps []bootstrapStep, opts *BootstrapOptions) ([]bootstrapStep, bool, error) {
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
			return nil, true, missingStepError(opts.OnlyStep, steps)
		}
		return []bootstrapStep{steps[idx]}, true, nil
	}

	// Filter from step onwards
	if strings.TrimSpace(opts.FromStep) != "" {
		idx, ok := stepIndex[opts.FromStep]
		if !ok {
			return nil, true, missingStepError(opts.FromStep, steps)
		}
		return steps[idx:], true, nil
	}

	return steps, ignoreState, nil
}

// runCommand executes a command in the specified directory
func (s *BootstrapService) runCommand(ctx context.Context, dir string, env map[string]string, name string, args ...string) error {
	_, err := s.runner.Run(ctx, dir, env, name, args...)
	return err
}

// runCommandWithInput executes a command with stdin input

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
