---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: keycloak
  namespace: keycloak
spec:
  parentRefs:
    - name: rmpk-gateway
      sectionName: keycloak-https
      namespace: rackspace-system
  hostnames:
    - "auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud"
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
    - name: rmpk-gateway
      namespace: rackspace-system
      sectionName: keycloak-http
  hostnames:
    - "auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud"
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301 # Permanent redirect
