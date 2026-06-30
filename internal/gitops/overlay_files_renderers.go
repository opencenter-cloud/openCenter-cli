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
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func init() {
	RegisterOverlayFilesRenderer("gateway", gatewayOverlayFilesRenderer)
	RegisterOverlayFilesRenderer("longhorn", longhornOverlayFilesRenderer)
}

func gatewayOverlayFilesRenderer(cfg v2.Config) (map[string]string, error) {
	files := map[string]string{}

	// Static files
	files["namespace.yaml"] = gatewayNamespaceContent
	files["gateway-class.yaml"] = gatewayClassContent
	files["envoy-proxy-config.yaml"] = envoyProxyConfigContent

	// Templated file
	content, err := renderOverlayTemplate(gatewayResourceTemplate, cfg)
	if err != nil {
		return nil, err
	}
	files["gateway.yaml"] = content

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

const gatewayResourceTemplate = `---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-default
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: keycloak-tls
    - name: keycloak-http
      hostname: {{ (index .OpenCenter.Services "keycloak").Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "gitops").Hostname | default (printf "gitops.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: gitops-tls
    - name: headlamp-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "headlamp").Hostname | default (printf "headlamp.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: headlamp-tls
    - name: prometheus-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "prometheus.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: prometheus-tls
    - name: alertmanager-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "alertmanager.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: alertmanager-tls
    - name: grafana-https
      port: 443
      protocol: HTTPS
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "grafana.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: grafana-tls
    - name: harbor-http
      protocol: HTTP
      port: 80
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
    - name: harbor-https
      protocol: HTTPS
      port: 443
      hostname: {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: harbor-tls
      allowedRoutes:
        namespaces:
          from: All
`

const longhornHTTPRouteTemplate = `---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: longhorn-gateway-route
  namespace: longhorn-system
spec:
  hostnames:
  - {{ (index .OpenCenter.Services "longhorn").Hostname | default (printf "longhorn.%s" .OpenCenter.Cluster.ClusterFQDN) | quote }}
  parentRefs:
  - group: gateway.networking.k8s.io
    kind: Gateway
    name: rmpk-gateway
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
