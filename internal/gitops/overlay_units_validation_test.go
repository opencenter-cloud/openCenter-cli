package gitops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	overlaycfg "github.com/opencenter-cloud/opencenter-cli/internal/config/overlay"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestValidateOverlayUnitConfigRejectsInvalidRepositoryScheme(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefault("overlay-validation")
	cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
		Enabled:        true,
		RepositoryName: "customer-apps",
		RepositoryURL:  "http://github.com/example/customer-apps.git",
		Kustomizations: []overlaycfg.CustomerManagedKustomization{
			{Name: "apps", Path: "/clusters/overlay-validation/apps"},
		},
	}

	err := validateOverlayUnitConfig(cfg)
	if err == nil {
		t.Fatal("expected invalid repository scheme error")
	}
	if !strings.Contains(err.Error(), "must use ssh or https") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateOverlayUnitConfigRejectsMissingSSHSecrets(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefault("overlay-validation")
	cfg.OpenCenter.GitOps.OverlayUnits.CustomerManaged = overlaycfg.CustomerManagedConfig{
		Enabled:        true,
		RepositoryName: "customer-apps",
		RepositoryURL:  "ssh://git@github.com/example/customer-apps.git",
		EmitSecret:     true,
		Kustomizations: []overlaycfg.CustomerManagedKustomization{
			{Name: "apps", Path: "/clusters/overlay-validation/apps"},
		},
	}

	err := validateOverlayUnitConfig(cfg)
	if err == nil {
		t.Fatal("expected missing ssh secret error")
	}
	if !strings.Contains(err.Error(), "secrets.overlay_units.customer_managed.identity") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateOverlayUnitConfigRejectsInvalidSOPSRules(t *testing.T) {
	t.Parallel()

	cfg := config.NewDefault("overlay-validation")
	cfg.OpenCenter.GitOps.OverlayUnits.SOPS = overlaycfg.SOPSGenerationConfig{
		Enabled: true,
		Rules: []overlaycfg.SOPSGenerationRule{
			{PathRegex: ".*", AgeRecipients: []string{""}},
		},
	}

	err := validateOverlayUnitConfig(cfg)
	if err == nil {
		t.Fatal("expected invalid sops rule error")
	}
	if !strings.Contains(err.Error(), "age_recipients[0] cannot be empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRenderSingleServiceRewritesOnlyTargetAndAggregates(t *testing.T) {
	cfg := config.NewDefault("single-service")
	cfg.OpenCenter.Cluster.ClusterName = "single-service"
	cfg.OpenCenter.GitOps.GitDir = t.TempDir()

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps: %v", err)
	}

	clusterRoot := filepath.Join(cfg.OpenCenter.GitOps.GitDir, "applications", "overlays", cfg.ClusterName())
	targetFile := filepath.Join(clusterRoot, "services", "cert-manager", "kustomization.yaml")
	aggregateFlux := filepath.Join(clusterRoot, "services", "fluxcd", "kustomization.yaml")
	aggregateSources := filepath.Join(clusterRoot, "services", "sources", "kustomization.yaml")
	unrelatedFile := filepath.Join(clusterRoot, "services", "headlamp", "kustomization.yaml")

	for _, path := range []string{targetFile, aggregateFlux, aggregateSources, unrelatedFile} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected rendered file %s: %v", path, err)
		}
	}

	const (
		targetSentinel    = "stale-target"
		aggregateSentinel = "stale-aggregate"
		unrelatedSentinel = "leave-me-alone"
	)
	if err := os.WriteFile(targetFile, []byte(targetSentinel), 0o644); err != nil {
		t.Fatalf("write target sentinel: %v", err)
	}
	if err := os.WriteFile(aggregateFlux, []byte(aggregateSentinel), 0o644); err != nil {
		t.Fatalf("write aggregate flux sentinel: %v", err)
	}
	if err := os.WriteFile(aggregateSources, []byte(aggregateSentinel), 0o644); err != nil {
		t.Fatalf("write aggregate sources sentinel: %v", err)
	}
	if err := os.WriteFile(unrelatedFile, []byte(unrelatedSentinel), 0o644); err != nil {
		t.Fatalf("write unrelated sentinel: %v", err)
	}

	if err := RenderSingleService(cfg, "cert-manager", false); err != nil {
		t.Fatalf("RenderSingleService: %v", err)
	}

	for _, path := range []string{targetFile, aggregateFlux, aggregateSources} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if strings.Contains(string(data), "stale-") {
			t.Fatalf("expected %s to be re-rendered, found sentinel content", path)
		}
	}

	data, err := os.ReadFile(unrelatedFile)
	if err != nil {
		t.Fatalf("read unrelated file: %v", err)
	}
	if string(data) != unrelatedSentinel {
		t.Fatalf("expected unrelated file to remain untouched, got %q", string(data))
	}
}
