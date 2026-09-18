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

package cmd

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestAddConfigComments(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		useComplete  bool
		wantComments []string
	}{
		{
			name:        "openstack provider",
			provider:    "openstack",
			useComplete: true,
			wantComments: []string{
				"Configuration schema version",
				"Cluster metadata and annotations",
				"OpenCenter cluster configuration",
				"Infrastructure provider configuration",
				"Cloud provider: openstack",
				"Kubernetes cluster configuration",
				"GitOps repository configuration",
			},
		},
		{
			name:        "aws provider",
			provider:    "aws",
			useComplete: false,
			wantComments: []string{
				"Configuration schema version",
				"Cloud provider: aws",
			},
		},
		{
			name:        "all providers",
			provider:    "all",
			useComplete: false,
			wantComments: []string{
				"Configuration schema version",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Generate config
			var cfg v2.Config
			if tt.useComplete {
				cfg = generateCompleteTemplate(tt.provider)
			} else {
				cfg = generateMinimalTemplate(tt.provider)
			}

			// Marshal to YAML
			data, err := yaml.Marshal(&cfg)
			if err != nil {
				t.Fatalf("failed to marshal config: %v", err)
			}

			// Add comments
			output := addConfigComments(data, tt.provider)
			outputStr := string(output)

			// Verify comments are present
			for _, wantComment := range tt.wantComments {
				if !strings.Contains(outputStr, wantComment) {
					t.Errorf("expected comment %q not found in output", wantComment)
				}
			}

			// Verify output is still valid YAML
			var node yaml.Node
			if err := yaml.Unmarshal(output, &node); err != nil {
				t.Errorf("output is not valid YAML: %v", err)
			}
		})
	}
}

func TestClusterTemplateMagnumProvider(t *testing.T) {
	for _, minimal := range []bool{false, true} {
		name := "complete"
		if minimal {
			name = "minimal"
		}

		t.Run(name, func(t *testing.T) {
			cmd := newClusterTemplateCmd()
			var output strings.Builder
			cmd.SetOut(&output)
			args := []string{"--provider", "magnum"}
			if minimal {
				args = append(args, "--minimal")
			}
			cmd.SetArgs(args)

			if err := cmd.Execute(); err != nil {
				t.Fatalf("cluster template failed: %v", err)
			}

			var cfg v2.Config
			if err := yaml.Unmarshal([]byte(output.String()), &cfg); err != nil {
				t.Fatalf("generated template is not valid YAML: %v", err)
			}
			if cfg.OpenCenter.Infrastructure.Provider != "magnum" {
				t.Fatalf("provider = %q, want magnum", cfg.OpenCenter.Infrastructure.Provider)
			}
			if cfg.OpenCenter.Infrastructure.Cloud.Magnum == nil {
				t.Fatal("expected cloud.magnum configuration")
			}
			if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
				t.Fatal("expected cloud.openstack to be omitted for Magnum")
			}

			magnum := cfg.OpenCenter.Infrastructure.Cloud.Magnum
			for field, value := range map[string]string{
				"auth_url":                      magnum.AuthURL,
				"region":                        magnum.Region,
				"project_id":                    magnum.ProjectID,
				"application_credential_id":     magnum.ApplicationCredentialID,
				"application_credential_secret": magnum.ApplicationCredentialSecret,
				"cluster_template":              magnum.ClusterTemplate,
			} {
				if strings.TrimSpace(value) == "" {
					t.Errorf("cloud.magnum.%s is empty", field)
				}
			}
		})
	}
}

func TestClusterTemplateMagnumProviderValidationAndHelp(t *testing.T) {
	cmd := newClusterTemplateCmd()
	if usage := cmd.Flags().Lookup("provider").Usage; !strings.Contains(usage, "magnum") {
		t.Fatalf("provider flag help does not mention magnum: %q", usage)
	}

	cmd.SetArgs([]string{"--provider", "unknown"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), "magnum") {
		t.Fatalf("invalid provider error = %v, want Magnum in supported list", err)
	}
}

