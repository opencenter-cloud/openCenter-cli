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
	"bytes"
	"context"
	"fmt"
	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/flux"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
// bootstrap FluxCD — all within a single `opencenter cluster deploy`
// invocation.
func (p *kindBootstrapProvider) BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error) {
	kindProvider := kindprovider.NewProvider()
	runtime := kindprovider.ResolveRuntime(opts.ContainerRuntime)
	env := kindprovider.BuildEnvironment(runtime)
	kindConfigPath := filepath.Join(clusterPaths.ClusterDir, "kind-config.yaml")
	kindClusterName := resolveKindClusterName(cfg)

	executor := localdev.NewExecutor()
	// stateDir is empty so the localdev layout resolves to <OPENCENTER_CONFIG_DIR>/local.
	stateDir := ""

	clusterIdentifier := cfg.ClusterName()
	if org := strings.TrimSpace(cfg.Organization()); org != "" {
		clusterIdentifier = org + "/" + clusterIdentifier
	}
	gitDir, gitDirErr := resolveGitDir(cfg, clusterPaths)

	// The GitOps provider determines which bootstrap steps apply. The local
	// Gitea provider requires attaching Gitea to the Kind network and rebasing/
	// pushing the local checkout against it. External providers (e.g. github)
	// bootstrap Flux directly against the remote, so those Gitea-only steps are
	// skipped: `flux bootstrap github` commits straight to the remote, leaving
	// no local Gitea to attach or push to.
	gitProvider := resolveKindGitProvider(cfg)
	useGitea := gitProvider == "gitea"

	steps := []bootstrapStep{
		{
			ID:          "kind-create",
			Description: "Create Kind cluster",
			Plan: BootstrapPlanStep{
				ID:         "kind-create",
				Action:     "Create Kind cluster",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("kind", "get", "clusters"),
					commandPlan("kind", "create", "cluster", "--name", kindClusterName, "--config", kindConfigPath),
				},
				Reads:       []string{kindConfigPath},
				Writes:      []string{fmt.Sprintf("local Kind cluster %q", kindClusterName)},
				Environment: kindPlanEnv(env),
				Notes:       []string{"Plan only; kind availability and config file existence were not checked."},
			},
			Run: func(ctx context.Context) error {
				if _, err := os.Stat(kindConfigPath); err != nil {
					if os.IsNotExist(err) {
						return fmt.Errorf("kind config not found at %s; run 'opencenter cluster generate %s' first", kindConfigPath, cfg.ClusterName())
					}
					return fmt.Errorf("stat kind config: %w", err)
				}
				return kindProvider.CreateCluster(ctx, kindClusterName, kindConfigPath, env)
			},
		},
		{
			ID:          "kind-export-kubeconfig",
			Description: "Export Kind kubeconfig",
			Plan: BootstrapPlanStep{
				ID:         "kind-export-kubeconfig",
				Action:     "Export Kind kubeconfig",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("kind", "export", "kubeconfig", "--name", kindClusterName, "--kubeconfig", opts.KubeconfigPath),
				},
				Writes:      []string{filepath.Dir(opts.KubeconfigPath), opts.KubeconfigPath},
				Environment: kindPlanEnv(env),
				Notes:       []string{"Plan only; kubeconfig directory creation and kind availability were not checked."},
			},
			Run: func(ctx context.Context) error {
				return kindProvider.ExportKubeconfig(ctx, kindClusterName, opts.KubeconfigPath, env)
			},
		},
		func() bootstrapStep {
			step := newKindWorkerLabelStep(opts.KubeconfigPath, p.runner)
			run := step.Run
			step.Run = func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				return run(ctx)
			}
			return step
		}(),
		{
			ID:          "kind-install-cni",
			Description: "Install Calico CNI (managed CNI mode)",
			Plan: BootstrapPlanStep{
				ID:         "kind-install-cni",
				Action:     "Install Calico CNI (managed CNI mode)",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("kubectl", "apply", "--server-side", "-f", "<calico-operator-crds>"),
					commandPlan("helm", "upgrade", "--install", "calico", calicoHelmChart),
				},
				Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: opts.KubeconfigPath}},
				Reads:       []string{opts.KubeconfigPath},
				Notes: []string{
					"Only runs when kind's default CNI is disabled and Calico is the enabled network plugin.",
					"Installs the CNI before flux-bootstrap so Flux controllers can schedule.",
				},
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				// Only install Calico in managed-CNI mode. In the default kind
				// mode kindnet provides the CNI and this step is a no-op.
				if !kindCalicoEnabled(cfg) {
					return nil
				}
				return p.installKindCalico(ctx, cfg, opts.KubeconfigPath)
			},
		},
		{
			ID:          "gitea-attach-kind",
			Description: "Attach local Gitea to the Kind network",
			NeverSkip:   true, // URL rewrite + origin align must run against the live config each deploy
			Plan: BootstrapPlanStep{
				ID:          "gitea-attach-kind",
				Action:      "Attach local Gitea to the Kind network",
				WorkingDir:  clusterPaths.ClusterDir,
				Environment: kindPlanEnv(env),
				Writes:      []string{"local Gitea container network attachments"},
				Notes:       []string{"Plan only; local Gitea status and container runtime availability were not checked."},
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
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
				var extraHosts []string
				if u, err := url.Parse(strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL)); err == nil && u.Hostname() != "" {
					extraHosts = append(extraHosts, u.Hostname())
				}
				if _, err := giteaService.AttachKind(ctx, extraHosts); err != nil {
					return fmt.Errorf("attach gitea to kind network: %w", err)
				}
				// The configured git_url may be unreachable from this process when it
				// uses a kind-network IP. Deploy re-homes it during its pre-check
				// phase, so no per-step URL rewrite is needed here.
				return nil
			},
		},
		{
			ID:          "flux-bootstrap",
			Description: fluxBootstrapDescription(gitProvider),
			Plan: BootstrapPlanStep{
				ID:         "flux-bootstrap",
				Action:     fluxBootstrapDescription(gitProvider),
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("flux", "bootstrap", "<provider-specific>", "--path=applications/overlays/"+cfg.ClusterName()),
				},
				Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: opts.KubeconfigPath}},
				Writes:      []string{"Flux bootstrap manifests and commits in the GitOps repository", "Flux resources in the Kind cluster"},
				Notes:       appendPlanNotes([]string{fluxBootstrapPlanNote(gitProvider)}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
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
		// No opencenter-base credential Secret step: the shared
		// openCenter-gitops-base repository is public, so its GitRepository
		// source is rendered anonymously (no secretRef).
		{
			ID:          sopsAgeSecretStepID,
			Description: "Reconcile SOPS Age decryption key for Flux",
			Plan:        newSopsAgeSecretStep(clusterPaths.SOPSKeyPath, opts.KubeconfigPath, p.runner).Plan,
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				return reconcileSopsAgeSecret(ctx, clusterPaths.SOPSKeyPath, opts.KubeconfigPath, p.runner)
			},
		},
		func() bootstrapStep {
			// kube-prometheus-stack's Grafana references an existingSecret named
			// grafana-admin-password in the observability namespace. Apply it
			// imperatively, mirroring the OpenStack and magnum providers; without
			// this the Grafana pod fails with CreateContainerConfigError on kind.
			step := newGrafanaAdminSecretStep(cfg, opts.KubeconfigPath, p.runner)
			run := step.Run
			step.Run = func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				return run(ctx)
			}
			return step
		}(),
		func() bootstrapStep {
			return bootstrapStep{
				ID:          "gitea-source-rehome",
				Description: "Re-home Flux GitRepository sources to an in-cluster Gitea URL",
				NeverSkip:   true,
				Plan: BootstrapPlanStep{
					ID:         "gitea-source-rehome",
					Action:     "Re-home Flux GitRepository sources to an in-cluster Gitea URL",
					WorkingDir: gitDir,
					Commands: []BootstrapPlanCommand{
						commandPlan("sed", "-i.rehomed", "s|<host>|gitea|", "<overlay>/services/sources/*.yaml"),
						commandPlan("rm", "-f", "<overlay>/services/sources/*.rehomed"),
					},
					Reads:  []string{gitDir + "/applications/overlays/" + cfg.ClusterName()},
					Writes: []string{"local GitOps checkout services/sources manifests"},
					Notes:  appendPlanNotes([]string{"No-op when the configured git_url already uses an in-cluster reachable host."}, gitDirErr),
				},
				Run: func(ctx context.Context) error {
					if os.Getenv("OPENCENTER_TEST_MODE") != "" || !useGitea {
						return nil
					}
					return p.rehomeInClusterSources(ctx, cfg, clusterPaths)
				},
			}
		}(),
		{
			ID:          "gitea-rebase",
			Description: "Rebase local checkout with Flux bootstrap commits from Gitea",
			Plan: BootstrapPlanStep{
				ID:         "gitea-rebase",
				Action:     "Rebase local checkout with Flux bootstrap commits from Gitea",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", "status", "--porcelain"),
					commandPlan("git", "add", "-A"),
					commandPlan("git", "commit", "-m", "stage local changes before rebase"),
					commandPlan("git", "pull", "--rebase", "origin", "<current-branch>"),
				},
				Reads:  []string{gitDir},
				Writes: []string{"local GitOps checkout history and working tree"},
				Notes:  appendPlanNotes([]string{"Plan only; Git remotes, token files, branch, and local working tree state were not checked."}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				gitopsService, err := gitops.NewService(executor, stateDir)
				if err != nil {
					return fmt.Errorf("create gitops service: %w", err)
				}
				gitDir, err := resolveGitDir(cfg, clusterPaths)
				if err != nil {
					return fmt.Errorf("resolve git dir: %w", err)
				}
				if _, err := gitopsService.PullRebase(ctx, clusterIdentifier, gitDir); err != nil {
					return fmt.Errorf("rebase from gitea: %w", err)
				}
				return nil
			},
		},
		{
			ID:          "gitops-push",
			Description: "Push GitOps repository to local Gitea",
			Plan: BootstrapPlanStep{
				ID:         "gitops-push",
				Action:     "Push GitOps repository to local Gitea",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", "push", "-u", "origin", "<current-branch>"),
				},
				Reads:  []string{gitDir},
				Writes: []string{"local Gitea Git repository"},
				Notes:  appendPlanNotes([]string{"Plan only; local Gitea status, token files, Git remote, and current branch were not checked."}, gitDirErr),
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
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
			ID:          "flux-verify",
			Description: "Verify Flux installation and source reconciliation",
			Plan: BootstrapPlanStep{
				ID:         "flux-verify",
				Action:     "Verify Flux installation and source reconciliation",
				WorkingDir: clusterPaths.ClusterDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("flux", "check"),
					commandPlan("flux", "get", "sources", "git", "-n", "flux-system"),
					commandPlan("flux", "get", "kustomizations", "-n", "flux-system"),
				},
				Environment: []BootstrapPlanEnv{{Name: "KUBECONFIG", Value: opts.KubeconfigPath}},
				Reads:       []string{opts.KubeconfigPath},
				Notes:       []string{"Plan only; Flux installation and source reconciliation were not checked."},
			},
			Run: func(ctx context.Context) error {
				if os.Getenv("OPENCENTER_TEST_MODE") != "" {
					return nil
				}
				kubeconfigEnv := map[string]string{"KUBECONFIG": opts.KubeconfigPath}

				// flux check — verifies Flux components are installed and healthy.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"check"},
				}); err != nil {
					return fmt.Errorf("flux check: %w", err)
				}

				// flux get sources git — verifies GitRepository sources are reconciled.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"get", "sources", "git", "-n", "flux-system"},
				}); err != nil {
					return fmt.Errorf("flux get sources git: %w", err)
				}

				// flux get kustomizations — verifies Kustomization objects are reconciled.
				if _, err := executor.Run(ctx, localdev.RunOptions{
					Name: "flux",
					Env:  kubeconfigEnv,
					Args: []string{"get", "kustomizations", "-n", "flux-system"},
				}); err != nil {
					return fmt.Errorf("flux get kustomizations: %w", err)
				}

				return nil
			},
		},
	}

	// For external Git providers, drop the Gitea-only steps. Flux bootstraps
	// directly against the remote (github/gitlab), so there is no local Gitea to
	// attach to the Kind network, and no local-checkout rebase/push cycle.
	if !useGitea {
		steps = filterOutSteps(steps, "gitea-attach-kind", "gitea-rebase", "gitops-push")
	}

	return steps, nil
}

