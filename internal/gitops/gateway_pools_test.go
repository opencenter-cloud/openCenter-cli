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
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
)

// origMonolithicGatewayTemplate is the pre-OCTR-762 single Gateway template,
// captured verbatim. It is the golden for the byte-identical guard: whatever the
// refactored per-pool renderer emits for a single-pool cluster must match what
// this one template produced. If a listener block's YAML ever needs to change,
// this const changes with it and the guard keeps single-pool output honest.
const origMonolithicGatewayTemplate = `---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
{{- $gw := (index .OpenCenter.Services "gateway") }}
{{- if not $gw.IsBYO }}
  annotations:
    cert-manager.io/cluster-issuer: {{ $gw.DefaultIssuer | default "rackspace-ca" }}
{{- end }}
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "keycloak" "keycloak-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: keycloak-http
      hostname: {{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" fqdn) }}
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "gitops").Hostname | default (printf "gitops.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "gitops" "gitops-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: headlamp-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "headlamp").Hostname | default (printf "headlamp.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "headlamp" "headlamp-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: prometheus-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").PrometheusHostname | default (printf "prometheus.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "prometheus" "prometheus-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: alertmanager-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").AlertmanagerHostname | default (printf "alertmanager.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "alertmanager" "alertmanager-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: grafana-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").GrafanaHostname | default (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "grafana.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "grafana" "grafana-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
    - name: harbor-http
      protocol: HTTP
      port: 80
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
    - name: harbor-https
      protocol: HTTPS
      port: 443
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" fqdn) }}
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: {{ $gw.TLSSecretFor "harbor" "harbor-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
      allowedRoutes:
        namespaces:
          from: All
    - name: longhorn-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "longhorn").Hostname | default (printf "longhorn.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: {{ $gw.TLSSecretFor "longhorn" "longhorn-tls" }}
{{- if $gw.TLSSecretNamespace }}
            namespace: {{ $gw.TLSSecretNamespace }}
{{- end }}
`

// TestGatewaySinglePoolByteIdentical is the backward-compatibility guard: for a
// cluster with no per-service pool selection, the refactored renderer must emit
// gateway.yaml byte-for-byte identical to the original single template. This
// covers both the default (cert-manager) and BYO-TLS shapes.
func TestGatewaySinglePoolByteIdentical(t *testing.T) {
	cases := []struct {
		name string
		gw   *services.GatewayConfig
	}{
		{
			name: "default-cert-manager",
			gw:   &services.GatewayConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		},
		{
			name: "byo-wildcard-with-per-listener-override",
			gw: &services.GatewayConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
				TLS: &services.GatewayTLSConfig{
					WildcardSecretName: "wildcard-tls",
					SecretNamespace:    "rackspace-system",
					PerListenerSecrets: map[string]string{"harbor": "harbor-custom-tls"},
				},
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := v2.Config{OpenCenter: v2.OpenCenterConfig{
				Cluster:  v2.ClusterConfig{ClusterFQDN: "rackai-dev.rax.io"},
				Services: v2.ServiceMap{"gateway": tc.gw},
			}}
			want, err := renderOverlayTemplate(origMonolithicGatewayTemplate, cfg)
			require.NoError(t, err)
			got, err := renderGatewayResources(cfg)
			require.NoError(t, err)
			require.Equal(t, want, got, "single-pool gateway.yaml must stay byte-identical to the original template")
		})
	}
}