func TestAddConfigCommentsPreservesStructure(t *testing.T) {
	// Generate a complete config
	cfg := generateCompleteTemplate("openstack")

	// Marshal to YAML
	data, err := yaml.Marshal(&cfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	// Add comments
	output := addConfigComments(data, "openstack")

	// Parse back into config
	var parsedCfg v2.Config
	if err := yaml.Unmarshal(output, &parsedCfg); err != nil {
		t.Fatalf("failed to unmarshal commented YAML: %v", err)
	}

	// Verify key fields are preserved
	if parsedCfg.SchemaVersion != cfg.SchemaVersion {
		t.Errorf("schema_version mismatch: got %q, want %q", parsedCfg.SchemaVersion, cfg.SchemaVersion)
	}

	if parsedCfg.OpenCenter.Meta.Name != cfg.OpenCenter.Meta.Name {
		t.Errorf("cluster name mismatch: got %q, want %q", parsedCfg.OpenCenter.Meta.Name, cfg.OpenCenter.Meta.Name)
	}

	if parsedCfg.OpenCenter.Infrastructure.Provider != cfg.OpenCenter.Infrastructure.Provider {
		t.Errorf("provider mismatch: got %q, want %q", parsedCfg.OpenCenter.Infrastructure.Provider, cfg.OpenCenter.Infrastructure.Provider)
	}
}

func TestAddCommentsToNodeHandlesNilNode(t *testing.T) {
	// Should not panic with nil node
	addCommentsToNode(nil, "openstack")
}

func TestAddCommentsToNodeHandlesEmptyNode(t *testing.T) {
	// Should not panic with empty node
	node := &yaml.Node{Kind: yaml.MappingNode}
	addCommentsToNode(node, "openstack")
}

func TestAddConfigCommentsHandlesInvalidYAML(t *testing.T) {
	// Invalid YAML should return data with header
	invalidYAML := []byte("invalid: yaml: data: [")
	output := addConfigComments(invalidYAML, "openstack")

	// Should contain header
	outputStr := string(output)
	if !strings.Contains(outputStr, "Complete opencenter Cluster Configuration Template") {
		t.Error("expected header in output for invalid YAML")
	}

	// Should contain original data
	if !strings.Contains(outputStr, "invalid: yaml: data: [") {
		t.Error("expected original data in output for invalid YAML")
	}
}

func TestSchemaVersionComments(t *testing.T) {
	cfg := generateMinimalTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	// Check for schema version comment
	if !strings.Contains(outputStr, "Configuration schema version (do not modify)") {
		t.Error("expected schema version comment")
	}

	if !strings.Contains(outputStr, "v2.0 schema") {
		t.Error("expected v2.0 schema line comment")
	}
}

func TestMetadataComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"Cluster metadata and annotations",
		"Timestamp when cluster was created",
		"Timestamp of last update",
		"User who created the cluster",
		"Key-value tags for organization",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected metadata comment %q not found", comment)
		}
	}
}

func TestInfrastructureComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"Infrastructure provider configuration",
		"Cloud provider: openstack",
		"Cloud provider-specific settings",
		"OpenStack provider configuration",
		"OpenStack Identity API endpoint",
		"OpenStack region",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected infrastructure comment %q not found", comment)
		}
	}
}

func TestClusterComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"Kubernetes cluster configuration",
		"Kubernetes cluster name",
		"Base DNS domain for cluster",
		"Administrator email address",
		"SSH public keys for node access",
		"Kubernetes version and node configuration",
		"Number of control plane nodes",
		"Number of worker nodes",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected cluster comment %q not found", comment)
		}
	}
}

func TestGitOpsComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"GitOps repository configuration",
		"Local GitOps repository directory",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected GitOps comment %q not found", comment)
		}
	}
}

