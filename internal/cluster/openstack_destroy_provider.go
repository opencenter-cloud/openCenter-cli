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
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/credentials"
)

type openstackDestroyProvider struct {
	runner lifecycleCommandRunner
	output io.Writer
	// result receives non-fatal warnings from step bodies so the CLI can
	// surface them. Set by DestroyService before BuildSteps runs.
	result *DestroyResult
	// cinderFactory creates the Cinder service client. Override in tests to
	// avoid live OpenStack calls.
	cinderFactory func(*credentials.OpenStackCredentials) (cinderVolumeService, error)
	// networkCleanupFactory creates the pre-destroy Octavia cleanup service.
	// Override in tests to avoid live OpenStack calls.
	networkCleanupFactory func(*credentials.OpenStackCredentials) (openStackNetworkCleanupService, error)
}

func newOpenStackDestroyProvider(runner lifecycleCommandRunner, output io.Writer) lifecycleDestroyProvider {
	return &openstackDestroyProvider{
		runner:                runner,
		output:                output,
		cinderFactory:         newGophercloudCinderService,
		networkCleanupFactory: newGophercloudOpenStackNetworkCleanupService,
	}
}

// warn records a non-fatal degradation for the attached destroy result.
// Safe to call when no result sink is attached (direct provider use in
// tests): the warning is then only visible in the log/output.
func (p *openstackDestroyProvider) warn(result *DestroyResult, format string, args ...any) {
	if result == nil {
		return
	}
	result.AddCleanupWarning(fmt.Errorf(format, args...))
}