// TestGatewaySinglePoolNoEnvoyProxyPerPoolFiles confirms that a single-pool
// cluster emits none of the per-pool EnvoyProxy overlay files and that its
// kustomization lists exactly the original four resources.
func TestGatewaySinglePoolNoEnvoyProxyPerPoolFiles(t *testing.T) {
	cfg := v2.Config{OpenCenter: v2.OpenCenterConfig{
		Cluster:  v2.ClusterConfig{ClusterFQDN: "rackai-dev.rax.io"},
		Services: v2.ServiceMap{"gateway": &services.GatewayConfig{BaseConfig: services.BaseConfig{Enabled: true}}},
	}}
	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	for name := range files {
		require.False(t, strings.HasPrefix(name, "envoy-proxy-config-"),
			"single-pool cluster must not emit per-pool EnvoyProxy file %q", name)
	}
	kust, err := gatewayKustomizationRenderer(cfg)
	require.NoError(t, err)
	require.Equal(t, "---\n"+
		"apiVersion: kustomize.config.k8s.io/v1beta1\n"+
		"kind: Kustomization\n"+
		"resources:\n"+
		"  - \"namespace.yaml\"\n"+
		"  - \"gateway-class.yaml\"\n"+
		"  - \"gateway.yaml\"\n"+
		"  - \"envoy-proxy-config.yaml\"\n", kust)
}

// multiPoolConfig builds a cluster where harbor exits through a non-default pool
// and everything else stays on the default pool.
func multiPoolConfig() v2.Config {
	autoAssign := true
	return v2.Config{OpenCenter: v2.OpenCenterConfig{
		Cluster: v2.ClusterConfig{ClusterFQDN: "rackai-dev.rax.io"},
		Services: v2.ServiceMap{
			"gateway": &services.GatewayConfig{BaseConfig: services.BaseConfig{Enabled: true}},
			"metallb": &services.MetalLBConfig{
				BaseConfig: services.BaseConfig{Enabled: true},
				IPAddressPools: []services.IPAddressPool{
					{Name: "default-pool", Addresses: []string{"10.0.0.0/24"}, Default: true, AutoAssign: &autoAssign},
					{Name: "harbor-pool", Addresses: []string{"10.0.1.0/24"}, AutoAssign: &autoAssign},
				},
			},
			"harbor":                &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true, AddressPool: "harbor-pool"}},
			"longhorn":              &services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true}},
			"keycloak":              &services.KeycloakConfig{BaseConfig: services.BaseConfig{Enabled: true}},
			"kube-prometheus-stack": &services.PrometheusStackConfig{BaseConfig: services.BaseConfig{Enabled: true}},
		},
	}}
}

func TestGatewayNameForServiceRespectsPool(t *testing.T) {
	cfg := multiPoolConfig()
	require.Equal(t, "rmpk-gateway-harbor-pool", gatewayNameForService(cfg, "harbor"))
	require.Equal(t, "rmpk-gateway", gatewayNameForService(cfg, "longhorn"))
	require.Equal(t, "rmpk-gateway", gatewayNameForService(cfg, "keycloak"))
	require.Equal(t, "rmpk-gateway", gatewayNameForService(cfg, "kube-prometheus-stack"))
	// A service explicitly naming the default pool still maps to rmpk-gateway.
	require.Equal(t, "rmpk-gateway", gatewayNameForPool(cfg, "default-pool"))
}

