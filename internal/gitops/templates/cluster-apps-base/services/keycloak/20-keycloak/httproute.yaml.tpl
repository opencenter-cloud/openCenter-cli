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
    - "auth.{{ .ClusterName }}.k8s.opencenter.cloud"
  rules:
    - backendRefs:
        - name: keycloak-service
          port: 8080
