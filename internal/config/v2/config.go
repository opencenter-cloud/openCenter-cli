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

package v2

// Config represents the root v2 configuration structure.
// Requirements: 1.1, 1.3, 1.4, 1.5, 1.6, 1.7
type Config struct {
	SchemaVersion string           `yaml:"schema_version" json:"schema_version" validate:"required,eq=2.0"`
	Metadata      ConfigMetadata   `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	OpenCenter    OpenCenterConfig `yaml:"opencenter" json:"opencenter" validate:"required"`
	Deployment    DeploymentConfig `yaml:"deployment,omitempty" json:"deployment,omitempty"`
	OpenTofu      OpenTofuConfig   `yaml:"opentofu,omitempty" json:"opentofu,omitempty"`
	Secrets       SecretsConfig    `yaml:"secrets" json:"secrets" validate:"required"`
}

// ConfigMetadata holds system-managed metadata about the configuration.
type ConfigMetadata struct {
	CreatedAt   string            `yaml:"created_at,omitempty" json:"created_at,omitempty"`
	UpdatedAt   string            `yaml:"updated_at,omitempty" json:"updated_at,omitempty"`
	Version     string            `yaml:"version,omitempty" json:"version,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"`
}

// OpenCenterConfig represents the main opencenter configuration with five domains.
// Requirements: 1.1, 1.3, 1.4, 1.5, 1.6, 1.7
type OpenCenterConfig struct {
	Meta            MetaConfig           `yaml:"meta" json:"meta" validate:"required"`
	Cluster         ClusterConfig        `yaml:"cluster" json:"cluster" validate:"required"`
	Infrastructure  InfrastructureConfig `yaml:"infrastructure" json:"infrastructure" validate:"required"`
	Services        ServiceMap           `yaml:"services,omitempty" json:"services,omitempty"`
	ManagedServices ServiceMap           `yaml:"managed_services,omitempty" json:"managed_services,omitempty"`
	GitOps          GitOpsConfig         `yaml:"gitops,omitempty" json:"gitops,omitempty"`
}

// MetaConfig contains cluster identity and organizational context.
// Requirements: 1.3
type MetaConfig struct {
	Name         string `yaml:"name" json:"name" validate:"required,dns1123"`
	Organization string `yaml:"organization" json:"organization" validate:"required"`
	Env          string `yaml:"env" json:"env" validate:"required,oneof=dev staging production"`
	Region       string `yaml:"region" json:"region" validate:"required"`
	Status       string `yaml:"status,omitempty" json:"status,omitempty"`
}

// OpenTofuConfig represents OpenTofu/Terraform backend configuration.
// Requirements: 20.1
type OpenTofuConfig struct {
	Backend BackendConfig `yaml:"backend,omitempty" json:"backend,omitempty"`
}

// BackendConfig represents OpenTofu backend configuration.
// Requirements: 20.1, 20.2, 20.3, 20.4
type BackendConfig struct {
	Type   string             `yaml:"type" json:"type" validate:"required,oneof=s3 local remote"`
	Local  *LocalBackendConfig `yaml:"local,omitempty" json:"local,omitempty"`
	S3     *S3BackendConfig    `yaml:"s3,omitempty" json:"s3,omitempty"`
	Config map[string]any     `yaml:"config,omitempty" json:"config,omitempty"`
}

// LocalBackendConfig represents local backend configuration.
// Requirements: 20.2
type LocalBackendConfig struct {
	Path string `yaml:"path" json:"path" validate:"required"`
}

// S3BackendConfig represents S3 backend configuration.
// Requirements: 20.3
type S3BackendConfig struct {
	Bucket string `yaml:"bucket" json:"bucket" validate:"required"`
	Key    string `yaml:"key" json:"key" validate:"required"`
	Region string `yaml:"region" json:"region" validate:"required"`
}

// SecretsConfig represents secrets configuration.
// Requirements: 18.1, 18.2, 18.3
type SecretsConfig struct {
	Global         GlobalSecrets  `yaml:"global,omitempty" json:"global,omitempty"`
	ServiceSecrets map[string]any `yaml:"service_secrets,omitempty" json:"service_secrets,omitempty"`
	SOPSConfig     SOPSConfig     `yaml:"sops,omitempty" json:"sops,omitempty"`
}

// GlobalSecrets holds infrastructure-wide credentials.
// Requirements: 18.2
type GlobalSecrets struct {
	AWSAccessKey       string `yaml:"aws_access_key,omitempty" json:"aws_access_key,omitempty"`
	AWSSecretKey       string `yaml:"aws_secret_key,omitempty" json:"aws_secret_key,omitempty"`
	OpenStackAuthURL   string `yaml:"openstack_auth_url,omitempty" json:"openstack_auth_url,omitempty"`
	OpenStackUsername  string `yaml:"openstack_username,omitempty" json:"openstack_username,omitempty"`
	OpenStackPassword  string `yaml:"openstack_password,omitempty" json:"openstack_password,omitempty"`
	OpenStackProjectID string `yaml:"openstack_project_id,omitempty" json:"openstack_project_id,omitempty"`
}

// SOPSConfig represents SOPS encryption configuration.
// Requirements: 18.5, 18.6
type SOPSConfig struct {
	Enabled        bool   `yaml:"enabled" json:"enabled"`
	AgeKeyFile     string `yaml:"age_key_file,omitempty" json:"age_key_file,omitempty" validate:"required_if=Enabled true"`
	EncryptedRegex string `yaml:"encrypted_regex,omitempty" json:"encrypted_regex,omitempty"`
}

// GitOpsConfig represents GitOps repository configuration.
// Requirements: 19.1
type GitOpsConfig struct {
	GitURL          string `yaml:"git_url" json:"git_url" validate:"required"`
	GitBranch       string `yaml:"git_branch,omitempty" json:"git_branch,omitempty"`
	GitPath         string `yaml:"git_path,omitempty" json:"git_path,omitempty"`
	BaseRepoURL     string `yaml:"base_repo_url,omitempty" json:"base_repo_url,omitempty"`
	BaseRepoRelease string `yaml:"base_repo_release,omitempty" json:"base_repo_release,omitempty"`
	FluxInterval    string `yaml:"flux_interval,omitempty" json:"flux_interval,omitempty"`
	FluxPrune       bool   `yaml:"flux_prune" json:"flux_prune"`
}

// ServiceMap is a polymorphic map of service configurations.
// Requirements: 17.1, 17.2, 17.7
type ServiceMap map[string]any
