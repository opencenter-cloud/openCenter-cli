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

package gitops

import (
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func gatewayOverlayFilesRenderer(cfg v2.Config) (map[string]string, error) {
	files := map[string]string{}

	// Static files (unchanged): the GatewayClass and its default EnvoyProxy.
	files["namespace.yaml"] = gatewayNamespaceContent
	files["gateway-class.yaml"] = gatewayClassContent
	files["envoy-proxy-config.yaml"] = envoyProxyConfigContent

	// gateway.yaml holds one Gateway document per pool group (OCTR-762). In the
	// single-pool case this is exactly the one rmpk-gateway that shipped before.
	gatewayContent, err := renderGatewayResources(cfg)
	if err != nil {
		return nil, err
	}
	files["gateway.yaml"] = gatewayContent

	// Non-default pools each get their own EnvoyProxy carrying the MetalLB
	// address-pool annotation. None are emitted in the single-pool case.
	for _, group := range gatewayPoolGroups(cfg) {
		if group.isDefault {
			continue
		}
		files[envoyProxyFileName(group.pool)] = renderPoolEnvoyProxy(group)
	}

	return files, nil
}

// envoyProxyFileName is the overlay filename for a non-default pool's EnvoyProxy.
func envoyProxyFileName(pool string) string {
	return "envoy-proxy-config-" + pool + ".yaml"
}

// gatewayKustomizationRenderer produces the gateway overlay kustomization.yaml.
// It lists the four base files (namespace, gateway-class, gateway, the default
// EnvoyProxy) plus one envoy-proxy-config-<pool>.yaml per non-default pool. For
// the single-pool case it emits exactly the string the previous static
// KustomizationContent held, keeping output byte-identical.
func gatewayKustomizationRenderer(cfg v2.Config) (string, error) {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\n")
	b.WriteString("kind: Kustomization\n")
	b.WriteString("resources:\n")
	b.WriteString("  - \"namespace.yaml\"\n")
	b.WriteString("  - \"gateway-class.yaml\"\n")
	b.WriteString("  - \"gateway.yaml\"\n")
	b.WriteString("  - \"envoy-proxy-config.yaml\"\n")
	// gatewayPoolGroups returns non-default groups already sorted by pool name.
	for _, group := range gatewayPoolGroups(cfg) {
		if group.isDefault {
			continue
		}
		b.WriteString("  - \"" + envoyProxyFileName(group.pool) + "\"\n")
	}
	return b.String(), nil
}

// renderGatewayResources renders every Gateway document (default first, then
// non-default pools) concatenated into a single gateway.yaml. For the
// single-pool case it produces byte-for-byte the original rmpk-gateway output.
func renderGatewayResources(cfg v2.Config) (string, error) {
	groups := gatewayPoolGroups(cfg)
	var out strings.Builder
	for _, group := range groups {
		doc, err := renderGatewayDoc(cfg, group)
		if err != nil {
			return "", err
		}
		out.WriteString(doc)
	}
	return out.String(), nil
}

// renderGatewayDoc renders a single Gateway document for one pool group: the
// header (name, optional cert-manager annotation, gatewayClassName, optional
// infrastructure.parametersRef) followed by the group's listener blocks in
// canonical order.
func renderGatewayDoc(cfg v2.Config, group gatewayPoolGroup) (string, error) {
	var out strings.Builder
	out.WriteString(renderGatewayHeader(cfg, group))
	for _, listener := range group.listeners {
		tmpl, ok := gatewayListenerTemplates[listener]
		if !ok {
			continue
		}
		block, err := renderOverlayTemplate(tmpl, cfg)
		if err != nil {
			return "", err
		}
		out.WriteString(block)
	}
	return out.String(), nil
}

// renderGatewayHeader builds the Gateway document header for a pool group: the
// metadata (name, namespace, optional cert-manager annotation) and the spec up
// to and including "listeners:". For the default group this reproduces the
// original rmpk-gateway header byte-for-byte; non-default groups additionally
// carry spec.infrastructure.parametersRef pointing at their EnvoyProxy.
func renderGatewayHeader(cfg v2.Config, group gatewayPoolGroup) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("apiVersion: gateway.networking.k8s.io/v1beta1\n")
	b.WriteString("kind: Gateway\n")
	b.WriteString("metadata:\n")
	b.WriteString("  name: " + group.gatewayName + "\n")
	b.WriteString("  namespace: " + gatewayNamespace + "\n")
	if !gatewayIsBYO(cfg) {
		b.WriteString("  annotations:\n")
		b.WriteString("    cert-manager.io/cluster-issuer: " + gatewayDefaultIssuer(cfg) + "\n")
	}
	b.WriteString("spec:\n")
	b.WriteString("  gatewayClassName: eg\n")
	if !group.isDefault {
		b.WriteString("  infrastructure:\n")
		b.WriteString("    parametersRef:\n")
		b.WriteString("      group: gateway.envoyproxy.io\n")
		b.WriteString("      kind: EnvoyProxy\n")
		b.WriteString("      name: " + group.envoyProxyName + "\n")
	}
	b.WriteString("  listeners:\n")
	return b.String()
}

