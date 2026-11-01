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
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/gitops"
	"github.com/opencenter-cloud/opencenter-cli/internal/importer"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
	"github.com/opencenter-cloud/opencenter-cli/internal/sops"
)

const (
	// bindNetworkOutputsStepID promotes the network/subnet OpenTofu created into
	// the cluster config.
	bindNetworkOutputsStepID = "bind-network-outputs"
	// rerenderCCMValuesStepID re-renders the CCM overlay from the promoted
	// network identity so the [LoadBalancer] subnet-id reaches the cluster.
	rerenderCCMValuesStepID = "rerender-ccm-network-values"
)

// networkOutputsBinding is the outcome of reading the applied OpenTofu outputs
// against the current config.
type networkOutputsBinding struct {
	// NetworkID and SubnetID are the effective values after binding: the
	// promoted value when the config was empty, otherwise the value the config
	// already held.
	NetworkID string
	SubnetID  string
	// Changed is true when this binding wrote a config value.
	Changed bool
}

// networkOutputsPersister writes a mutated config to disk. It is injected so the
// binding never reaches for a process-global config manager, and so deploy can
// keep the promotion in memory while `cluster promote` persists it.
type networkOutputsPersister func(ctx context.Context, cfg *v2.Config) error

// bindNetworkOutputs promotes the network/subnet identity OpenTofu created into
// the in-memory config and, when persist is non-nil, to disk.
//
// This closes the loop the CCM needs: `cluster provider openstack` deliberately
// clears network_id/subnet_id so OpenTofu can create the tenant network
// (--create-internal-network, the default), and nothing has ever written the
// created IDs back. The CCM renders [LoadBalancer] subnet-id from
// networking.subnet_id, so a tofu-managed cluster shipped a blank one and
// allocated VIPs from the wrong scope.
//
// A field the config already holds is never overwritten. A populated subnet_id
// means the operator chose the network with --create-internal-network=false, and
// repointing it would change a running cluster's addressing. Those cases are
// reported as conflicts by `cluster observe` instead.
//
// An absent infrastructure directory or a contract v2 module yields no change and
// no error, so this stays inert for Kind, Magnum, VMware, baremetal, and every
// cluster predating contract v3. clusterDir is the rendered infrastructure
// directory; tofuPath overrides the OpenTofu binary and defaults to "tofu".
func bindNetworkOutputs(ctx context.Context, cfg *v2.Config, clusterDir, tofuPath string, persist networkOutputsPersister) (*networkOutputsBinding, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		// Nothing owns these fields on a non-OpenStack provider.
		return &networkOutputsBinding{}, nil
	}

	inferred, conflicts, err := importer.ObserveTerraform(ctx, cfg, clusterDir, tofuPath)
	if err != nil {
		return nil, err
	}
	// Surfaced rather than silently ignored: the operator needs to know the
	// config and the infrastructure disagree, and which one won.
	for _, conflict := range conflicts {
		logging.Debugf("bind-network-outputs: %s config=%v deployed=%v; kept config",
			conflict.Path, conflict.GitOpsValue, conflict.LiveValue)
	}
	if len(inferred) == 0 {
		return &networkOutputsBinding{
			NetworkID: openstackEffectiveNetworkID(cfg),
			SubnetID:  openstackEffectiveSubnetID(cfg),
		}, nil
	}

	bound := &networkOutputsBinding{}
	for _, field := range inferred {
		if importer.IsProtectedField(field.Path) {
			continue
		}
		value, ok := field.Value.(string)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		if applyNetworkField(cfg, field.Path, strings.TrimSpace(value)) {
			bound.Changed = true
		}
	}

	bound.NetworkID = openstackEffectiveNetworkID(cfg)
	bound.SubnetID = openstackEffectiveSubnetID(cfg)

	if bound.Changed && persist != nil {
		if err := persist(ctx, cfg); err != nil {
			return bound, fmt.Errorf("persist promoted network identity: %w", err)
		}
	}
	return bound, nil
}

