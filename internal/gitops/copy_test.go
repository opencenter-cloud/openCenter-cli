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

package gitops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/provision"
	"gopkg.in/yaml.v3"
)

func TestMain(m *testing.M) {
	if err := provision.Init(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

func TestCopyBase(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("test")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := CopyBase(cfg, false); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(dst, ".gitignore")); os.IsNotExist(err) {
		t.Error(".gitignore was not copied")
	}

	files, err := filepath.Glob(filepath.Join(dst, "*"))
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("files in dst: %v", files)
}

func TestRenderInfrastructureClusterRendersConfigValues(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("render-test")
	cfg.OpenCenter.Cluster.ClusterName = "render-test"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.Cluster.Kubernetes.Version = "9.9.9"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.local/v3/"

	if err := RenderInfrastructureCluster(cfg); err != nil {
		t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
	}

	mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	data, err := os.ReadFile(mainTF)
	if err != nil {
		t.Fatalf("failed to read rendered main.tf: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, cfg.OpenCenter.Cluster.Kubernetes.Version) {
		t.Fatalf("rendered main.tf missing kubernetes version %q\ncontent:\n%s", cfg.OpenCenter.Cluster.Kubernetes.Version, content)
	}
	if !strings.Contains(content, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL) {
		t.Fatalf("rendered main.tf missing auth_url %q\ncontent:\n%s", cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL, content)
	}
}

func TestRenderClusterAppsRendersClusterName(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("cluster-apps")
	cfg.OpenCenter.Cluster.ClusterName = "cluster-apps"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps returned error: %v", err)
	}

	sourcesFile := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "managed-services", "fluxcd", "sources.yaml")
	data, err := os.ReadFile(sourcesFile)
	if err != nil {
		t.Fatalf("failed to read rendered sources.yaml: %v", err)
	}
	if !strings.Contains(string(data), cfg.ClusterName()) {
		t.Fatalf("rendered sources.yaml missing cluster name %q\ncontent:\n%s", cfg.ClusterName(), string(data))
	}
}

func TestRenderClusterAppsSkipsDisabledServices(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("disabled-services-test")
	cfg.OpenCenter.Cluster.ClusterName = "disabled-services-test"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	// Disable some services
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{BaseConfig: services.BaseConfig{Enabled: false}}
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{BaseConfig: services.BaseConfig{Enabled: false}}
	cfg.OpenCenter.ManagedServices["alert-proxy"] = &services.AlertProxyConfig{BaseConfig: services.BaseConfig{Enabled: false}}

	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps returned error: %v", err)
	}

	// Check that disabled service directories are not created
	certManagerDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "cert-manager")
	if _, err := os.Stat(certManagerDir); !os.IsNotExist(err) {
		t.Errorf("disabled cert-manager service directory should not exist: %s", certManagerDir)
	}

	veleroDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "velero")
	if _, err := os.Stat(veleroDir); !os.IsNotExist(err) {
		t.Errorf("disabled velero service directory should not exist: %s", veleroDir)
	}

	alertProxyDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "managed-services", "alert-proxy")
	if _, err := os.Stat(alertProxyDir); !os.IsNotExist(err) {
		t.Errorf("disabled alert-proxy managed service directory should not exist: %s", alertProxyDir)
	}

	// Check that enabled services are still created
	sourcesDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "sources")
	if _, err := os.Stat(sourcesDir); os.IsNotExist(err) {
		t.Errorf("enabled sources service directory should exist: %s", sourcesDir)
	}

	// Check that the fluxcd kustomization files reflect the disabled services
	servicesKustomization := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "kustomization.yaml")
	data, err := os.ReadFile(servicesKustomization)
	if err != nil {
		t.Fatalf("failed to read services fluxcd kustomization.yaml: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "./cert-manager.yaml") {
		t.Errorf("disabled cert-manager should not be in services fluxcd kustomization.yaml")
	}
	if strings.Contains(content, "./velero.yaml") {
		t.Errorf("disabled velero should not be in services fluxcd kustomization.yaml")
	}

	// Check that managed services kustomization reflects disabled alert-proxy
	managedServicesKustomization := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "managed-services", "fluxcd", "kustomization.yaml")
	data, err = os.ReadFile(managedServicesKustomization)
	if err != nil {
		t.Fatalf("failed to read managed-services fluxcd kustomization.yaml: %v", err)
	}
	content = string(data)
	if strings.Contains(content, "./alert-proxy.yaml") {
		t.Errorf("disabled alert-proxy should not be in managed-services fluxcd kustomization.yaml")
	}
	// Since all managed services are disabled, sources.yaml should also not be included
	if strings.Contains(content, "./sources.yaml") {
		t.Errorf("sources.yaml should not be included when all managed services are disabled")
	}
}

