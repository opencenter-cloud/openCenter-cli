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
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/tofuoutput"
)

// fakeTofu writes a stub `tofu` script that answers `output -json` with the
// supplied body, so the observer is exercised without OpenTofu or a backend.
func fakeTofu(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	script := filepath.Join(dir, "tofu")
	err := os.WriteFile(script, []byte("#!/bin/sh\nprintf '%s' "+shellQuote(body)+"\n"), 0o700)
	if err != nil {
		t.Fatalf("write stub tofu: %v", err)
	}
	return script
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func openstackCfg() *v2.Config {
	cfg := &v2.Config{}
	cfg.OpenCenter.Meta.Name = "obs"
	cfg.OpenCenter.Meta.Organization = "acme"
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack = &v2.OpenStackCloudConfig{}
	cfg.OpenCenter.GitOps.Repository.LocalDir = "/tmp/gitops"
	return cfg
}

func outputJSON(pairs map[string]string) string {
	body := "{"
	first := true
	for name, value := range pairs {
		if !first {
			body += ","
		}
		first = false
		body += `"` + name + `":{"value":"` + value + `"}`
	}
	return body + "}"
}

// The whole point of the observer: a tofu-managed cluster has empty
// network_id/subnet_id because `cluster provider openstack` cleared them so
// OpenTofu could create the network. Without promotion the CCM renders a blank
// [LoadBalancer] subnet-id, so VIPs are allocated from the wrong scope.
func TestObserveTerraformPromotesEmptyNetworkFields(t *testing.T) {
	cfg := openstackCfg()
	dir := t.TempDir()
	tofu := fakeTofu(t, outputJSON(map[string]string{
		TerraformOutputNetworkID: "net-1",
		TerraformOutputSubnetID:  "sub-1",
	}))

	inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, dir, tofu)
	if err != nil {
		t.Fatalf("ObserveTerraform: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %#v", conflicts)
	}
	got := map[string]any{}
	for _, field := range inferred {
		got[field.Path] = field.Value
	}
	for _, path := range []string{
		"opencenter.infrastructure.cloud.openstack.network_id",
		"opencenter.infrastructure.cloud.openstack.networking.network_id",
	} {
		if got[path] != "net-1" {
			t.Errorf("%s = %v, want net-1", path, got[path])
		}
	}
	for _, path := range []string{
		"opencenter.infrastructure.cloud.openstack.subnet_id",
		"opencenter.infrastructure.cloud.openstack.networking.subnet_id",
	} {
		if got[path] != "sub-1" {
			t.Errorf("%s = %v, want sub-1", path, got[path])
		}
	}
	for _, field := range inferred {
		if field.Origin != FieldOriginTerraform {
			t.Errorf("%s origin = %q, want %q", field.Path, field.Origin, FieldOriginTerraform)
		}
		if len(field.Evidence) == 0 || field.Evidence[0].Source != "tofu" {
			t.Errorf("%s evidence = %#v, want a tofu source", field.Path, field.Evidence)
		}
	}
}

// A populated subnet_id is an operator selection made with
// --create-internal-network=false. Overwriting it would repoint a running
// cluster's addressing, so a mismatch must surface as a conflict for review and
// never as an automatic promotion.
func TestObserveTerraformReportsConflictInsteadOfOverwrite(t *testing.T) {
	cfg := openstackCfg()
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.SubnetID = "operator-sub"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking = &v2.OpenStackNetworkingConfig{SubnetID: "operator-sub"}
	dir := t.TempDir()
	tofu := fakeTofu(t, outputJSON(map[string]string{TerraformOutputSubnetID: "created-sub"}))

	inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, dir, tofu)
	if err != nil {
		t.Fatalf("ObserveTerraform: %v", err)
	}
	for _, field := range inferred {
		t.Fatalf("populated field promoted without approval: %#v", field)
	}
	if len(conflicts) != 2 {
		t.Fatalf("conflicts = %d, want the flat and nested subnet paths", len(conflicts))
	}
	for _, conflict := range conflicts {
		if conflict.GitOpsValue != "operator-sub" || conflict.LiveValue != "created-sub" {
			t.Errorf("conflict = %#v, want operator-sub vs created-sub", conflict)
		}
	}
}

// An identical value is agreement, not drift: neither a promotion nor a conflict.
func TestObserveTerraformIgnoresAgreeingValues(t *testing.T) {
	cfg := openstackCfg()
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.SubnetID = "sub-1"
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.Networking = &v2.OpenStackNetworkingConfig{SubnetID: "sub-1"}
	dir := t.TempDir()
	tofu := fakeTofu(t, outputJSON(map[string]string{TerraformOutputSubnetID: "sub-1"}))

	inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, dir, tofu)
	if err != nil {
		t.Fatalf("ObserveTerraform: %v", err)
	}
	if len(inferred) != 0 || len(conflicts) != 0 {
		t.Fatalf("agreement reported work: inferred=%#v conflicts=%#v", inferred, conflicts)
	}
}

