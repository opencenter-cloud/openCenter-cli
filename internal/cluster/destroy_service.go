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
	"errors"
	"fmt"
	"io"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// DestroyService orchestrates infrastructure destruction for clusters.
type DestroyService struct {
	runner lifecycleCommandRunner
	output io.Writer
	// providerOverride injects a provider (tests only); nil selects by cfg.
	providerOverride lifecycleDestroyProvider
}

// DestroyOptions contains options for the destroy operation.
type DestroyOptions struct {
	// AutoApprove skips the tofu destroy confirmation prompt.
	AutoApprove bool
	// DeleteVolumes deletes CSI-provisioned Cinder volumes after infrastructure
	// destruction. When false, orphaned volumes are only reported.
	DeleteVolumes bool
	// RequireClusterAccess aborts destroy when the cluster kubeconfig is
	// missing or unreachable instead of skipping CSI volume capture.
	RequireClusterAccess bool
	// SkipClusterAccess omits every destroy step that requires access to the
	// live cluster API (the CSI volume-capture kubectl step), so destroy
	// proceeds against an unreachable endpoint without a capture warning.
	SkipClusterAccess bool
	// AllowCleanupFailures downgrades pre-destroy cleanup steps from fatal
	// to warning (historical lenient behavior, explicit opt-in).
	AllowCleanupFailures bool
	// DestroyRetries is the number of additional attempts for the
	// opentofu-destroy step after a transient failure (default 1).
	DestroyRetries int
	// DestroyParallelism sets the tofu -parallelism flag when > 0.
	DestroyParallelism int
}

// DestroyResult contains the result of a destroy operation.
type DestroyResult struct {
	// InfraDestroyed indicates whether infrastructure was successfully destroyed.
	InfraDestroyed bool
	// StepsCompleted lists the IDs of completed destroy steps.
	StepsCompleted []string
	// Error contains any error that occurred during destruction.
	Error error
	// CleanupWarnings collects non-fatal step failures (e.g. CSI volume
	// deletion errors, downgraded cleanup steps). The destroy still
	// succeeds, but callers must surface these so leaked resources are
	// never silently swallowed.
	CleanupWarnings []error
}

// AddCleanupWarning records a non-fatal warning for the caller. Providers
// use this when a step degrades instead of failing (e.g. an unreachable
// Cinder API after the infrastructure is already gone).
func (r *DestroyResult) AddCleanupWarning(err error) {
	if err == nil {
		return
	}
	r.CleanupWarnings = append(r.CleanupWarnings, err)
}

// CleanupWarning returns the combined cleanup warnings as a single error,
// or nil when there are none.
func (r *DestroyResult) CleanupWarning() error {
	return errors.Join(r.CleanupWarnings...)
}

// NewDestroyService creates a new DestroyService with the default command runner.
func NewDestroyService(output io.Writer) *DestroyService {
	return &DestroyService{
		runner: newExecLifecycleCommandRunner(),
		output: output,
	}
}

// NewDestroyServiceWithRunner creates a new DestroyService with a custom command runner.
// This is primarily used for testing.
func NewDestroyServiceWithRunner(runner lifecycleCommandRunner, output io.Writer) *DestroyService {
	return &DestroyService{
		runner: runner,
		output: output,
	}
}