// fluxBootstrapDescription returns a provider-appropriate description for the
// flux-bootstrap step.
func fluxBootstrapDescription(gitProvider string) string {
	if gitProvider == "gitea" || gitProvider == "" {
		return "Bootstrap FluxCD from local Gitea"
	}
	return fmt.Sprintf("Bootstrap FluxCD from %s", gitProvider)
}

// fluxBootstrapPlanNote returns a provider-appropriate dry-run note for the
// flux-bootstrap step. Only the local Gitea provider has a Gitea status to
// check; external providers rely on the configured repository and token file.
func fluxBootstrapPlanNote(gitProvider string) string {
	if gitProvider == "gitea" || gitProvider == "" {
		return "Plan only; local Gitea status, token files, current branch, and kubeconfig were not checked."
	}
	return "Plan only; token files, current branch, and kubeconfig were not checked."
}

// resolveKindGitProvider returns the configured GitOps token provider (lowercased)
// for a Kind cluster, defaulting to "gitea" when unset. This mirrors the flux
// service's provider resolution so the plan's step list matches what Bootstrap
// will actually execute.
func resolveKindGitProvider(cfg *v2.Config) string {
	if cfg.OpenCenter.GitOps.Auth.Token != nil {
		if p := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.GitOps.Auth.Token.Provider)); p != "" {
			return p
		}
	}
	return "gitea"
}