// TestCopyBaseAtomic tests atomic file operations for CopyBase.
func TestCopyBaseAtomic(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := newDefault("test-atomic")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Copy base files atomically
	if err := CopyBaseAtomic(cfg, false, workspace); err != nil {
		t.Fatalf("CopyBaseAtomic failed: %v", err)
	}

	// Verify files were created
	if !workspace.Exists(".gitignore") {
		t.Error(".gitignore was not copied")
	}

	// Verify file content
	gitignoreContent, err := os.ReadFile(workspace.GetPath(".gitignore"))
	if err != nil {
		t.Fatalf("Failed to read .gitignore: %v", err)
	}

	if len(gitignoreContent) == 0 {
		t.Error(".gitignore is empty")
	}
}

// TestRenderInfrastructureClusterAtomic tests atomic file operations for infrastructure rendering.
func TestRenderInfrastructureClusterAtomic(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := newDefault("render-test-atomic")
	cfg.OpenCenter.Cluster.ClusterName = "render-test-atomic"
	cfg.OpenCenter.Cluster.Kubernetes.Version = "9.9.9"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://auth.example.local/v3/"

	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Render infrastructure atomically
	if err := RenderInfrastructureClusterAtomic(cfg, workspace); err != nil {
		t.Fatalf("RenderInfrastructureClusterAtomic failed: %v", err)
	}

	// Verify main.tf was created
	mainTfPath := filepath.Join("infrastructure", "clusters", cfg.ClusterName(), "main.tf")
	if !workspace.Exists(mainTfPath) {
		t.Error("main.tf was not created")
	}

	// Verify content
	data, err := os.ReadFile(workspace.GetPath(mainTfPath))
	if err != nil {
		t.Fatalf("Failed to read main.tf: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, cfg.OpenCenter.Cluster.Kubernetes.Version) {
		t.Errorf("main.tf missing kubernetes version %q", cfg.OpenCenter.Cluster.Kubernetes.Version)
	}

	if !strings.Contains(content, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL) {
		t.Errorf("main.tf missing auth_url %q", cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
	}
}

func TestRenderInfrastructureClusterAtomicKindTemplate(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := newDefault("kind-render-atomic")
	cfg.OpenCenter.Meta.Organization = "opencenter"
	if err := applyProviderDefaults(&cfg, "kind"); err != nil {
		t.Fatalf("apply provider defaults: %v", err)
	}

	cfg.OpenCenter.Infrastructure.Kind.ControlPlaneCount = 2
	cfg.OpenCenter.Infrastructure.Kind.WorkerCount = 3
	cfg.OpenCenter.Infrastructure.Kind.APIServerAddress = "127.0.0.2"
	cfg.OpenCenter.Infrastructure.Kind.APIServerPort = 7443
	cfg.OpenCenter.Infrastructure.Kind.PodSubnet = "10.250.0.0/16"
	cfg.OpenCenter.Infrastructure.Kind.ServiceSubnet = "10.251.0.0/16"
	cfg.OpenCenter.Infrastructure.Kind.NodeImage = "kindest/node:v1.31.0"
	cfg.OpenCenter.Infrastructure.Kind.ExtraPortMappings = []v2.KindPortMapping{
		{
			ContainerPort: 80,
			HostPort:      8080,
			ListenAddress: "127.0.0.1",
			Protocol:      "TCP",
		},
	}
	cfg.OpenCenter.Infrastructure.Kind.ExtraMounts = []v2.KindMount{
		{
			HostPath:      "/tmp/kind-cache",
			ContainerPath: "/var/cache/kind",
			ReadOnly:      true,
		},
	}

	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	if err := RenderInfrastructureClusterAtomic(cfg, workspace); err != nil {
		t.Fatalf("RenderInfrastructureClusterAtomic failed: %v", err)
	}

	kindConfigPath := filepath.Join("infrastructure", "clusters", cfg.ClusterName(), "kind-config.yaml")
	if !workspace.Exists(kindConfigPath) {
		t.Fatal("kind-config.yaml was not created")
	}

	data, err := os.ReadFile(workspace.GetPath(kindConfigPath))
	if err != nil {
		t.Fatalf("read kind-config.yaml: %v", err)
	}

	var rendered struct {
		Kind       string `yaml:"kind"`
		APIVersion string `yaml:"apiVersion"`
		Networking struct {
			APIServerAddress string `yaml:"apiServerAddress"`
			APIServerPort    int    `yaml:"apiServerPort"`
			PodSubnet        string `yaml:"podSubnet"`
			ServiceSubnet    string `yaml:"serviceSubnet"`
		} `yaml:"networking"`
		Nodes []struct {
			Role              string `yaml:"role"`
			Image             string `yaml:"image"`
			ExtraPortMappings []struct {
				ContainerPort int    `yaml:"containerPort"`
				HostPort      int    `yaml:"hostPort"`
				ListenAddress string `yaml:"listenAddress"`
				Protocol      string `yaml:"protocol"`
			} `yaml:"extraPortMappings"`
			ExtraMounts []struct {
				HostPath      string `yaml:"hostPath"`
				ContainerPath string `yaml:"containerPath"`
				ReadOnly      bool   `yaml:"readOnly"`
			} `yaml:"extraMounts"`
		} `yaml:"nodes"`
	}
	if err := yaml.Unmarshal(data, &rendered); err != nil {
		t.Fatalf("unmarshal kind-config.yaml: %v\ncontent:\n%s", err, string(data))
	}

	if rendered.Kind != "Cluster" || rendered.APIVersion != "kind.x-k8s.io/v1alpha4" {
		t.Fatalf("unexpected kind config header: %#v", rendered)
	}
	if rendered.Networking.APIServerAddress != "127.0.0.2" || rendered.Networking.APIServerPort != 7443 {
		t.Fatalf("unexpected networking api settings: %#v", rendered.Networking)
	}
	if rendered.Networking.PodSubnet != "10.250.0.0/16" || rendered.Networking.ServiceSubnet != "10.251.0.0/16" {
		t.Fatalf("unexpected networking subnets: %#v", rendered.Networking)
	}
	if len(rendered.Nodes) != 5 {
		t.Fatalf("expected 5 nodes, got %d\ncontent:\n%s", len(rendered.Nodes), string(data))
	}

	controlPlanes := 0
	workers := 0
	for i, node := range rendered.Nodes {
		if node.Image != "kindest/node:v1.31.0" {
			t.Fatalf("unexpected node image for node %d: %s", i, node.Image)
		}

		switch node.Role {
		case "control-plane":
			controlPlanes++
			if len(node.ExtraPortMappings) != 1 {
				t.Fatalf("expected control-plane node %d to have a port mapping", i)
			}
			if len(node.ExtraMounts) != 1 {
				t.Fatalf("expected control-plane node %d to have an extra mount", i)
			}
		case "worker":
			workers++
			if len(node.ExtraMounts) != 1 {
				t.Fatalf("expected worker node %d to have an extra mount", i)
			}
		default:
			t.Fatalf("unexpected node role: %s", node.Role)
		}
	}

	if controlPlanes != 2 || workers != 3 {
		t.Fatalf("expected 2 control-plane and 3 worker nodes, got %d and %d", controlPlanes, workers)
	}
}

// TestRenderClusterAppsAtomic tests atomic file operations for cluster apps rendering.
func TestRenderClusterAppsAtomic(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := newDefault("cluster-apps-atomic")
	cfg.OpenCenter.Cluster.ClusterName = "cluster-apps-atomic"

	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Render cluster apps atomically
	if err := RenderClusterAppsAtomic(cfg, workspace); err != nil {
		t.Fatalf("RenderClusterAppsAtomic failed: %v", err)
	}

	// Verify sources.yaml was created
	sourcesPath := filepath.Join("applications", "overlays", cfg.ClusterName(), "managed-services", "fluxcd", "sources.yaml")
	if !workspace.Exists(sourcesPath) {
		t.Error("sources.yaml was not created")
	}

	// Verify content
	data, err := os.ReadFile(workspace.GetPath(sourcesPath))
	if err != nil {
		t.Fatalf("Failed to read sources.yaml: %v", err)
	}

	if !strings.Contains(string(data), cfg.ClusterName()) {
		t.Errorf("sources.yaml missing cluster name %q", cfg.ClusterName())
	}
}

// TestAtomicOperationsPreventPartialWrites tests that atomic operations prevent partial file writes.
func TestAtomicOperationsPreventPartialWrites(t *testing.T) {
	tempDir := t.TempDir()
	manager := NewWorkspaceManager(tempDir)
	ctx := context.Background()

	cfg := newDefault("partial-write-test")
	workspace, err := manager.CreateWorkspace(ctx, cfg)
	if err != nil {
		t.Fatalf("Failed to create workspace: %v", err)
	}
	defer manager.CleanupWorkspace(ctx, workspace)

	// Create a transaction with multiple operations
	tx := NewTransaction(workspace)

	// Add multiple file operations
	tx.WriteFile("file1.txt", []byte("content1"), 0o644)
	tx.WriteFile("file2.txt", []byte("content2"), 0o644)
	tx.WriteFile("file3.txt", []byte("content3"), 0o644)

	// Add an operation that will fail (invalid path with null byte)
	tx.WriteFile("invalid\x00path.txt", []byte("content"), 0o644)

	// Commit transaction (should fail and rollback)
	err = tx.Commit()
	if err == nil {
		t.Error("Transaction should fail with invalid path")
	}

	// Verify no files were created (all rolled back)
	if workspace.Exists("file1.txt") {
		t.Error("file1.txt should not exist after rollback")
	}

	if workspace.Exists("file2.txt") {
		t.Error("file2.txt should not exist after rollback")
	}

	if workspace.Exists("file3.txt") {
		t.Error("file3.txt should not exist after rollback")
	}

	// Now test successful atomic operations
	tx2 := NewTransaction(workspace)
	tx2.WriteFile("success1.txt", []byte("success content 1"), 0o644)
	tx2.WriteFile("success2.txt", []byte("success content 2"), 0o644)

	if err := tx2.Commit(); err != nil {
		t.Fatalf("Successful transaction should not fail: %v", err)
	}

	// Verify all files were created
	if !workspace.Exists("success1.txt") {
		t.Error("success1.txt should exist after successful commit")
	}

	if !workspace.Exists("success2.txt") {
		t.Error("success2.txt should exist after successful commit")
	}

	// Verify content
	content1, _ := os.ReadFile(workspace.GetPath("success1.txt"))
	if string(content1) != "success content 1" {
		t.Errorf("Expected 'success content 1', got '%s'", string(content1))
	}
}

func TestCopyBaseRendersAllSourceAuthBlocksAndPreservesServiceRepo(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("source-auth")
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst
	cfg.OpenCenter.GitOps.Repository.URL = "ssh://git@gitlab.example.com/customer/cluster-config.git"
	cfg.OpenCenter.GitOps.Repository.Branch = "customer-main"
	cfg.OpenCenter.GitOps.BaseRepo.Release = ""
	cfg.OpenCenter.GitOps.ResolvedAuthMethod = gitopsAuthMethodToken
	cfg.OpenCenter.Services["cert-manager"] = &services.CertManagerConfig{
		BaseConfig: services.BaseConfig{
			Enabled: true,
			Source: services.ServiceSource{
				Repo:    "ssh://git@gitlab.example.com/team/cert-manager.git",
				Release: "v1.2.3",
			},
		},
	}

	if err := CopyBase(cfg, true); err != nil {
		t.Fatalf("CopyBase() error = %v", err)
	}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("RenderClusterApps() error = %v", err)
	}

	certSource := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "sources", "opencenter-cert-manager.yaml")
	certContents, err := os.ReadFile(certSource)
	if err != nil {
		t.Fatalf("read cert-manager source: %v", err)
	}
	certText := string(certContents)
	if !strings.Contains(certText, "# --- token auth (active) ---") ||
		!strings.Contains(certText, "url: https://gitlab.example.com/team/cert-manager.git") ||
		!strings.Contains(certText, `tag: "v1.2.3"`) ||
		!strings.Contains(certText, "# url: ssh://git@gitlab.example.com/team/cert-manager.git") {
		t.Fatalf("cert-manager source did not preserve custom repository/ref or render both variants:\n%s", certText)
	}

	keycloakSource := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "sources", "opencenter-keycloak-config.yaml")
	keycloakContents, err := os.ReadFile(keycloakSource)
	if err != nil {
		t.Fatalf("read keycloak config source: %v", err)
	}
	keycloakText := string(keycloakContents)
	if !strings.Contains(keycloakText, "# --- token auth (active) ---") ||
		!strings.Contains(keycloakText, "url: https://gitlab.example.com/customer/cluster-config.git") ||
		!strings.Contains(keycloakText, "# url: ssh://git@gitlab.example.com/customer/cluster-config.git") ||
		!strings.Contains(keycloakText, "name: flux-system") {
		t.Fatalf("keycloak-config source did not use the shared customer-repository block:\n%s", keycloakText)
	}
}