// A contract v2 module exposes no network outputs, and VLAN mode leaves the
// network outside OpenTofu. Both must yield an empty observation rather than an
// error, or observe would fail on every cluster that predates contract v3.
func TestObserveTerraformToleratesMissingOutputs(t *testing.T) {
	tests := map[string]string{
		"contract v2 module":  `{}`,
		"null outputs":        `{"opencenter_network_id":null,"opencenter_subnet_id":{"value":null}}`,
		"non-string outputs":  `{"opencenter_network_id":{"value":["a"]}}`,
		"empty string output": `{"opencenter_subnet_id":{"value":""}}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := openstackCfg()
			inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, t.TempDir(), fakeTofu(t, body))
			if err != nil {
				t.Fatalf("ObserveTerraform: %v", err)
			}
			if len(inferred) != 0 || len(conflicts) != 0 {
				t.Fatalf("inferred=%#v conflicts=%#v, want none", inferred, conflicts)
			}
		})
	}
}

// Kind, Magnum, and never-applied clusters have no infrastructure directory.
// observe must say "nothing to report", not fail.
func TestObserveTerraformWithoutInfrastructureDirectory(t *testing.T) {
	cfg := openstackCfg()
	missing := filepath.Join(t.TempDir(), "absent")

	inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, missing, "tofu")
	if err != nil {
		t.Fatalf("ObserveTerraform: %v", err)
	}
	if len(inferred) != 0 || len(conflicts) != 0 {
		t.Fatalf("inferred=%#v conflicts=%#v, want none", inferred, conflicts)
	}

	inferred, conflicts, err = ObserveTerraform(context.Background(), cfg, "", "tofu")
	if err != nil {
		t.Fatalf("ObserveTerraform empty dir: %v", err)
	}
	if len(inferred) != 0 || len(conflicts) != 0 {
		t.Fatalf("empty dir inferred=%#v conflicts=%#v, want none", inferred, conflicts)
	}
}

// A directory that exists but has no state is the never-applied case.
func TestObserveTerraformWithoutState(t *testing.T) {
	cfg := openstackCfg()
	dir := t.TempDir()
	script := filepath.Join(dir, "tofu")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho 'No state file was found' >&2\nexit 1\n"), 0o700)
	if err != nil {
		t.Fatalf("write stub tofu: %v", err)
	}

	inferred, conflicts, err := ObserveTerraform(context.Background(), cfg, t.TempDir(), script)
	if err != nil {
		t.Fatalf("ObserveTerraform treated a missing state as an error: %v", err)
	}
	if len(inferred) != 0 || len(conflicts) != 0 {
		t.Fatalf("inferred=%#v conflicts=%#v, want none", inferred, conflicts)
	}
}

// A real tofu failure (bad credentials, broken backend) must not be swallowed
// into "no drift", or observe would report a healthy cluster over a broken one.
func TestObserveTerraformSurfacesRealFailure(t *testing.T) {
	cfg := openstackCfg()
	dir := t.TempDir()
	script := filepath.Join(dir, "tofu")
	err := os.WriteFile(script, []byte("#!/bin/sh\necho 'Error: Failed to select workspace' >&2\nexit 1\n"), 0o700)
	if err != nil {
		t.Fatalf("write stub tofu: %v", err)
	}

	if _, _, err := ObserveTerraform(context.Background(), cfg, t.TempDir(), script); err == nil {
		t.Fatal("ObserveTerraform reported success over a failed tofu invocation")
	}
}

// ObserveTerraformFromConfig must resolve the same directory the deploy path
// renders into, or observe and deploy would inspect different trees.
func TestObserveTerraformFromConfigResolvesInfrastructureDir(t *testing.T) {
	root := t.TempDir()
	cfg := openstackCfg()
	cfg.OpenCenter.GitOps.Repository.LocalDir = root
	clusterDir := filepath.Join(root, "infrastructure", "clusters", "obs")
	if err := os.MkdirAll(clusterDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	tofu := fakeTofu(t, outputJSON(map[string]string{TerraformOutputNetworkID: "net-9"}))

	inferred, _, err := ObserveTerraformFromConfig(context.Background(), cfg, tofu)
	if err != nil {
		t.Fatalf("ObserveTerraformFromConfig: %v", err)
	}
	if len(inferred) != 2 {
		t.Fatalf("inferred = %#v, want the flat and nested network_id paths", inferred)
	}
	for _, field := range inferred {
		if field.Value != "net-9" {
			t.Errorf("%s = %v, want net-9", field.Path, field.Value)
		}
	}
}

func TestObserveTerraformOutputsDirect(t *testing.T) {
	cfg := openstackCfg()
	raw := []byte(`{"` + TerraformOutputNetworkID + `":{"value":"net-direct"},"` + TerraformOutputSubnetID + `":{"value":"sub-direct"}}`)
	outputs, err := tofuoutput.Parse(raw)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	inferred, conflicts, err := ObserveTerraformOutputs(cfg, "/infra/dir", outputs)
	if err != nil {
		t.Fatalf("ObserveTerraformOutputs: %v", err)
	}
	if len(conflicts) != 0 {
		t.Fatalf("unexpected conflicts: %v", conflicts)
	}
	if len(inferred) != 4 {
		t.Fatalf("expected 4 inferred results (2 flat + 2 nested), got %d", len(inferred))
	}
}
