package v2

import (
	"strings"
	"testing"

	registrydefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	"gopkg.in/yaml.v3"
)

func TestNewV2DefaultProviders(t *testing.T) {
	loader := NewConfigLoader(registrydefaults.NewRegistry())

	tests := []struct {
		provider           string
		expectOpenStackCSI bool
		expectVSphereCSI   bool
		expectOpenStack    bool
		expectVMware       bool
	}{
		{provider: "openstack", expectOpenStackCSI: true, expectOpenStack: true},
		{provider: "kind"},
		{provider: "baremetal"},
		{provider: "vmware", expectVSphereCSI: true, expectVMware: true},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg, err := NewV2Default("test-cluster", tt.provider)
			if err != nil {
				t.Fatalf("NewV2Default() error = %v", err)
			}

			if cfg.OpenCenter.Infrastructure.Provider != tt.provider {
				t.Fatalf("provider = %q, want %q", cfg.OpenCenter.Infrastructure.Provider, tt.provider)
			}
			if cfg.OpenCenter.Meta.Stage != "init" {
				t.Fatalf("stage = %q, want init", cfg.OpenCenter.Meta.Stage)
			}
			if cfg.OpenCenter.Meta.Status != "success" {
				t.Fatalf("status = %q, want success", cfg.OpenCenter.Meta.Status)
			}
			if cfg.Secrets.SOPSConfig.AgeKeyFile == "" {
				t.Fatal("expected deterministic SOPS age key placeholder")
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}
			if _, err := loader.LoadFromBytes(data); err != nil {
				t.Fatalf("LoadFromBytes() error = %v", err)
			}

			openStackCSIEnabled := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi != nil &&
				cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled
			if openStackCSIEnabled != tt.expectOpenStackCSI {
				t.Fatalf("CinderCsi enabled = %v, want %v", openStackCSIEnabled, tt.expectOpenStackCSI)
			}

			vsphereCSIEnabled := cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi != nil &&
				cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.VSphereCsi.Enabled
			if vsphereCSIEnabled != tt.expectVSphereCSI {
				t.Fatalf("VSphereCsi enabled = %v, want %v", vsphereCSIEnabled, tt.expectVSphereCSI)
			}

			if (cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil) != tt.expectOpenStack {
				t.Fatalf("OpenStack cloud present = %v, want %v", cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil, tt.expectOpenStack)
			}
			if (cfg.OpenCenter.Infrastructure.Cloud.VMware != nil) != tt.expectVMware {
				t.Fatalf("VMware cloud present = %v, want %v", cfg.OpenCenter.Infrastructure.Cloud.VMware != nil, tt.expectVMware)
			}
		})
	}
}

func TestRenderFullTemplateYAMLRoundTrip(t *testing.T) {
	loader := NewConfigLoader(registrydefaults.NewRegistry())

	data, err := RenderFullTemplateYAML("full-one", "openstack")
	if err != nil {
		t.Fatalf("RenderFullTemplateYAML() error = %v", err)
	}

	if strings.Contains(string(data), "\niac:") {
		t.Fatalf("expected full template to avoid legacy iac section, got:\n%s", string(data))
	}

	if _, err := loader.LoadFromBytes(data); err != nil {
		t.Fatalf("LoadFromBytes() error = %v", err)
	}
}

