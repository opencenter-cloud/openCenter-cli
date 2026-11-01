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
	"os"
	"path/filepath"
	"strings"
	"testing"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// writeStubTofu installs a fake `tofu` that answers `tofu output -json` with the
// supplied payload, so the binding path can be exercised without OpenTofu or a
// real state file.
func writeStubTofu(t *testing.T, payload string) string {
	t.Helper()
	dir := t.TempDir()
	body := strings.ReplaceAll(payload, `'`, `'\''`)
	script := "#!/bin/sh\nprintf '%s' '" + body + "'\n"
	path := filepath.Join(dir, "tofu")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub tofu: %v", err)
	}
	return path
}

func TestBindNetworkOutputsPromotesEmptyFields(t *testing.T) {
	cfg := mustNewClusterTestConfig("bind-empty", "openstack")
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	openstack.NetworkID = ""
	openstack.SubnetID = ""
	openstack.Networking.NetworkID = ""
	openstack.Networking.SubnetID = ""

	tofuPath := writeStubTofu(t, `{"opencenter_network_id":{"value":"net-1"},"opencenter_subnet_id":{"value":"subnet-1"}}`)

	var persisted int
	persist := func(context.Context, *v2.Config) error {
		persisted++
		return nil
	}

	bound, err := bindNetworkOutputs(context.Background(), &cfg, t.TempDir(), tofuPath, persist)
	if err != nil {
		// The stub path is passed through the tofuPath argument, so a real
		// failure here means the binding is not delegating correctly.
		t.Fatalf("bindNetworkOutputs: %v", err)
	}
	if !bound.Changed {
		t.Fatalf("expected Changed for an empty config, got %+v", bound)
	}
	if openstack.Networking.SubnetID != "subnet-1" {
		t.Errorf("nested subnet_id = %q, want subnet-1", openstack.Networking.SubnetID)
	}
	if openstack.Networking.NetworkID != "net-1" {
		t.Errorf("nested network_id = %q, want net-1", openstack.Networking.NetworkID)
	}
	if bound.SubnetID != "subnet-1" || bound.NetworkID != "net-1" {
		t.Errorf("bound = %+v, want net-1/subnet-1", bound)
	}
	if persisted != 1 {
		t.Errorf("persist calls = %d, want 1", persisted)
	}
}

// A populated subnet_id is an operator selection made with
// --create-internal-network=false. Repointing it would change a running
// cluster's addressing, so the config value has to survive.
func TestBindNetworkOutputsKeepsOperatorSelectedNetwork(t *testing.T) {
	cfg := mustNewClusterTestConfig("bind-kept", "openstack")
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	openstack.NetworkID = "chosen-net"
	openstack.SubnetID = "chosen-subnet"
	openstack.Networking.NetworkID = "chosen-net"
	openstack.Networking.SubnetID = "chosen-subnet"
	// The infrastructure disagrees with the config; the config must still win.
	tofuPath := writeStubTofu(t, `{"opencenter_network_id":{"value":"tofu-net"},"opencenter_subnet_id":{"value":"tofu-subnet"}}`)
	var persisted int
	persist := func(context.Context, *v2.Config) error {
		persisted++
		return nil
	}

	bound, err := bindNetworkOutputs(context.Background(), &cfg, t.TempDir(), tofuPath, persist)
	if err != nil {
		t.Fatalf("bindNetworkOutputs: %v", err)
	}
	if bound.Changed {
		t.Errorf("Changed = true, want false: the config already pinned the network")
	}
	if openstack.Networking.SubnetID != "chosen-subnet" || openstack.SubnetID != "chosen-subnet" {
		t.Errorf("operator selection was overwritten: flat=%q nested=%q",
			openstack.SubnetID, openstack.Networking.SubnetID)
	}
	if bound.SubnetID != "chosen-subnet" {
		t.Errorf("bound.SubnetID = %q, want the config value chosen-subnet", bound.SubnetID)
	}
	if persisted != 0 {
		t.Errorf("persist calls = %d, want 0 when nothing changed", persisted)
	}
}

// Kind, Magnum, VMware, baremetal and pre-contract-v3 clusters must be untouched:
// no infrastructure directory, no outputs, no write.
func TestBindNetworkOutputsInertWithoutInfrastructure(t *testing.T) {
	cfg := mustNewClusterTestConfig("bind-inert", "openstack")
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	openstack.Networking.SubnetID = ""

	bound, err := bindNetworkOutputs(context.Background(), &cfg, filepath.Join(t.TempDir(), "missing"), "tofu", nil)
	if err != nil {
		t.Fatalf("a missing infrastructure directory must not fail deploy: %v", err)
	}
	if bound.Changed {
		t.Errorf("Changed = true with no infrastructure directory")
	}
	if bound.SubnetID != "" {
		t.Errorf("bound.SubnetID = %q, want empty", bound.SubnetID)
	}
}

