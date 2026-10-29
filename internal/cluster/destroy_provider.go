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
	// Fatal marks a step whose failure must abort the destroy pipeline.
	// When false, the failure is recorded as a cleanup warning and the
	// remaining steps still run. Severity is explicit: the runner must
	// never infer it from the step ID.
	Fatal bool
	Run   func(ctx context.Context) error
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
	// RequireClusterAccess turns a CSI volume-capture failure (missing or
	// unreachable cluster kubeconfig) into a fatal error instead of a
	// warning, so destroy aborts before any infrastructure is torn down.
	RequireClusterAccess bool
	// AllowCleanupFailures downgrades pre-destroy cleanup steps (e.g. the
	// Octavia load-balancer sweep) from fatal to warning, preserving the
	// historical lenient behavior when the operator explicitly accepts the
	// risk of leaked resources.
	AllowCleanupFailures bool
	// DestroyRetries is the number of additional attempts for the
	// opentofu-destroy step after a transient failure (default 1).
	DestroyRetries int
	// DestroyParallelism sets the tofu -parallelism flag when > 0.
	DestroyParallelism int
}