// TestNewV2Default_NoKnownLegacyFieldContamination verifies that NewV2Default produces
// YAML that contains only current schema fields and none of the known legacy fields.
func TestNewV2Default_NoKnownLegacyFieldContamination(t *testing.T) {
	// Known legacy fields that must NOT appear anywhere in current output.
	// Fields like flavor_master and master_count exist in v2 too but under
	// infrastructure.compute — the loader's KnownFields(true) check already
	// ensures they don't appear in the wrong section.
	knownLegacyFields := []string{
		"kubespray_version:",
		"cni_iface:",
		"calico_interface_autodetect:",
		"autodetect_cidr:",
		"encapsulation_type:",
		"nat_outgoing:",
		"operator_enabled:",
		"kubeProxyReplacement:",
		"cilium_integration:",
		"kube_oidc_url:",
		"kube_oidc_client_id:",
		"kube_oidc_ca_file:",
		"kube_oidc_username_claim:",
		"kube_oidc_groups_claim:",
		"windows_workers:",
		"k8s_hardening:",
		"pod_security_exemptions:",
		"ssh_authorized_keys:",
		"aws_access_key:",
		"aws_secret_access_key:",
		"managed-service:",
		"ssh_user:",
	}

	// v2 structural fields that MUST appear
	v2RequiredFields := []string{
		"schema_version:",
		"infrastructure:",
		"  ssh:",
		"    authorized_keys:",
		"  networking:",
		"  compute:",
		"  storage:",
		"  cloud:",
		"cluster:",
		"  kubernetes:",
		"    network_plugin:",
		"deployment:",
		"  method:",
		"opentofu:",
		"  backend:",
		"secrets:",
		"  sops:",
	}

	providers := []string{"openstack", "kind", "vmware", "baremetal"}

	for _, provider := range providers {
		t.Run(provider, func(t *testing.T) {
			cfg, err := NewV2Default("v2-check", provider)
			if err != nil {
				t.Fatalf("NewV2Default() error = %v", err)
			}

			data, err := yaml.Marshal(cfg)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}
			yamlStr := string(data)

			// Check no known legacy fields leaked
			for _, field := range knownLegacyFields {
				if strings.Contains(yamlStr, field) {
					t.Errorf("known legacy field %q found in %s v2 output", field, provider)
				}
			}

			// Check v2 structural fields present
			for _, field := range v2RequiredFields {
				if !strings.Contains(yamlStr, field) {
					t.Errorf("v2 field %q missing from %s output", field, provider)
				}
			}

			// Verify schema version
			if cfg.SchemaVersion != "2.0" {
				t.Errorf("schema_version = %q, want '2.0'", cfg.SchemaVersion)
			}

			// Verify v2 structural placement: compute fields are under infrastructure, not cluster
			if strings.Contains(yamlStr, "cluster:\n") {
				// Find the cluster section and verify it doesn't contain compute fields
				clusterIdx := strings.Index(yamlStr, "cluster:\n")
				infraIdx := strings.Index(yamlStr, "infrastructure:\n")
				if clusterIdx > 0 && infraIdx > 0 {
					// cluster section should NOT contain master_count or flavor_*
					// (those belong in infrastructure.compute in v2)
					clusterSection := yamlStr[clusterIdx:]
					if nextTopLevel := strings.Index(clusterSection[9:], "\n    "); nextTopLevel > 0 {
						clusterSection = clusterSection[:nextTopLevel+9]
					}
					if strings.Contains(clusterSection, "master_count") {
						t.Error("master_count found in cluster section (should be in infrastructure.compute)")
					}
				}
			}
		})
	}
}

// TestNewV2Default_KindSpecificBehavior verifies Kind-specific defaults.
func TestNewV2Default_KindSpecificBehavior(t *testing.T) {
	cfg, err := NewV2Default("kind-test", "kind")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}

	if cfg.OpenCenter.Cluster.Kubernetes.KubeVIPEnabled {
		t.Error("KubeVIP should be disabled for Kind")
	}
	if cfg.OpenCenter.Infrastructure.Networking.VRRPEnabled {
		t.Error("VRRP should be disabled for Kind")
	}
	if cfg.OpenCenter.Infrastructure.Bastion.Enabled {
		t.Error("Bastion should be disabled for Kind")
	}
	if cfg.OpenTofu.Enabled {
		t.Error("OpenTofu should be disabled for Kind")
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		t.Error("OpenStack cloud config should be nil for Kind")
	}
	if cfg.OpenCenter.Infrastructure.Cloud.VMware != nil {
		t.Error("VMware cloud config should be nil for Kind")
	}

	// Kind-specific overrides from kind.yaml defaults
	if got := cfg.OpenCenter.Cluster.Kubernetes.Version; got != "1.33.7" {
		t.Errorf("Kubernetes version = %q, want %q", got, "1.33.7")
	}
	if got := cfg.OpenCenter.Cluster.Kubernetes.APIPort; got != 6443 {
		t.Errorf("API port = %d, want %d", got, 6443)
	}
	if got := cfg.OpenCenter.Infrastructure.Compute.MasterCount; got != 1 {
		t.Errorf("MasterCount = %d, want %d", got, 1)
	}
	if got := cfg.OpenCenter.Infrastructure.Compute.WorkerCount; got != 2 {
		t.Errorf("WorkerCount = %d, want %d", got, 2)
	}
	if got := cfg.OpenCenter.Cluster.Kubernetes.SubnetPods; got != "10.244.0.0/16" {
		t.Errorf("SubnetPods = %q, want %q", got, "10.244.0.0/16")
	}
	if got := cfg.OpenCenter.Cluster.Kubernetes.SubnetServices; got != "10.96.0.0/16" {
		t.Errorf("SubnetServices = %q, want %q", got, "10.96.0.0/16")
	}
}