func TestBindNetworkOutputsNilConfigAndNonOpenStack(t *testing.T) {
	if _, err := bindNetworkOutputs(context.Background(), nil, "/tmp", "tofu", nil); err == nil {
		t.Error("expected an error for a nil config")
	}

	cfg := mustNewClusterTestConfig("bind-kind", "kind")
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack != nil {
		// Force the non-openstack shape the guard is written against.
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack = nil
	}
	bound, err := bindNetworkOutputs(context.Background(), &cfg, t.TempDir(), "tofu", nil)
	if err != nil {
		t.Fatalf("bindNetworkOutputs: %v", err)
	}
	if bound.Changed {
		t.Error("a provider with no openstack block must not report a change")
	}
}

func TestNeedsCCMNetworkRerender(t *testing.T) {
	openstack := func(useOctavia bool) *v2.Config {
		cfg := mustNewClusterTestConfig("rerender", "openstack")
		cfg.OpenCenter.Infrastructure.Cloud.OpenStack.UseOctavia = useOctavia
		return &cfg
	}

	tests := []struct {
		name  string
		cfg   *v2.Config
		bound *networkOutputsBinding
		want  bool
	}{
		{
			name:  "promoted subnet on the VRRP path re-renders",
			cfg:   openstack(false),
			bound: &networkOutputsBinding{SubnetID: "subnet-1", Changed: true},
			want:  true,
		},
		{
			// Octavia owns VIP allocation and the chart omits subnet-id, so a
			// newly recovered subnet changes nothing the CCM reads.
			name:  "octavia path skips the re-render",
			cfg:   openstack(true),
			bound: &networkOutputsBinding{SubnetID: "subnet-1", Changed: true},
			want:  false,
		},
		{
			name:  "nothing promoted skips the re-render",
			cfg:   openstack(false),
			bound: &networkOutputsBinding{SubnetID: "subnet-1", Changed: false},
			want:  false,
		},
		{
			name:  "promoted network without a subnet skips the re-render",
			cfg:   openstack(false),
			bound: &networkOutputsBinding{NetworkID: "net-1", Changed: true},
			want:  false,
		},
		{name: "nil binding", cfg: openstack(false), bound: nil, want: false},
		{name: "nil config", cfg: nil, bound: &networkOutputsBinding{Changed: true}, want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := needsCCMNetworkRerender(test.cfg, test.bound); got != test.want {
				t.Errorf("needsCCMNetworkRerender() = %v, want %v", got, test.want)
			}
		})
	}
}

func TestApplyNetworkField(t *testing.T) {
	cfg := mustNewClusterTestConfig("apply-field", "openstack")
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	openstack.NetworkID = ""
	openstack.SubnetID = ""
	openstack.Networking.NetworkID = ""
	openstack.Networking.SubnetID = ""

	cases := []struct {
		path string
		want func() bool
	}{
		{"opencenter.infrastructure.cloud.openstack.network_id", func() bool { return openstack.NetworkID == "v" }},
		{"opencenter.infrastructure.cloud.openstack.subnet_id", func() bool { return openstack.SubnetID == "v" }},
		{"opencenter.infrastructure.cloud.openstack.networking.network_id", func() bool { return openstack.Networking.NetworkID == "v" }},
		{"opencenter.infrastructure.cloud.openstack.networking.subnet_id", func() bool { return openstack.Networking.SubnetID == "v" }},
	}
	for _, test := range cases {
		if !applyNetworkField(&cfg, test.path, "v") {
			t.Errorf("applyNetworkField(%q) = false, want true", test.path)
			continue
		}
		if !test.want() {
			t.Errorf("applyNetworkField(%q) did not write the value", test.path)
		}
	}

	// An unknown path must be refused rather than silently dropped.
	if applyNetworkField(&cfg, "opencenter.infrastructure.cloud.openstack.flavor_master", "v") {
		t.Error("applyNetworkField accepted a path it does not own")
	}

	// A second write to the same path is refused: the first value wins.
	if applyNetworkField(&cfg, "opencenter.infrastructure.cloud.openstack.subnet_id", "other") {
		t.Error("applyNetworkField overwrote a value it had just written")
	}
	if openstack.SubnetID != "v" {
		t.Errorf("SubnetID = %q, want the first value v", openstack.SubnetID)
	}
}

func TestRerenderCCMNetworkValuesRequiresGitDir(t *testing.T) {
	cfg := mustNewClusterTestConfig("rerender-nogitdir", "openstack")
	cfg.OpenCenter.GitOps.Repository.LocalDir = ""
	if err := rerenderCCMNetworkValues(context.Background(), &cfg); err == nil {
		t.Error("expected an error when gitops.git_dir is unset")
	}
	if err := rerenderCCMNetworkValues(context.Background(), nil); err == nil {
		t.Error("expected an error for a nil config")
	}
}
