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
	"os"
	"path/filepath"
	"strings"

	configpersistence "github.com/opencenter-cloud/opencenter-cli/internal/config/persistence"
	"gopkg.in/yaml.v3"
)

// DefaultBaseDomain is the built-in placeholder base domain baked into new
// cluster configs by `cluster init`. A cluster config still holding this value
// has never been given a real domain and is therefore eligible to adopt the
// settings-file default.
func DefaultBaseDomain() string { return defaultBaseDomain }

// settingsClusterDefaults is the minimal projection of the CLI settings file
// (<config-dir>/settings.yaml) needed by load-time hydration. It is read
// directly (no ConfigManager) so the v2 loader keeps its single dependency on
// the filesystem abstraction.
type settingsClusterDefaults struct {
	ClusterDefaults struct {
		BaseDomain string `yaml:"base_domain"`
	} `yaml:"cluster_defaults"`
}

// settingsBaseDomainPath resolves the CLI settings file path. It is a variable
// so tests can point hydration at a fixture settings file.
var settingsBaseDomainPath = defaultSettingsYAMLPath

func defaultSettingsYAMLPath() (string, error) {
	configDir, err := configpersistence.ResolveConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "settings.yaml"), nil
}

// ReadSettingsBaseDomain returns the settings-file cluster_defaults.base_domain.
// A missing file, unreadable file, or empty value yields "" (no opinion): the
// settings file simply has no base domain to propagate.
func ReadSettingsBaseDomain() string {
	path, err := settingsBaseDomainPath()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	var settings settingsClusterDefaults
	if err := yaml.Unmarshal(data, &settings); err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(settings.ClusterDefaults.BaseDomain))
}

// applySettingsProvenance is loader stage 4b: settings-file hydration for
// provenance-tagged identity fields.
//
//   - base_domain tagged managed.base_domain: default → the settings file owns
//     the value; every load re-applies cluster_defaults.base_domain and
//     unconditionally re-derives cluster_fqdn (healing any stored FQDN that
//     drifted), so a settings change propagates through
//     generate/validate/deploy without rewriting the cluster config file.
//   - legacy configs (no provenance) whose base_domain is still the init
//     placeholder adopt the settings domain and are tagged default, provided
//     the stored FQDN still matches the placeholder derivation. A stored FQDN
//     that differs proves a deliberate cluster-side edit; the field pins
//     itself instead of being overwritten.
//   - everything else is pinned: the cluster config is the authority.
//
// cluster_fqdn is then re-derived whenever it is empty, or whenever this stage
// just moved base_domain, so the FQDN and every hostname derived from it
// (cert-manager DNS names, gateway hosts, terraform vars) always agree with
// base_domain.
func applySettingsProvenance(cfg *Config) {
	cluster := &cfg.OpenCenter.Cluster
	settingsDomain := ReadSettingsBaseDomain()
	region := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Region))

	switch {
	case cluster.Managed.BaseDomainManaged():
		if settingsDomain != "" && settingsDomain != cluster.BaseDomain {
			cluster.BaseDomain = settingsDomain
		}
		// A managed field always re-derives the FQDN, even when the domain
		// already matches settings: the in-memory derivation is free, and it
		// heals a cluster_fqdn that drifted from its base_domain (hand edit,
		// older CLI) instead of leaving the identity fields disagreeing.
		cluster.ClusterFQDN = cluster.DeriveClusterFQDN(region)
	case cluster.BaseDomain == "" || strings.EqualFold(strings.TrimSpace(cluster.BaseDomain), defaultBaseDomain):
		if settingsDomain != "" && settingsDomain != defaultBaseDomain && fqdnStillPlaceholder(cluster, region) {
			cluster.BaseDomain = settingsDomain
			cluster.Managed.DefaultBaseDomain()
			cluster.ClusterFQDN = cluster.DeriveClusterFQDN(region)
		} else {
			cluster.Managed.PinBaseDomain()
		}
	default:
		cluster.Managed.PinBaseDomain()
	}

	if strings.TrimSpace(cluster.ClusterFQDN) == "" {
		cluster.ClusterFQDN = cluster.DeriveClusterFQDN(region)
	}
}

// fqdnStillPlaceholder reports whether the stored cluster_fqdn is still the
// value `cluster init` derived from the placeholder base domain (or is empty).
// Any other value proves the cluster deliberately owns its identity.
func fqdnStillPlaceholder(cluster *ClusterConfig, region string) bool {
	stored := strings.ToLower(strings.TrimSpace(cluster.ClusterFQDN))
	if stored == "" {
		return true
	}
	placeholder := ClusterConfig{ClusterName: cluster.ClusterName, BaseDomain: defaultBaseDomain}
	return stored == placeholder.DeriveClusterFQDN(region)
}

// SyncBaseDomainFromSettings applies the settings-file
// cluster_defaults.base_domain to a cluster config and re-derives the FQDN.
// It is the explicit remediation behind `cluster generate --sync-base-domain`
// for clusters where base_domain is pinned (settings changes never touch a
// pinned field without the user asking for it). The cluster keeps its pinned
// tag: syncing once is not the same as handing ownership back to settings.
func SyncBaseDomainFromSettings(cfg *Config, settingsBaseDomain string) (changed bool) {
	settingsBaseDomain = strings.ToLower(strings.TrimSpace(settingsBaseDomain))
	if settingsBaseDomain == "" {
		return false
	}
	region := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Meta.Region))
	if strings.EqualFold(strings.TrimSpace(cfg.OpenCenter.Cluster.BaseDomain), settingsBaseDomain) {
		derived := cfg.OpenCenter.Cluster.DeriveClusterFQDN(region)
		if derived != "" && derived != cfg.OpenCenter.Cluster.ClusterFQDN {
			cfg.OpenCenter.Cluster.ClusterFQDN = derived
			return true
		}
		return false
	}
	cfg.OpenCenter.Cluster.BaseDomain = settingsBaseDomain
	cfg.OpenCenter.Cluster.ClusterFQDN = cfg.OpenCenter.Cluster.DeriveClusterFQDN(region)
	return true
}