func TestGenerateCompleteTemplateUsesSharedOpenStackDefaults(t *testing.T) {
	t.Setenv("OPENCENTER_CONFIG_DIR", t.TempDir())
	expectedPtr, err := v2.NewV2Default("example-cluster", "openstack")
	if err != nil {
		t.Fatalf("NewV2Default() error = %v", err)
	}
	expected := *expectedPtr

	cfg := generateCompleteTemplate("openstack")

	if cfg.OpenCenter.Infrastructure.Provider != expected.OpenCenter.Infrastructure.Provider {
		t.Fatalf("expected provider %q, got %q", expected.OpenCenter.Infrastructure.Provider, cfg.OpenCenter.Infrastructure.Provider)
	}
	if expected.OpenCenter.Meta.Region != "DFW3" {
		t.Fatalf("shared default Meta.Region = %q, want DFW3", expected.OpenCenter.Meta.Region)
	}
	if expected.OpenCenter.Infrastructure.Cloud.OpenStack.Region != "DFW3" {
		t.Fatalf("shared default OpenStack region = %q, want DFW3", expected.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
	}
	if expected.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL != "https://keystone.api.dfw3.rackspacecloud.com/v3/" {
		t.Fatalf("shared default auth URL = %q, want lowercase catalog hostname", expected.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region != expected.OpenCenter.Infrastructure.Cloud.OpenStack.Region {
		t.Fatalf("expected region %q, got %q", expected.OpenCenter.Infrastructure.Cloud.OpenStack.Region, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region)
	}
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID != expected.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID {
		t.Fatalf("expected image ID %q, got %q", expected.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID, cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID)
	}
	if cfg.OpenCenter.Infrastructure.Compute.FlavorMaster != expected.OpenCenter.Infrastructure.Compute.FlavorMaster {
		t.Fatalf("expected master flavor %q, got %q", expected.OpenCenter.Infrastructure.Compute.FlavorMaster, cfg.OpenCenter.Infrastructure.Compute.FlavorMaster)
	}
	if cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass != expected.OpenCenter.Infrastructure.Storage.DefaultStorageClass {
		t.Fatalf("expected storage class %q, got %q", expected.OpenCenter.Infrastructure.Storage.DefaultStorageClass, cfg.OpenCenter.Infrastructure.Storage.DefaultStorageClass)
	}
	if cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled != expected.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled {
		t.Fatalf("expected cinder enabled %v, got %v", expected.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled, cfg.OpenCenter.Cluster.Kubernetes.StoragePlugin.CinderCsi.Enabled)
	}
}

func TestProviderSpecificComments(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		wantComments []string
	}{
		{
			name:     "openstack comments",
			provider: "openstack",
			wantComments: []string{
				"OpenStack provider configuration",
				"OpenStack Identity API endpoint",
				"Application credential ID",
				"Base OS image ID for nodes",
			},
		},
		{
			name:     "vmware comments",
			provider: "vmware",
			wantComments: []string{
				"VMware vSphere configuration",
				"vCenter server hostname or IP",
				"VMware datacenter name",
				"Pre-provisioned VM nodes",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := generateCompleteTemplate(tt.provider)
			data, _ := yaml.Marshal(&cfg)
			output := addConfigComments(data, tt.provider)
			outputStr := string(output)

			for _, comment := range tt.wantComments {
				if !strings.Contains(outputStr, comment) {
					t.Errorf("expected provider-specific comment %q not found", comment)
				}
			}
		})
	}
}

func TestSecretsComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"Secrets management configuration",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected secrets comment %q not found", comment)
		}
	}
}

func TestOpenTofuComments(t *testing.T) {
	cfg := generateCompleteTemplate("openstack")
	data, _ := yaml.Marshal(&cfg)
	output := addConfigComments(data, "openstack")
	outputStr := string(output)

	expectedComments := []string{
		"OpenTofu/Terraform configuration",
		"Enable infrastructure provisioning with OpenTofu",
	}

	for _, comment := range expectedComments {
		if !strings.Contains(outputStr, comment) {
			t.Errorf("expected OpenTofu comment %q not found", comment)
		}
	}
}
