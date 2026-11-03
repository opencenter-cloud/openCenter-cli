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
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
)

// ReconcileOptions holds options for cluster GitOps push and Flux reconciliation.
type ReconcileOptions struct {
	ClusterName    string
	Organization   string
	Services       []string
	All            bool
	Message        string
	Remote         string
	SkipFlux       bool // Skip Phase 2 (Flux in-cluster reconciliation)
	PushOnly       bool // Alias for SkipFlux
	FluxOnly       bool // Skip Phase 1 (GitOps commit & push)
	Generate       bool // Re-render before commit & push
	Wait           bool // Wait for Flux resources to report Ready (default true)
	Timeout        time.Duration
	DryRun         bool
	KubeconfigPath string
}

// ReconcileResult reports the result of a reconciliation run.
type ReconcileResult struct {
	ClusterName    string
	GitDir         string
	Services       []string
	CommitSHA      string
	Pushed         bool
	FluxReconciled bool
	Duration       time.Duration
	Plan           *ReconcilePlan
}

// ReconcilePlan describes the dry-run execution plan.
type ReconcilePlan struct {
	ClusterName    string              `json:"cluster_name"`
	Organization   string              `json:"organization,omitempty"`
	GitDir         string              `json:"git_dir"`
	KubeconfigPath string              `json:"kubeconfig_path,omitempty"`
	TargetServices []string            `json:"target_services"`
	Steps          []BootstrapPlanStep `json:"steps"`
}

// ReconcileService orchestrates Day-2 GitOps convergence and Day-1 deploy pushes.
type ReconcileService struct {
	pathResolver     *paths.PathResolver
	runner           lifecycleCommandRunner
	configurationMgr *config.ConfigurationManager
}

// NewReconcileService creates a ReconcileService with default execution runner.
func NewReconcileService(pathResolver *paths.PathResolver) *ReconcileService {
	return NewReconcileServiceWithRunner(pathResolver, newExecLifecycleCommandRunner())
}

// NewReconcileServiceWithRunner creates a ReconcileService with an injected command runner.
func NewReconcileServiceWithRunner(pathResolver *paths.PathResolver, runner lifecycleCommandRunner) *ReconcileService {
	cfgMgr, _ := config.NewConfigurationManager()
	return &ReconcileService{
		pathResolver:     pathResolver,
		runner:           runner,
		configurationMgr: cfgMgr,
	}
}

// Reconcile executes or plans the GitOps commit & push and Flux reconciliation workflow.
func (s *ReconcileService) Reconcile(ctx context.Context, opts ReconcileOptions) (*ReconcileResult, error) {
	return s.ReconcileWithConfig(ctx, opts, nil, nil)
}

