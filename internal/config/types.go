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

// Config represents the simplified root configuration for a cluster based on the new schema.
// The structure matches the testdata/schema.yaml format with opencenter, opentofu, cloud, and secrets sections.
// In v2.0.0, only schema version 2.0 is supported.
type Config struct {
	SchemaVersion string               `yaml:"schema_version,omitempty" json:"schema_version,omitempty" validate:"omitempty,oneof=2.0"`
	Metadata      ConfigMetadata       `yaml:"metadata,omitempty" json:"metadata,omitempty"`
	OpenCenter    SimplifiedOpenCenter `yaml:"opencenter" json:"opencenter" validate:"required"`
	OpenTofu      SimplifiedOpenTofu   `yaml:"opentofu" json:"opentofu" validate:"required"`
	Secrets       Secrets              `yaml:"secrets" json:"secrets" validate:"required"`
	Deployment    Deployment           `yaml:"deployment,omitempty" json:"deployment,omitempty"`
	Overrides     map[string]any       `yaml:"overrides,omitempty" json:"overrides,omitempty"`
}

// Cluster Stages
const (
	StageInit      = "init"
	StagePreflight = "preflight"
	StageSetup     = "setup"
	StageBootstrap = "bootstrap"
	StageValidate  = "validate"
	StageDestroy   = "destroy"
	StageRender    = "render"
	StagePlan      = "plan"
	StageApply     = "apply"
)

// Cluster Statuses
const (
	StatusPending = "pending"
	StatusRunning = "running"
	StatusSuccess = "success"
	StatusFailed  = "failed"
)

// Helper methods for backward compatibility

// ClusterName returns the cluster name from the simplified structure
func (c Config) ClusterName() string {
	return c.OpenCenter.Cluster.ClusterName
}

// GitOps returns the GitOps configuration from the simplified structure
func (c Config) GitOps() GitOpsConfig {
	return c.OpenCenter.GitOps
}
