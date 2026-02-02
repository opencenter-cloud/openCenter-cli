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
	internalconfig "github.com/rackerlabs/opencenter-cli/internal/config"
)

// Re-export types from internal/config for use in internal/core/config.
// This allows internal/core/config to work with the same types without circular dependencies.
// The actual type definitions remain in internal/config to maintain compatibility
// with existing code while the architectural refactoring is completed.

type (
	// Config represents the simplified root configuration for a cluster.
	Config = internalconfig.Config

	// ConfigMetadata contains metadata about the configuration file.
	ConfigMetadata = internalconfig.ConfigMetadata

	// SimplifiedOpenCenter represents the opencenter section of the configuration.
	SimplifiedOpenCenter = internalconfig.SimplifiedOpenCenter

	// SimplifiedOpenTofu represents the opentofu section of the configuration.
	SimplifiedOpenTofu = internalconfig.SimplifiedOpenTofu

	// Secrets holds all sensitive configuration data.
	Secrets = internalconfig.Secrets

	// Deployment holds deployment-related configuration.
	Deployment = internalconfig.Deployment

	// Infrastructure represents the infrastructure configuration block.
	Infrastructure = internalconfig.Infrastructure

	// ClusterConfig represents the cluster configuration section.
	ClusterConfig = internalconfig.ClusterConfig

	// GitOpsConfig holds configuration related to GitOps scaffolding.
	GitOpsConfig = internalconfig.GitOpsConfig

	// StorageConfig represents the storage configuration for the cluster.
	StorageConfig = internalconfig.StorageConfig

	// ServiceMap handles polymorphic unmarshalling of service configurations.
	ServiceMap = internalconfig.ServiceMap
)

// Re-export constants
const (
	StageInit      = internalconfig.StageInit
	StagePreflight = internalconfig.StagePreflight
	StageSetup     = internalconfig.StageSetup
	StageBootstrap = internalconfig.StageBootstrap
	StageValidate  = internalconfig.StageValidate
	StageDestroy   = internalconfig.StageDestroy
	StageRender    = internalconfig.StageRender
	StagePlan      = internalconfig.StagePlan
	StageApply     = internalconfig.StageApply

	StatusPending = internalconfig.StatusPending
	StatusRunning = internalconfig.StatusRunning
	StatusSuccess = internalconfig.StatusSuccess
	StatusFailed  = internalconfig.StatusFailed
)

// Re-export helper functions
var NewConfigMetadata = internalconfig.NewConfigMetadata
