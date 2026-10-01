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

package cluster

import (
	"context"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// destroyStep represents a single step in the infrastructure destruction process.
type destroyStep struct {
	ID          string
	Description string
	Run         func(ctx context.Context) error
}

// lifecycleDestroyProvider defines the interface for provider-specific destroy logic.
type lifecycleDestroyProvider interface {
	BuildSteps(cfg *v2.Config, opts *DestroyInfraOptions) ([]destroyStep, error)
}

// DestroyInfraOptions contains options for infrastructure destruction.
type DestroyInfraOptions struct {
	// AutoApprove skips the tofu destroy confirmation prompt.
	AutoApprove bool
	// DeleteVolumes deletes CSI-provisioned Cinder volumes after infrastructure
	// destruction. When false, orphaned volumes are only reported.
	DeleteVolumes bool
}
