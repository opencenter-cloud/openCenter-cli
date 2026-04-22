package importer

import (
	"context"
	"path/filepath"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestScannerScanRepoDiscoversExampleCustomerClusters(t *testing.T) {
	repoPath := filepath.Join("..", "..", "testdata", "100000-example-inc")

	scanner := NewScanner()
	result, err := scanner.ScanRepo(context.Background(), repoPath)
	if err != nil {
		t.Fatalf("ScanRepo() error = %v", err)
	}

	if result.Summary.ClustersDiscovered != 5 {
		t.Fatalf("expected 5 discovered clusters, got %d", result.Summary.ClustersDiscovered)
	}

	dev := findClusterResult(t, result, "k8s-dev")
	if dev.Sources.LegacyConfigPath == "" {
		t.Fatal("expected k8s-dev to use legacy GitOps config evidence")
	}
	if dev.ProposedConfig == nil {
		t.Fatal("expected k8s-dev proposed config to be populated")
	}
	if dev.ProposedConfig.OpenCenter.Meta.Organization != "1643323-Federal-Farm-Credit" {
		t.Fatalf("unexpected organization %q", dev.ProposedConfig.OpenCenter.Meta.Organization)
	}
	if dev.ProposedConfig.OpenCenter.Meta.Region != "iad3" {
		t.Fatalf("expected k8s-dev region iad3, got %q", dev.ProposedConfig.OpenCenter.Meta.Region)
	}
	if dev.ProposedConfig.OpenCenter.Infrastructure.Compute.MasterCount != 3 {
		t.Fatalf("expected k8s-dev master count 3, got %d", dev.ProposedConfig.OpenCenter.Infrastructure.Compute.MasterCount)
	}
	if dev.ProposedConfig.OpenCenter.Infrastructure.Compute.WorkerCount != 2 {
		t.Fatalf("expected k8s-dev GitOps worker count 2, got %d", dev.ProposedConfig.OpenCenter.Infrastructure.Compute.WorkerCount)
	}
	if !serviceEnabled(dev.ProposedConfig, "cert-manager") {
		t.Fatal("expected cert-manager to be enabled for k8s-dev")
	}
	if !serviceEnabled(dev.ProposedConfig, "keycloak") {
		t.Fatal("expected keycloak to be enabled for k8s-dev")
	}

	prod := findClusterResult(t, result, "k8s-prod")
	if prod.Sources.LegacyConfigPath != "" {
		t.Fatalf("expected k8s-prod to rely on non-legacy sources, got %q", prod.Sources.LegacyConfigPath)
	}
	if prod.ProposedConfig == nil {
		t.Fatal("expected k8s-prod proposed config to be populated")
	}
	if prod.ProposedConfig.OpenCenter.Meta.Region != "ord1" {
		t.Fatalf("expected k8s-prod region ord1, got %q", prod.ProposedConfig.OpenCenter.Meta.Region)
	}
	if prod.ProposedConfig.OpenCenter.Infrastructure.Provider != "vmware" {
		t.Fatalf("expected k8s-prod provider vmware, got %q", prod.ProposedConfig.OpenCenter.Infrastructure.Provider)
	}
	if prod.ProposedConfig.OpenCenter.Infrastructure.Compute.MasterCount != 3 {
		t.Fatalf("expected k8s-prod master count 3, got %d", prod.ProposedConfig.OpenCenter.Infrastructure.Compute.MasterCount)
	}
	if prod.ProposedConfig.OpenCenter.Infrastructure.Compute.WorkerCount != 3 {
		t.Fatalf("expected k8s-prod worker count 3, got %d", prod.ProposedConfig.OpenCenter.Infrastructure.Compute.WorkerCount)
	}
	if !serviceEnabled(prod.ProposedConfig, "velero") {
		t.Fatal("expected velero to be enabled for k8s-prod from overlay evidence")
	}
}

func findClusterResult(t *testing.T, result *ImportScanResult, clusterName string) ClusterImportResult {
	t.Helper()

	for _, cluster := range result.Clusters {
		if cluster.ClusterName == clusterName {
			return cluster
		}
	}

	t.Fatalf("cluster %q not found in scan result", clusterName)
	return ClusterImportResult{}
}

func serviceEnabled(cfg *v2.Config, serviceName string) bool {
	if cfg == nil {
		return false
	}
	service, ok := cfg.OpenCenter.Services[serviceName]
	if !ok {
		return false
	}
	enabler, ok := service.(interface{ IsEnabled() bool })
	return ok && enabler.IsEnabled()
}