func TestRenderInfrastructureClusterRendersBastionAddressFromConfig(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		address  string
		want     string
	}{
		{name: "baremetal empty address renders empty", provider: "baremetal", address: "", want: `address_bastion                         = ""`},
		{name: "baremetal explicit address renders through", provider: "baremetal", address: "bastion.example.com", want: `address_bastion                         = "bastion.example.com"`},
		{name: "vmware empty address renders empty", provider: "vmware", address: "", want: `address_bastion                         = ""`},
		{name: "vmware explicit address renders through", provider: "vmware", address: "bastion.example.com", want: `address_bastion                         = "bastion.example.com"`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := mustNewGitOpsTestConfig("bastion-address-"+tc.provider+"-"+tc.address, tc.provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Bastion.Address = tc.address

			if err := RenderInfrastructureCluster(cfg); err != nil {
				t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
			}

			mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			data, err := os.ReadFile(mainTF)
			if err != nil {
				t.Fatalf("failed to read rendered main.tf: %v", err)
			}
			content := string(data)
			if !strings.Contains(content, tc.want) {
				t.Fatalf("rendered main.tf missing %q\ncontent:\n%s", tc.want, content)
			}
		})
	}
}

func TestRenderInfrastructureClusterRendersKubeVipInterface(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "baremetal", provider: "baremetal"},
		{name: "vmware", provider: "vmware"},
		{name: "openstack", provider: "openstack"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := mustNewGitOpsTestConfig("vip-interface-"+tc.name, tc.provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Networking.VIPInterface = "mgmt"

			if err := RenderInfrastructureCluster(cfg); err != nil {
				t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
			}

			mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			data, err := os.ReadFile(mainTF)
			if err != nil {
				t.Fatalf("failed to read rendered main.tf: %v", err)
			}
			content := string(data)

			wantLocal := `kube_vip_interface                      = "mgmt"`
			if !strings.Contains(content, wantLocal) {
				t.Fatalf("rendered main.tf missing local kube_vip_interface assignment %q\ncontent:\n%s", wantLocal, content)
			}
			if !strings.Contains(content, "kube_vip_interface                      = local.kube_vip_interface") {
				t.Fatalf("rendered main.tf missing kube_vip_interface module argument\ncontent:\n%s", content)
			}
		})
	}
}