// ReconcileWithConfig executes or plans reconciliation with preloaded config and paths.
func (s *ReconcileService) ReconcileWithConfig(
	ctx context.Context,
	opts ReconcileOptions,
	cfg *v2.Config,
	clusterPaths *paths.ClusterPaths,
) (*ReconcileResult, error) {
	startTime := time.Now()

	clusterName := strings.TrimSpace(opts.ClusterName)
	if clusterName == "" && cfg != nil {
		clusterName = cfg.ClusterName()
	}
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name is required for reconcile")
	}

	organization := strings.TrimSpace(opts.Organization)
	if strings.Contains(clusterName, "/") {
		parts := strings.SplitN(clusterName, "/", 2)
		if organization == "" {
			organization = parts[0]
		}
		clusterName = parts[1]
	}
	if organization == "" && cfg != nil {
		organization = cfg.Organization()
	}

	// Load configuration if not provided
	if cfg == nil {
		configIdentifier := clusterName
		if organization != "" {
			configIdentifier = organization + "/" + clusterName
		}
		if s.configurationMgr == nil {
			var err error
			s.configurationMgr, err = config.NewConfigurationManager()
			if err != nil {
				return nil, fmt.Errorf("create configuration manager: %w", err)
			}
		}
		loaded, err := s.configurationMgr.Load(ctx, configIdentifier)
		if err != nil {
			return nil, fmt.Errorf("load cluster configuration for %s: %w", configIdentifier, err)
		}
		cfg = loaded
	}

	// Resolve cluster paths if not provided
	if clusterPaths == nil {
		if s.pathResolver == nil {
			s.pathResolver = paths.NewPathResolver(config.ResolveClustersDir())
		}
		resolved, err := s.pathResolver.Resolve(ctx, clusterName, organization)
		if err != nil {
			return nil, fmt.Errorf("resolve cluster paths for %s: %w", clusterName, err)
		}
		clusterPaths = resolved
	}

	gitDir := strings.TrimSpace(cfg.GitDir())
	if gitDir == "" && clusterPaths != nil {
		gitDir = clusterPaths.GitOpsDir
	}
	if gitDir == "" {
		return nil, fmt.Errorf("git_dir is not configured for cluster %s", clusterName)
	}

	kubeconfig := strings.TrimSpace(opts.KubeconfigPath)
	if kubeconfig == "" && clusterPaths != nil {
		kubeconfig = clusterPaths.KubeconfigPath
	}

	skipFlux := opts.SkipFlux || opts.PushOnly
	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	// Determine target services
	targetServices := make([]string, 0, len(opts.Services))
	for _, svc := range opts.Services {
		svc = strings.TrimSpace(svc)
		if svc != "" {
			targetServices = append(targetServices, svc)
		}
	}

	// Auto-detect target services if not specified
	if len(targetServices) == 0 {
		if opts.All {
			targetServices = s.resolveAllServices(cfg)
		} else if !opts.DryRun {
			// Inspect git status to detect modified service directories
			rawStatus, err := s.runner.Run(ctx, gitDir, nil, "git", "status", "--porcelain")
			if err == nil {
				targetServices = detectModifiedServices(string(rawStatus), clusterName)
			}
		}
	}
	sort.Strings(targetServices)

	// Step 0: Single-service scoped generate if requested
	if opts.Generate && !opts.DryRun {
		if err := s.runGenerate(ctx, cfg, targetServices); err != nil {
			return nil, fmt.Errorf("generate failed: %w", err)
		}
		// If services were auto-detected, refresh detected services after generate
		if len(opts.Services) == 0 && !opts.All {
			rawStatus, err := s.runner.Run(ctx, gitDir, nil, "git", "status", "--porcelain")
			if err == nil {
				targetServices = detectModifiedServices(string(rawStatus), clusterName)
				sort.Strings(targetServices)
			}
		}
	}

	result := &ReconcileResult{
		ClusterName: clusterName,
		GitDir:      gitDir,
		Services:    targetServices,
	}

	// Resolve Git branch and remote
	branch := cfg.GitBranchOrDefault()
	remote := strings.TrimSpace(opts.Remote)

	scopedPaths := buildScopedGitPaths(clusterName, targetServices)

	commitMsg := strings.TrimSpace(opts.Message)
	if commitMsg == "" {
		if len(targetServices) > 0 {
			commitMsg = fmt.Sprintf("chore(%s): sync %s gitops manifests", clusterName, strings.Join(targetServices, ", "))
		} else {
			commitMsg = fmt.Sprintf("chore(%s): sync gitops manifests", clusterName)
		}
	}

	// Build DryRun plan or execute
	if opts.DryRun {
		plan := &ReconcilePlan{
			ClusterName:    clusterName,
			Organization:   organization,
			GitDir:         gitDir,
			KubeconfigPath: kubeconfig,
			TargetServices: targetServices,
		}

		if !opts.FluxOnly {
			// Step 1: Stage and Commit
			plan.Steps = append(plan.Steps, BootstrapPlanStep{
				ID:         "gitops-stage-commit",
				Action:     "Stage and commit cluster GitOps manifests",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", append([]string{"add", "--"}, scopedPaths...)...),
					commandPlan("opencenter", "cluster", "validate-manifests", "--repo-path", gitDir, "--staged", "--security-only"),
					commandPlan("git", "commit", "-m", commitMsg),
				},
				Notes: []string{"Only stages files within cluster scope and validates no unencrypted secrets are committed."},
			})

			// Step 2: Push
			planRemote := remote
			if planRemote == "" {
				planRemote = "upstream"
			}
			planBranch := branch
			if planBranch == "" {
				planBranch = "main"
			}
			plan.Steps = append(plan.Steps, BootstrapPlanStep{
				ID:         "gitops-push",
				Action:     "Pull rebase and push to remote GitOps repository",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("git", "pull", "--rebase", planRemote, planBranch),
					commandPlan("git", "push", planRemote, planBranch),
				},
			})
		}

		if !skipFlux {
			// Step 3: Flux reconcile source
			plan.Steps = append(plan.Steps, BootstrapPlanStep{
				ID:         "flux-reconcile-source",
				Action:     "Reconcile Flux GitRepository source",
				WorkingDir: gitDir,
				Commands: []BootstrapPlanCommand{
					commandPlan("flux", "reconcile", "source", "git", "flux-system", "-n", "flux-system", "--kubeconfig", kubeconfig),
				},
			})

			// Step 4+: Service reconciliations
			for _, svc := range targetServices {
				target, err := gitops.ResolveFluxServiceTarget(cfg, svc)
				if err != nil {
					continue
				}

				svcCommands := make([]BootstrapPlanCommand, 0)
				for _, extraRepo := range target.GitRepositories {
					if extraRepo != "flux-system" {
						svcCommands = append(svcCommands, commandPlan("flux", "reconcile", "source", "git", extraRepo, "-n", "flux-system", "--kubeconfig", kubeconfig))
					}
				}
				for _, ks := range target.KustomizationOrder {
					ksArgs := []string{"reconcile", "kustomization", ks, "-n", "flux-system"}
					if strings.HasSuffix(ks, "-override") {
						ksArgs = append(ksArgs, "--with-source")
					}
					if !opts.Wait {
						ksArgs = append(ksArgs, "--wait=false")
					}
					if timeout > 0 {
						ksArgs = append(ksArgs, "--timeout="+timeout.String())
					}
					ksArgs = append(ksArgs, "--kubeconfig", kubeconfig)
					svcCommands = append(svcCommands, commandPlan("flux", ksArgs...))
				}
				if target.HelmReleaseName != "" {
					hrArgs := []string{"reconcile", "helmrelease", target.HelmReleaseName, "-n", target.TargetNamespace}
					if !opts.Wait {
						hrArgs = append(hrArgs, "--wait=false")
					}
					if timeout > 0 {
						hrArgs = append(hrArgs, "--timeout="+timeout.String())
					}
					hrArgs = append(hrArgs, "--kubeconfig", kubeconfig)
					svcCommands = append(svcCommands, commandPlan("flux", hrArgs...))
				}

				if len(svcCommands) > 0 {
					plan.Steps = append(plan.Steps, BootstrapPlanStep{
						ID:         "flux-reconcile-" + svc,
						Action:     fmt.Sprintf("Reconcile Flux resources for %s", svc),
						WorkingDir: gitDir,
						Commands:   svcCommands,
					})
				}
			}
		}

		result.Plan = plan
		result.Duration = time.Since(startTime)
		return result, nil
	}

	// Execution Mode
	// Phase 1: GitOps Commit & Push
	if !opts.FluxOnly {
		// Stage existing scoped paths
		existingPaths := filterExistingPaths(gitDir, scopedPaths)
		if len(existingPaths) > 0 {
			addArgs := append([]string{"add", "--"}, existingPaths...)
			if _, err := s.runner.Run(ctx, gitDir, nil, "git", addArgs...); err != nil {
				return nil, fmt.Errorf("git add: %w", err)
			}
		}

		// Security scan on staged changes
		findings, err := gitops.ScanGitOpsSecretsWithOptions(ctx, gitops.SecretScanOptions{
			Root:   gitDir,
			Staged: true,
		})
		if err != nil {
			return nil, fmt.Errorf("gitops security scan: %w", err)
		}
		if len(findings) > 0 {
			var msgs []string
			for _, f := range findings {
				msgs = append(msgs, fmt.Sprintf("%s: %s: %s", f.Path, f.Rule, f.Message))
			}
			return nil, fmt.Errorf("refusing to commit: unencrypted secrets detected:\n  %s", strings.Join(msgs, "\n  "))
		}

		// Check if staged changes exist
		_, diffErr := s.runner.Run(ctx, gitDir, nil, "git", "diff", "--cached", "--quiet")
		if diffErr != nil {
			// Changes staged; commit them
			if _, err := s.runner.Run(ctx, gitDir, nil, "git", "commit", "-m", commitMsg); err != nil {
				return nil, fmt.Errorf("git commit: %w", err)
			}
		}

		// Resolve branch
		if branch == "" {
			rawBranch, _ := s.runner.Run(ctx, gitDir, nil, "git", "branch", "--show-current")
			branch = strings.TrimSpace(string(rawBranch))
		}
		if branch == "" {
			branch = "main"
		}

		// Resolve remote
		if remote == "" {
			rawRemote, _ := s.runner.Run(ctx, gitDir, nil, "git", "config", "--get", "branch."+branch+".remote")
			remote = strings.TrimSpace(string(rawRemote))
		}
		if remote == "" {
			rawRemotes, _ := s.runner.Run(ctx, gitDir, nil, "git", "remote")
			remoteList := strings.Fields(string(rawRemotes))
			for _, r := range []string{"upstream", "origin"} {
				for _, avail := range remoteList {
					if avail == r {
						remote = r
						break
					}
				}
				if remote != "" {
					break
				}
			}
		}
		if remote == "" {
			return nil, fmt.Errorf("no git remote found for repository in %s", gitDir)
		}

		// Pull rebase
		if _, err := s.runner.Run(ctx, gitDir, nil, "git", "pull", "--rebase", remote, branch); err != nil {
			return nil, fmt.Errorf("git pull --rebase %s %s: %w", remote, branch, err)
		}

		// Push
		if _, err := s.runner.Run(ctx, gitDir, nil, "git", "push", remote, branch); err != nil {
			return nil, fmt.Errorf("git push %s %s: %w", remote, branch, err)
		}
		result.Pushed = true

		// Capture HEAD commit SHA
		rawRev, err := s.runner.Run(ctx, gitDir, nil, "git", "rev-parse", "HEAD")
		if err == nil {
			result.CommitSHA = strings.TrimSpace(string(rawRev))
		}
	}

	// Phase 2: Flux In-Cluster Synchronization
	if !skipFlux {
		if strings.TrimSpace(kubeconfig) == "" {
			return nil, fmt.Errorf("kubeconfig is required for Flux reconciliation")
		}

		// Reconcile source git flux-system
		fluxSrcArgs := []string{"reconcile", "source", "git", "flux-system", "-n", "flux-system", "--kubeconfig", kubeconfig}
		if _, err := s.runner.Run(ctx, gitDir, nil, "flux", fluxSrcArgs...); err != nil {
			return nil, fmt.Errorf("flux reconcile source git flux-system: %w", err)
		}

		// Reconcile services
		for _, svc := range targetServices {
			target, err := gitops.ResolveFluxServiceTarget(cfg, svc)
			if err != nil {
				continue
			}

			// Reconcile any extra git repositories
			for _, extraRepo := range target.GitRepositories {
				if extraRepo != "flux-system" {
					extraArgs := []string{"reconcile", "source", "git", extraRepo, "-n", "flux-system", "--kubeconfig", kubeconfig}
					if _, err := s.runner.Run(ctx, gitDir, nil, "flux", extraArgs...); err != nil {
						return nil, fmt.Errorf("flux reconcile source git %s: %w", extraRepo, err)
					}
				}
			}

			// Reconcile ordered Kustomizations
			for _, ks := range target.KustomizationOrder {
				ksArgs := []string{"reconcile", "kustomization", ks, "-n", "flux-system"}
				if strings.HasSuffix(ks, "-override") {
					ksArgs = append(ksArgs, "--with-source")
				}
				if !opts.Wait {
					ksArgs = append(ksArgs, "--wait=false")
				}
				if timeout > 0 {
					ksArgs = append(ksArgs, "--timeout="+timeout.String())
				}
				ksArgs = append(ksArgs, "--kubeconfig", kubeconfig)
				if _, err := s.runner.Run(ctx, gitDir, nil, "flux", ksArgs...); err != nil {
					return nil, fmt.Errorf("flux reconcile kustomization %s: %w", ks, err)
				}
			}

			// Reconcile HelmRelease if applicable
			if target.HelmReleaseName != "" {
				hrArgs := []string{"reconcile", "helmrelease", target.HelmReleaseName, "-n", target.TargetNamespace}
				if !opts.Wait {
					hrArgs = append(hrArgs, "--wait=false")
				}
				if timeout > 0 {
					hrArgs = append(hrArgs, "--timeout="+timeout.String())
				}
				hrArgs = append(hrArgs, "--kubeconfig", kubeconfig)
				if _, err := s.runner.Run(ctx, gitDir, nil, "flux", hrArgs...); err != nil {
					return nil, fmt.Errorf("flux reconcile helmrelease %s in namespace %s: %w", target.HelmReleaseName, target.TargetNamespace, err)
				}
			}
		}

		result.FluxReconciled = true
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// runGenerate executes single-service scoped rendering or full tree generation.
func (s *ReconcileService) runGenerate(ctx context.Context, cfg *v2.Config, targetServices []string) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if len(targetServices) > 0 {
		prune := false
		for _, svc := range targetServices {
			if _, err := gitops.RenderSingleServiceWithEncryptionResult(
				ctx, *cfg, svc, false,
				sops.NewSOPSManager().EncryptServiceOverrideValues,
				gitops.PromoteOptions{Prune: &prune},
			); err != nil {
				return fmt.Errorf("scoped render for service %s: %w", svc, err)
			}
		}
		return nil
	}
	return nil
}

