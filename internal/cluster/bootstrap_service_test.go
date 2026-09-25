package cluster

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation/validators"
	testhelpers "github.com/opencenter-cloud/opencenter-cli/internal/testing"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
)

// createTestBootstrapService creates a BootstrapService with test dependencies
func createTestBootstrapService(pathResolver *paths.PathResolver) *BootstrapService {
	errorHandler := errors.NewDefaultErrorHandlerWithoutMasking()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validation.NewValidationEngine()
	configValidator := validators.NewConfigValidator()
	validator.Register(configValidator)
	cache := v2.NewConfigCache()
	loader := v2.NewConfigIOHandler(fileSystem)
	configMgr := config.NewConfigurationManagerWithDeps(loader, validator, cache, pathResolver, fileSystem)

	return NewBootstrapServiceWithConfigMgr(pathResolver, validator, configMgr, fileSystem)
}

func TestBootstrapService_Bootstrap(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver with test directory
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create bootstrap service with test dependencies
	bootstrapService := createTestBootstrapService(pathResolver)

	tests := []struct {
		name    string
		opts    BootstrapOptions
		wantErr bool
		setup   func(t *testing.T) string // Returns cluster name
	}{
		{
			name: "dry run mode",
			opts: BootstrapOptions{
				ClusterName:    "test-cluster",
				Organization:   "test-org",
				DryRun:         true,
				SkipValidation: true,
				Timeout:        5 * time.Second,
			},
			wantErr: false,
			setup: func(t *testing.T) string {
				clusterName := "test-cluster"
				ctx := context.Background()

				// Create cluster directories
				if err := pathResolver.CreateClusterDirectories(ctx, clusterName, "test-org"); err != nil {
					t.Fatalf("Failed to create cluster directories: %v", err)
				}

				// Create a minimal config file
				_, err := pathResolver.Resolve(ctx, clusterName, "test-org")
				if err != nil {
					t.Fatalf("Failed to resolve cluster paths: %v", err)
				}

				// Create minimal config
				cfg := mustNewClusterTestConfig(clusterName, "kind")
				cfg.OpenCenter.Meta.Organization = "test-org"
				cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops")

				// Save config
				testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

				return clusterName
			},
		},
		{
			name: "skip validation",
			opts: BootstrapOptions{
				ClusterName:    "test-cluster-2",
				Organization:   "test-org",
				DryRun:         true,
				SkipValidation: true,
				Timeout:        5 * time.Second,
			},
			wantErr: false,
			setup: func(t *testing.T) string {
				clusterName := "test-cluster-2"
				ctx := context.Background()

				// Create cluster directories
				if err := pathResolver.CreateClusterDirectories(ctx, clusterName, "test-org"); err != nil {
					t.Fatalf("Failed to create cluster directories: %v", err)
				}

				// Create a minimal config file
				_, err := pathResolver.Resolve(ctx, clusterName, "test-org")
				if err != nil {
					t.Fatalf("Failed to resolve cluster paths: %v", err)
				}

				// Create minimal config
				cfg := mustNewClusterTestConfig(clusterName, "kind")
				cfg.OpenCenter.Meta.Organization = "test-org"
				cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops")

				// Save config
				testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

				return clusterName
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run setup if provided
			if tt.setup != nil {
				clusterName := tt.setup(t)
				tt.opts.ClusterName = clusterName
			}

			ctx := context.Background()
			result, err := bootstrapService.Bootstrap(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("Bootstrap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if result == nil {
					t.Error("Bootstrap() returned nil result")
					return
				}

				// In dry-run mode, nothing should be provisioned
				if tt.opts.DryRun {
					if result.InfrastructureProvisioned {
						t.Error("Bootstrap() provisioned infrastructure in dry-run mode")
					}
					if result.ClusterDeployed {
						t.Error("Bootstrap() deployed cluster in dry-run mode")
					}
					if result.ClusterReady {
						t.Error("Bootstrap() marked cluster as ready in dry-run mode")
					}
				}

				if result.Duration == 0 {
					t.Error("Bootstrap() returned zero duration")
				}
			}
		})
	}
}

