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
	"fmt"
	"log/slog"
	"reflect"
	"sort"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// typedServiceConfig returns the typed config for a service, or the zero value
// when the service is absent or stored as a different type.
func typedServiceConfig[T any](cfg *Config, name string) T {
	var zero T
	if cfg == nil {
		return zero
	}
	svc, ok := cfg.OpenCenter.Services[name].(T)
	if !ok {
		return zero
	}
	return svc
}

// typedManagedServiceConfig returns the typed config for a managed service, or
// the zero value when it is absent or stored as a different type.
func typedManagedServiceConfig[T any](cfg *Config, name string) T {
	var zero T
	if cfg == nil {
		return zero
	}
	svc, ok := cfg.OpenCenter.ManagedServices[name].(T)
	if !ok {
		return zero
	}
	return svc
}

// ServiceHostnamePrefixes maps a hostname-bearing config field to the label
// `cluster init` bakes in front of the cluster FQDN. The field path is the
// dotted config path (the same spelling a dotted --opencenter... override
// uses); it doubles as the identity key for provenance tagging and as the
// repair log field, so it must stay stable.
//
// Every entry is a field whose built-in default is
// fmt.Sprintf("<prefix>%s", clusterFQDN) in defaults.go. The kube-prometheus-stack
// legacy `hostname` alias is deliberately absent: it is retained only as a
// Grafana fallback and is swept through grafana_hostname so the two cannot
// disagree.
var ServiceHostnamePrefixes = map[string]string{
	"opencenter.services.keycloak.hostname":                           "auth.",
	"opencenter.services.headlamp.hostname":                           "dashboard.",
	"opencenter.services.harbor.hostname":                             "harbor.",
	"opencenter.services.longhorn.hostname":                           "longhorn.",
	"opencenter.services.kube-prometheus-stack.prometheus_hostname":   "prometheus.",
	"opencenter.services.kube-prometheus-stack.alertmanager_hostname": "alertmanager.",
	"opencenter.services.kube-prometheus-stack.grafana_hostname":      "grafana.",
	"opencenter.managed_services.alert-proxy.http_route_fqdn":         "alerts.",
}

// DefaultBaseDomainPlaceholder is exported so the provider-binding path can
// recognise a hostname baked from the placeholder identity.
func DefaultBaseDomainPlaceholder() string { return defaultBaseDomain }

// PlaceholderDerivedHostname reports whether host is exactly the value
// `cluster init` baked for `prefix` under one of the cluster's known
// *placeholder* identities, as opposed to a name an operator chose.
//
// The candidate set is the built-in placeholder domain (defaultBaseDomain)
// crossed with every region the cluster could have been created under. It
// never includes the cluster's current identity: that is what the sweep is
// about to rewrite the hostname to, and treating the target as already-repaired
// would make the check circular.
//
// Matching is exact string equality, never a suffix match. A suffix match would
// classify auth.sso.internal.example.com as repairable whenever the placeholder
// domain happened to be a suffix of it, and the sweep would then destroy a
// deliberate choice.
func PlaceholderDerivedHostname(host, clusterName, prefix string, regions []string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	if host == "" {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(clusterName))
	for _, region := range regions {
		region = strings.ToLower(strings.TrimSpace(region))
		for _, base := range []string{defaultBaseDomain} {
			candidates := []string{
				fmt.Sprintf("%s%s.%s.%s", prefix, name, region, base),
				fmt.Sprintf("%s%s.%s", prefix, name, base),
			}
			for _, candidate := range candidates {
				if host == candidate {
					return true
				}
			}
		}
	}
	return false
}

// hostnameSweepRegions returns every region value a hostname could have been
// baked from: the cluster's current region, the CLI init default, and the
// provider-binding default. `cluster init` bakes hostnames from
// NewV2DefaultWithRegion's region argument, which is the CLI default when
// --region is not passed; a later --os-cloud binding then rewrites meta.region
// without re-baking. Both guesses have to be recognised as replaceable.
func hostnameSweepRegions(cfg *Config) []string {
	regions := make([]string, 0, 3)
	appendRegion := func(region string) {
		region = strings.ToLower(strings.TrimSpace(region))
		if region == "" {
			return
		}
		for _, seen := range regions {
			if seen == region {
				return
			}
		}
		regions = append(regions, region)
	}
	appendRegion(cfg.OpenCenter.Meta.Region)
	appendRegion(InitDefaultRegion)
	return regions
}

