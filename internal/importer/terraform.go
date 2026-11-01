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

package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofuoutput"
)

// TerraformOutputNames are the OpenTofu outputs `cluster observe` promotes into
// the cluster config. They are emitted by the generated infrastructure tree at
// lifecycle contract v3; a contract v2 module exposes none of them, which is an
// empty observation rather than an error.
const (
	TerraformOutputNetworkID = "opencenter_network_id"
	TerraformOutputSubnetID  = "opencenter_subnet_id"
)

// terraformFieldPaths maps an OpenTofu output name to the cluster-config path
// that owns the value. Both the flat and the nested openstack networking fields
// are promoted together, matching what the guided configure flow writes
// (internal/cluster/openstack_configure_orchestrator.go) so the two representations
// cannot drift apart.
var terraformFieldPaths = map[string][]string{
	TerraformOutputNetworkID: {
		"opencenter.infrastructure.cloud.openstack.network_id",
		"opencenter.infrastructure.cloud.openstack.networking.network_id",
	},
	TerraformOutputSubnetID: {
		"opencenter.infrastructure.cloud.openstack.subnet_id",
		"opencenter.infrastructure.cloud.openstack.networking.subnet_id",
	},
}

// ObserveTerraform reads the post-apply OpenTofu outputs for a cluster and
// returns the config fields whose stored value differs from what the
// infrastructure actually created.
//
// The point of this is the tofu-managed network: `cluster provider openstack`
// clears network_id/subnet_id so OpenTofu can create them, and until now nothing
// ever wrote the created IDs back. The CCM renders its [LoadBalancer] subnet-id
// from networking.subnet_id, so a tofu-managed cluster shipped a blank one.
//
// A field that already holds a value is reported as a conflict, never as an
// automatic promotion: a populated subnet_id may be an operator selection made
// with --create-internal-network=false, and silently rewriting it would repoint
// a running cluster's addressing.
//
// clusterDir is the generated infrastructure directory (the `tofu` working
// directory). An absent directory or a module that predates contract v3 yields
// no observations and no error, so observe stays usable on Kind, Magnum,
// baremetal, and VMware clusters.
func ObserveTerraform(ctx context.Context, cfg *v2.Config, clusterDir string, tofuPath string) ([]FieldInferenceResult, []FieldConflict, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("configuration cannot be nil")
	}
	clusterDir = strings.TrimSpace(clusterDir)
	if clusterDir == "" {
		return nil, nil, nil
	}
	if info, err := os.Stat(clusterDir); err != nil || !info.IsDir() {
		return nil, nil, nil
	}

	outputs, err := tofuoutput.Run(ctx, clusterDir, tofuPath)
	if err != nil {
		return nil, nil, err
	}
	if len(outputs) == 0 {
		return nil, nil, nil
	}

	var inferred []FieldInferenceResult
	var conflicts []FieldConflict
	for _, outputName := range []string{TerraformOutputNetworkID, TerraformOutputSubnetID} {
		value, ok := outputs.String(outputName)
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		paths, known := terraformFieldPaths[outputName]
		if !known {
			continue
		}
		for _, path := range paths {
			current := strings.TrimSpace(cfgPathValue(cfg, path))
			evidence := []EvidenceRef{{
				Source: "tofu",
				Path:   clusterDir,
				Detail: fmt.Sprintf("output %s", outputName),
			}}
			switch {
			case current == "":
				inferred = append(inferred, FieldInferenceResult{
					Path:       path,
					Value:      value,
					Confidence: ConfidenceHigh,
					Origin:     FieldOriginTerraform,
					Evidence:   evidence,
				})
			case current != value:
				conflicts = append(conflicts, FieldConflict{
					Path:        path,
					GitOpsValue: current,
					LiveValue:   value,
					Recommended: "keep-config",
					Evidence:    evidence,
				})
			}
		}
	}
	return inferred, conflicts, nil
}

// ObserveTerraformFromConfig resolves the infrastructure directory from the
// cluster config and observes it. It mirrors the path the deploy path renders
// into, so observe and deploy never look at different directories.
func ObserveTerraformFromConfig(ctx context.Context, cfg *v2.Config, tofuPath string) ([]FieldInferenceResult, []FieldConflict, error) {
	if cfg == nil {
		return nil, nil, fmt.Errorf("configuration cannot be nil")
	}
	gitDir := strings.TrimSpace(cfg.GitDir())
	clusterName := strings.TrimSpace(cfg.ClusterName())
	if gitDir == "" || clusterName == "" {
		return nil, nil, nil
	}
	return ObserveTerraform(ctx, cfg, filepath.Join(gitDir, "infrastructure", "clusters", clusterName), tofuPath)
}

// cfgPathValue reads a dotted cluster-config path generically: the public
// serialization is decoded into a YAML document and the path is walked, so
// every field the config file can hold is readable without a per-field switch
// that drifts from the writer. An unreadable path is "" (the field is absent),
// matching how the config file itself would present it.
func cfgPathValue(cfg *v2.Config, path string) string {
	if cfg == nil {
		return ""
	}
	data, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		return ""
	}
	var document yaml.Node
	if err := yaml.Unmarshal(data, &document); err != nil || len(document.Content) == 0 {
		return ""
	}
	node := document.Content[0]
	for _, part := range strings.Split(strings.TrimSpace(path), ".") {
		if node == nil || node.Kind != yaml.MappingNode {
			return ""
		}
		node = findMappingValue(node, part)
	}
	if node == nil || node.Kind != yaml.ScalarNode {
		return ""
	}
	return node.Value
}