func TestBootstrapService_filterSteps(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := validation.NewValidationEngine()

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

	// Create test steps
	steps := []bootstrapStep{
		{ID: "step1", Description: "Step 1"},
		{ID: "step2", Description: "Step 2"},
		{ID: "step3", Description: "Step 3"},
	}

	tests := []struct {
		name        string
		opts        BootstrapOptions
		wantCount   int
		wantIgnore  bool
		wantFirstID string
	}{
		{
			name: "all steps",
			opts: BootstrapOptions{
				OnlyStep: "",
				FromStep: "",
				Restart:  false,
			},
			wantCount:   3,
			wantIgnore:  false,
			wantFirstID: "step1",
		},
		{
			name: "only step",
			opts: BootstrapOptions{
				OnlyStep: "step2",
				FromStep: "",
				Restart:  false,
			},
			wantCount:   1,
			wantIgnore:  true,
			wantFirstID: "step2",
		},
		{
			name: "from step",
			opts: BootstrapOptions{
				OnlyStep: "",
				FromStep: "step2",
				Restart:  false,
			},
			wantCount:   2,
			wantIgnore:  true,
			wantFirstID: "step2",
		},
		{
			name: "restart",
			opts: BootstrapOptions{
				OnlyStep: "",
				FromStep: "",
				Restart:  true,
			},
			wantCount:   3,
			wantIgnore:  true,
			wantFirstID: "step1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered, ignoreState, err := bootstrapService.filterSteps(steps, &tt.opts)
			if err != nil {
				t.Fatalf("filterSteps() unexpected error: %v", err)
			}

			if len(filtered) != tt.wantCount {
				t.Errorf("filterSteps() returned %d steps, want %d", len(filtered), tt.wantCount)
			}

			if ignoreState != tt.wantIgnore {
				t.Errorf("filterSteps() ignoreState = %v, want %v", ignoreState, tt.wantIgnore)
			}

			if len(filtered) > 0 && filtered[0].ID != tt.wantFirstID {
				t.Errorf("filterSteps() first step ID = %v, want %v", filtered[0].ID, tt.wantFirstID)
			}
		})
	}
}

func TestBootstrapService_executeBootstrapStepsPrintsDebugPreambleBeforeStepRun(t *testing.T) {
	pathResolver := paths.NewPathResolver(t.TempDir())
	bootstrapService := NewBootstrapService(pathResolver, validation.NewValidationEngine())
	out := &bytes.Buffer{}
	bootstrapService.SetOutput(out)

	var runOutput string
	steps := []bootstrapStep{
		{
			ID:          "terraform-init",
			Description: "Initialize Terraform",
			Plan: BootstrapPlanStep{
				ID:         "terraform-init",
				Action:     "Initialize Terraform",
				WorkingDir: "/tmp/opencenter/cluster",
				Commands: []BootstrapPlanCommand{
					commandPlan("terraform", "init"),
				},
				Environment: []BootstrapPlanEnv{
					{Name: "KUBECONFIG", Value: "/tmp/opencenter/kubeconfig"},
					{Name: "OS_PASSWORD", Redacted: true},
					{Name: "PATH", Value: "/usr/local/bin:/usr/bin"},
				},
			},
			Run: func(ctx context.Context) error {
				runOutput = out.String()
				return nil
			},
		},
	}

	result := &BootstrapResult{}
	state := bootstrapService.newBootstrapState()
	err := bootstrapService.executeBootstrapSteps(context.Background(), steps, false, false, "", state, result, &BootstrapOptions{Debug: true})
	if err != nil {
		t.Fatalf("executeBootstrapSteps() error = %v", err)
	}

	for _, want := range []string{
		"-----",
		`Step: "Initialize Terraform"`,
		"Environment:",
		"  KUBECONFIG=/tmp/opencenter/kubeconfig",
		"  OS_PASSWORD=<redacted>",
		"  PATH=/usr/local/bin:/usr/bin",
		"PATH: /tmp/opencenter/cluster",
		"Command: terraform init",
	} {
		if !strings.Contains(runOutput, want) {
			t.Fatalf("expected debug preamble before step run to contain %q, got:\n%s", want, runOutput)
		}
	}
	if strings.Contains(runOutput, "✓ Initialize Terraform") {
		t.Fatalf("debug preamble should be printed before the step completes, got:\n%s", runOutput)
	}
}

