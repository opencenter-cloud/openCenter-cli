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
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// TerraformOutputNames are the OpenTofu outputs `cluster observe` promotes into
// the cluster config. They are emitted by the generated infrastructure tree at
// lifecycle contract v3; a contract v2 module exposes none of them, which is an
// empty observation rather than an error.
const (
	TerraformOutputNetworkID = "opencenter_network_id"
	TerraformOutputSubnetID  = "opencenter_subnet_id"
)

// TerraformObservation is one OpenTofu output paired with the config path it
// promotes to. The path is empty when no config field owns the output, which
// keeps the mapping in one place instead of spreading it across callers.
type TerraformObservation struct {
	Output string
	Path   string
	Value  string
}

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
	if strings.TrimSpace(tofuPath) == "" {
		tofuPath = "tofu"
	}

	outputs, err := readTerraformOutputs(ctx, clusterDir, tofuPath)
	if err != nil {
		return nil, nil, err
	}
	if len(outputs) == 0 {
		return nil, nil, nil
	}

	var inferred []FieldInferenceResult
	var conflicts []FieldConflict
	for _, outputName := range []string{TerraformOutputNetworkID, TerraformOutputSubnetID} {
		value, ok := outputs[outputName]
		if !ok || strings.TrimSpace(value) == "" {
			continue
		}
		paths, known := terraformFieldPaths[outputName]
		if !known {
			continue
		}
		for _, path := range paths {
			current := strings.TrimSpace(readConfigPath(cfg, path))
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

// readTerraformOutputs runs `tofu output -json` and flattens it to name → string
// value. Outputs whose value is not a string (or is null) are skipped: the
// contract only defines string identity outputs, and a list-valued output is not
// something a config scalar can hold.
func readTerraformOutputs(ctx context.Context, clusterDir, tofuPath string) (map[string]string, error) {
	cmd := exec.CommandContext(ctx, tofuPath, "output", "-json")
	cmd.Dir = clusterDir
	output, err := cmd.Output()
	if err != nil {
		// No state yet (never applied) is an ordinary observation, not a
		// failure: there is simply nothing to promote.
		if len(output) == 0 {
			if ee, ok := err.(*exec.ExitError); ok && looksLikeMissingState(string(ee.Stderr)) {
				return nil, nil
			}
		}
		return nil, fmt.Errorf("read OpenTofu outputs in %s: %w", clusterDir, err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(output, &raw); err != nil {
		return nil, fmt.Errorf("parse OpenTofu outputs in %s: %w", clusterDir, err)
	}
	result := make(map[string]string, len(raw))
	for name, value := range raw {
		if string(value) == "null" {
			continue
		}
		// `tofu output -json` wraps each output as {"value": ...}. Accept a bare
		// value too, matching the tolerant parser used during deploy.
		var envelope map[string]json.RawMessage
		if err := json.Unmarshal(value, &envelope); err == nil {
			if nested, exists := envelope["value"]; exists {
				value = nested
			}
		}
		var text string
		if err := json.Unmarshal(value, &text); err != nil {
			continue
		}
		result[name] = text
	}
	return result, nil
}

func looksLikeMissingState(stderr string) bool {
	lowered := strings.ToLower(stderr)
	return strings.Contains(lowered, "no state file was found") ||
		strings.Contains(lowered, "command \"output\" failed")
}

// readConfigPath reads a dotted cluster-config path for the subset observe
// understands. It is deliberately a closed switch rather than reflection: an
// unmapped path returns "" so it is reported as promotable, and adding an output
// means adding a case here plus its entry in terraformFieldPaths, which keeps
// the two lists reviewable together.
func readConfigPath(cfg *v2.Config, path string) string {
	if cfg == nil {
		return ""
	}
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack == nil {
		return ""
	}
	switch path {
	case "opencenter.infrastructure.cloud.openstack.network_id":
		return openstack.NetworkID
	case "opencenter.infrastructure.cloud.openstack.subnet_id":
		return openstack.SubnetID
	case "opencenter.infrastructure.cloud.openstack.networking.network_id":
		if openstack.Networking != nil {
			return openstack.Networking.NetworkID
		}
	case "opencenter.infrastructure.cloud.openstack.networking.subnet_id":
		if openstack.Networking != nil {
			return openstack.Networking.SubnetID
		}
	}
	return ""
}
