package cluster

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rackerlabs/opencenter-cli/internal/config"
	"github.com/rackerlabs/opencenter-cli/internal/core/paths"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

func TestBootstrapService_Bootstrap(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver with test directory
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine with config validator
	validationEngine := validation.NewValidationEngine()
	configValidator := validators.NewConfigValidator()
	if err := validationEngine.Register(configValidator); err != nil {
		t.Fatalf("Failed to register config validator: %v", err)
	}

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

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
				cfg := config.Config{
					SchemaVersion: "2.0",
				}
				cfg.OpenCenter.Meta.Organization = "test-org"
				cfg.OpenCenter.Infrastructure.Provider = "kind"
				cfg.OpenCenter.Cluster.ClusterName = clusterName
				cfg.OpenCenter.GitOps.GitDir = filepath.Join(tmpDir, "gitops")

				// Save config
				if err := config.Save(cfg); err != nil {
					t.Fatalf("Failed to save config: %v", err)
				}

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
				cfg := config.Config{
					SchemaVersion: "2.0",
				}
				cfg.OpenCenter.Meta.Organization = "test-org"
				cfg.OpenCenter.Infrastructure.Provider = "kind"
				cfg.OpenCenter.Cluster.ClusterName = clusterName
				cfg.OpenCenter.GitOps.GitDir = filepath.Join(tmpDir, "gitops")

				// Save config
				if err := config.Save(cfg); err != nil {
					t.Fatalf("Failed to save config: %v", err)
				}

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

func TestBootstrapService_resolveContainerRuntime(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := validation.NewValidationEngine()

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

	tests := []struct {
		name      string
		flagValue string
		envVars   map[string]string
		want      string
	}{
		{
			name:      "flag value takes precedence",
			flagValue: "podman",
			envVars: map[string]string{
				"CONTAINER_RUNTIME": "docker",
			},
			want: "podman",
		},
		{
			name:      "CONTAINER_RUNTIME env var",
			flagValue: "",
			envVars: map[string]string{
				"CONTAINER_RUNTIME": "podman",
			},
			want: "podman",
		},
		{
			name:      "KIND_EXPERIMENTAL_PROVIDER env var",
			flagValue: "",
			envVars: map[string]string{
				"KIND_EXPERIMENTAL_PROVIDER": "podman",
			},
			want: "podman",
		},
		{
			name:      "default to docker",
			flagValue: "",
			envVars:   map[string]string{},
			want:      "docker",
		},
		{
			name:      "uppercase flag value",
			flagValue: "PODMAN",
			envVars:   map[string]string{},
			want:      "podman",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variables
			for k, v := range tt.envVars {
				os.Setenv(k, v)
				defer os.Unsetenv(k)
			}

			got := bootstrapService.resolveContainerRuntime(tt.flagValue)
			if got != tt.want {
				t.Errorf("resolveContainerRuntime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBootstrapService_buildEnvironment(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := validation.NewValidationEngine()

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

	tests := []struct {
		name           string
		kubeconfigPath string
		wantKubeconfig bool
		wantPath       bool
	}{
		{
			name:           "with kubeconfig",
			kubeconfigPath: "/path/to/kubeconfig",
			wantKubeconfig: true,
			wantPath:       true,
		},
		{
			name:           "without kubeconfig",
			kubeconfigPath: "",
			wantKubeconfig: false,
			wantPath:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set PATH environment variable
			os.Setenv("PATH", "/usr/bin:/bin")
			defer os.Unsetenv("PATH")

			env := bootstrapService.buildEnvironment(tt.kubeconfigPath)

			if tt.wantKubeconfig {
				if env["KUBECONFIG"] != tt.kubeconfigPath {
					t.Errorf("buildEnvironment() KUBECONFIG = %v, want %v", env["KUBECONFIG"], tt.kubeconfigPath)
				}
			} else {
				if _, ok := env["KUBECONFIG"]; ok {
					t.Error("buildEnvironment() set KUBECONFIG when not expected")
				}
			}

			if tt.wantPath {
				if env["PATH"] == "" {
					t.Error("buildEnvironment() did not preserve PATH")
				}
			}
		})
	}
}

func TestBootstrapService_buildKindEnvironment(t *testing.T) {
	// Create temporary directory for test
	tmpDir := t.TempDir()

	// Create path resolver
	pathResolver := paths.NewPathResolver(tmpDir)

	// Create validation engine
	validationEngine := validation.NewValidationEngine()

	// Create bootstrap service
	bootstrapService := NewBootstrapService(pathResolver, validationEngine)

	tests := []struct {
		name    string
		runtime string
		wantEnv map[string]string
	}{
		{
			name:    "podman runtime",
			runtime: "podman",
			wantEnv: map[string]string{
				"KIND_EXPERIMENTAL_PROVIDER": "podman",
			},
		},
		{
			name:    "docker runtime",
			runtime: "docker",
			wantEnv: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set PATH environment variable
			os.Setenv("PATH", "/usr/bin:/bin")
			defer os.Unsetenv("PATH")

			env := bootstrapService.buildKindEnvironment(tt.runtime)

			for k, v := range tt.wantEnv {
				if env[k] != v {
					t.Errorf("buildKindEnvironment() %s = %v, want %v", k, env[k], v)
				}
			}

			// Verify PATH is preserved
			if env["PATH"] == "" {
				t.Error("buildKindEnvironment() did not preserve PATH")
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
			filtered, ignoreState := bootstrapService.filterSteps(steps, &tt.opts)

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