func TestBootstrapService_executeBootstrapStepsNeverSkipRunsDespiteState(t *testing.T) {
	pathResolver := paths.NewPathResolver(t.TempDir())
	bootstrapService := NewBootstrapService(pathResolver, validation.NewValidationEngine())

	var executed []string
	steps := []bootstrapStep{
		{
			ID:          "preflight",
			Description: "Preflight checks",
			Run: func(ctx context.Context) error {
				executed = append(executed, "preflight")
				return nil
			},
		},
		{
			ID:          "opentofu-init",
			Description: "Initialize OpenTofu",
			NeverSkip:   true,
			Run: func(ctx context.Context) error {
				executed = append(executed, "opentofu-init")
				return nil
			},
		},
		{
			ID:          "opentofu-apply",
			Description: "Apply OpenTofu infrastructure",
			Run: func(ctx context.Context) error {
				executed = append(executed, "opentofu-apply")
				return nil
			},
		},
	}

	// Simulate stale state where all steps were previously marked as success
	state := bootstrapService.newBootstrapState()
	bootstrapService.setStepStatus(state, "preflight", bootstrapStatusSuccess, "")
	bootstrapService.setStepStatus(state, "opentofu-init", bootstrapStatusSuccess, "")
	bootstrapService.setStepStatus(state, "opentofu-apply", bootstrapStatusSuccess, "")

	result := &BootstrapResult{}
	// stateEnabled=true, ignoreState=false — normal resume scenario
	err := bootstrapService.executeBootstrapSteps(context.Background(), steps, false, true, "", state, result, &BootstrapOptions{})
	if err != nil {
		t.Fatalf("executeBootstrapSteps() error = %v", err)
	}

	// preflight and opentofu-apply should be skipped (state says success)
	// opentofu-init should still run because NeverSkip=true
	if len(executed) != 1 {
		t.Fatalf("expected 1 step to run, got %d: %v", len(executed), executed)
	}
	if executed[0] != "opentofu-init" {
		t.Errorf("expected opentofu-init to run, got %q", executed[0])
	}
}

func TestBootstrapService_OpenStackDryRunDoesNotUseLegacyConfigValidator(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "openstack-bootstrap"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("Failed to create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")

	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:    clusterName,
		Organization:   organization,
		DryRun:         true,
		SkipValidation: false,
		Timeout:        5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Bootstrap() unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected bootstrap result")
		return
	}
	if result.InfrastructureProvisioned || result.ClusterDeployed || result.ClusterReady {
		t.Fatalf("dry-run bootstrap should not mark provisioning complete: %#v", result)
	}
}

func TestBootstrapService_DryRunKindBuildsProviderPlan(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "kind-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:      clusterName,
		Organization:     organization,
		DryRun:           true,
		SkipValidation:   false,
		ContainerRuntime: "docker",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	if result.Plan.Provider != "kind" {
		t.Fatalf("provider = %q, want kind", result.Plan.Provider)
	}
	wantIDs := []string{"kind-create", "kind-export-kubeconfig", "kind-install-cni", "gitea-attach-kind", "flux-bootstrap", "reconcile-sops-age-secret", "reconcile-grafana-admin-secret", "gitea-rebase", "gitops-push", "flux-verify"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("plan steps = %v, want %v", got, wantIDs)
	}
	create := result.Plan.Steps[0]
	if len(create.Commands) != 2 {
		t.Fatalf("kind-create commands = %#v, want two commands", create.Commands)
	}
	if create.Commands[0].Name != "kind" || strings.Join(create.Commands[0].Args, " ") != "get clusters" {
		t.Fatalf("unexpected first kind-create command: %#v", create.Commands[0])
	}
	if !containsString(create.Reads, filepath.Join(tmpDir, "gitops", organization, "infrastructure", "clusters", clusterName, "kind-config.yaml")) {
		t.Fatalf("kind-create reads missing kind config path: %#v", create.Reads)
	}
}

