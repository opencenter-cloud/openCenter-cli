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
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

// errKubeconfigNotFound distinguishes "the cluster kubeconfig was never
// written here" from "the cluster is unreachable": the former is an expected
// skip during destroy (the cluster may already be gone), the latter is a
// real failure that --require-cluster-access turns fatal.
var errKubeconfigNotFound = errors.New("kubeconfig not found")

// resolveClusterKubeconfigPath returns the cluster-owned kubeconfig location
// for destroy-time cluster access.
//
// The canonical location is the secrets zone
// (<secrets>/<org>/<cluster>/kubernetes/kubeconfig.yaml), which is where
// bootstrap writes the kubeconfig (see paths.ClusterPaths.KubeconfigPath).
// The legacy infrastructure directory path (<gitops>/infrastructure/clusters/
// <name>/kubeconfig.yaml) is kept as a fallback for clusters generated
// before the secure-zones migration.
//
// Resolution is a pure path build (no filesystem existence gate): during
// destroy the blueprint directories may already be removed by a previous
// --remove-files run, and a resolver that requires them would fail exactly
// when the fallback is needed.
func resolveClusterKubeconfigPath(cfg *v2.Config, clusterDir string) string {
	if cfg != nil {
		secretsRoot := config.GetSecretsDir()
		if secretsRoot != "" {
			strategy := corePaths.NewOrgBasedStrategyWithRoots(corePaths.PathRoots{
				SecretsDir: secretsRoot,
			})
			if clusterPaths, err := strategy.Resolve(context.Background(), cfg.ClusterName(), cfg.Organization()); err == nil && clusterPaths != nil {
				if path := strings.TrimSpace(clusterPaths.KubeconfigPath); path != "" {
					if _, err := os.Stat(path); err == nil {
						return path
					}
				}
			}
		}
	}

	// Legacy fallback: kubeconfig written next to the OpenTofu state.
	return filepath.Join(clusterDir, "kubeconfig.yaml")
}

// kubeconfigForDestroy returns the first existing candidate kubeconfig,
// preferring the secrets zone. The returned error wraps errKubeconfigNotFound
// when no candidate exists on disk; any other error is a real stat failure.
func kubeconfigForDestroy(cfg *v2.Config, clusterDir string) (string, error) {
	candidates := []string{resolveClusterKubeconfigPath(cfg, clusterDir)}
	if legacy := filepath.Join(clusterDir, "kubeconfig.yaml"); legacy != candidates[0] {
		candidates = append(candidates, legacy)
	}

	var statErr error
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		} else if !os.IsNotExist(err) {
			statErr = fmt.Errorf("stat kubeconfig %s: %w", candidate, err)
		}
	}
	if statErr != nil {
		return "", statErr
	}
	return "", fmt.Errorf("%w at %s", errKubeconfigNotFound, strings.Join(candidates, ", "))
}