func TestRenderInfrastructureClusterKubeVipInterfaceEmptyByDefault(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "baremetal", provider: "baremetal"},
		{name: "vmware", provider: "vmware"},
		{name: "openstack", provider: "openstack"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := mustNewGitOpsTestConfig("vip-interface-default-"+tc.name, tc.provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			cfg.OpenCenter.Infrastructure.Networking.VIPInterface = ""

			if err := RenderInfrastructureCluster(cfg); err != nil {
				t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
			}

			mainTF := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "main.tf")
			data, err := os.ReadFile(mainTF)
			if err != nil {
				t.Fatalf("failed to read rendered main.tf: %v", err)
			}
			content := string(data)

			wantLocal := `kube_vip_interface                      = ""`
			if !strings.Contains(content, wantLocal) {
				t.Fatalf("rendered main.tf missing empty kube_vip_interface local %q\ncontent:\n%s", wantLocal, content)
			}
		})
	}
}

func TestRenderInfrastructureClusterRendersKubesprayNetworkYaml(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		wantAnsHost  bool
	}{
		{name: "baremetal", provider: "baremetal", wantAnsHost: true},
		{name: "vmware", provider: "vmware", wantAnsHost: true},
		{name: "openstack", provider: "openstack", wantAnsHost: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dst := t.TempDir()
			cfg := mustNewGitOpsTestConfig("network-yaml-"+tc.provider, tc.provider)
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst

			if err := RenderInfrastructureCluster(cfg); err != nil {
				t.Fatalf("RenderInfrastructureCluster returned error: %v", err)
			}

			path := filepath.Join(dst, "infrastructure", "clusters", cfg.ClusterName(), "inventory", "group_vars", "all", "network.yml")
			data, err := os.ReadFile(path)

			if !tc.wantAnsHost {
				// Non-baremetal/vmware providers may either skip the file or
				// render it empty; both are acceptable so long as no
				// ansible_host pin leaks through.
				if err != nil && os.IsNotExist(err) {
					return
				}
				if err != nil {
					t.Fatalf("unexpected error reading network.yml: %v", err)
				}
				if strings.Contains(string(data), "ansible_host") {
					t.Fatalf("rendered network.yml for %s should not pin ip to ansible_host\ncontent:\n%s", tc.provider, string(data))
				}
				return
			}

			if err != nil {
				t.Fatalf("failed to read rendered network.yml: %v", err)
			}
			content := string(data)
			wantIP := `ip: "{{ ansible_host }}"`
			wantAccessIP := `access_ip: "{{ ansible_host }}"`
			if !strings.Contains(content, wantIP) {
				t.Fatalf("rendered network.yml missing %q\ncontent:\n%s", wantIP, content)
			}
			if !strings.Contains(content, wantAccessIP) {
				t.Fatalf("rendered network.yml missing %q\ncontent:\n%s", wantAccessIP, content)
			}
		})
	}
}