func (s *ReconcileService) resolveAllServices(cfg *v2.Config) []string {
	if cfg == nil || cfg.OpenCenter.Services == nil {
		return nil
	}
	services := make([]string, 0, len(cfg.OpenCenter.Services))
	for svcName := range cfg.OpenCenter.Services {
		services = append(services, svcName)
	}
	sort.Strings(services)
	return services
}

// detectModifiedServices parses git status --porcelain output to detect affected services.
func detectModifiedServices(rawStatus, clusterName string) []string {
	prefix := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services")) + "/"
	fluxcdPrefix := filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services", "fluxcd")) + "/"

	serviceSet := make(map[string]struct{})
	for _, line := range strings.Split(rawStatus, "\n") {
		line = strings.TrimSpace(line)
		if len(line) < 4 {
			continue
		}
		// git status porcelain line format: "XY path" or "XY orig -> path"
		filePath := strings.TrimSpace(line[2:])
		if arrowIdx := strings.Index(filePath, " -> "); arrowIdx != -1 {
			filePath = filePath[arrowIdx+4:]
		}
		filePath = filepath.ToSlash(filePath)

		if strings.HasPrefix(filePath, fluxcdPrefix) {
			rel := strings.TrimPrefix(filePath, fluxcdPrefix)
			base := filepath.Base(rel)
			if strings.HasSuffix(base, ".yaml") {
				svc := strings.TrimSuffix(base, ".yaml")
				svc = strings.TrimSuffix(svc, "-namespace")
				if svc != "kustomization" && svc != "" {
					serviceSet[svc] = struct{}{}
				}
			}
		} else if strings.HasPrefix(filePath, prefix) {
			rel := strings.TrimPrefix(filePath, prefix)
			parts := strings.Split(rel, "/")
			if len(parts) > 0 && parts[0] != "" && parts[0] != "fluxcd" && parts[0] != "sources" {
				serviceSet[parts[0]] = struct{}{}
			}
		}
	}

	services := make([]string, 0, len(serviceSet))
	for svc := range serviceSet {
		services = append(services, svc)
	}
	sort.Strings(services)
	return services
}

