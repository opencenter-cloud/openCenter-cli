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
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func newClusterTemplateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "template",
		Short:  "Generate a complete cluster configuration template with all fields",
		Hidden: true, // Hidden from help - development/documentation command
		Long: `Generate a complete cluster configuration template showing all available fields.

This command creates a comprehensive YAML configuration file that includes:
- All configuration sections (opencenter, opentofu, secrets, metadata)
- All service configurations with their specific fields
- All GA provider options (OpenStack, VMware, Kind, Baremetal)
- Inline documentation and examples
- Default values for all fields

The generated template is useful for:
- Understanding the complete configuration schema
- Documentation and examples
- IDE autocomplete reference
- Migration from older schema versions
- Testing configuration validation

Unlike 'cluster init', which creates a minimal working configuration,
'cluster template' generates a comprehensive reference showing every
available option.`,
		Example: `  # Generate template to stdout
  opencenter cluster template

  # Save template to file
  opencenter cluster template --out complete-config.yaml

  # Generate template for specific provider
  opencenter cluster template --provider openstack --out openstack-template.yaml

  # Generate with comments explaining each field
  opencenter cluster template --comments --out documented-config.yaml

  # Generate minimal template (only required fields)
  opencenter cluster template --minimal --out minimal-config.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			outPath, _ := cmd.Flags().GetString("out")
			provider, _ := cmd.Flags().GetString("provider")
			withComments, _ := cmd.Flags().GetBool("comments")
			minimal, _ := cmd.Flags().GetBool("minimal")

			// Validate provider
			validProviders := []string{"openstack", "kind", "baremetal", "vmware", "all"}
			if provider != "" && provider != "all" {
				valid := false
				for _, p := range validProviders {
					if p == provider {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid provider '%s', must be one of: openstack, kind, baremetal, vmware, all", provider)
				}
			}

			// Generate template configuration
			var cfg v2.Config
			if minimal {
				cfg = generateMinimalTemplate(provider)
			} else {
				cfg = generateCompleteTemplate(provider)
			}

			// Marshal to YAML
			data, err := yaml.Marshal(&cfg)
			if err != nil {
				return fmt.Errorf("failed to marshal configuration: %w", err)
			}

			// Add comments if requested
			var output []byte
			if withComments {
				output = addConfigComments(data, provider)
			} else {
				output = data
			}

			// Write to file or stdout
			if outPath == "" {
				fmt.Fprint(cmd.OutOrStdout(), string(output))
				return nil
			}

			// Ensure directory exists
			if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
				return fmt.Errorf("failed to create output directory: %w", err)
			}

			if err := os.WriteFile(outPath, output, 0o644); err != nil {
				return fmt.Errorf("failed to write template file: %w", err)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Complete cluster configuration template written to %s\n", outPath)
			fmt.Fprintf(cmd.OutOrStdout(), "Schema version: %s\n", cfg.SchemaVersion)
			return nil
		},
	}

	cmd.Flags().String("out", "", "output file path (default stdout)")
	cmd.Flags().String("provider", "all", "generate template for specific provider (openstack, kind, baremetal, vmware, all)")
	cmd.Flags().Bool("comments", false, "include inline comments explaining each field")
	cmd.Flags().Bool("minimal", false, "generate minimal template with only required fields")

	return cmd
}

// generateCompleteTemplate creates a configuration with all fields populated
func templateBaseConfig(provider string) v2.Config {
	selectedProvider := provider
	if selectedProvider == "" || selectedProvider == "all" {
		selectedProvider = "openstack"
	}

	cfg, err := v2.NewV2Default("example-cluster", selectedProvider)
	if err != nil {
		cfg = &v2.Config{
			SchemaVersion: "2.0",
			OpenCenter: v2.OpenCenterConfig{
				Meta: v2.MetaConfig{
					Name:         "example-cluster",
					Organization: "opencenter",
					Env:          "dev",
					Region:       "sjc3",
				},
				Cluster: v2.ClusterConfig{
					ClusterName: "example-cluster",
					BaseDomain:  "k8s.opencenter.cloud",
					ClusterFQDN: "example-cluster.sjc3.k8s.opencenter.cloud",
					AdminEmail:  "admin@example.com",
				},
				Infrastructure: v2.InfrastructureConfig{
					Provider: selectedProvider,
				},
				GitOps: v2.GitOpsConfig{
					Repository: v2.GitOpsRepository{
						LocalDir: "clusters/opencenter",
						URL:      "https://example.invalid/opencenter/example-cluster.git",
					},
				},
			},
			OpenTofu: v2.OpenTofuConfig{
				Enabled: true,
				Backend: v2.BackendConfig{Type: "local"},
			},
			Secrets: v2.SecretsConfig{},
		}
	}

	return *cfg
}

func generateCompleteTemplate(provider string) v2.Config {
	cfg := templateBaseConfig(provider)

	// Populate all optional fields with example values
	cfg.Metadata.CreatedBy = "admin@example.com"
	cfg.Metadata.Labels = map[string]string{
		"environment": "production",
		"cost-center": "engineering",
		"team":        "platform",
	}
	cfg.Metadata.Annotations = map[string]string{
		"description": "Complete cluster configuration template",
		"owner":       "Platform Team",
	}

	// Populate cluster fields
	cfg.OpenCenter.Cluster.BaseDomain = "k8s.opencenter.cloud"
	cfg.OpenCenter.Cluster.ClusterFQDN = "example-cluster.sjc3.k8s.opencenter.cloud"
	cfg.OpenCenter.Cluster.AdminEmail = "admin@example.com"
	cfg.OpenCenter.Infrastructure.SSH.AuthorizedKeys = []string{
		"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFAKEPLACEHOLDEREXAMPLEKEY opencenter@example",
	}

	// Populate Kubernetes configuration
	cfg.OpenCenter.Cluster.Kubernetes.Version = "1.33.5"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 3

	// Configure provider-specific settings
	switch provider {
	case "openstack":
		cfg.OpenCenter.Infrastructure.Provider = "openstack"
		populateOpenStackConfig(&cfg)
	case "kind":
		cfg.OpenCenter.Infrastructure.Provider = "kind"
	case "baremetal":
		cfg.OpenCenter.Infrastructure.Provider = "baremetal"
		populateBaremetalConfig(&cfg)
	case "vmware":
		cfg.OpenCenter.Infrastructure.Provider = "vmware"
		populateVMwareConfig(&cfg)
	case "all", "":
		// Include all provider configurations
		populateOpenStackConfig(&cfg)
		populateVMwareConfig(&cfg)
	}

	return cfg
}

// generateMinimalTemplate creates a configuration with only required fields
func generateMinimalTemplate(provider string) v2.Config {
	cfg := templateBaseConfig(provider)
	cfg.Metadata = v2.ConfigMetadata{}
	cfg.OpenCenter.Services = nil
	cfg.OpenCenter.ManagedServices = nil
	cfg.OpenCenter.LegacyManaged = nil
	return cfg
}

// populateOpenStackConfig adds OpenStack-specific configuration
func populateOpenStackConfig(cfg *v2.Config) {
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL = "https://identity.api.sjc3.rackspacecloud.com/v3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Region = "sjc3"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Domain = "Default"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialID = "app-credential-id"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ApplicationCredentialSecret = "app-credential-secret"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ImageID = "799dcf97-3656-4361-8187-13ab1b295e33"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.FloatingIPPool = "PUBLICNET"
}

// populateBaremetalConfig adds baremetal-specific configuration
func populateBaremetalConfig(cfg *v2.Config) {
	cfg.OpenCenter.Infrastructure.Provider = "baremetal"
	cfg.OpenCenter.Infrastructure.SSH.Username = "ubuntu"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1
}

// addConfigComments adds inline comments to the YAML output using yaml.v3 Node API
func addConfigComments(data []byte, provider string) []byte {
	// Parse YAML into node tree
	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		// If parsing fails, return original data with header
		header := []byte(`# Complete opencenter Cluster Configuration Template