func TestBootstrapService_DryRunKindGitHubProviderPlanSkipsGitea(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "kind-github-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	// Select an external GitHub GitOps provider instead of the local Gitea
	// default. The plan must then drop the Gitea-only steps.
	cfg.OpenCenter.GitOps.Repository.URL = "https://github.com/example-org/gitops-repo.git"
	cfg.OpenCenter.GitOps.Auth.Token = &v2.GitOpsTokenAuth{
		Provider:  "github",
		TokenFile: "secrets/github-token.txt",
		Owner:     "example-org",
	}
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:      clusterName,
		Organization:     organization,
		DryRun:           true,
		SkipValidation:   false,
		ContainerRuntime: "docker",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	if result.Plan.Provider != "kind" {
		t.Fatalf("provider = %q, want kind", result.Plan.Provider)
	}
	// Gitea-only steps (gitea-attach-kind, gitea-rebase, gitops-push) must be
	// absent for an external GitHub provider; Flux bootstraps straight to the
	// remote.
	wantIDs := []string{"kind-create", "kind-export-kubeconfig", "kind-install-cni", "flux-bootstrap", "reconcile-sops-age-secret", "reconcile-grafana-admin-secret", "flux-verify"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("plan steps = %v, want %v", got, wantIDs)
	}
	for _, step := range result.Plan.Steps {
		switch step.ID {
		case "gitea-attach-kind", "gitea-rebase", "gitops-push":
			t.Fatalf("gitea-only step %q must not be present for github provider", step.ID)
		}
	}
	// The flux-bootstrap step should reference github, not local Gitea.
	var fluxStep *BootstrapPlanStep
	for i := range result.Plan.Steps {
		if result.Plan.Steps[i].ID == "flux-bootstrap" {
			fluxStep = &result.Plan.Steps[i]
			break
		}
	}
	if fluxStep == nil {
		t.Fatal("expected flux-bootstrap step in plan")
		return
	}
	if strings.Contains(strings.ToLower(fluxStep.Action), "gitea") {
		t.Fatalf("flux-bootstrap action should not mention gitea for github provider: %q", fluxStep.Action)
	}
}

func TestBootstrapService_DryRunOpenStackBuildsPlanWithoutPrerequisites(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "openstack-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	gitDir := filepath.Join(tmpDir, "gitops-repo")
	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run should not require credentials or infra dir: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	wantIDs := []string{"preflight", "opentofu-init", "opentofu-apply", "openstack-normalize-kubeconfig", "openstack-install-network-plugin"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("plan steps = %v, want %v", got, wantIDs)
	}
	initStep := result.Plan.Steps[1]
	wantDir := filepath.Join(gitDir, "infrastructure", "clusters", clusterName)
	if initStep.WorkingDir != wantDir {
		t.Fatalf("opentofu-init working dir = %q, want %q", initStep.WorkingDir, wantDir)
	}
	if len(initStep.Commands) != 1 || (initStep.Commands[0].Name != "tofu" && initStep.Commands[0].Name != "terraform") || strings.Join(initStep.Commands[0].Args, " ") != "init" {
		t.Fatalf("unexpected opentofu-init command: %#v", initStep.Commands)
	}
	if !envHasRedacted(initStep.Environment, "OS_APPLICATION_CREDENTIAL_SECRET") {
		t.Fatalf("expected redacted OpenStack credential env, got %#v", initStep.Environment)
	}
}

func TestBootstrapService_DryRunOpenStackStepFiltersNetworkPluginPlan(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "openstack-filter-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "openstack")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
		OnlyStep:     "openstack-install-network-plugin",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run with --step error: %v", err)
	}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != "openstack-install-network-plugin" {
		t.Fatalf("--step plan steps = %v, want [openstack-install-network-plugin]", got)
	}
	if !strings.Contains(result.Plan.Filter, "--step openstack-install-network-plugin") {
		t.Fatalf("expected filter description, got %q", result.Plan.Filter)
	}
	networkPluginPlan := renderPlanCommands(result.Plan.Steps[0].Commands)
	for _, want := range []string{
		"helm repo add projectcalico",
		"helm upgrade --install calico projectcalico/tigera-operator",
		"--namespace tigera-operator",
		"--create-namespace",
		"tigerastatus/calico",
	} {
		if !strings.Contains(networkPluginPlan, want) {
			t.Fatalf("expected Calico dry-run plan to contain %q, got:\n%s", want, networkPluginPlan)
		}
	}

	result, err = bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
		FromStep:     "openstack-install-network-plugin",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run with --from-step error: %v", err)
	}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != "openstack-install-network-plugin" {
		t.Fatalf("--from-step plan steps = %v, want [openstack-install-network-plugin]", got)
	}
	if !strings.Contains(result.Plan.Filter, "--from-step openstack-install-network-plugin") {
		t.Fatalf("expected filter description, got %q", result.Plan.Filter)
	}
}