// buildScopedGitPaths builds repository-relative paths to stage for the target cluster.
func buildScopedGitPaths(clusterName string, targetServices []string) []string {
	if len(targetServices) > 0 {
		paths := make([]string, 0)
		for _, svc := range targetServices {
			paths = append(paths,
				filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services", svc)),
				filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services", "fluxcd", svc+".yaml")),
				filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, "services", "fluxcd", svc+"-namespace.yaml")),
			)
		}
		paths = append(paths,
			filepath.ToSlash(filepath.Join("applications", "overlays", clusterName, ".opencenter-secret-artifacts.json")),
			filepath.ToSlash(filepath.Join(".opencenter", "ownership", "clusters", clusterName+".json")),
		)
		return paths
	}

	return []string{
		filepath.ToSlash(filepath.Join("applications", "overlays", clusterName)),
		filepath.ToSlash(filepath.Join("infrastructure", "clusters", clusterName)),
		filepath.ToSlash(filepath.Join(".opencenter", "ownership", "clusters", clusterName+".json")),
		filepath.ToSlash(filepath.Join(".opencenter", "ownership", "global.json")),
		".gitignore",
		".sops.yaml",
	}
}

// filterExistingPaths filters paths to those that exist on disk or in git.
func filterExistingPaths(gitDir string, candidates []string) []string {
	result := make([]string, 0, len(candidates))
	for _, p := range candidates {
		full := filepath.Join(gitDir, filepath.FromSlash(p))
		if _, err := os.Stat(full); err == nil {
			result = append(result, p)
		}
	}
	return result
}