// gatewayServiceConfig returns the typed gateway service config, or nil.
func gatewayServiceConfig(cfg v2.Config) *services.GatewayConfig {
	if svc, ok := cfg.OpenCenter.Services["gateway"]; ok {
		if gw, ok := svc.(*services.GatewayConfig); ok {
			return gw
		}
	}
	return nil
}

// gatewayIsBYO reports whether the gateway uses bring-your-own TLS.
func gatewayIsBYO(cfg v2.Config) bool {
	return gatewayServiceConfig(cfg).IsBYO()
}

// gatewayDefaultIssuer returns the cert-manager cluster issuer, defaulting to
// "rackspace-ca" to match the original template's `default` pipeline.
func gatewayDefaultIssuer(cfg v2.Config) string {
	gw := gatewayServiceConfig(cfg)
	if gw != nil && gw.DefaultIssuer != "" {
		return gw.DefaultIssuer
	}
	return "rackspace-ca"
}

// renderPoolEnvoyProxy renders the EnvoyProxy for a non-default pool group,
// annotating the LoadBalancer Service Envoy creates with the MetalLB pool.
//
// The per-pool EnvoyProxy lives in the Gateway's namespace (rackspace-system),
// NOT envoy-gateway-system, because a Gateway's spec.infrastructure.parametersRef
// carries no namespace field and Envoy Gateway resolves it in the Gateway's own
// namespace. (The default GatewayClass-level EnvoyProxy differs: it stays in
// envoy-gateway-system because the GatewayClass parametersRef names that
// namespace explicitly.) Emitting it elsewhere makes the Gateway report
// Accepted=False "failed to find envoyproxy ..." and never program.
func renderPoolEnvoyProxy(group gatewayPoolGroup) string {
	return "apiVersion: gateway.envoyproxy.io/v1alpha1\n" +
		"kind: EnvoyProxy\n" +
		"metadata:\n" +
		"  name: " + group.envoyProxyName + "\n" +
		"  namespace: " + gatewayNamespace + "\n" +
		"spec:\n" +
		"  provider:\n" +
		"    type: Kubernetes\n" +
		"    kubernetes:\n" +
		"      envoyService:\n" +
		"        annotations:\n" +
		"          " + metallbAddressPoolAnnotation + ": " + group.pool + "\n" +
		"      envoyDaemonSet:\n" +
		"        patch:\n" +
		"          value:\n" +
		"            spec:\n" +
		"              template:\n" +
		"                spec:\n" +
		"                  nodeSelector:\n" +
		"                    node-role.kubernetes.io/worker: \"worker\"\n"
}

func kubePrometheusStackOverlayFilesRenderer(cfg v2.Config) (map[string]string, error) {
	templates := map[string]string{
		"prometheus-http-route.yaml":   prometheusHTTPRouteTemplate,
		"alertmanager-http-route.yaml": alertmanagerHTTPRouteTemplate,
		"grafana-http-route.yaml":      grafanaHTTPRouteTemplate,
	}
	files := make(map[string]string, len(templates))
	for name, tmpl := range templates {
		content, err := renderOverlayTemplate(tmpl, cfg)
		if err != nil {
			return nil, err
		}
		files[name] = content
	}
	return files, nil
}

func longhornOverlayFilesRenderer(cfg v2.Config) (map[string]string, error) {
	files := map[string]string{}

	content, err := renderOverlayTemplate(longhornHTTPRouteTemplate, cfg)
	if err != nil {
		return nil, err
	}
	files["longhorn-http-route.yaml"] = content

	return files, nil
}