// TestGatewayMultiPoolEmitsSecondGatewayAndEnvoyProxy verifies the multi-pool
// structure: a second Gateway named after the pool, carrying an
// infrastructure.parametersRef to a per-pool EnvoyProxy, whose LoadBalancer
// Service is annotated for MetalLB. The moved listeners leave the default gw.
func TestGatewayMultiPoolEmitsSecondGatewayAndEnvoyProxy(t *testing.T) {
	cfg := multiPoolConfig()
	files, err := gatewayOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	// A dedicated EnvoyProxy overlay file exists for the harbor pool.
	envoy, ok := files["envoy-proxy-config-harbor-pool.yaml"]
	require.True(t, ok, "expected per-pool EnvoyProxy file for harbor-pool")
	require.Contains(t, envoy, "name: custom-proxy-config-harbor-pool")
	require.Contains(t, envoy, "metallb.universe.tf/address-pool: harbor-pool")
	require.Contains(t, envoy, "envoyService:")

	// The kustomization lists the new EnvoyProxy file.
	kust, err := gatewayKustomizationRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, kust, "- \"envoy-proxy-config-harbor-pool.yaml\"")

	docs, err := decodeYAMLDocuments([]byte(files["gateway.yaml"]))
	require.NoError(t, err)

	gws := map[string]map[string]any{}
	for _, doc := range docs {
		if doc["kind"] == "Gateway" {
			gws[objectKey(doc)] = doc
		}
	}

	def, ok := gws["rackspace-system/rmpk-gateway"]
	require.True(t, ok, "default gateway must still exist")
	pool, ok := gws["rackspace-system/rmpk-gateway-harbor-pool"]
	require.True(t, ok, "per-pool gateway must exist")

	// harbor listeners moved off the default gateway onto the pool gateway.
	defListeners := listenerNameSet(def)
	require.NotContains(t, defListeners, "harbor-http")
	require.NotContains(t, defListeners, "harbor-https")
	require.Contains(t, defListeners, "keycloak-https")
	require.Contains(t, defListeners, "longhorn-https")

	poolListeners := listenerNameSet(pool)
	require.Contains(t, poolListeners, "harbor-http")
	require.Contains(t, poolListeners, "harbor-https")
	require.Len(t, poolListeners, 2, "pool gateway should carry only harbor listeners")

	// The pool gateway points at its per-pool EnvoyProxy via infrastructure.
	spec, _ := pool["spec"].(map[string]any)
	infra, _ := spec["infrastructure"].(map[string]any)
	require.NotNil(t, infra, "pool gateway must set spec.infrastructure")
	paramsRef, _ := infra["parametersRef"].(map[string]any)
	require.Equal(t, "custom-proxy-config-harbor-pool", paramsRef["name"])
	require.Equal(t, "EnvoyProxy", paramsRef["kind"])

	// The default gateway must NOT carry an infrastructure ref (byte-compat).
	defSpec, _ := def["spec"].(map[string]any)
	_, hasInfra := defSpec["infrastructure"]
	require.False(t, hasInfra, "default gateway must not set spec.infrastructure")
}

// TestMultiPoolHTTPRoutesResolveToCorrectGateway plans the full cluster with a
// moved service and confirms every generated HTTPRoute parentRef resolves to a
// real listener on the Gateway backing its pool, including the moved service.
func TestMultiPoolHTTPRoutesResolveToCorrectGateway(t *testing.T) {
	cfg := planMultiPoolConfig(t)
	actions, err := planClusterAppActions(cfg)
	require.NoError(t, err)

	listeners := map[string]map[string]bool{}
	var refs []httpRouteParentRef
	for _, action := range actions {
		docs, err := decodeYAMLDocuments([]byte(action.Content))
		if err != nil {
			continue
		}
		for _, doc := range docs {
			switch doc["kind"] {
			case "Gateway":
				key := objectKey(doc)
				if listeners[key] == nil {
					listeners[key] = map[string]bool{}
				}
				for _, name := range gatewayListenerNames(doc) {
					listeners[key][name] = true
				}
			case "HTTPRoute":
				refs = append(refs, httpRouteParentRefs(action.Output, doc)...)
			}
		}
	}

	require.NotEmpty(t, refs)
	for _, ref := range refs {
		known, ok := listeners[ref.gateway]
		require.Truef(t, ok, "%s: HTTPRoute %q references Gateway %q not generated (have %v)",
			ref.file, ref.route, ref.gateway, sortedKeys(listeners))
		require.Truef(t, known[ref.sectionName],
			"%s: HTTPRoute %q wants listener %q on Gateway %q; present: %v",
			ref.file, ref.route, ref.sectionName, ref.gateway, sortedSet(known))
	}

	// Confirm at least one route actually resolved to the monitoring pool
	// gateway, so the check is not vacuous for the moved service. (Harbor and
	// keycloak routes ship in .tpl base files rendered on the copy path, not via
	// planClusterAppActions, so we assert on the overlay-generated monitoring
	// routes here.)
	var sawPoolGateway bool
	for _, ref := range refs {
		if ref.gateway == "rackspace-system/rmpk-gateway-monitoring-pool" {
			sawPoolGateway = true
		}
	}
	require.True(t, sawPoolGateway, "expected a monitoring HTTPRoute to target the per-pool gateway")
}