func TestRenderClusterFluxBridgeCreatesBridgeFiles(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("bridge-test")
	cfg.OpenCenter.Cluster.ClusterName = "bridge-test"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	if err := RenderClusterFluxBridge(cfg); err != nil {
		t.Fatalf("RenderClusterFluxBridge returned error: %v", err)
	}

	bridgeDir := filepath.Join(dst, "clusters", cfg.ClusterName())

	// Only services.yaml is emitted. Flux's kustomize-controller
	// auto-generates a top-level kustomization.yaml at reconcile time.
	if _, err := os.Stat(filepath.Join(bridgeDir, "kustomization.yaml")); !os.IsNotExist(err) {
		t.Fatalf("bridge unexpectedly emitted a kustomization.yaml at %s (err=%v)", bridgeDir, err)
	}

	services, err := os.ReadFile(filepath.Join(bridgeDir, "services.yaml"))
	if err != nil {
		t.Fatalf("read cluster services.yaml: %v", err)
	}
	sText := string(services)
	wantPath := "./applications/overlays/" + cfg.ClusterName() + "/services/fluxcd"
	for _, want := range []string{
		"kind: Kustomization",
		"apiVersion: kustomize.toolkit.fluxcd.io/v1",
		"namespace: flux-system",
		"name: services",
		wantPath,
	} {
		if !strings.Contains(sText, want) {
			t.Fatalf("cluster services.yaml missing %q\ncontent:\n%s", want, sText)
		}
	}
}

