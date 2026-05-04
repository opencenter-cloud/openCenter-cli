package v2

import (
	"strings"
	"testing"
)

func TestApplyTalosDeploymentDefaults(t *testing.T) {
	cfg, err := NewV2Default("talos-defaults", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	ApplyTalosDeploymentDefaults(cfg)

	if cfg.Deployment.Method != "talos" {
		t.Fatalf("deployment method = %q, want talos", cfg.Deployment.Method)
	}
	if cfg.Deployment.Kubespray != nil {
		t.Fatalf("expected kubespray config removed, got %#v", cfg.Deployment.Kubespray)
	}
	if cfg.Deployment.Talos == nil {
		t.Fatal("expected deployment.talos")
	}
	if cfg.Deployment.Talos.Install.Disk != "/dev/sda" {
		t.Fatalf("install disk = %q, want /dev/sda", cfg.Deployment.Talos.Install.Disk)
	}
	if cfg.Deployment.Talos.Network.TalosAPIPort != 50000 {
		t.Fatalf("talos api port = %d, want 50000", cfg.Deployment.Talos.Network.TalosAPIPort)
	}
	if len(cfg.Deployment.Talos.Network.ManagementCIDRs) != 0 {
		t.Fatalf("management cidrs = %v, want empty list", cfg.Deployment.Talos.Network.ManagementCIDRs)
	}
	if cfg.OpenCenter.Cluster.Kubernetes.APIPort != 443 {
		t.Fatalf("kubernetes api port = %d, want 443", cfg.OpenCenter.Cluster.Kubernetes.APIPort)
	}
	if cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico != nil && cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.Enabled {
		t.Fatal("expected Calico disabled")
	}
	if cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium == nil || !cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium.Enabled {
		t.Fatalf("expected Cilium enabled, got %#v", cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Cilium)
	}
}

func TestValidatorTalosRequiresManagementCIDRs(t *testing.T) {
	cfg := validTalosConfigForTest(t)
	cfg.Deployment.Talos.Network.ManagementCIDRs = nil

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected empty management_cidrs to fail")
	}
	if !strings.Contains(err.Error(), "deployment.talos.network.management_cidrs is required") {
		t.Fatalf("error = %q, want management_cidrs validation", err.Error())
	}
}

func TestValidatorTalosRejectsInvalidManagementCIDR(t *testing.T) {
	cfg := validTalosConfigForTest(t)
	cfg.Deployment.Talos.Network.ManagementCIDRs = []string{"not-a-cidr"}

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected invalid management CIDR to fail")
	}
	if !strings.Contains(err.Error(), "cidrv4") {
		t.Fatalf("error = %q, want cidrv4 validation", err.Error())
	}
}

func TestValidatorTalosEndpointMustUseHTTPS443(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{
			name:     "rejects api port 6443",
			endpoint: "https://talos.example.com:6443",
			want:     "deployment.talos.endpoint must use port 443",
		},
		{
			name:     "rejects http",
			endpoint: "http://talos.example.com:443",
			want:     "deployment.talos.endpoint must use https scheme",
		},
		{
			name:     "rejects missing explicit port",
			endpoint: "https://talos.example.com",
			want:     "deployment.talos.endpoint must include explicit port 443",
		},
		{
			name:     "accepts https 443",
			endpoint: "https://talos.example.com:443",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validTalosConfigForTest(t)
			cfg.Deployment.Talos.Endpoint = tt.endpoint

			err := NewValidator().Validate(cfg)
			if tt.want == "" {
				if err != nil {
					t.Fatalf("Validate() error = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("expected %q to fail", tt.endpoint)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %q, want %q", err.Error(), tt.want)
			}
		})
	}
}

func TestValidatorTalosRequiresKubernetesAPI443(t *testing.T) {
	cfg := validTalosConfigForTest(t)
	cfg.OpenCenter.Cluster.Kubernetes.APIPort = 6443

	err := NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected Kubernetes API port 6443 to fail")
	}
	if !strings.Contains(err.Error(), "opencenter.cluster.kubernetes.api_port must be 443") {
		t.Fatalf("error = %q, want Kubernetes API 443 validation", err.Error())
	}
}

func TestValidatorTalosRequiresOpenStack(t *testing.T) {
	cfg, err := NewV2Default("talos-vmware", "vmware")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	ApplyTalosDeploymentDefaults(cfg)

	err = NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected Talos on VMware to fail")
	}
	if !strings.Contains(err.Error(), "deployment.method: talos requires opencenter.infrastructure.provider: openstack") {
		t.Fatalf("error = %q, want OpenStack-only validation", err.Error())
	}
}

func TestValidatorRejectsTalosInfrastructureProvider(t *testing.T) {
	cfg, err := NewV2Default("talos-provider", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "talos"

	err = NewValidator().Validate(cfg)
	if err == nil {
		t.Fatal("expected provider talos to fail")
	}
	if !strings.Contains(err.Error(), "talos is a deployment method, not an infrastructure provider") {
		t.Fatalf("error = %q, want provider misuse guidance", err.Error())
	}
}

func validTalosConfigForTest(t *testing.T) *Config {
	t.Helper()

	cfg, err := NewV2Default("talos-valid", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	ApplyTalosDeploymentDefaults(cfg)
	cfg.Deployment.Talos.Network.ManagementCIDRs = []string{"203.0.113.10/32"}
	return cfg
}