// getHostnameField reads the string value at a ServiceHostnamePrefixes field
// path from the config struct.
func getHostnameField(cfg *Config, field string) (string, bool) {
	switch field {
	case "opencenter.services.keycloak.hostname":
		return fieldOf(typedServiceConfig[*services.KeycloakConfig](cfg, "keycloak"), func(c *services.KeycloakConfig) string { return c.Hostname }), true
	case "opencenter.services.headlamp.hostname":
		return fieldOf(typedServiceConfig[*services.HeadlampConfig](cfg, "headlamp"), func(c *services.HeadlampConfig) string { return c.Hostname }), true
	case "opencenter.services.harbor.hostname":
		return fieldOf(typedServiceConfig[*services.HarborConfig](cfg, "harbor"), func(c *services.HarborConfig) string { return c.Hostname }), true
	case "opencenter.services.longhorn.hostname":
		return fieldOf(typedServiceConfig[*services.LonghornConfig](cfg, "longhorn"), func(c *services.LonghornConfig) string { return c.Hostname }), true
	case "opencenter.services.kube-prometheus-stack.prometheus_hostname":
		return fieldOf(typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"), func(c *services.PrometheusStackConfig) string { return c.PrometheusHostname }), true
	case "opencenter.services.kube-prometheus-stack.alertmanager_hostname":
		return fieldOf(typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"), func(c *services.PrometheusStackConfig) string { return c.AlertmanagerHostname }), true
	case "opencenter.services.kube-prometheus-stack.grafana_hostname":
		return fieldOf(typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"), func(c *services.PrometheusStackConfig) string { return c.GrafanaHostname }), true
	case "opencenter.managed_services.alert-proxy.http_route_fqdn":
		return fieldOf(typedManagedServiceConfig[*services.AlertProxyConfig](cfg, "alert-proxy"), func(c *services.AlertProxyConfig) string { return c.HTTPRouteFQDN }), true
	}
	return "", false
}

// fieldOf reads one string off a possibly-nil typed service config.
//
// The nil test goes through reflect rather than `any(cfg) == nil`: a typed nil
// pointer (*services.KeycloakConfig)(nil) boxes into a NON-nil interface, so the
// naive comparison passes and `get` then dereferences nil. A config that omits a
// service block entirely is exactly that case.
func fieldOf[T any](cfg T, get func(T) string) string {
	value := reflect.ValueOf(cfg)
	if !value.IsValid() || (value.Kind() == reflect.Ptr && value.IsNil()) {
		return ""
	}
	return get(cfg)
}

// setHostnameField writes the string value at a ServiceHostnamePrefixes field
// path. The kube-prometheus-stack legacy `hostname` alias is kept in lockstep
// with grafana_hostname so the two spellings can never render different names.
func setHostnameField(cfg *Config, field, value string) {
	switch field {
	case "opencenter.services.keycloak.hostname":
		if c := typedServiceConfig[*services.KeycloakConfig](cfg, "keycloak"); c != nil {
			c.Hostname = value
		}
	case "opencenter.services.headlamp.hostname":
		if c := typedServiceConfig[*services.HeadlampConfig](cfg, "headlamp"); c != nil {
			c.Hostname = value
		}
	case "opencenter.services.harbor.hostname":
		if c := typedServiceConfig[*services.HarborConfig](cfg, "harbor"); c != nil {
			c.Hostname = value
		}
	case "opencenter.services.longhorn.hostname":
		if c := typedServiceConfig[*services.LonghornConfig](cfg, "longhorn"); c != nil {
			c.Hostname = value
		}
	case "opencenter.services.kube-prometheus-stack.prometheus_hostname":
		if c := typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"); c != nil {
			c.PrometheusHostname = value
		}
	case "opencenter.services.kube-prometheus-stack.alertmanager_hostname":
		if c := typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"); c != nil {
			c.AlertmanagerHostname = value
		}
	case "opencenter.services.kube-prometheus-stack.grafana_hostname":
		if c := typedServiceConfig[*services.PrometheusStackConfig](cfg, "kube-prometheus-stack"); c != nil {
			c.GrafanaHostname = value
			if c.Hostname != "" {
				c.Hostname = value
			}
		}
	case "opencenter.managed_services.alert-proxy.http_route_fqdn":
		if c := typedManagedServiceConfig[*services.AlertProxyConfig](cfg, "alert-proxy"); c != nil {
			c.HTTPRouteFQDN = value
		}
	}
}

// RepairPlaceholderHostnames rewrites every service hostname that still equals
// the value `cluster init` baked from a placeholder identity, so hostnames
// follow base_domain/region the way cluster_fqdn already does.
//
// This is the propagation step `cluster init --base-domain` and the
// provider-binding region rewrite both omit: those paths re-derive
// cluster_fqdn but leave the hostname scalars, which were materialized as
// literal YAML from the pre-override FQDN, frozen at the old domain. Every
// render template resolves a hostname as `<service>.hostname | default
// (printf "auth.%s" fqdn)`, so a stale scalar wins over the derived FQDN and
// one Gateway ends up with listeners in two different DNS domains.
//
// The gate is the whole design: only placeholder-derived values are touched, so
// a hostname an operator set to an unrelated domain survives. Hostnames that
// are already correct, or that are empty (so the fqdn default applies), are
// left alone.
//
// Repairs are logged rather than silent: this mutates TLS DNS names on a read
// path, and an unlogged rename is undiagnosable. The returned field paths are
// what callers surface to the operator.
func RepairPlaceholderHostnames(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	fqdn := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Cluster.ClusterFQDN))
	if fqdn == "" {
		return nil
	}
	clusterName := cfg.ClusterName()
	regions := hostnameSweepRegions(cfg)

	fields := make([]string, 0, len(ServiceHostnamePrefixes))
	for _, field := range sortedHostnameFields() {
		prefix := ServiceHostnamePrefixes[field]
		current, ok := getHostnameField(cfg, field)
		if !ok {
			continue
		}
		current = strings.ToLower(strings.TrimSpace(current))
		if current == "" {
			continue
		}
		want := prefix + fqdn
		if current == want {
			continue
		}
		if !PlaceholderDerivedHostname(current, clusterName, prefix, regions) {
			continue
		}
		setHostnameField(cfg, field, want)
		fields = append(fields, field)
		slog.Info("repaired placeholder-derived service hostname",
			"field", field,
			"cluster", clusterName,
			"from", current,
			"to", want,
		)
	}
	return fields
}