func renderPlanCommands(commands []BootstrapPlanCommand) string {
	var b strings.Builder
	for _, command := range commands {
		b.WriteString(command.Name)
		if len(command.Args) > 0 {
			b.WriteByte(' ')
			b.WriteString(strings.Join(command.Args, " "))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func TestBootstrapService_DryRunStepFiltersPlan(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "filter-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
		OnlyStep:     "flux-bootstrap",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run with --step error: %v", err)
	}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != "flux-bootstrap" {
		t.Fatalf("--step plan steps = %v, want [flux-bootstrap]", got)
	}
	if !strings.Contains(result.Plan.Filter, "--step flux-bootstrap") {
		t.Fatalf("expected filter description, got %q", result.Plan.Filter)
	}

	result, err = bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
		FromStep:     "gitea-rebase",
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run with --from-step error: %v", err)
	}
	wantIDs := []string{"gitea-rebase", "gitops-push", "flux-verify"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("--from-step plan steps = %v, want %v", got, wantIDs)
	}
	if !strings.Contains(result.Plan.Filter, "--from-step gitea-rebase") {
		t.Fatalf("expected filter description, got %q", result.Plan.Filter)
	}
}

func TestBootstrapService_DryRunUnknownStepReturnsAvailableSteps(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "unknown-step-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)
	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	_, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
		OnlyStep:     "missing-step",
	})
	if err == nil {
		t.Fatal("expected unknown step error")
	}
	if !strings.Contains(err.Error(), `unknown bootstrap step "missing-step"`) || !strings.Contains(err.Error(), "kind-create") {
		t.Fatalf("expected unknown step error with available steps, got: %v", err)
	}
}

func TestBootstrapService_DryRunDoesNotWriteRuntimeFilesOrExecuteSteps(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")
	t.Setenv("OPENCENTER_STATE_DIR", stateDir)

	clusterName := "no-side-effects-plan"
	organization := "test-org"
	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)
	bootstrapService.runner = failingLifecycleRunner{t: t}

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "kind")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	if _, err := os.Stat(result.Plan.LogPath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create log file %s: %v", result.Plan.LogPath, err)
	}
	if _, err := os.Stat(result.Plan.ResumeStatePath); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create state file %s: %v", result.Plan.ResumeStatePath, err)
	}
}

type failingLifecycleRunner struct {
	t *testing.T
}

func (r failingLifecycleRunner) Run(ctx context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	r.t.Fatalf("dry-run must not execute lifecycle command %s %v in %s", name, args, dir)
	return nil, nil
}

func planStepIDs(plan *BootstrapPlan) []string {
	if plan == nil {
		return nil
	}
	ids := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		ids = append(ids, step.ID)
	}
	return ids
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestBootstrapService_DryRunVMwareBuildsPlan(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "vmware-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "vmware")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	if result.Plan.Provider != "vmware" {
		t.Fatalf("provider = %q, want vmware", result.Plan.Provider)
	}
	wantIDs := []string{"preflight", "opentofu-init", "opentofu-apply", "openstack-normalize-kubeconfig", "openstack-install-network-plugin"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("plan steps = %v, want %v", got, wantIDs)
	}
	// VMware plan should have VSPHERE env vars redacted
	initStep := result.Plan.Steps[1]
	if !envHasRedacted(initStep.Environment, "VSPHERE_PASSWORD") {
		t.Fatalf("expected redacted VSPHERE_PASSWORD env, got %#v", initStep.Environment)
	}
}

func TestValidateStaticNodes(t *testing.T) {
	tests := []struct {
		name    string
		cfg     func() *v2.Config
		wantErr string
	}{
		{
			name: "valid vmware with cloud nodes",
			cfg: func() *v2.Config {
				cfg := mustNewClusterTestConfig("test", "vmware")
				return &cfg
			},
			wantErr: "",
		},
		{
			name: "valid baremetal with master nodes",
			cfg: func() *v2.Config {
				cfg := mustNewClusterTestConfig("test", "baremetal")
				cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []v2.StaticNode{
					{Name: "master-1", AccessIPv4: "10.0.0.1"},
				}
				cfg.OpenCenter.Infrastructure.SSH.User = "ubuntu"
				return &cfg
			},
			wantErr: "",
		},
		{
			name: "missing master nodes for baremetal",
			cfg: func() *v2.Config {
				cfg := mustNewClusterTestConfig("test", "baremetal")
				cfg.OpenCenter.Infrastructure.Compute.MasterNodes = nil
				cfg.OpenCenter.Infrastructure.Cloud.VMware = nil
				return &cfg
			},
			wantErr: "no master nodes defined",
		},
		{
			name: "missing ssh user",
			cfg: func() *v2.Config {
				cfg := mustNewClusterTestConfig("test", "baremetal")
				cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []v2.StaticNode{
					{Name: "master-1", AccessIPv4: "10.0.0.1"},
				}
				cfg.OpenCenter.Infrastructure.SSH.User = ""
				cfg.OpenCenter.Infrastructure.SSH.Username = ""
				return &cfg
			},
			wantErr: "ssh user must be set",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStaticNodes(tt.cfg())
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateStaticNodes() unexpected error: %v", err)
				}
			} else {
				if err == nil {
					t.Fatal("validateStaticNodes() expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("validateStaticNodes() error = %q, want containing %q", err.Error(), tt.wantErr)
				}
			}
		})
	}
}