func (p *openstackDestroyProvider) BuildSteps(cfg *v2.Config, opts *DestroyInfraOptions) ([]destroyStep, error) {
	clusterDir, err := infrastructureClusterDir(cfg)
	if err != nil {
		return nil, err
	}

	if !cfg.OpenTofu.Enabled {
		return nil, fmt.Errorf("opentofu must be enabled for openstack destroy")
	}

	// An absent infrastructure directory means there is nothing left to run
	// OpenTofu against: the infrastructure was already destroyed, or the
	// GitOps working directory (gitops.repository.local_dir) no longer exists
	// (e.g. a checkout under a deleted temp directory). Treat it as an
	// idempotent no-op so `cluster destroy --remove-files` can still complete
	// local cleanup instead of failing permanently on a vanished directory.
	infraAlreadyGone := false
	if _, err := os.Stat(clusterDir); err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("checking cluster infrastructure directory: %w", err)
		}
		infraAlreadyGone = true
		p.logf("Cluster infrastructure directory not found: %s\n", clusterDir)
		p.logf("Nothing to destroy via OpenTofu; infrastructure may already be destroyed.\n")
	}

	var (
		env          map[string]string
		openTofuPath string
		destroyArgs  []string
		creds        *credentials.OpenStackCredentials
		credsErr     error
	)

	// Credentials are extracted whenever the config provides them: the
	// Octavia sweep and Cinder cleanup target resources that can outlive the
	// infrastructure directory. The tofu binary is only resolved when there
	// is a directory to destroy. A broken credential section with no
	// infrastructure directory must not block local file cleanup: record the
	// error and let the remote steps degrade instead of failing BuildSteps.
	extracted, extractErr := credentials.NewExtractor(*cfg).ExtractOpenStack()
	if extractErr == nil {
		creds = extracted
		env = buildDestroyEnvironment()
		mergeBootstrapEnvironment(env, extracted.ToEnvMap())
	}
	if !infraAlreadyGone {
		if extractErr != nil {
			return nil, fmt.Errorf("preparing openstack destroy: %w", extractErr)
		}
		if credsErr = p.prepareOpenStackDestroy(cfg, env, &openTofuPath, &destroyArgs, opts); credsErr != nil {
			return nil, fmt.Errorf("preparing openstack destroy: %w", credsErr)
		}
	} else {
		credsErr = extractErr
	}

	// With the infrastructure directory gone AND credentials unavailable,
	// there is nothing left to sweep or delete remotely; the networking and
	// Cinder steps are skipped rather than failing on credentials that were
	// only needed for the live destroy.
	skipRemoteSteps := infraAlreadyGone && credsErr != nil

	// CSI volume cleanup is only applicable to the OpenStack provider. The
	// VMware provider shares this same struct but does not use Cinder.
	isOpenStack := strings.EqualFold(cfg.Provider(), "openstack")

	// Resolve the cluster kubeconfig once: the CSI capture step reads it and
	// the OpenTofu environment exports it as KUBECONFIG (bootstrap parity).
	// The canonical location is the secrets zone; the infrastructure-dir
	// path is a legacy fallback.
	kubeconfigPath := resolveClusterKubeconfigPath(cfg, clusterDir)
	if kubeconfigPath != "" {
		if _, err := os.Stat(kubeconfigPath); err == nil && env != nil {
			env["KUBECONFIG"] = kubeconfigPath
		}
	}

	// capturedHandles is shared between the capture step and the cleanup step
	// via closure. It is populated before opentofu-destroy runs.
	var capturedHandles []string

	var steps []destroyStep

	if isOpenStack {
		networkID := ""
		if openstackCfg := cfg.OpenCenter.Infrastructure.Cloud.OpenStack; openstackCfg != nil {
			networkID = strings.TrimSpace(openstackCfg.NetworkID)
			if networkID == "" && openstackCfg.Networking != nil {
				networkID = strings.TrimSpace(openstackCfg.Networking.NetworkID)
			}
		}

		// Pre-flight summary: log the resolved inputs so a post-mortem can
		// explain silent skips (e.g. an empty network ID disabling the LB
		// sweep, or a missing kubeconfig disabling CSI capture).
		kubeconfigExists := false
		if kubeconfigPath != "" {
			if _, err := os.Stat(kubeconfigPath); err == nil {
				kubeconfigExists = true
			}
		}
		p.logf("Destroy inputs: infrastructure_dir=%s (exists=%v), kubeconfig=%s (exists=%v), tofu_binary=%s, network_id=%q\n",
			clusterDir, !infraAlreadyGone, kubeconfigPath, kubeconfigExists, displayTofuBinary(openTofuPath, infraAlreadyGone), networkID)

		requireClusterAccess := opts != nil && opts.RequireClusterAccess
		steps = append(steps, destroyStep{
			ID:          "capture-csi-volumes",
			Description: "Capture CSI-provisioned Cinder volume handles from cluster",
			Fatal:       requireClusterAccess,
			Run: func(ctx context.Context) error {
				// Distinguish "kubeconfig absent" (cluster may already be
				// gone: expected skip) from "cluster unreachable" (real
				// failure: volumes may be orphaned without a record).
				effectivePath, statErr := kubeconfigForDestroy(cfg, clusterDir)
				if statErr != nil {
					if requireClusterAccess {
						return fmt.Errorf("capture CSI volume handles: %w", statErr)
					}
					p.logfCtx(ctx, "Warning: skipping CSI volume capture: %v\n", statErr)
					p.logfCtx(ctx, "Orphaned Cinder volumes may need manual cleanup after destroy.\n")
					p.warn(p.result, "capture-csi-volumes: %v", statErr)
					return nil
				}
				handles, err := collectCSIVolumeHandles(ctx, effectivePath, p.runner)
				if err != nil {
					if requireClusterAccess {
						return fmt.Errorf("capture CSI volume handles: %w", err)
					}
					p.logfCtx(ctx, "Warning: could not capture CSI volume handles: %v\n", err)
					p.logfCtx(ctx, "Orphaned Cinder volumes may need manual cleanup after destroy.\n")
					p.warn(p.result, "capture-csi-volumes: %v", err)
					return nil
				}
				capturedHandles = handles
				p.logfCtx(ctx, "Captured %d CSI-provisioned Cinder volume handle(s).\n", len(handles))
				return nil
			},
		})
		// The Octavia sweep runs BEFORE tofu destroy: skipping it on failure
		// leaks cluster-owned load balancers, so its failure is fatal unless
		// the operator explicitly accepts the risk. When the infrastructure
		// directory is already gone there is no tofu state left to own the
		// load balancers; sweeping with placeholder or unscoped credentials
		// cannot succeed, so the step is skipped (with credentials present
		// it still runs and can clean up real leftovers).
		networkingUseful := creds != nil && networkID != ""
		if !skipRemoteSteps && (!infraAlreadyGone || networkingUseful) {
			steps = append(steps, destroyStep{
				ID:          "cleanup-openstack-networking",
				Description: "Remove cluster-owned Octavia load balancers before OpenTofu destroy",
				Fatal:       opts == nil || !opts.AllowCleanupFailures,
				Run: func(ctx context.Context) error {
					svc, err := p.networkCleanupFactory(creds)
					if err != nil {
						return fmt.Errorf("create OpenStack networking cleanup client: %w", err)
					}
					if err := svc.CleanupClusterLoadBalancers(ctx, cfg.ClusterName(), networkID); err != nil {
						return fmt.Errorf("clean up cluster networking: %w", err)
					}
					return nil
				},
			})
		}
	}

	if !infraAlreadyGone {
		retries := 0
		if opts != nil {
			retries = opts.DestroyRetries
		}
		steps = append(steps,
			destroyStep{
				ID:          "opentofu-init",
				Description: "Initialize OpenTofu",
				Fatal:       true,
				Run: func(ctx context.Context) error {
					p.logfCtx(ctx, "Running OpenTofu in %s\n", clusterDir)
					_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, "init")
					return err
				},
			},
			destroyStep{
				ID:          "opentofu-destroy",
				Description: "Destroy OpenTofu infrastructure",
				Fatal:       true,
				Run: func(ctx context.Context) error {
					p.logfCtx(ctx, "Running OpenTofu in %s\n", clusterDir)
					var lastErr error
					for attempt := 0; attempt <= retries; attempt++ {
						if attempt > 0 {
							if ctx.Err() != nil {
								return ctx.Err()
							}
							p.logfCtx(ctx, "Retrying OpenTofu destroy (attempt %d/%d) after transient failure: %v\n", attempt+1, retries+1, lastErr)
							logBootstrapMessage(ctx, "destroy: retrying opentofu-destroy (attempt %d/%d)", attempt+1, retries+1)
						}
						_, err := p.runner.Run(ctx, clusterDir, env, openTofuPath, destroyArgs...)
						if err == nil {
							return nil
						}
						lastErr = err
						// Cancellation is never transient; stop immediately.
						if ctx.Err() != nil {
							return err
						}
					}
					return lastErr
				},
			},
		)
	}

	if isOpenStack && !skipRemoteSteps {
		deleteVolumes := opts != nil && opts.DeleteVolumes
		steps = append(steps, destroyStep{
			ID:          "cleanup-csi-volumes",
			Description: "Delete or report orphaned CSI-provisioned Cinder volumes",
			Run: func(ctx context.Context) error {
				if len(capturedHandles) == 0 {
					p.logfCtx(ctx, "No CSI-provisioned Cinder volumes to clean up.\n")
					return nil
				}
				svc, err := p.cinderFactory(creds)
				if err != nil {
					// Non-fatal: infra is already destroyed; warn and list handles.
					p.logfCtx(ctx, "Warning: could not create Cinder client for volume cleanup: %v\n", err)
					p.logfCtx(ctx, "The following %d volume(s) may need manual cleanup:\n", len(capturedHandles))
					for _, h := range capturedHandles {
						p.logfCtx(ctx, "  - %s\n", h)
					}
					p.warn(p.result, "cleanup-csi-volumes: cinder client: %v", err)
					return nil
				}
				if err := cleanupCSIVolumes(ctx, capturedHandles, svc, deleteVolumes, p.output); err != nil {
					// In delete-mode: surface error so caller can signal non-zero exit.
					// In report-only: log warning and continue (destruction already succeeded).
					if deleteVolumes {
						p.logfCtx(ctx, "ERROR: CSI volume cleanup failed: %v\n", err)
						p.logfCtx(ctx, "Verify that all volumes were deleted. Some may still be orphaned.\n")
						// Return the error; the service tracks it as a cleanup
						// warning (infra is already destroyed; aborting now
						// would hide the remaining cleanup report).
						return fmt.Errorf("volume cleanup: %w", err)
					}
					p.logfCtx(ctx, "Warning: CSI volume cleanup encountered errors: %v\n", err)
					p.logfCtx(ctx, "Some volumes may need manual cleanup.\n")
					p.warn(p.result, "cleanup-csi-volumes: %v", err)
				}
				return nil
			},
		})
	}

	return steps, nil
}