// planMultiPoolConfig returns a full default config with all services enabled,
// metallb configured with two pools, and harbor pinned to the non-default pool.
func planMultiPoolConfig(t *testing.T) v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("k8s-multipool", "openstack")
	require.NoError(t, err)
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).S3Endpoint = "https://harbor-s3.example"

	for _, serviceCfg := range cfg.OpenCenter.Services {
		if base := extractBaseConfig(serviceCfg); base != nil {
			base.Enabled = true
		}
	}
	autoAssign := true
	metallb := cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig)
	metallb.Enabled = true
	metallb.IPAddressPools = []services.IPAddressPool{
		{Name: "default-pool", Addresses: []string{"10.0.0.0/24"}, Default: true, AutoAssign: &autoAssign},
		{Name: "harbor-pool", Addresses: []string{"10.0.1.0/24"}, AutoAssign: &autoAssign},
		{Name: "monitoring-pool", Addresses: []string{"10.0.2.0/24"}, AutoAssign: &autoAssign},
	}
	cfg.OpenCenter.Services["harbor"].(*services.HarborConfig).AddressPool = "harbor-pool"
	// kube-prometheus-stack owns CLI-generated overlay HTTPRoutes (prometheus /
	// alertmanager / grafana), so moving it to a non-default pool exercises the
	// overlay-const repoint end-to-end through planClusterAppActions.
	extractBaseConfig(cfg.OpenCenter.Services["kube-prometheus-stack"]).AddressPool = "monitoring-pool"
	return *cfg
}

func listenerNameSet(doc map[string]any) map[string]bool {
	out := map[string]bool{}
	for _, n := range gatewayListenerNames(doc) {
		out[n] = true
	}
	return out
}

// TestTplHTTPRoutesRepointToPoolGateway renders the full cluster to disk with
// harbor and keycloak pinned to non-default pools and confirms the base .tpl
// HTTPRoutes (which are rendered on the copy path, not via planClusterAppActions)
// have their parentRefs repointed to the per-pool Gateway. keycloak stays on the
// default pool to prove the helper leaves default-pool services untouched.
func TestTplHTTPRoutesRepointToPoolGateway(t *testing.T) {
	cfg := newDefault("tpl-repoint")
	dst := t.TempDir()
	cfg.OpenCenter.GitOps.Repository.LocalDir = dst

	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.Enabled = true
	harbor.S3Endpoint = "https://harbor-s3.example"
	harbor.AddressPool = "harbor-pool"

	autoAssign := true
	metallb := cfg.OpenCenter.Services["metallb"].(*services.MetalLBConfig)
	metallb.Enabled = true
	metallb.IPAddressPools = []services.IPAddressPool{
		{Name: "default-pool", Addresses: []string{"10.0.0.0/24"}, Default: true, AutoAssign: &autoAssign},
		{Name: "harbor-pool", Addresses: []string{"10.0.1.0/24"}, AutoAssign: &autoAssign},
	}

	require.NoError(t, RenderClusterApps(cfg))

	harborRoute := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(),
		"services", "harbor", "httproute.yaml"))
	require.Contains(t, harborRoute, "name: rmpk-gateway-harbor-pool",
		"harbor .tpl HTTPRoute must repoint to the per-pool gateway")
	require.NotContains(t, harborRoute, "name: rmpk-gateway\n",
		"harbor route must not still target the default gateway")

	keycloakRoute := mustReadFile(t, filepath.Join(dst, "applications", "overlays", cfg.ClusterName(),
		"services", "keycloak", "20-keycloak", "httproute.yaml"))
	require.Contains(t, keycloakRoute, "name: rmpk-gateway\n",
		"keycloak (default pool) .tpl HTTPRoute must stay on rmpk-gateway")
}
