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
	"os"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type openstackDestroyProvider struct {
	runner lifecycleCommandRunner
}

func newOpenStackDestroyProvider(runner lifecycleCommandRunner) lifecycleDestroyProvider {
	return &openstackDestroyProvider{runner: runner}
}

func (p *openstackDestroyProvider) BuildSteps(cfg *v2.Config, opts *DestroyInfraOptions) ([]destroyStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}

	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for openstack destroy")
	}

	// Check if infrastructure directory exists - if not, nothing to destroy
	if _, err := os.Stat(clusterDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("cluster infrastructure directory not found: %s (infrastructure may already be destroyed)", clusterDir)
	}

	extractor := credentials.NewExtractor(*cfg)
	creds, err := extractor.ExtractOpenStack()
	if err != nil {
		return nil, fmt.Errorf("extract openstack credentials: %w", err)
	}

	env := buildDestroyEnvironment()
	mergeBootstrapEnvironment(env, creds.ToEnvMap())

	openTofuPath, err := resolveTofuBinary(cfg.OpenTofu.Path)
	if err != nil {
		return nil, err
	}

	destroyArgs := []string{"destroy"}
	if opts != nil && opts.AutoApprove {
		destroyArgs = append(destroyArgs, "-auto-approve")
	}

	return []destroyStep{
		{
			ID:          "opentofu-init",
			Description: "Initialize OpenTofu",
			Run: func(ctx context.Context) error {
				_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
				return err
			},
		},
		{
			ID:          "opentofu-destroy",
			Description: "Destroy OpenTofu infrastructure",
			Run: func(ctx context.Context) error {
				_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, destroyArgs...)
				return err
			},
		},
	}, nil
}

// buildDestroyEnvironment creates the environment variables for destroy operations.
func buildDestroyEnvironment() map[string]string {
	env := make(map[string]string)

	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}
