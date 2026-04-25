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

package config

import (
	"fmt"
	"strings"
)

// SuggestionEngine provides intelligent suggestions for configuration errors.
type SuggestionEngine struct {
	fieldSuggestions map[string][]string
	typeSuggestions  map[string][]string
}

// NewSuggestionEngine creates a new suggestion engine with predefined suggestions.
func NewSuggestionEngine() *SuggestionEngine {
	engine := &SuggestionEngine{
		fieldSuggestions: make(map[string][]string),
		typeSuggestions:  make(map[string][]string),
	}
	engine.initializeFieldSuggestions()
	engine.initializeTypeSuggestions()
	return engine
}

// GetSuggestionsForField returns suggestions for a specific configuration field.
func (se *SuggestionEngine) GetSuggestionsForField(field string, value interface{}) []string {
	// Check for exact field match
	if suggestions, exists := se.fieldSuggestions[field]; exists {
		return suggestions
	}

	// Check for partial field match (e.g., any field ending with "password")
	for pattern, suggestions := range se.fieldSuggestions {
		if strings.HasSuffix(field, pattern) || strings.Contains(field, pattern) {
			return suggestions
		}
	}

	// Generate context-aware suggestions based on field name
	return se.generateContextAwareSuggestions(field, value)
}

// GetSuggestionsForType returns suggestions for a specific error type.
func (se *SuggestionEngine) GetSuggestionsForType(errorType string) []string {
	if suggestions, exists := se.typeSuggestions[errorType]; exists {
		return suggestions
	}
	return []string{}
}

// GetSuggestionsForMissingField returns suggestions for a missing required field.
func (se *SuggestionEngine) GetSuggestionsForMissingField(field string) []string {
	suggestions := []string{
		fmt.Sprintf("Add the required field '%s' to your configuration", field),
	}

	// Add field-specific guidance
	if fieldSuggestions := se.GetSuggestionsForField(field, nil); len(fieldSuggestions) > 0 {
		suggestions = append(suggestions, fieldSuggestions...)
	}

	return suggestions
}

// GetSuggestionsForInvalidValue returns suggestions for an invalid field value.
func (se *SuggestionEngine) GetSuggestionsForInvalidValue(field string, value interface{}, expectedFormat string) []string {
	suggestions := []string{
		fmt.Sprintf("The value '%v' is invalid for field '%s'", value, field),
	}

	if expectedFormat != "" {
		suggestions = append(suggestions, fmt.Sprintf("Expected format: %s", expectedFormat))
	}

	// Add field-specific suggestions
	if fieldSuggestions := se.GetSuggestionsForField(field, value); len(fieldSuggestions) > 0 {
		suggestions = append(suggestions, fieldSuggestions...)
	}

	return suggestions
}

// GetSuggestionsForConflict returns suggestions for conflicting configuration values.
func (se *SuggestionEngine) GetSuggestionsForConflict(field1, field2 string) []string {
	return []string{
		fmt.Sprintf("Fields '%s' and '%s' have conflicting values", field1, field2),
		"Review the configuration requirements for these fields",
		"Ensure only one of these options is enabled or configured",
		"Check the documentation for valid combinations",
	}
}