// PrintReconcilePlan writes a human-readable execution plan to w.
func PrintReconcilePlan(w io.Writer, plan *ReconcilePlan) {
	if plan == nil {
		return
	}
	fmt.Fprintf(w, "Reconcile Plan for cluster: %s\n", plan.ClusterName)
	if plan.GitDir != "" {
		fmt.Fprintf(w, "GitOps Repository: %s\n", plan.GitDir)
	}
	if plan.KubeconfigPath != "" {
		fmt.Fprintf(w, "Kubeconfig:        %s\n", plan.KubeconfigPath)
	}
	if len(plan.TargetServices) > 0 {
		fmt.Fprintf(w, "Target Services:   %s\n", strings.Join(plan.TargetServices, ", "))
	} else {
		fmt.Fprintf(w, "Target Services:   (all / repository source)\n")
	}
	fmt.Fprintln(w)

	for i, step := range plan.Steps {
		fmt.Fprintf(w, "Step %d: %s\n", i+1, step.ID)
		if step.WorkingDir != "" {
			fmt.Fprintf(w, "  Working Dir: %s\n", step.WorkingDir)
		}
		if len(step.Commands) > 0 {
			fmt.Fprintf(w, "  Commands:\n")
			for _, cmd := range step.Commands {
				fmt.Fprintf(w, "    %s\n", formatDebugCommand(cmd))
			}
		}
		if len(step.Notes) > 0 {
			fmt.Fprintf(w, "  Notes:\n")
			for _, note := range step.Notes {
				fmt.Fprintf(w, "    %s\n", note)
			}
		}
		fmt.Fprintln(w)
	}
}