func TestValidateProviderBootstrap_VMwareRequiresCredentials(t *testing.T) {
	cfg := mustNewClusterTestConfig("test", "vmware")
	// No vSphere secrets set
	cfg.Secrets.ServiceSecrets = nil

	err := validateProviderBootstrap(&cfg, "vmware")
	if err == nil {
		t.Fatal("expected error for missing vmware credentials")
	}
	if !strings.Contains(err.Error(), "vmware credentials incomplete") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateProviderBootstrap_BaremetalNoCredentials(t *testing.T) {
	cfg := mustNewClusterTestConfig("test", "baremetal")
	cfg.OpenCenter.Infrastructure.Compute.MasterNodes = []v2.StaticNode{
		{Name: "master-1", AccessIPv4: "10.0.0.1"},
	}
	cfg.OpenCenter.Infrastructure.SSH.User = "ubuntu"

	err := validateProviderBootstrap(&cfg, "baremetal")
	if err != nil {
		t.Fatalf("validateProviderBootstrap(baremetal) unexpected error: %v", err)
	}
}

func TestBootstrapService_DryRunBaremetalBuildsPlan(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "baremetal-plan"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "baremetal")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	if result.Plan.Provider != "baremetal" {
		t.Fatalf("provider = %q, want baremetal", result.Plan.Provider)
	}
	wantIDs := []string{"preflight", "opentofu-init", "opentofu-apply", "openstack-normalize-kubeconfig", "openstack-install-network-plugin"}
	if got := planStepIDs(result.Plan); strings.Join(got, ",") != strings.Join(wantIDs, ",") {
		t.Fatalf("plan steps = %v, want %v", got, wantIDs)
	}
	// Baremetal plan should NOT have VSPHERE or OS_ env vars
	initStep := result.Plan.Steps[1]
	if envHasRedacted(initStep.Environment, "OS_APPLICATION_CREDENTIAL_SECRET") {
		t.Fatalf("baremetal plan should not have OpenStack env vars, got %#v", initStep.Environment)
	}
	if envHasRedacted(initStep.Environment, "VSPHERE_PASSWORD") {
		t.Fatalf("baremetal plan should not have vSphere env vars, got %#v", initStep.Environment)
	}
}

func TestBootstrapService_VMwareNoGitOpsPush(t *testing.T) {
	tmpDir := t.TempDir()
	clusterName := "vmware-no-push"
	organization := "test-org"

	pathResolver := paths.NewPathResolver(tmpDir)
	bootstrapService := createTestBootstrapService(pathResolver)

	ctx := context.Background()
	if err := pathResolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		t.Fatalf("create cluster directories: %v", err)
	}

	cfg := mustNewClusterTestConfig(clusterName, "vmware")
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.GitOps.Repository.LocalDir = filepath.Join(tmpDir, "gitops-repo")
	cfg.OpenCenter.GitOps.Repository.URL = "ssh://git@github.com/test-org/test-repo.git"
	testhelpers.SaveConfigWithPathResolver(t, cfg, pathResolver)

	result, err := bootstrapService.Bootstrap(ctx, BootstrapOptions{
		ClusterName:  clusterName,
		Organization: organization,
		DryRun:       true,
	})
	if err != nil {
		t.Fatalf("Bootstrap() dry-run error: %v", err)
	}
	if result.Plan == nil {
		t.Fatal("expected dry-run plan")
	}
	// Verify no gitops-push step exists
	for _, step := range result.Plan.Steps {
		if step.ID == "gitops-push" {
			t.Fatal("vmware provider should not include gitops-push step")
		}
	}
}

func envHasRedacted(values []BootstrapPlanEnv, name string) bool {
	for _, value := range values {
		if value.Name == name && value.Redacted {
			return true
		}
	}
	return false
}