// applyNetworkField writes one promoted value into the config. Both the flat and
// the nested openstack representation are written, because the guided configure
// flow keeps them in sync (openstack_configure_orchestrator.go) and a
// half-updated pair would render differently depending on which one a template
// reads. Returns false when the target already held a value.
func applyNetworkField(cfg *v2.Config, path, value string) bool {
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return false
	}
	switch path {
	case "opencenter.infrastructure.cloud.openstack.network_id":
		if strings.TrimSpace(openstack.NetworkID) != "" {
			return false
		}
		openstack.NetworkID = value
	case "opencenter.infrastructure.cloud.openstack.subnet_id":
		if strings.TrimSpace(openstack.SubnetID) != "" {
			return false
		}
		openstack.SubnetID = value
	case "opencenter.infrastructure.cloud.openstack.networking.network_id":
		if openstack.Networking == nil || strings.TrimSpace(openstack.Networking.NetworkID) != "" {
			return false
		}
		openstack.Networking.NetworkID = value
	case "opencenter.infrastructure.cloud.openstack.networking.subnet_id":
		if openstack.Networking == nil || strings.TrimSpace(openstack.Networking.SubnetID) != "" {
			return false
		}
		openstack.Networking.SubnetID = value
	default:
		return false
	}
	return true
}

func openstackEffectiveNetworkID(cfg *v2.Config) string {
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return ""
	}
	if value := strings.TrimSpace(openstack.NetworkID); value != "" {
		return value
	}
	if openstack.Networking != nil {
		return strings.TrimSpace(openstack.Networking.NetworkID)
	}
	return ""
}

func openstackEffectiveSubnetID(cfg *v2.Config) string {
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return ""
	}
	if value := strings.TrimSpace(openstack.SubnetID); value != "" {
		return value
	}
	if openstack.Networking != nil {
		return strings.TrimSpace(openstack.Networking.SubnetID)
	}
	return ""
}

// needsCCMNetworkRerender reports whether the CCM overlay has to be re-rendered.
//
// Only the VRRP path is affected: with use_octavia=true the CCM omits subnet-id
// entirely (Octavia owns VIP allocation), so a newly recovered subnet changes
// nothing the CCM reads. Re-rendering there would dirty a SOPS-encrypted overlay
// for no behavioural gain, and every unnecessary write to that file is a chance
// to collide with an operator's own edits.
func needsCCMNetworkRerender(cfg *v2.Config, bound *networkOutputsBinding) bool {
	if cfg == nil || bound == nil || !bound.Changed {
		return false
	}
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil || openstack.UseOctavia {
		return false
	}
	return strings.TrimSpace(bound.SubnetID) != ""
}

// rerenderCCMNetworkValues re-renders only the openstack-ccm service from the
// now-complete config and promotes it into the GitOps worktree.
//
// It deliberately uses the scoped single-service render rather than a full
// GenerateClusterTree: a full render would also re-render main.tf from the
// mutated config, changing the network_id/subnet_id inputs OpenTofu already
// applied and producing a spurious tofu diff in the middle of a deploy. The
// scoped path touches services/openstack-ccm plus its namespace/sources wiring,
// runs the same SOPS encryption hook the render command uses, and updates the
// ownership ledger through the normal promotion path so the next `cluster
// generate` sees no phantom change.
//
// Prune is off: pruning the overlay scope could delete files this step does not
// own.
func rerenderCCMNetworkValues(ctx context.Context, cfg *v2.Config) error {
	if cfg == nil {
		return fmt.Errorf("configuration is nil")
	}
	if strings.TrimSpace(cfg.GitDir()) == "" {
		return fmt.Errorf("gitops.git_dir must be configured to re-render the CCM overlay")
	}
	prune := false
	if _, err := gitops.RenderSingleServiceWithEncryptionResult(
		ctx, *cfg, "openstack-ccm", false,
		sops.NewSOPSManager().EncryptServiceOverrideValues,
		gitops.PromoteOptions{Prune: &prune},
	); err != nil {
		return fmt.Errorf("re-render openstack-ccm overlay with the promoted network identity: %w", err)
	}
	return nil
}
