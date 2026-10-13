package gitops

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestRenderClusterAppsValidationBeforeMutation(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("validation-before-mutation")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	before := snapshotFiles(t, root)
	bad := cfg
	bad.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	bad.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig:       services.BaseConfig{Enabled: true},
		IPAddressPools:   []services.IPAddressPool{{Name: "public", Addresses: []string{"10.0.0.1/32"}}},
		L2Advertisements: []services.L2Advertisement{{Name: "invalid", IPAddressPools: []string{"missing"}}},
	}

	if err := RenderClusterApps(bad); err == nil {
		t.Fatal("expected invalid MetalLB configuration to fail")
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("invalid configuration mutated the target overlay")
	}
}

func TestRenderClusterAppsRenderFailureLeavesExistingTreeIntact(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("render-failure-survival")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	before := snapshotFiles(t, root)
	failed := cfg
	failed.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	failed.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig:       services.BaseConfig{Enabled: true},
		L2Advertisements: []services.L2Advertisement{{Name: "bad", IPAddressPools: []string{"does-not-exist"}}},
	}

	if err := RenderClusterApps(failed); err == nil {
		t.Fatal("expected renderer failure")
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("renderer failure changed the existing target tree")
	}
}

func TestRenderSingleServiceMatchesOwnershipChecks(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("single-service-ownership")
	useMetalLBTestOwnership(&cfg)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}

	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	unknown := filepath.Join(root, "services", "metallb", "hand-authored.yaml")
	writeTestFile(t, unknown, "keep")
	if err := RenderSingleService(cfg, "metallb", false); err == nil || !strings.Contains(err.Error(), "hand-authored.yaml") {
		t.Fatalf("expected single-service ownership refusal, got %v", err)
	}
	if err := RenderClusterApps(cfg); err == nil || !strings.Contains(err.Error(), "hand-authored.yaml") {
		t.Fatalf("expected full-render ownership refusal, got %v", err)
	}
	if err := os.Remove(unknown); err != nil {
		t.Fatal(err)
	}

	custom := filepath.Join(root, "services", "metallb", "custom", "hand-authored.yaml")
	writeTestFile(t, custom, "kind: ConfigMap\n")
	if err := RenderSingleService(cfg, "metallb", false); err != nil {
		t.Fatalf("single-service render with custom file: %v", err)
	}
	data, err := os.ReadFile(custom)
	if err != nil || string(data) != "kind: ConfigMap\n" {
		t.Fatalf("custom file changed: %q, %v", data, err)
	}
}

func TestPlanClusterAppsPromotionReportsChangesWithoutWriting(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("dry-run-report")
	useMetalLBTestOwnership(&cfg)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("initial render: %v", err)
	}
	root := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	before := snapshotFiles(t, root)

	pending := cfg
	pending.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	pending.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: []services.IPAddressPool{{Name: "public", Addresses: []string{"10.0.0.1/32"}}},
	}
	result, err := PlanClusterAppsPromotion(pending)
	if err != nil {
		t.Fatalf("plan pending change: %v", err)
	}
	if len(result.Added)+len(result.Updated) == 0 {
		t.Fatalf("expected pending change, got added=%v updated=%v", result.Added, result.Updated)
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("dry-run changed the target overlay")
	}

	disabled := cfg
	disabled.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	disabled.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{BaseConfig: services.BaseConfig{Enabled: false}}
	before = snapshotFiles(t, root)
	result, err = PlanClusterAppsPromotion(disabled)
	if err != nil {
		t.Fatalf("plan disabled service: %v", err)
	}
	if len(result.Pruned) == 0 {
		t.Fatal("expected disabled service to report pruned files")
	}
	if after := snapshotFiles(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("dry-run prune plan changed the target overlay")
	}
}

func cloneServiceMap(source v2.ServiceMap) v2.ServiceMap {
	result := make(v2.ServiceMap, len(source))
	for name, service := range source {
		result[name] = service
	}
	return result
}
