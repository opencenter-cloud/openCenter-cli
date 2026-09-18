---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: keycloak
  namespace: keycloak
spec:
  parentRefs:
    - name: {{ gatewayNameFor "keycloak" }}
      sectionName: keycloak-https
      namespace: rackspace-system
  hostnames:
    - {{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) | quote }}
  rules:
    - backendRefs:
        - name: keycloak-service
          port: 8080
      matches:
        - path:
            type: PathPrefix
            value: /
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: keycloak-http-redirect
  namespace: keycloak
spec:
  parentRefs:
    - name: {{ gatewayNameFor "keycloak" }}
      namespace: rackspace-system
      sectionName: keycloak-http
  hostnames:
    - {{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) | quote }}
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301 # Permanent redirect
