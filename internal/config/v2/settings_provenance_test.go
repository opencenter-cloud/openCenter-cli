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
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// withSettingsBaseDomain points load-time hydration at a fixture settings file
// carrying the given cluster_defaults.base_domain for the duration of fn.
func withSettingsBaseDomain(t *testing.T, baseDomain string, fn func()) {
	t.Helper()
	dir := t.TempDir()
	settings := "cluster_defaults:\n  base_domain: " + baseDomain + "\n"
	if err := os.WriteFile(filepath.Join(dir, "settings.yaml"), []byte(settings), 0o600); err != nil {
		t.Fatalf("write settings fixture: %v", err)
	}
	original := settingsBaseDomainPath
	settingsBaseDomainPath = func() (string, error) {
		return filepath.Join(dir, "settings.yaml"), nil
	}
	defer func() { settingsBaseDomainPath = original }()
	fn()
}

func provenanceTestConfig() *Config {
	return &Config{
		OpenCenter: OpenCenterConfig{
			Meta: MetaConfig{Name: "acme", Region: "sjc3"},
			Cluster: ClusterConfig{
				ClusterName: "acme",
				BaseDomain:  defaultBaseDomain,
				ClusterFQDN: "acme.sjc3." + defaultBaseDomain,
			},
		},
	}
}

func TestApplySettingsProvenanceAdoptsPlaceholderIntoSettingsDomain(t *testing.T) {
	withSettingsBaseDomain(t, "k8s.example.com", func() {
		cfg := provenanceTestConfig()
		applySettingsProvenance(cfg)

		if cfg.OpenCenter.Cluster.BaseDomain != "k8s.example.com" {
			t.Fatalf("base_domain = %q, want adoption of settings value", cfg.OpenCenter.Cluster.BaseDomain)
		}
		if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.k8s.example.com" {
			t.Fatalf("cluster_fqdn = %q, want re-derived FQDN", cfg.OpenCenter.Cluster.ClusterFQDN)
		}
		if !cfg.OpenCenter.Cluster.Managed.BaseDomainManaged() {
			t.Fatal("base_domain should be tagged managed after placeholder adoption")
		}
	})
}

func TestApplySettingsProvenanceManagedFollowsSettingsChange(t *testing.T) {
	withSettingsBaseDomain(t, "new.example.org", func() {
		cfg := provenanceTestConfig()
		cfg.OpenCenter.Cluster.BaseDomain = "old.example.org"
		cfg.OpenCenter.Cluster.ClusterFQDN = "acme.sjc3.old.example.org"
		cfg.OpenCenter.Cluster.Managed.DefaultBaseDomain()

		applySettingsProvenance(cfg)

		if cfg.OpenCenter.Cluster.BaseDomain != "new.example.org" {
			t.Fatalf("base_domain = %q, want propagated settings value", cfg.OpenCenter.Cluster.BaseDomain)
		}
		if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.new.example.org" {
			t.Fatalf("cluster_fqdn = %q, want re-derived FQDN", cfg.OpenCenter.Cluster.ClusterFQDN)
		}
	})
}

func TestApplySettingsProvenancePinnedIsNeverOverwritten(t *testing.T) {
	withSettingsBaseDomain(t, "settings.example.com", func() {
		cfg := provenanceTestConfig()
		cfg.OpenCenter.Cluster.BaseDomain = "custom.example.net"
		cfg.OpenCenter.Cluster.ClusterFQDN = "acme.sjc3.custom.example.net"
		cfg.OpenCenter.Cluster.Managed.PinBaseDomain()

		applySettingsProvenance(cfg)

		if cfg.OpenCenter.Cluster.BaseDomain != "custom.example.net" {
			t.Fatalf("pinned base_domain = %q, must not change", cfg.OpenCenter.Cluster.BaseDomain)
		}
		if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.custom.example.net" {
			t.Fatalf("pinned cluster_fqdn = %q, must not change", cfg.OpenCenter.Cluster.ClusterFQDN)
		}
	})
}

func TestApplySettingsProvenancePlaceholderWithEditedFQDNSelfPins(t *testing.T) {
	withSettingsBaseDomain(t, "settings.example.com", func() {
		cfg := provenanceTestConfig()
		// Placeholder domain but a hand-edited FQDN: the cluster deliberately
		// owns its identity, so hydration pins instead of overwriting.
		cfg.OpenCenter.Cluster.ClusterFQDN = "acme.internal.example.net"

		applySettingsProvenance(cfg)

		if cfg.OpenCenter.Cluster.BaseDomain != defaultBaseDomain {
			t.Fatalf("base_domain = %q, must keep the edited cluster's value", cfg.OpenCenter.Cluster.BaseDomain)
		}
		if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.internal.example.net" {
			t.Fatalf("cluster_fqdn = %q, must keep the hand-edited value", cfg.OpenCenter.Cluster.ClusterFQDN)
		}
		if cfg.OpenCenter.Cluster.Managed.BaseDomainManaged() {
			t.Fatal("self-pin failed: base_domain must be pinned after an FQDN edit")
		}
	})
}

