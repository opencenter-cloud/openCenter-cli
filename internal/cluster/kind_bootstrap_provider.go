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
	"path/filepath"
	"strings"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/flux"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitops"
)

type kindBootstrapProvider struct {
	runner lifecycleCommandRunner
}

func newKindBootstrapProvider(runner lifecycleCommandRunner) lifecycleBootstrapProvider {
	return &kindBootstrapProvider{runner: runner}
}

// BuildSteps returns the ordered bootstrap steps for a Kind cluster.
//
// The sequence mirrors the production OpenStack workflow: provision
// infrastructure, attach the Git remote, push the GitOps tree, and
// bootstrap FluxCD — all within a single `opencenter cluster bootstrap`
// invocation.
func (p *kindBootstrapProvider) BuildSteps(cfg *config.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	kindProvider := kindprovider.NewProvider()
	runtime := kindprovider.ResolveRuntime(opts.ContainerRuntime)
	env := kindprovider.BuildEnvironment(runtime)
	kindConfigPath := filepath.Join(clusterPaths.ClusterDir, "kind-config.yaml")
	kindClusterName := resolveKindClusterName(cfg)

	executor := localdev.NewExecutor()
	// stateDir is empty so the localdev layout resolves to <OPENCENTER_CONFIG_DIR>/local.
	stateDir := ""

	clusterIdentifier := cfg.ClusterName()
	if org := strings.TrimSpace(cfg.OpenCenter.Meta.Organization); org != "" {
		clusterIdentifier = org + "/" + clusterIdentifier
	}

	return []bootstrapStep{
		{
			ID:          "kind-create",
			Description: "Create Kind cluster",
			Run: func(ctx context.Context) error {
				if _, err := os.Stat(kindConfigPath); err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("kind config not found at %s; run 'opencenter cluster setup %s' first", kindConfigPath, cfg.ClusterName())
					}
					return fmt.Errorf("stat kind config: %w", err)
				}
				return kindProvider.CreateCluster(ctx, kindClusterName, kindConfigPath, env)
			},
		},
		{
			ID:          "kind-export-kubeconfig",
			Description: "Export Kind kubeconfig",
			Run: func(ctx context.Context) error {
				return kindProvider.ExportKubeconfig(ctx, kindClusterName, opts.KubeconfigPath, env)
			},
		},
		{
			ID:          "gitea-attach-kind",
			Description: "Attach local Gitea to the Kind network",
			Run: func(ctx context.Context) error {
				giteaService, err := gitea.NewService(executor, stateDir, gitea.DefaultSettings(runtime))
				if err != nil {
					return fmt.Errorf("create gitea service: %w", err)
				}
				status, err := giteaService.Status(ctx)
				if err != nil {
					return fmt.Errorf("check gitea status: %w", err)
				}
				if !status.Running {
					return fmt.Errorf("local gitea is not running; run 'opencenter local gitea up' first")
				}
				if _, err := giteaService.AttachKind(ctx); err != nil {
					return fmt.Errorf("attach gitea to kind network: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "gitops-push",
			Description: "Push GitOps repository to local Gitea",
			Run: func(ctx context.Context) error {
				gitopsService, err := gitops.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create gitops service: %w", err)
				}
				if _, err := gitopsService.Push(ctx, clusterIdentifier); err != nil {
					return fmt.Errorf("push gitops repo: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "flux-bootstrap",
			Description: "Bootstrap FluxCD from local Gitea",
			Run: func(ctx context.Context) error {
				fluxService, err := flux.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create flux service: %w", err)
				}
				if _, err := fluxService.Bootstrap(ctx, clusterIdentifier); err != nil {
					return fmt.Errorf("flux bootstrap: %w", err)
				}
				return nil
			},
		},
	}, nil
}

// resolveKindClusterName returns the Kind cluster name, preferring an explicit
// override from the config when set.
func resolveKindClusterName(cfg *config.Config) string {
	if cfg != nil && cfg.OpenCenter.Infrastructure.Kind != nil {
		if name := strings.TrimSpace(cfg.OpenCenter.Infrastructure.Kind.ClusterNameOverride); name != "" {
			return name
		}
	}
	return cfg.ClusterName()
}