// initializeFieldSuggestions sets up field-specific suggestions.
func (se *SuggestionEngine) initializeFieldSuggestions() {
	// Cluster name suggestions
	se.fieldSuggestions["cluster_name"] = []string{
		"Use alphanumeric characters, hyphens, and underscores only",
		"Start with an alphanumeric character",
		"Keep length under 255 characters",
		"Example: 'my-cluster-prod'",
	}

	// Email suggestions
	se.fieldSuggestions["email"] = []string{
		"Use valid email format (e.g., admin@example.com)",
		"Ensure email contains @ symbol and valid domain",
	}

	// Domain suggestions
	se.fieldSuggestions["domain"] = []string{
		"Use valid domain format (e.g., k8s.opencenter.cloud)",
		"Domain must contain at least one dot and valid TLD",
	}

	se.fieldSuggestions["base_domain"] = []string{
		"Use valid domain format (e.g., k8s.opencenter.cloud)",
		"Domain must contain at least one dot and valid TLD",
		"This will be used as the base for cluster services",
	}

	se.fieldSuggestions["cluster_fqdn"] = []string{
		"Use valid FQDN format (e.g., my-cluster.sjc3.k8s.opencenter.cloud)",
		"FQDN must contain at least one dot and valid TLD",
		"This should be the full qualified domain name for the cluster",
	}

	// Kubernetes version suggestions
	se.fieldSuggestions["kubernetes.version"] = []string{
		"Use semantic versioning format (e.g., 1.31.4)",
		"Check Kubernetes release notes for supported versions",
		"Ensure the version is compatible with your provider",
	}

	// Node count suggestions
	se.fieldSuggestions["master_count"] = []string{
		"Set master_count to 1 for development or 3 for production",
		"Use odd numbers (1, 3, 5) for etcd quorum",
		"High availability requires at least 3 masters",
	}

	se.fieldSuggestions["worker_count"] = []string{
		"Set worker_count to 0 or higher",
		"Use at least 2 workers for production workloads",
		"Consider workload requirements when sizing",
	}

	// GitOps suggestions
	se.fieldSuggestions["git_dir"] = []string{
		"Set git_dir to a valid directory path",
		"Use a path where GitOps repository will be created",
		"Example: './gitops' or '/path/to/gitops'",
	}

	// OpenTofu/Terraform suggestions
	se.fieldSuggestions["opentofu.path"] = []string{
		"Set path to the directory containing Terraform files",
		"Use 'opentofu' for default path",
		"Ensure the directory exists or will be created",
	}

	se.fieldSuggestions["backend.type"] = []string{
		"Use 'local' for local state storage",
		"Use 's3' for remote state in AWS S3",
		"Remote backends are recommended for team collaboration",
	}

	// S3 backend suggestions
	se.fieldSuggestions["s3.bucket"] = []string{
		"S3 bucket names must be lowercase",
		"Bucket names must be globally unique",
		"Use descriptive names like 'my-org-terraform-state'",
	}

	se.fieldSuggestions["s3.region"] = []string{
		"Set region to an AWS region (e.g., 'us-east-1')",
		"Check AWS documentation for available regions",
		"Use the region closest to your infrastructure",
	}

	// Network plugin suggestions
	se.fieldSuggestions["network_plugin"] = []string{
		"Enable Calico for most use cases",
		"Enable Cilium for advanced networking features",
		"Enable Kube-OVN for overlay networking",
		"Only one network plugin can be enabled at a time",
	}

	se.fieldSuggestions["cni_iface"] = []string{
		"Set cni_iface to the network interface name (e.g., 'enp3s0')",
		"Use 'interface' for automatic detection",
		"Check your system's network interfaces with 'ip link'",
	}

	// Subnet suggestions
	se.fieldSuggestions["subnet_pods"] = []string{
		"Set subnet_pods to a CIDR range (e.g., '10.42.0.0/16')",
		"Ensure it doesn't conflict with node or service subnets",
		"Use a /16 or larger for adequate pod IP space",
	}

	se.fieldSuggestions["subnet_services"] = []string{
		"Set subnet_services to a CIDR range (e.g., '10.43.0.0/16')",
		"Ensure it doesn't conflict with node or pod subnets",
		"A /16 subnet provides 65,536 service IPs",
	}

	// OpenStack suggestions
	se.fieldSuggestions["openstack.auth_url"] = []string{
		"Set auth_url to your OpenStack Keystone endpoint",
		"Example: https://keystone.api.example.com/v3/",
		"Ensure the URL ends with /v3/ for Identity API v3",
	}

	se.fieldSuggestions["openstack.region"] = []string{
		"Set region to your OpenStack region name",
		"Check with your OpenStack administrator for available regions",
		"Use 'openstack region list' to see available regions",
	}

	se.fieldSuggestions["openstack.tenant_name"] = []string{
		"Set tenant_name to your OpenStack project/tenant",
		"Use project ID or project name",
		"Check with your OpenStack administrator if unsure",
	}

	se.fieldSuggestions["application_credential"] = []string{
		"Set application_credential_id and application_credential_secret",
		"Use SOPS to encrypt sensitive credentials",
		"Create application credentials in OpenStack dashboard",
		"Application credentials are more secure than username/password",
	}

	// AWS suggestions
	se.fieldSuggestions["aws.region"] = []string{
		"Set region to an AWS region (e.g., 'us-east-1')",
		"Check AWS documentation for available regions",
		"Use the region closest to your users",
	}

	se.fieldSuggestions["aws.vpc_id"] = []string{
		"Set vpc_id to use existing VPC",
		"Leave empty to create new VPC",
		"Ensure the VPC has appropriate CIDR blocks",
	}

	se.fieldSuggestions["aws_access_key"] = []string{
		"Set AWS access key for authentication",
		"Use SOPS to encrypt sensitive credentials",
		"Consider using IAM roles instead of access keys",
	}

	// SSH suggestions
	se.fieldSuggestions["ssh_authorized_keys"] = []string{
		"Add SSH public keys for cluster access",
		"Use ssh-keygen to generate key pairs if needed",
		"Format: 'ssh-rsa AAAAB3... user@host'",
	}

	// Password/secret suggestions
	se.fieldSuggestions["password"] = []string{
		"Use SOPS to encrypt sensitive credentials",
		"Generate strong passwords with sufficient entropy",
		"Never commit plaintext passwords to version control",
	}

	se.fieldSuggestions["secret"] = []string{
		"Use SOPS to encrypt sensitive credentials",
		"Store secrets in the secrets section of configuration",
		"Never commit plaintext secrets to version control",
	}

	// Service-specific suggestions
	se.fieldSuggestions["loki.storage_type"] = []string{
		"Set storage_type to 's3' or 'swift'",
		"Swift is recommended for OpenStack deployments",
		"S3 is recommended for AWS deployments",
	}

	se.fieldSuggestions["loki.bucket_name"] = []string{
		"Set bucket_name to your storage bucket or container name",
		"Use a descriptive name like 'loki-logs-prod'",
		"Ensure the bucket/container exists or will be created",
	}

	se.fieldSuggestions["cert_manager"] = []string{
		"cert-manager requires AWS credentials for Route53 DNS validation",
		"Set aws_access_key and aws_secret_access_key in secrets",
		"Use SOPS to encrypt sensitive credentials",
	}

	se.fieldSuggestions["keycloak.admin_password"] = []string{
		"Set admin_password for Keycloak admin user",
		"Use SOPS to encrypt sensitive credentials",
		"Generate a strong password with sufficient entropy",
	}

	se.fieldSuggestions["grafana.admin_password"] = []string{
		"Set admin_password for Grafana admin user",
		"Use SOPS to encrypt sensitive credentials",
		"Generate a strong password with sufficient entropy",
	}

	se.fieldSuggestions["weave_gitops.password_hash"] = []string{
		"Set password_hash (bcrypt hash)",
		"Use 'htpasswd -nbBC 10 admin <password>' to generate hash",
		"Use SOPS to encrypt sensitive credentials",
	}

	// VRRP suggestions
	se.fieldSuggestions["vrrp_ip"] = []string{
		"Set vrrp_ip to a valid IP address",
		"Example: '10.0.4.10'",
		"Or enable Octavia by setting use_octavia: true",
		"VRRP IP must be in the same subnet as cluster nodes",
	}
}