func TestApplySettingsProvenanceNoSettingsValueIsNoOp(t *testing.T) {
	dir := t.TempDir()
	original := settingsBaseDomainPath
	settingsBaseDomainPath = func() (string, error) {
		return filepath.Join(dir, "missing-settings.yaml"), nil
	}
	defer func() { settingsBaseDomainPath = original }()

	cfg := provenanceTestConfig()
	applySettingsProvenance(cfg)

	if cfg.OpenCenter.Cluster.BaseDomain != defaultBaseDomain {
		t.Fatalf("base_domain = %q, want unchanged", cfg.OpenCenter.Cluster.BaseDomain)
	}
	if cfg.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("without a settings value the field must stay pinned")
	}
}

func TestApplySettingsProvenanceDerivesEmptyFQDN(t *testing.T) {
	dir := t.TempDir()
	original := settingsBaseDomainPath
	settingsBaseDomainPath = func() (string, error) {
		return filepath.Join(dir, "missing-settings.yaml"), nil
	}
	defer func() { settingsBaseDomainPath = original }()

	cfg := provenanceTestConfig()
	cfg.OpenCenter.Cluster.BaseDomain = "acme.internal"
	cfg.OpenCenter.Cluster.ClusterFQDN = ""
	cfg.OpenCenter.Cluster.Managed.PinBaseDomain()

	applySettingsProvenance(cfg)

	if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.acme.internal" {
		t.Fatalf("cluster_fqdn = %q, want derived value", cfg.OpenCenter.Cluster.ClusterFQDN)
	}
}

func TestApplySettingsProvenanceManagedHealsDriftedFQDN(t *testing.T) {
	// A managed cluster whose stored cluster_fqdn drifted from its
	// base_domain (hand edit, older CLI) is re-derived at load even when the
	// domain already matches settings: the identity fields cannot disagree.
	withSettingsBaseDomain(t, "k8s.example.com", func() {
		cfg := provenanceTestConfig()
		cfg.OpenCenter.Cluster.BaseDomain = "k8s.example.com"
		cfg.OpenCenter.Cluster.ClusterFQDN = "stale.example.net"
		cfg.OpenCenter.Cluster.Managed.DefaultBaseDomain()

		applySettingsProvenance(cfg)

		if cfg.OpenCenter.Cluster.BaseDomain != "k8s.example.com" {
			t.Fatalf("base_domain = %q, want unchanged", cfg.OpenCenter.Cluster.BaseDomain)
		}
		if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.k8s.example.com" {
			t.Fatalf("cluster_fqdn = %q, want healed derivation", cfg.OpenCenter.Cluster.ClusterFQDN)
		}
	})
}

func TestSyncBaseDomainFromSettingsRewritesDerivedFQDN(t *testing.T) {
	cfg := provenanceTestConfig()
	cfg.OpenCenter.Cluster.BaseDomain = "old.example.org"
	cfg.OpenCenter.Cluster.ClusterFQDN = "acme.sjc3.old.example.org"
	cfg.OpenCenter.Cluster.Managed.PinBaseDomain()

	if !SyncBaseDomainFromSettings(cfg, "NEW.example.com ") {
		t.Fatal("sync reported no change")
	}
	if cfg.OpenCenter.Cluster.BaseDomain != "new.example.com" {
		t.Fatalf("base_domain = %q, want normalized settings value", cfg.OpenCenter.Cluster.BaseDomain)
	}
	if cfg.OpenCenter.Cluster.ClusterFQDN != "acme.sjc3.new.example.com" {
		t.Fatalf("cluster_fqdn = %q, want re-derived FQDN", cfg.OpenCenter.Cluster.ClusterFQDN)
	}
	if cfg.OpenCenter.Cluster.Managed.BaseDomainManaged() {
		t.Fatal("a one-off sync must not hand ownership back to settings")
	}

	if SyncBaseDomainFromSettings(cfg, "new.example.com") {
		t.Fatal("re-sync of an identical domain must report no change")
	}
	if SyncBaseDomainFromSettings(cfg, "") {
		t.Fatal("empty settings domain must report no change")
	}
}

func TestDeriveClusterFQDN(t *testing.T) {
	cluster := ClusterConfig{ClusterName: "Acme", BaseDomain: "K8S.Example.COM"}
	if got := cluster.DeriveClusterFQDN("SJC3"); got != "acme.sjc3.k8s.example.com" {
		t.Fatalf("DeriveClusterFQDN = %q", got)
	}
	if got := cluster.DeriveClusterFQDN(""); got != "acme.k8s.example.com" {
		t.Fatalf("DeriveClusterFQDN without region = %q", got)
	}
	if got := (ClusterConfig{BaseDomain: "x.com"}).DeriveClusterFQDN("r"); got != "" {
		t.Fatalf("DeriveClusterFQDN without name = %q, want empty", got)
	}
}

func TestDecodePublicConfigAcceptsProvenanceFields(t *testing.T) {
	data := []byte(`---
cluster:
  cluster_name: acme
  base_domain: example.com
  managed:
    base_domain: default
  last_generated_base_domain: old.example.com
`)
	var probe struct {
		Cluster ClusterConfig `yaml:"cluster"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&probe); err != nil {
		t.Fatalf("strict decode of provenance fields: %v", err)
	}
	if probe.Cluster.Managed.BaseDomain != ProvenanceDefault {
		t.Fatalf("managed.base_domain = %q", probe.Cluster.Managed.BaseDomain)
	}
	if probe.Cluster.LastGeneratedBaseDomain != "old.example.com" {
		t.Fatalf("last_generated_base_domain = %q", probe.Cluster.LastGeneratedBaseDomain)
	}
}