// DestroyInfrastructure destroys the infrastructure for a cluster.
// It returns an error if the provider doesn't support infrastructure destruction
// or if the destruction fails.
func (s *DestroyService) DestroyInfrastructure(ctx context.Context, cfg *v2.Config, opts *DestroyOptions) (*DestroyResult, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	result := &DestroyResult{}

	if opts == nil {
		opts = &DestroyOptions{}
	}
	if opts.SkipClusterAccess && opts.RequireClusterAccess {
		return nil, fmt.Errorf("SkipClusterAccess and RequireClusterAccess are mutually exclusive: " +
			"one demands live cluster access, the other forbids touching the cluster API")
	}

	if opts.DestroyRetries == 0 {
		opts.DestroyRetries = 1
	} else if opts.DestroyRetries < 0 {
		opts.DestroyRetries = 0
	}

	provider := s.providerOverride
	if provider == nil {
		var err error
		provider, err = s.getDestroyProvider(cfg)
		if err != nil {
			return nil, err
		}
	}
	// Give the provider a sink for non-fatal degradations its steps swallow
	// (unreachable Cinder, skipped CSI capture) so they reach the caller.
	if osp, ok := provider.(*openstackDestroyProvider); ok {
		osp.result = result
	}

	infraOpts := &DestroyInfraOptions{
		AutoApprove:          opts.AutoApprove,
		DeleteVolumes:        opts.DeleteVolumes,
		RequireClusterAccess: opts.RequireClusterAccess,
		SkipClusterAccess:    opts.SkipClusterAccess,
		AllowCleanupFailures: opts.AllowCleanupFailures,
		DestroyRetries:       opts.DestroyRetries,
		DestroyParallelism:   destroyParallelism(opts),
	}

	steps, err := provider.BuildSteps(cfg, infraOpts)
	if err != nil {
		return nil, fmt.Errorf("build destroy steps: %w", err)
	}

	for _, step := range steps {
		s.logf("Running: %s\n", step.Description)
		logBootstrapMessage(ctx, "destroy: running step %s (%s)", step.ID, step.Description)

		if err := step.Run(ctx); err != nil {
			if step.Fatal {
				result.Error = fmt.Errorf("step %q failed: %w", step.ID, err)
				logBootstrapMessage(ctx, "destroy: step %s failed: %v", step.ID, err)
				return result, result.Error
			}
			// Non-fatal step: record the warning so the CLI surfaces it,
			// then continue with the remaining steps.
			result.AddCleanupWarning(fmt.Errorf("step %s: %w", step.ID, err))
			s.logf("Warning: %s encountered issues: %v\n", step.Description, err)
			logBootstrapMessage(ctx, "destroy: warning in step %s: %v", step.ID, err)
		}

		result.StepsCompleted = append(result.StepsCompleted, step.ID)
		s.logf("Completed: %s\n", step.Description)
		logBootstrapMessage(ctx, "destroy: completed step %s (%s)", step.ID, step.Description)
	}

	result.InfraDestroyed = true
	return result, nil
}

func destroyParallelism(opts *DestroyOptions) int {
	if opts == nil || opts.DestroyParallelism < 0 {
		return 0
	}
	return opts.DestroyParallelism
}

// SupportsInfraDestroy returns true if the provider supports infrastructure destruction.
func (s *DestroyService) SupportsInfraDestroy(cfg *v2.Config) bool {
	if cfg == nil {
		return false
	}

	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))
	if provider == "magnum" {
		// Provider selection is sufficient here. Configuration errors must be
		// surfaced by DestroyInfrastructure rather than disabling removal.
		return true
	}

	// Only providers with OpenTofu enabled support infrastructure destruction
	if !cfg.OpenTofu.Enabled {
		return false
	}

	switch provider {
	case "openstack", "vmware":
		return true
	default:
		return false
	}
}

// getDestroyProvider returns the appropriate destroy provider for the configuration.
func (s *DestroyService) getDestroyProvider(cfg *v2.Config) (lifecycleDestroyProvider, error) {
	provider := strings.ToLower(strings.TrimSpace(cfg.Provider()))

	switch provider {
	case "openstack":
		return newOpenStackDestroyProvider(s.runner, s.output), nil
	case "vmware":
		// VMware uses the same OpenTofu-based destroy pattern
		return newOpenStackDestroyProvider(s.runner, s.output), nil
	case "magnum":
		return newMagnumDestroyProvider(cfg)
	default:
		return nil, fmt.Errorf("provider %q does not support infrastructure destruction via this service", provider)
	}
}

func (s *DestroyService) logf(format string, args ...any) {
	if s.output != nil {
		fmt.Fprintf(s.output, format, args...)
	}
}