// filterOutSteps returns a new slice with any step whose ID matches one of the
// provided IDs removed, preserving the original order.
func filterOutSteps(steps []bootstrapStep, removeIDs ...string) []bootstrapStep {
	remove := make(map[string]struct{}, len(removeIDs))
	for _, id := range removeIDs {
		remove[id] = struct{}{}
	}
	filtered := make([]bootstrapStep, 0, len(steps))
	for _, step := range steps {
		if _, drop := remove[step.ID]; drop {
			continue
		}
		filtered = append(filtered, step)
	}
	return filtered
}

// resolveGitDir returns the GitOps directory for the cluster, falling back to
// the cluster paths when the config does not specify one.
func resolveGitDir(cfg *v2.Config, clusterPaths *paths.ClusterPaths) (string, error) {
	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" {
		gitDir = clusterPaths.GitOpsDir
	}
	if gitDir == "" {
		return "", fmt.Errorf("cluster %q does not define a git_dir", cfg.ClusterName())
	}
	return gitDir, nil
}

// rehomeInClusterSources rewrites the url field of every GitRepository manifest
// under applications/overlays/<cluster>/services/sources from the configured
// git_url host to the podman DNS alias "gitea", which is resolvable from inside
// the cluster. The local checkout is updated and committed; deploy's gitea-rebase
// step then fetches, rebases, and pushes the corrected sources. No-op when the
// configured URL already uses an in-cluster reachable host or the path does not
// exist.
func (p *kindBootstrapProvider) rehomeInClusterSources(ctx context.Context, cfg *v2.Config, clusterPaths *paths.ClusterPaths) error {
	rawURL := strings.TrimSpace(cfg.OpenCenter.GitOps.Repository.URL)
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("parse git_url %q: %w", rawURL, err)
	}
	// Skip hosts that are already in-cluster resolvable. Bare IPs are reachable
	// on the Kind network only when Gitea keeps its current IP; re-homing those to
	// "gitea" (podman DNS alias) is safer after a container restart.
	if u.Hostname() == "gitea" {
		logging.Debugf("rehomeInClusterSources: host %q already uses in-cluster alias; skipping", u.Hostname())
		return nil
	}
	overlayDir := filepath.Join(clusterPaths.GitOpsDir, "applications", "overlays", cfg.ClusterName())
	srcDirs := []string{
		filepath.Join(overlayDir, "services", "sources"),
		filepath.Join(overlayDir, "flux-system"),
	}
	var matches []string
	for _, d := range srcDirs {
		m, err := filepath.Glob(filepath.Join(d, "*.yaml"))
		if err != nil {
			return err
		}
		matches = append(matches, m...)
	}
	if len(matches) == 0 {
		logging.Debugf("rehomeInClusterSources: no source manifests found; skipping")
		return nil
	}
	oldHost := u.Host
	port := ""
	if u.Port() != "" {
		port = ":" + u.Port()
	}
	newHost := "gitea" + port
	for _, f := range matches {
		data, rerr := os.ReadFile(f)
		if rerr != nil {
			continue
		}
		updated := bytes.ReplaceAll(data, []byte(oldHost), []byte(newHost))
		if !bytes.Equal(data, updated) {
			if werr := os.WriteFile(f, updated, 0o644); werr != nil {
				return werr
			}
			logging.Debugf("rehomeInClusterSources: rewrote %s", f)
		}
	}
	gitDir := clusterPaths.GitOpsDir
	runGit := func(args ...string) error {
		cmd := exec.CommandContext(ctx, "git", append([]string{"-C", gitDir}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("git %s: %w\n%s", args[0], err, out)
		}
		return nil
	}
	if err := runGit("add", "-A"); err != nil {
		return err
	}
	return nil
}

// resolveKindClusterName returns the Kind cluster name, preferring an explicit
// override from the config when set.
func resolveKindClusterName(cfg *v2.Config) string {
	return cfg.ClusterName()
}
