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

package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// hostnameRepairTestConfig builds a config whose service hostnames are exactly
// what `cluster init` bakes under the placeholder identity, then applies the
// caller's identity change without re-baking hostnames -- the state a real
// `cluster init --base-domain` leaves behind.
func hostnameRepairTestConfig(t *testing.T, clusterName, region, baseDomain string) *Config {
	t.Helper()
	cfg, err := NewV2DefaultWithRegion(clusterName, "openstack", InitDefaultRegion)
	if err != nil {
		t.Fatalf("NewV2DefaultWithRegion() error = %v", err)
	}
	cfg.OpenCenter.Meta.Region = region
	cfg.OpenCenter.Cluster.BaseDomain = baseDomain
	cfg.OpenCenter.Cluster.ClusterFQDN = cfg.OpenCenter.Cluster.DeriveClusterFQDN(region)
	return cfg
}

func TestRepairPlaceholderHostnamesFollowsBaseDomain(t *testing.T) {
	cfg := hostnameRepairTestConfig(t, "doritos", "IAD3", "k8s.opensnack.cloud")

	before := keycloakHostname(t, cfg)
	wantBefore := "auth.doritos." + strings.ToLower(InitDefaultRegion) + "." + defaultBaseDomain
	if before != wantBefore {
		t.Fatalf("fixture must start at the baked placeholder hostname %q, got %q", wantBefore, before)
	}

	repaired := append(
		RepairPlaceholderHostnames(cfg),
		RepairPlaceholderDNSZoneNames(cfg)...)

	wantFQDN := "doritos.iad3.k8s.opensnack.cloud"
	// Only keycloak/headlamp/longhorn/alert-proxy carry a baked hostname in the
	// built-in defaults. The kube-prometheus-stack hostnames default to empty so
	// the renderers' `| default (printf "prometheus.%s" fqdn)` applies and they
	// already follow cluster_fqdn; the sweep has nothing to repair there, and an
	// empty value must stay empty rather than be materialized.
	cases := map[string]string{
		"opencenter.services.keycloak.hostname":                   "auth." + wantFQDN,
		"opencenter.services.headlamp.hostname":                   "dashboard." + wantFQDN,
		"opencenter.services.longhorn.hostname":                   "longhorn." + wantFQDN,
		"opencenter.managed_services.alert-proxy.http_route_fqdn": "alerts." + wantFQDN,
	}
	for field, want := range cases {
		got, ok := getHostnameField(cfg, field)
		if !ok {
			t.Fatalf("getHostnameField(%q) reported an unknown field", field)
		}
		if got != want {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
		if !containsName(repaired, field) {
			t.Errorf("RepairPlaceholderHostnames() did not report %s as repaired: %v", field, repaired)
		}
	}

	for _, field := range []string{
		"opencenter.services.kube-prometheus-stack.prometheus_hostname",
		"opencenter.services.kube-prometheus-stack.alertmanager_hostname",
		"opencenter.services.kube-prometheus-stack.grafana_hostname",
	} {
		if got, _ := getHostnameField(cfg, field); got != "" {
			t.Errorf("%s = %q, want empty so the renderer's fqdn default applies", field, got)
		}
		if containsName(repaired, field) {
			t.Errorf("RepairPlaceholderHostnames() repaired an unset field %s", field)
		}
	}

	// Nothing in the repaired config may still name the placeholder domain.
	if data, err := MarshalPublicConfig(cfg); err != nil {
		t.Fatalf("MarshalPublicConfig() error = %v", err)
	} else if strings.Contains(string(data), defaultBaseDomain) {
		t.Errorf("repaired config still contains the placeholder domain %q:\n%s", defaultBaseDomain, data)
	}
}

// TestRepairPlaceholderHostnamesPreservesOperatorChoice is the gate: the sweep
// must never touch a hostname an operator chose. A regression here silently
// rewrites customer DNS names.
func TestRepairPlaceholderHostnamesPreservesOperatorChoice(t *testing.T) {
	cfg := hostnameRepairTestConfig(t, "doritos", "IAD3", "k8s.opensnack.cloud")

	const operatorChoice = "auth.sso.internal.example.com"
	if kc := typedServiceConfig[*services.KeycloakConfig](cfg, "keycloak"); kc != nil {
		kc.Hostname = operatorChoice
	} else {
		t.Fatal("keycloak service config missing from defaults")
	}

	if repaired := RepairPlaceholderHostnames(cfg); containsName(repaired, "opencenter.services.keycloak.hostname") {
		t.Fatalf("sweep reported an operator-chosen hostname as repaired: %v", repaired)
	}
	if got := keycloakHostname(t, cfg); got != operatorChoice {
		t.Fatalf("operator-chosen hostname was rewritten to %q, want %q", got, operatorChoice)
	}
}

// TestPlaceholderDerivedHostnameRejectsSuffixLookalikes guards the exact-match
// rule: a hostname that merely ends in the placeholder domain is not proof that
// init baked it.
func TestPlaceholderDerivedHostnameRejectsSuffixLookalikes(t *testing.T) {
	regions := []string{"iad3", strings.ToLower(InitDefaultRegion)}

	if PlaceholderDerivedHostname("auth.attacker.k8s.opencenter.cloud", "doritos", "auth.", regions) {
		t.Error("suffix lookalike classified as placeholder-derived")
	}
	if !PlaceholderDerivedHostname("auth.doritos.dfw3.k8s.opencenter.cloud", "doritos", "auth.", regions) {
		t.Error("init-baked hostname not classified as placeholder-derived")
	}
	if PlaceholderDerivedHostname("auth.doritos.iad3.k8s.opensnack.cloud", "doritos", "auth.", regions) {
		t.Error("already-repaired hostname classified as needing repair")
	}
}

func TestRepairPlaceholderHostnamesFollowsLateRegionChange(t *testing.T) {
	// init baked everything for DFW3; the cloud then reports IAD3.
	cfg := hostnameRepairTestConfig(t, "kit-kat", "DFW3", defaultBaseDomain)
	cfg.OpenCenter.Meta.Region = "IAD3"
	cfg.OpenCenter.Cluster.ClusterFQDN = cfg.OpenCenter.Cluster.DeriveClusterFQDN("IAD3")

	RepairPlaceholderHostnames(cfg)

	if got, want := keycloakHostname(t, cfg), "auth.kit-kat.iad3."+defaultBaseDomain; got != want {
		t.Errorf("keycloak hostname = %q, want %q after a late region change", got, want)
	}
}

func keycloakHostname(t *testing.T, cfg *Config) string {
	t.Helper()
	kc := typedServiceConfig[*services.KeycloakConfig](cfg, "keycloak")
	if kc == nil {
		t.Fatal("keycloak service config missing")
	}
	return kc.Hostname
}

// TestRepairPlaceholderHostnamesToleratesMissingServiceBlocks covers a config
// that declares no service blocks at all, plus one block stored as a typed nil
// pointer. Both were a nil dereference: a typed nil boxes into a non-nil
// interface, so the guard that skipped absent services let `get` dereference it.
func TestRepairPlaceholderHostnamesToleratesMissingServiceBlocks(t *testing.T) {
	cfg := &Config{}
	cfg.OpenCenter.Cluster.ClusterName = "bare"
	cfg.OpenCenter.Meta.Region = "IAD3"
	cfg.OpenCenter.Cluster.BaseDomain = "k8s.example.cloud"
	cfg.OpenCenter.Services = ServiceMap{
		"keycloak": (*services.KeycloakConfig)(nil),
	}

	if got := RepairPlaceholderHostnames(cfg); len(got) != 0 {
		t.Errorf("RepairPlaceholderHostnames() = %v, want no repairs for an empty config", got)
	}
	if got := RepairPlaceholderDNSZoneNames(cfg); len(got) != 0 {
		t.Errorf("RepairPlaceholderDNSZoneNames() = %v, want no repairs for an empty config", got)
	}
}
