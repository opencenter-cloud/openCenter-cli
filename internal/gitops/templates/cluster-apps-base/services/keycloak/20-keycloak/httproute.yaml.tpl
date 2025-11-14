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
    - "auth.dev.sjc3.rmpk.dev"
  rules:
    - backendRefs:
        - name: keycloak-service
          port: 8080
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
    - "auth.dev.sjc3.rmpk.dev"
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301 # Permanent redirect
