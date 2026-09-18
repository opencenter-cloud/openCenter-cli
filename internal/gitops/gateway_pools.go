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

package gitops

import (
	"sort"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// OCTR-762: per-service MetalLB pool selection.
//
// Each user-facing service that owns a CLI-generated Gateway listener may
// declare which MetalLB address pool its traffic exits through (address_pool).
// The generator groups those services by resolved pool and emits one Gateway
// (and, for non-default pools, one EnvoyProxy carrying the MetalLB pool
// annotation) per pool actually in use.
//
// Backward compatibility is the hard constraint: when every service resolves to
// the same (default) pool - the situation for every cluster today - the output
// must be byte-identical to the single hardcoded rmpk-gateway that shipped
// before this change. Only when services genuinely span more than one pool do
// we split into multiple Gateways.

const (
	// defaultGatewayName is the Gateway that the default pool always uses. It
	// must stay "rmpk-gateway" because upstream-base HTTPRoutes (headlamp,
	// gitops) hardcode this parentRef and the CLI cannot repoint them.
	defaultGatewayName = "rmpk-gateway"
	// gatewayNamespace is the namespace all Gateways live in.
	gatewayNamespace = "rackspace-system"
	// defaultEnvoyProxyName is the GatewayClass-level EnvoyProxy, unchanged.
	defaultEnvoyProxyName = "custom-proxy-config"
	// metallbAddressPoolAnnotation selects a MetalLB pool for a LoadBalancer.
	metallbAddressPoolAnnotation = "metallb.universe.tf/address-pool"
)

// poolListenerService names a service whose CLI-generated Gateway listener(s)
// can move between pools, plus the listener section names it owns. Services
// whose HTTPRoutes ship in the upstream base (headlamp, gitops) are absent on
// purpose: they always stay on the default Gateway.
type poolListenerService struct {
	// service is the key under opencenter.services that carries address_pool.
	service string
	// listeners are the Gateway listener section names this service owns.
	listeners []string
}

// movableListenerServices is the fixed set of services whose listeners the CLI
// generates and can therefore assign to a chosen pool. Order here is not
// significant; listener emission order is controlled by gatewayListenerOrder.
var movableListenerServices = []poolListenerService{
	{service: "keycloak", listeners: []string{"keycloak-https", "keycloak-http"}},
	{service: "harbor", listeners: []string{"harbor-http", "harbor-https"}},
	{service: "longhorn", listeners: []string{"longhorn-https"}},
	{service: "kube-prometheus-stack", listeners: []string{"prometheus-https", "alertmanager-https", "grafana-https"}},
}

// pinnedDefaultListeners are listeners whose HTTPRoutes live in the upstream
// base. Their listeners always stay on the default Gateway so those routes
// resolve. They are emitted on the default Gateway regardless of any pool.
var pinnedDefaultListeners = []string{"gitops-https", "headlamp-https"}

// gatewayPoolGroup is one Gateway's worth of listeners bound to a single pool.
type gatewayPoolGroup struct {
	// pool is the resolved MetalLB pool name ("" when metallb is not driving
	// pool selection at all).
	pool string
	// gatewayName is the Gateway resource name for this group.
	gatewayName string
	// isDefault marks the default-pool group (Gateway rmpk-gateway). The default
	// group never gets an infrastructure.parametersRef or MetalLB annotation, so
	// the single-pool case stays byte-identical to today.
	isDefault bool
	// envoyProxyName is the per-Gateway EnvoyProxy name for non-default groups;
	// empty for the default group.
	envoyProxyName string
	// listeners are the listener section names on this Gateway, already ordered.
	listeners []string
}

// metallbConfig returns the typed MetalLB config when the service is enabled,
// else nil. Mirrors the extraction the validator performs.
func metallbConfig(cfg v2.Config) *services.MetalLBConfig {
	svc, ok := cfg.OpenCenter.Services["metallb"]
	if !ok {
		return nil
	}
	if IsServiceDisabled(svc) {
		return nil
	}
	mlb, ok := svc.(*services.MetalLBConfig)
	if !ok {
		return nil
	}
	return mlb
}

// defaultPoolName returns the cluster's default MetalLB pool name, or "" when
// metallb is not enabled or defines no pools (today's single-Gateway case).
func defaultPoolName(cfg v2.Config) string {
	mlb := metallbConfig(cfg)
	if mlb == nil {
		return ""
	}
	return mlb.DefaultPoolName()
}

// resolveServicePool returns the pool a service's listeners exit through: its
// declared address_pool when set, otherwise the cluster default pool.
func resolveServicePool(cfg v2.Config, serviceName string) string {
	if svc, ok := cfg.OpenCenter.Services[serviceName]; ok {
		if pooler, ok := svc.(interface{ GetAddressPool() string }); ok {
			if p := pooler.GetAddressPool(); p != "" {
				return p
			}
		}
	}
	return defaultPoolName(cfg)
}

// gatewayNameForPool maps a resolved pool to its Gateway resource name. The
// default pool (and the "" no-metallb case) always maps to rmpk-gateway;
// non-default pools map to rmpk-gateway-<pool>.
func gatewayNameForPool(cfg v2.Config, pool string) string {
	if pool == "" || pool == defaultPoolName(cfg) {
		return defaultGatewayName
	}
	return defaultGatewayName + "-" + pool
}

// gatewayNameForService returns the Gateway resource name that a service's
// HTTPRoutes must target. Services with no movable listener (or pinned to the
// base) resolve to the default Gateway.
func gatewayNameForService(cfg v2.Config, serviceName string) string {
	return gatewayNameForPool(cfg, resolveServicePool(cfg, serviceName))
}

// gatewayListenerOrder is the canonical listener emission order. It matches the
// order listeners appeared in the original single flat Gateway so that the
// default-pool Gateway stays byte-identical.
var gatewayListenerOrder = []string{
	"keycloak-https",
	"keycloak-http",
	"gitops-https",
	"headlamp-https",
	"prometheus-https",
	"alertmanager-https",
	"grafana-https",
	"harbor-http",
	"harbor-https",
	"longhorn-https",
}

// gatewayPoolGroups computes the ordered set of Gateways to emit. The default
// group is always first and always present; non-default groups follow in
// deterministic (pool-name-sorted) order, one per pool actually used by at
// least one movable service.
func gatewayPoolGroups(cfg v2.Config) []gatewayPoolGroup {
	defPool := defaultPoolName(cfg)

	// Assign each movable listener to its service's resolved pool.
	listenerPool := map[string]string{}
	for _, m := range movableListenerServices {
		pool := resolveServicePool(cfg, m.service)
		for _, l := range m.listeners {
			listenerPool[l] = pool
		}
	}
	// Pinned listeners always live on the default Gateway.
	for _, l := range pinnedDefaultListeners {
		listenerPool[l] = defPool
	}

	// Collect listeners per pool, preserving canonical order.
	poolListeners := map[string][]string{}
	for _, l := range gatewayListenerOrder {
		pool, ok := listenerPool[l]
		if !ok {
			continue
		}
		poolListeners[pool] = append(poolListeners[pool], l)
	}

	// Default group: every listener whose pool is the default (or "").
	groups := []gatewayPoolGroup{{
		pool:        defPool,
		gatewayName: defaultGatewayName,
		isDefault:   true,
		listeners:   poolListeners[defPool],
	}}

	// Non-default groups, sorted by pool name for determinism.
	var nonDefault []string
	for pool := range poolListeners {
		if pool == defPool || pool == "" {
			continue
		}
		nonDefault = append(nonDefault, pool)
	}
	sort.Strings(nonDefault)
	for _, pool := range nonDefault {
		groups = append(groups, gatewayPoolGroup{
			pool:           pool,
			gatewayName:    gatewayNameForPool(cfg, pool),
			isDefault:      false,
			envoyProxyName: defaultEnvoyProxyName + "-" + pool,
			listeners:      poolListeners[pool],
		})
	}
	return groups
}

// isMultiPool reports whether services span more than one pool, i.e. at least
// one non-default Gateway must be emitted. When false the output stays
// byte-identical to the pre-OCTR-762 single Gateway.
func isMultiPool(cfg v2.Config) bool {
	return len(gatewayPoolGroups(cfg)) > 1
}