func TestRenderClusterFluxBridgePreservesFluxSystemDir(t *testing.T) {
	dst := t.TempDir()
	cfg := newDefault("bridge-preserve")
	cfg.OpenCenter.Cluster.ClusterName = "bridge-preserve"
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	// Simulate `flux bootstrap` having already run: write files under
	// clusters/<cluster>/flux-system/ that the bridge render must not touch.
	fluxSystemDir := filepath.Join(dst, "clusters", cfg.ClusterName(), "flux-system")
	if err := os.MkdirAll(fluxSystemDir, 0o755); err != nil {
		t.Fatalf("mkdir flux-system: %v", err)
	}
	sentinel := filepath.Join(fluxSystemDir, "gotk-components.yaml")
	if err := os.WriteFile(sentinel, []byte("sentinel"), 0o600); err != nil {
		t.Fatalf("write sentinel: %v", err)
	}

	if err := RenderClusterFluxBridge(cfg); err != nil {
		t.Fatalf("RenderClusterFluxBridge returned error: %v", err)
	}

	got, err := os.ReadFile(sentinel)
	if err != nil {
		t.Fatalf("sentinel gotk-components.yaml was removed or altered: %v", err)
	}
	if string(got) != "sentinel" {
		t.Fatalf("sentinel content changed: got %q", string(got))
	}
}
