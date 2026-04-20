package gitops

import (
	"strings"
	"testing"

	overlaycfg "github.com/opencenter-cloud/opencenter-cli/internal/config/overlay"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TestPlanClusterAppActionsRejectsInvalidOverlayConfig verifies that
// planClusterAppActions fails when overlay unit config is invalid.
func TestPlanClusterAppActionsRejectsInvalidOverlayConfig(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		mutate  func(*v2.Config)
		wantErr string
	}{
		{
			name: "http repository scheme rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
					Enabled:        true,
					RepositoryName: "test-repo",
					RepositoryURL:  "http://github.com/example/repo.git",
					Kustomizations: []overlaycfg.CustomerManagedKustomization{
						{Name: "apps", Path: "/apps"},
					},
				}
			},
			wantErr: "must use ssh or https",
		},
		{
			name: "empty repository name rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
					Enabled:        true,
					RepositoryName: "",
					RepositoryURL:  "ssh://git@github.com/example/repo.git",
					Kustomizations: []overlaycfg.CustomerManagedKustomization{
						{Name: "apps", Path: "/apps"},
					},
				}
			},
			wantErr: "requires repository_name",
		},
		{
			name: "emit_secret without identity rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
					Enabled:        true,
					RepositoryName: "test-repo",
					RepositoryURL:  "ssh://git@github.com/example/repo.git",
					EmitSecret:     true,
					Kustomizations: []overlaycfg.CustomerManagedKustomization{
						{Name: "apps", Path: "/apps"},
					},
				}
			},
			wantErr: "identity",
		},
		{
			name: "sops with empty age recipient rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.SOPS = overlaycfg.SOPSGenerationConfig{
					Enabled: true,
					Rules: []overlaycfg.SOPSGenerationRule{
						{PathRegex: ".*", AgeRecipients: []string{""}},
					},
				}
			},
			wantErr: "cannot be empty",
		},
		{
			name: "sops with no rules rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.SOPS = overlaycfg.SOPSGenerationConfig{
					Enabled: true,
					Rules:   []overlaycfg.SOPSGenerationRule{},
				}
			},
			wantErr: "requires at least one rule",
		},
		{
			name: "kustomization without leading slash rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
					Enabled:        true,
					RepositoryName: "test-repo",
					RepositoryURL:  "ssh://git@github.com/example/repo.git",
					Kustomizations: []overlaycfg.CustomerManagedKustomization{
						{Name: "apps", Path: "apps/no-leading-slash"},
					},
				}
			},
			wantErr: "must start with /",
		},
		{
			name: "emit_secret over https rejected",
			mutate: func(cfg *v2.Config) {
				cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
					Enabled:        true,
					RepositoryName: "test-repo",
					RepositoryURL:  "https://github.com/example/repo.git",
					EmitSecret:     true,
					Kustomizations: []overlaycfg.CustomerManagedKustomization{
						{Name: "apps", Path: "/apps"},
					},
				}
			},
			wantErr: "only supported for ssh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cfg := newDefault("negative-test")
			cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
			tt.mutate(&cfg)

			_, err := planClusterAppActions(cfg)
			if err == nil {
				t.Fatal("expected error from planClusterAppActions")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

// TestRenderDiagnosticsPopulated verifies that planClusterAppActions
// populates the lastRenderDiagnostics variable with structured output.
func TestRenderDiagnosticsPopulated(t *testing.T) {
	cfg := newDefault("diagnostics-test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()

	actions, err := planClusterAppActions(cfg)
	if err != nil {
		t.Fatalf("planClusterAppActions: %v", err)
	}

	if lastRenderDiagnostics == nil {
		t.Fatal("expected lastRenderDiagnostics to be populated")
	}

	if lastRenderDiagnostics.Cluster != cfg.ClusterName() {
		t.Fatalf("expected cluster %q, got %q", cfg.ClusterName(), lastRenderDiagnostics.Cluster)
	}

	if len(lastRenderDiagnostics.Descriptors) == 0 {
		t.Fatal("expected at least one descriptor decision")
	}

	if len(lastRenderDiagnostics.Actions) != len(actions) {
		t.Fatalf("expected %d action diagnostics, got %d", len(actions), len(lastRenderDiagnostics.Actions))
	}

	// Verify each descriptor decision has a non-empty reason.
	for _, decision := range lastRenderDiagnostics.Descriptors {
		if decision.Name == "" {
			t.Fatal("descriptor decision has empty name")
		}
		if decision.Reason == "" {
			t.Fatalf("descriptor %q has empty reason", decision.Name)
		}
	}

	// Verify JSON serialization works.
	data, err := lastRenderDiagnostics.JSON()
	if err != nil {
		t.Fatalf("diagnostics JSON: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON output")
	}
}