# Schema Version: 2.0
# Generated by: opencenter cluster template
#
# This template shows all available configuration options.
# Remove or comment out sections you don't need.
# See https://docs.opencenter.cloud for detailed documentation.

`)
		return append(header, data...)
	}

	// Add comments to nodes
	addCommentsToNode(&node, provider)

	// Re-marshal with comments
	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(&node); err != nil {
		// If encoding fails, return original data with header
		header := []byte(`# Complete opencenter Cluster Configuration Template
# Schema Version: 2.0
# Generated by: opencenter cluster template
#
# This template shows all available configuration options.
# Remove or comment out sections you don't need.
# See https://docs.opencenter.cloud for detailed documentation.

`)
		return append(header, data...)
	}

	return buf.Bytes()
}

// addCommentsToNode recursively adds comments to YAML nodes
func addCommentsToNode(node *yaml.Node, provider string) {
	if node == nil {
		return
	}

	// Handle document node
	if node.Kind == yaml.DocumentNode {
		for _, child := range node.Content {
			addCommentsToNode(child, provider)
		}
		return
	}

	// Only process mapping nodes
	if node.Kind != yaml.MappingNode {
		return
	}

	// Add comments to specific fields
	for i := 0; i < len(node.Content); i += 2 {
		if i+1 >= len(node.Content) {
			break
		}

		key := node.Content[i]
		value := node.Content[i+1]

		switch key.Value {
		case "schema_version":
			addSchemaVersionComments(key, value)
		case "metadata":
			addMetadataComments(key, value)
		case "opencenter":
			addOpenCenterComments(key, value, provider)
		case "opentofu":
			addOpenTofuComments(key, value)
		case "secrets":
			addSecretsComments(key, value)
		case "deployment":
			addDeploymentComments(key, value)
		}

		// Recurse into nested nodes
		addCommentsToNode(value, provider)
	}
}

// addSchemaVersionComments adds comments for the schema_version field
func addSchemaVersionComments(key, value *yaml.Node) {
	key.HeadComment = "Configuration schema version (do not modify)"
	value.LineComment = "v2.0 schema"
}

// addMetadataComments adds comments for the metadata section
func addMetadataComments(key, value *yaml.Node) {
	key.HeadComment = "Cluster metadata and annotations"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "created_at":
			subKey.LineComment = "Timestamp when cluster was created"
		case "updated_at":
			subKey.LineComment = "Timestamp of last update"
		case "created_by":
			subKey.LineComment = "User who created the cluster"
		case "tags", "labels":
			subKey.LineComment = "Key-value tags for organization"
		case "annotations":
			subKey.LineComment = "Additional metadata annotations"
		}
	}
}

// addOpenCenterComments adds comments for the opencenter section
func addOpenCenterComments(key, value *yaml.Node, provider string) {
	key.HeadComment = "OpenCenter cluster configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		subValue := value.Content[i+1]

		switch subKey.Value {
		case "meta":
			addMetaComments(subKey, subValue)
		case "infrastructure":
			addInfrastructureComments(subKey, subValue, provider)
		case "cluster":
			addClusterComments(subKey, subValue)
		case "gitops":
			addGitOpsComments(subKey, subValue)
		case "talos":
			addTalosComments(subKey, subValue)
		}
	}
}

// addMetaComments adds comments for the meta section
func addMetaComments(key, value *yaml.Node) {
	key.HeadComment = "Cluster identification"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "name":
			subKey.LineComment = "Unique cluster name"
		case "organization":
			subKey.LineComment = "Organization or team name"
		}
	}
}

// addInfrastructureComments adds comments for the infrastructure section
func addInfrastructureComments(key, value *yaml.Node, provider string) {
	key.HeadComment = "Infrastructure provider configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		subValue := value.Content[i+1]

		switch subKey.Value {
		case "provider":
			subKey.LineComment = fmt.Sprintf("Cloud provider: %s (openstack, kind, baremetal, vmware)", provider)
		case "ssh":
			addSSHComments(subKey, subValue)
		case "compute":
			addComputeComments(subKey, subValue)
		case "cloud":
			addCloudComments(subKey, subValue, provider)
		case "bastion":
			subKey.LineComment = "Bastion host configuration for baremetal/vmware"
		}
	}
}

func addSSHComments(key, value *yaml.Node) {
	key.HeadComment = "SSH configuration for node access"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "authorized_keys":
			subKey.LineComment = "SSH public keys for node access"
		case "username":
			subKey.LineComment = "SSH username for cluster nodes"
		}
	}
}

func addComputeComments(key, value *yaml.Node) {
	key.HeadComment = "Compute sizing and node counts"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "master_count":
			subKey.LineComment = "Number of control plane nodes (odd number recommended)"
		case "worker_count":
			subKey.LineComment = "Number of worker nodes"
		case "flavor_master":
			subKey.LineComment = "Flavor for control plane nodes"
		case "flavor_worker":
			subKey.LineComment = "Flavor for worker nodes"
		}
	}
}

// addCloudComments adds comments for cloud provider configurations
func addCloudComments(key, value *yaml.Node, provider string) {
	key.HeadComment = "Cloud provider-specific settings"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		subValue := value.Content[i+1]

		switch subKey.Value {
		case "openstack":
			addOpenStackComments(subKey, subValue)
		case "vmware":
			addVMwareComments(subKey, subValue)
		}
	}
}

// addOpenStackComments adds comments for OpenStack configuration
func addOpenStackComments(key, value *yaml.Node) {
	key.HeadComment = "OpenStack provider configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "auth_url":
			subKey.LineComment = "OpenStack Identity API endpoint"
		case "region":
			subKey.LineComment = "OpenStack region"
		case "domain":
			subKey.LineComment = "OpenStack domain (usually 'Default')"
		case "application_credential_id":
			subKey.LineComment = "Application credential ID (preferred over password)"
		case "application_credential_secret":
			subKey.LineComment = "Application credential secret"
		case "image_id":
			subKey.LineComment = "Base OS image ID for nodes"
		case "networking":
			subKey.LineComment = "OpenStack networking configuration"
		}
	}
}

// addClusterComments adds comments for the cluster section
func addClusterComments(key, value *yaml.Node) {
	key.HeadComment = "Kubernetes cluster configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		subValue := value.Content[i+1]

		switch subKey.Value {
		case "cluster_name":
			subKey.LineComment = "Kubernetes cluster name"
		case "base_domain":
			subKey.LineComment = "Base DNS domain for cluster"
		case "cluster_fqdn":
			subKey.LineComment = "Full cluster domain name"
		case "admin_email":
			subKey.LineComment = "Administrator email address"
		case "ssh_authorized_keys":
			subKey.LineComment = "SSH public keys for node access"
		case "kubernetes":
			addKubernetesComments(subKey, subValue)
		case "networking":
			subKey.LineComment = "Cluster networking configuration"
		}
	}
}

// addKubernetesComments adds comments for Kubernetes configuration
func addKubernetesComments(key, value *yaml.Node) {
	key.HeadComment = "Kubernetes version and node configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "version":
			subKey.LineComment = "Kubernetes version (e.g., 1.33.5)"
		case "master_count":
			subKey.LineComment = "Number of control plane nodes (odd number recommended)"
		case "worker_count":
			subKey.LineComment = "Number of worker nodes"
		case "master_nodes":
			subKey.LineComment = "Static control plane node definitions (baremetal)"
		case "worker_nodes":
			subKey.LineComment = "Static worker node definitions (baremetal)"
		}
	}
}

// addGitOpsComments adds comments for GitOps configuration
func addGitOpsComments(key, value *yaml.Node) {
	key.HeadComment = "GitOps repository configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		subValue := value.Content[i+1]
		switch subKey.Value {
		case "repository":
			subKey.LineComment = "Cluster-specific repository settings"
			addGitOpsRepositoryComments(subValue)
		case "base_repo":
			subKey.LineComment = "Upstream template repository settings"
		case "auth":
			subKey.LineComment = "Authentication configuration"
		case "flux":
			subKey.LineComment = "FluxCD reconciliation settings"
		}
	}
}

// addGitOpsRepositoryComments adds comments for GitOps repository sub-fields
func addGitOpsRepositoryComments(value *yaml.Node) {
	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "local_dir":
			subKey.LineComment = "Local GitOps repository directory"
		case "url":
			subKey.LineComment = "Remote GitOps repository URL"
		case "branch":
			subKey.LineComment = "Git branch for cluster manifests"
		case "path":
			subKey.LineComment = "Directory within repo for this cluster"
		case "secret_name":
			subKey.LineComment = "K8s secret name for repository access"
		}
	}
}

// addTalosComments adds comments for Talos configuration
func addTalosComments(key, value *yaml.Node) {
	key.HeadComment = "Talos Linux configuration (if using Talos provider)"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "enabled":
			subKey.LineComment = "Enable Talos Linux"
		case "version":
			subKey.LineComment = "Talos version"
		case "install_disk":
			subKey.LineComment = "Disk device for Talos installation"
		}
	}
}

// addOpenTofuComments adds comments for OpenTofu configuration
func addOpenTofuComments(key, value *yaml.Node) {
	key.HeadComment = "OpenTofu/Terraform configuration"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "enabled":
			subKey.LineComment = "Enable infrastructure provisioning with OpenTofu"
		case "version":
			subKey.LineComment = "OpenTofu version"
		case "backend":
			subKey.LineComment = "Terraform backend configuration"
		}
	}
}

// addSecretsComments adds comments for secrets configuration
func addSecretsComments(key, value *yaml.Node) {
	key.HeadComment = "Secrets management configuration (SOPS/Age)"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "sops_age_key_file":
			subKey.LineComment = "Path to Age encryption key"
		case "sops_age_recipients":
			subKey.LineComment = "Age public keys for encryption"
		}
	}
}

// addDeploymentComments adds comments for deployment configuration
func addDeploymentComments(key, value *yaml.Node) {
	key.HeadComment = "Deployment tracking and status"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "stage":
			subKey.LineComment = "Current deployment stage"
		case "status":
			subKey.LineComment = "Deployment status"
		case "last_updated":
			subKey.LineComment = "Last deployment update timestamp"
		}
	}
}

// populateVMwareConfig adds VMware-specific configuration
// VMware is treated as baremetal - requires pre-provisioned VMs
func populateVMwareConfig(cfg *v2.Config) {
	if cfg.OpenCenter.Infrastructure.Cloud.VMware == nil {
		cfg.OpenCenter.Infrastructure.Cloud.VMware = &v2.VMwareCloudConfig{}
	}
	cfg.OpenCenter.Infrastructure.Cloud.VMware.VCenterServer = "vcenter.example.com"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Datacenter = "Datacenter1"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Datastore = "datastore1"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Cluster = "Cluster1"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Folder = "/vm/kubernetes"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Network = "VM Network"
	cfg.OpenCenter.Infrastructure.Cloud.VMware.Template = "ubuntu-2404-template"
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 1
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 1
}

// addVMwareComments adds comments for VMware configuration
func addVMwareComments(key, value *yaml.Node) {
	key.HeadComment = "VMware vSphere configuration (Pre-provisioned VM nodes are represented through native v2 compute counts)"

	if value.Kind != yaml.MappingNode {
		return
	}

	for i := 0; i < len(value.Content); i += 2 {
		if i+1 >= len(value.Content) {
			break
		}
		subKey := value.Content[i]
		switch subKey.Value {
		case "vcenter_server":
			subKey.LineComment = "vCenter server hostname or IP"
		case "datacenter":
			subKey.LineComment = "VMware datacenter name"
		case "datastore":
			subKey.LineComment = "Default datastore for persistent volumes"
		case "cluster":
			subKey.LineComment = "VMware compute cluster name"
		case "resource_pool":
			subKey.LineComment = "Resource pool for VMs (optional)"
		case "folder":
			subKey.LineComment = "VM folder path (optional)"
		case "network":
			subKey.LineComment = "Network name for VMs"
		case "nodes":
			subKey.LineComment = "Pre-provisioned VM nodes (name, ip, role required)"
		}
	}
}