// prepareOpenStackDestroy resolves the tofu binary and assembles the destroy
// arguments (credentials/env are extracted by the caller). It is only
// invoked when the infrastructure directory exists; failures are fatal at
// that point because there is real infrastructure that must not be abandoned
// half-destroyed.
func (p *openstackDestroyProvider) prepareOpenStackDestroy(
	cfg *v2.Config,
	env map[string]string,
	openTofuPath *string,
	destroyArgs *[]string,
	opts *DestroyInfraOptions,
) error {
	resolved, err := resolveTofuBinary(cfg.OpenTofu.Path)
	if err != nil {
		return err
	}
	*openTofuPath = resolved

	args := []string{"destroy"}
	if opts != nil && opts.AutoApprove {
		args = append(args, "-auto-approve")
	}
	if opts != nil && opts.DestroyParallelism > 0 {
		args = append(args, fmt.Sprintf("-parallelism=%d", opts.DestroyParallelism))
	}
	*destroyArgs = args
	return nil
}

// displayTofuBinary renders the tofu binary for the pre-flight log line,
// including the case where credentials/binary resolution was skipped
// because the infrastructure directory is gone.
func displayTofuBinary(path string, infraAlreadyGone bool) string {
	if infraAlreadyGone {
		return "<not resolved: infrastructure directory absent>"
	}
	if strings.TrimSpace(path) == "" {
		return "<unresolved>"
	}
	return path
}

// logf writes provider messages to the command output. BuildSteps-time
// messages have no step context; step bodies must use logfCtx so the same
// lines land in the destroy action log (installed by the destroy command
// via logging.WithActionLogWriter).
func (p *openstackDestroyProvider) logf(format string, args ...any) {
	if p.output != nil {
		fmt.Fprintf(p.output, format, args...)
	}
}

// logfCtx writes provider messages to the command output and to the action
// log carried by ctx, so warnings are never stdout-only.
func (p *openstackDestroyProvider) logfCtx(ctx context.Context, format string, args ...any) {
	p.logf(format, args...)
	if writer := bootstrapLogWriter(ctx); writer != nil {
		fmt.Fprintf(writer, format, args...)
	}
}

// buildDestroyEnvironment creates the environment variables for destroy operations.
func buildDestroyEnvironment() map[string]string {
	env := make(map[string]string)

	if path := os.Getenv("PATH"); path != "" {
		env["PATH"] = path
	}

	return env
}