func renderOverlayTemplate(tmpl string, cfg v2.Config) (string, error) {
	funcMap := sprig.TxtFuncMap()
	// fqdn returns the cluster FQDN for hostname interpolation: the literal
	// cluster_fqdn value normally, or the Flux postBuild placeholder (e.g.
	// ${CLUSTER_FQDN}) when hostname_substitution is enabled (OCTR-759).
	funcMap["fqdn"] = clusterFQDNFunc(cfg)
	// gw returns the gateway service config for BYO-TLS resolution in the
	// per-listener blocks. Returning a non-nil zero value keeps templates safe
	// when the gateway service is absent (pointer methods handle nil).
	funcMap["gw"] = func() *services.GatewayConfig { return gatewayServiceConfig(cfg) }
	// gatewayNameFor returns the Gateway resource name a service's HTTPRoutes
	// must target, given per-service MetalLB pool selection (OCTR-762). Defaults
	// to rmpk-gateway when the service is on the default pool.
	funcMap["gatewayNameFor"] = func(serviceName string) string {
		return gatewayNameForService(cfg, serviceName)
	}
	funcMap["kubePrometheusStack"] = func() (kubePrometheusStackRenderContract, error) {
		return resolveKubePrometheusStackContract(cfg)
	}
	t, err := template.New("overlay").Funcs(funcMap).Parse(tmpl)
	if err != nil {
		return "", err
	}
	var buf strings.Builder
	if err := t.Execute(&buf, cfg); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// clusterFQDNFunc returns a template helper that yields the cluster FQDN as a
// literal, or the Flux postBuild placeholder when hostname substitution is on.
func clusterFQDNFunc(cfg v2.Config) func() string {
	return func() string {
		if p := cfg.OpenCenter.Cluster.HostnameSubstitution.FQDNPlaceholder(); p != "" {
			return p
		}
		return cfg.OpenCenter.Cluster.ClusterFQDN
	}
}

const gatewayNamespaceContent = `---
apiVersion: v1
kind: Namespace
metadata:
  name: rackspace-system
`

const gatewayClassContent = `---
apiVersion: gateway.networking.k8s.io/v1
kind: GatewayClass
metadata:
  name: eg
spec:
  controllerName: gateway.envoyproxy.io/gatewayclass-controller
  parametersRef:
    group: gateway.envoyproxy.io
    kind: EnvoyProxy
    name: custom-proxy-config
    namespace: envoy-gateway-system
`

const envoyProxyConfigContent = `apiVersion: gateway.envoyproxy.io/v1alpha1
kind: EnvoyProxy
metadata:
  name: custom-proxy-config
  namespace: envoy-gateway-system
spec:
  provider:
    type: Kubernetes
    kubernetes:
      envoyDaemonSet:
        patch:
          value:
            spec:
              template:
                spec:
                  nodeSelector:
                    node-role.kubernetes.io/worker: "worker"
`

// gatewayListenerTemplates maps each Gateway listener section name to its YAML
// block. The blocks are rendered against the full v2.Config (so the fqdn and gw
// helpers resolve exactly as before) and concatenated after a Gateway header by
// renderGatewayDoc. Splitting the original monolithic Gateway template into
// per-listener blocks lets the generator place each listener on the Gateway
// backing its service's pool (OCTR-762) while keeping the emitted YAML for any
// given listener byte-identical to what shipped before.
//
// The `gw` template helper returns the gateway service config so BYO-TLS
// resolution ((gw).TLSSecretFor / (gw).TLSSecretNamespace) works without a
// per-block variable declaration that would perturb whitespace.
var gatewayListenerTemplates = map[string]string{
	"keycloak-https":     gatewayListenerKeycloakHTTPS,
	"keycloak-http":      gatewayListenerKeycloakHTTP,
	"gitops-https":       gatewayListenerGitopsHTTPS,
	"headlamp-https":     gatewayListenerHeadlampHTTPS,
	"prometheus-https":   gatewayListenerPrometheusHTTPS,
	"alertmanager-https": gatewayListenerAlertmanagerHTTPS,
	"grafana-https":      gatewayListenerGrafanaHTTPS,
	"harbor-http":        gatewayListenerHarborHTTP,
	"harbor-https":       gatewayListenerHarborHTTPS,
	"longhorn-https":     gatewayListenerLonghornHTTPS,
}

const gatewayListenerKeycloakHTTPS = `    - name: keycloak-https
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
            name: {{ (gw).TLSSecretFor "keycloak" "keycloak-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerKeycloakHTTP = `    - name: keycloak-http
      hostname: {{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" fqdn) }}
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
`

const gatewayListenerGitopsHTTPS = `    - name: gitops-https
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
            name: {{ (gw).TLSSecretFor "gitops" "gitops-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerHeadlampHTTPS = `    - name: headlamp-https
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
            name: {{ (gw).TLSSecretFor "headlamp" "headlamp-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerPrometheusHTTPS = `    - name: prometheus-https
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
            name: {{ (gw).TLSSecretFor "prometheus" "prometheus-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerAlertmanagerHTTPS = `    - name: alertmanager-https
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
            name: {{ (gw).TLSSecretFor "alertmanager" "alertmanager-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerGrafanaHTTPS = `    - name: grafana-https
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
            name: {{ (gw).TLSSecretFor "grafana" "grafana-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const gatewayListenerHarborHTTP = `    - name: harbor-http
      protocol: HTTP
      port: 80
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" fqdn) }}
      allowedRoutes:
        namespaces:
          from: All
`

const gatewayListenerHarborHTTPS = `    - name: harbor-https
      protocol: HTTPS
      port: 443
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" fqdn) }}
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: {{ (gw).TLSSecretFor "harbor" "harbor-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
      allowedRoutes:
        namespaces:
          from: All
`

const gatewayListenerLonghornHTTPS = `    - name: longhorn-https
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
            name: {{ (gw).TLSSecretFor "longhorn" "longhorn-tls" }}
{{- if (gw).TLSSecretNamespace }}
            namespace: {{ (gw).TLSSecretNamespace }}
{{- end }}
`

const longhornHTTPRouteTemplate = `---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: longhorn-gateway-route
  namespace: longhorn-system
spec:
  hostnames:
  - {{ (index .OpenCenter.Services "longhorn").Hostname | default (printf "longhorn.%s" fqdn) | quote }}
  parentRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: {{ gatewayNameFor "longhorn" }}
    namespace: rackspace-system
    sectionName: longhorn-https
  rules:
  - backendRefs:
    - group: ""
      kind: Service
      name: longhorn-frontend
      port: 80
      weight: 1
    matches:
      - path:
          type: PathPrefix
          value: /
`

const prometheusHTTPRouteTemplate = `---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: prometheus-gateway-route
  namespace: {{ (kubePrometheusStack).Namespace }}
spec:
  hostnames:
    - {{ (index .OpenCenter.Services "kube-prometheus-stack").PrometheusHostname | default (printf "prometheus.%s" fqdn) | quote }}
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: {{ gatewayNameFor "kube-prometheus-stack" }}
      namespace: rackspace-system
      sectionName: prometheus-https
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - group: ""
          kind: Service
          name: {{ (kubePrometheusStack).PrometheusServiceName }}
          port: 9090
`

const alertmanagerHTTPRouteTemplate = `---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: alertmanager-gateway-route
  namespace: {{ (kubePrometheusStack).Namespace }}
spec:
  hostnames:
    - {{ (index .OpenCenter.Services "kube-prometheus-stack").AlertmanagerHostname | default (printf "alertmanager.%s" fqdn) | quote }}
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: {{ gatewayNameFor "kube-prometheus-stack" }}
      namespace: rackspace-system
      sectionName: alertmanager-https
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - group: ""
          kind: Service
          name: {{ (kubePrometheusStack).AlertmanagerServiceName }}
          port: 9093
`

const grafanaHTTPRouteTemplate = `---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: grafana-gateway-route
  namespace: {{ (kubePrometheusStack).Namespace }}
spec:
  hostnames:
    - {{ (index .OpenCenter.Services "kube-prometheus-stack").GrafanaHostname | default (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "grafana.%s" fqdn) | quote }}
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: {{ gatewayNameFor "kube-prometheus-stack" }}
      namespace: rackspace-system
      sectionName: grafana-https
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - group: ""
          kind: Service
          name: {{ (kubePrometheusStack).GrafanaServiceName }}
          port: 80
`