// sortedHostnameFields returns the sweep field paths in a stable order so the
// repair log and the returned slice do not depend on map iteration order.
func sortedHostnameFields() []string {
	fields := make([]string, 0, len(ServiceHostnamePrefixes))
	for field := range ServiceHostnamePrefixes {
		fields = append(fields, field)
	}
	sort.Strings(fields)
	return fields
}

// RepairPlaceholderDNSZoneNames heals the dns_zone_name fields, which are
// region-and-domain derived exactly like the service hostnames and are baked by
// defaults.go from the same pre-override clusterFQDN.
//
// This one matters more than a cosmetic rename: dns_zone_name is the Designate
// zone the OpenTofu module creates and points the load-balancer A record at.
// When it names a stale domain, every hostname the Gateway serves resolves
// somewhere else, and the cluster deploys "successfully" with no reachable
// endpoint.
func RepairPlaceholderDNSZoneNames(cfg *Config) []string {
	if cfg == nil {
		return nil
	}
	fqdn := strings.ToLower(strings.TrimSpace(cfg.OpenCenter.Cluster.ClusterFQDN))
	if fqdn == "" {
		return nil
	}
	clusterName := strings.ToLower(strings.TrimSpace(cfg.ClusterName()))

	// A zone is placeholder-derived when it is <cluster>.<region>.<placeholder>
	// for one of the regions init could have guessed.
	isPlaceholder := func(zone string) bool {
		zone = strings.ToLower(strings.TrimSpace(zone))
		if zone == "" {
			return false
		}
		for _, region := range hostnameSweepRegions(cfg) {
			if zone == clusterName+"."+region+"."+defaultBaseDomain ||
				zone == clusterName+"."+defaultBaseDomain {
				return true
			}
		}
		return false
	}

	var repaired []string
	repair := func(field string, current *string) {
		if !isPlaceholder(*current) || strings.ToLower(strings.TrimSpace(*current)) == fqdn {
			return
		}
		slog.Info("repaired placeholder-derived DNS zone",
			"field", field, "cluster", cfg.ClusterName(), "from", *current, "to", fqdn)
		*current = fqdn
		repaired = append(repaired, field)
	}

	if networking := &cfg.OpenCenter.Infrastructure.Networking; networking != nil {
		repair("opencenter.infrastructure.networking.dns_zone_name", &networking.DNSZoneName)
	}
	if openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack; openstack != nil {
		repair("opencenter.infrastructure.cloud.openstack.dns_zone_name", &openstack.DNSZoneName)
		if openstack.Networking != nil {
			repair("opencenter.infrastructure.cloud.openstack.networking.designate.dns_zone_name",
				&openstack.Networking.Designate.DNSZoneName)
		}
	}
	return repaired
}