// initializeTypeSuggestions sets up error type-specific suggestions.
func (se *SuggestionEngine) initializeTypeSuggestions() {
	se.typeSuggestions["validation"] = []string{
		"Review the configuration file for syntax errors",
		"Check the JSON schema for valid field names and types",
		"Use 'opencenter cluster validate' to check configuration",
	}

	se.typeSuggestions["provider"] = []string{
		"Verify provider-specific configuration is complete",
		"Check provider credentials are correctly set",
		"Ensure provider resources are accessible",
	}

	se.typeSuggestions["network"] = []string{
		"Verify network configuration doesn't have conflicts",
		"Ensure CIDR ranges don't overlap",
		"Check that network plugin is properly configured",
	}

	se.typeSuggestions["service"] = []string{
		"Verify service dependencies are enabled",
		"Check service-specific configuration is complete",
		"Ensure required secrets are provided",
	}

	se.typeSuggestions["secret"] = []string{
		"Use SOPS to encrypt sensitive credentials",
		"Ensure secrets are in the correct section",
		"Verify secret format matches requirements",
	}
}

// generateContextAwareSuggestions generates suggestions based on field name patterns.
func (se *SuggestionEngine) generateContextAwareSuggestions(field string, value interface{}) []string {
	fieldLower := strings.ToLower(field)

	// Password/secret fields
	if strings.Contains(fieldLower, "password") || strings.Contains(fieldLower, "secret") || strings.Contains(fieldLower, "token") {
		return []string{
			"Use SOPS to encrypt sensitive credentials",
			"Never commit plaintext secrets to version control",
			"Generate strong secrets with sufficient entropy",
		}
	}

	// URL fields
	if strings.Contains(fieldLower, "url") || strings.Contains(fieldLower, "endpoint") {
		return []string{
			"Ensure URL is properly formatted (e.g., https://example.com)",
			"Include protocol (http:// or https://)",
			"Verify the endpoint is accessible",
		}
	}

	// Email fields
	if strings.Contains(fieldLower, "email") {
		return []string{
			"Use valid email format (e.g., user@example.com)",
			"Ensure email contains @ symbol and valid domain",
		}
	}

	// Count/number fields
	if strings.Contains(fieldLower, "count") || strings.Contains(fieldLower, "size") {
		return []string{
			"Use positive integers for count fields",
			"Consider resource requirements when setting counts",
			"Use odd numbers for quorum-based systems (e.g., etcd)",
		}
	}

	// Path fields
	if strings.Contains(fieldLower, "path") || strings.Contains(fieldLower, "dir") || strings.Contains(fieldLower, "directory") {
		return []string{
			"Use absolute or relative paths",
			"Ensure the path is accessible",
			"Check directory permissions",
		}
	}

	// Region fields
	if strings.Contains(fieldLower, "region") {
		return []string{
			"Set region to a valid cloud provider region",
			"Check provider documentation for available regions",
			"Use the region closest to your infrastructure",
		}
	}

	// Enabled/boolean fields
	if strings.Contains(fieldLower, "enabled") || strings.Contains(fieldLower, "enable") {
		return []string{
			"Set to true to enable or false to disable",
			"Check dependencies when enabling features",
		}
	}

	// Default generic suggestions
	return []string{
		fmt.Sprintf("Check the documentation for field '%s'", field),
		"Verify the value matches the expected format",
		"Use 'opencenter config ide --schema-only' to generate the schema for valid fields",
	}
}