func TestBootstrapService_bootstrapState(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := validation.NewValidationEngine()

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

	// Test state path
	statePath := filepath.Join(tmpDir, "bootstrap-state.json")

	t.Run("new state", func(t *testing.T) {
		state := bootstrapService.newBootstrapState()

		if state.Version != bootstrapStateVersion {
			t.Errorf("newBootstrapState() version = %v, want %v", state.Version, bootstrapStateVersion)
		}

		if state.Steps == nil {
			t.Error("newBootstrapState() steps is nil")
		}

		if len(state.Steps) != 0 {
			t.Errorf("newBootstrapState() steps count = %v, want 0", len(state.Steps))
		}
	})

	t.Run("set and check status", func(t *testing.T) {
		state := bootstrapService.newBootstrapState()

		// Set step as running
		bootstrapService.setStepStatus(state, "step1", bootstrapStatusRunning, "")

		if state.Steps["step1"].Status != bootstrapStatusRunning {
			t.Error("setStepStatus() did not set running status")
		}

		// Set step as success
		bootstrapService.setStepStatus(state, "step1", bootstrapStatusSuccess, "")

		if !bootstrapService.isStepSuccess(state, "step1") {
			t.Error("isStepSuccess() returned false for successful step")
		}

		// Set step as failed
		bootstrapService.setStepStatus(state, "step2", bootstrapStatusFailed, "error message")

		if bootstrapService.isStepSuccess(state, "step2") {
			t.Error("isStepSuccess() returned true for failed step")
		}

		if state.Steps["step2"].Error != "error message" {
			t.Errorf("setStepStatus() error = %v, want 'error message'", state.Steps["step2"].Error)
		}
	})

	t.Run("save and load state", func(t *testing.T) {
		state := bootstrapService.newBootstrapState()
		bootstrapService.setStepStatus(state, "step1", bootstrapStatusSuccess, "")
		bootstrapService.setStepStatus(state, "step2", bootstrapStatusFailed, "test error")

		// Save state
		if err := bootstrapService.saveBootstrapState(statePath, state); err != nil {
			t.Fatalf("saveBootstrapState() error = %v", err)
		}

		// Verify file exists
		if _, err := os.Stat(statePath); os.IsNotExist(err) {
			t.Error("saveBootstrapState() did not create state file")
		}

		// Load state
		loadedState, enabled, err := bootstrapService.loadBootstrapState(statePath)
		if err != nil {
			t.Fatalf("loadBootstrapState() error = %v", err)
		}

		if !enabled {
			t.Error("loadBootstrapState() returned enabled = false")
		}

		if loadedState.Version != state.Version {
			t.Errorf("loadBootstrapState() version = %v, want %v", loadedState.Version, state.Version)
		}

		if len(loadedState.Steps) != len(state.Steps) {
			t.Errorf("loadBootstrapState() steps count = %v, want %v", len(loadedState.Steps), len(state.Steps))
		}

		if loadedState.Steps["step1"].Status != bootstrapStatusSuccess {
			t.Error("loadBootstrapState() did not preserve step1 status")
		}

		if loadedState.Steps["step2"].Status != bootstrapStatusFailed {
			t.Error("loadBootstrapState() did not preserve step2 status")
		}

		if loadedState.Steps["step2"].Error != "test error" {
			t.Error("loadBootstrapState() did not preserve step2 error")
		}
	})

	t.Run("load non-existent state", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "non-existent-state.json")

		state, enabled, err := bootstrapService.loadBootstrapState(nonExistentPath)
		if err != nil {
			t.Fatalf("loadBootstrapState() error = %v", err)
		}

		if !enabled {
			t.Error("loadBootstrapState() returned enabled = false for non-existent file")
		}

		if state == nil {
			t.Error("loadBootstrapState() returned nil state")
			return
		}

		if len(state.Steps) != 0 {
			t.Errorf("loadBootstrapState() steps count = %v, want 0", len(state.Steps))
		}
	})

	t.Run("empty state path", func(t *testing.T) {
		state, enabled, err := bootstrapService.loadBootstrapState("")
		if err != nil {
			t.Fatalf("loadBootstrapState() error = %v", err)
		}

		if enabled {
			t.Error("loadBootstrapState() returned enabled = true for empty path")
		}

		if state == nil {
			t.Error("loadBootstrapState() returned nil state")
		}
	})
}
