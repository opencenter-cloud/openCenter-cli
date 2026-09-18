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
	"fmt"
	"io"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// DestroyService orchestrates infrastructure destruction for clusters.
type DestroyService struct {
	runner lifecycleCommandRunner
	output io.Writer
}

// DestroyOptions contains options for the destroy operation.
type DestroyOptions struct {
	// AutoApprove skips the tofu destroy confirmation prompt.
	AutoApprove bool
}

// DestroyResult contains the result of a destroy operation.
type DestroyResult struct {
	// InfraDestroyed indicates whether infrastructure was successfully destroyed.
	InfraDestroyed bool
	// StepsCompleted lists the IDs of completed destroy steps.
	StepsCompleted []string
	// Error contains any error that occurred during destruction.
	Error error
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

	provider, err := s.getDestroyProvider(cfg)
	if err != nil {
		return nil, err
	}

	infraOpts := &DestroyInfraOptions{
		AutoApprove: opts != nil && opts.AutoApprove,
	}

	steps, err := provider.BuildSteps(cfg, infraOpts)
	if err != nil {
		return nil, fmt.Errorf("build destroy steps: %w", err)
	}

	for _, step := range steps {
		s.logf("Running: %s\n", step.Description)

		if err := step.Run(ctx); err != nil {
			result.Error = fmt.Errorf("step %q failed: %w", step.ID, err)
			return result, result.Error
		}

		result.StepsCompleted = append(result.StepsCompleted, step.ID)
		s.logf("Completed: %s\n", step.Description)
	}

	result.InfraDestroyed = true
	return result, nil
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
		return newOpenStackDestroyProvider(s.runner), nil
	case "vmware":
		// VMware uses the same OpenTofu-based destroy pattern
		return newOpenStackDestroyProvider(s.runner), nil
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