// FormatSuggestions formats a list of suggestions for display.
func (se *SuggestionEngine) FormatSuggestions(suggestions []string) string {
	if len(suggestions) == 0 {
		return ""
	}

	var formatted strings.Builder
	formatted.WriteString("\nSuggestions:\n")
	for i, suggestion := range suggestions {
		formatted.WriteString(fmt.Sprintf("  %d. %s\n", i+1, suggestion))
	}
	return formatted.String()
}

// GetRelatedFields returns fields that are commonly configured together.
func (se *SuggestionEngine) GetRelatedFields(field string) []string {
	relatedFields := map[string][]string{
		"opencenter.infrastructure.provider": {
			"opencenter.infrastructure.cloud.openstack",
			"opencenter.infrastructure.cloud.aws",
		},
		"opencenter.cluster.kubernetes.network_plugin.calico.enabled": {
			"opencenter.cluster.kubernetes.network_plugin.cilium.enabled",
			"opencenter.cluster.kubernetes.network_plugin.kube-ovn.enabled",
		},
		"opentofu.backend.type": {
			"opentofu.backend.local",
			"opentofu.backend.s3",
		},
		"opencenter.services.loki.loki_storage_type": {
			"opencenter.services.loki.swift_auth_url",
			"opencenter.services.loki.loki_s3_region",
		},
	}

	if related, exists := relatedFields[field]; exists {
		return related
	}
	return []string{}
}
