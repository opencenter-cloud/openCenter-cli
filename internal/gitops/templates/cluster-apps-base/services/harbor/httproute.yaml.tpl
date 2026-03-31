---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: harbor-redirect
spec:
  parentRefs:
    - name: rmpk-gateway
      sectionName: harbor-http
      namespace: rackspace-system
  hostnames:
    - {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
  rules:
    - filters:
        - type: RequestRedirect
          requestRedirect:
            scheme: https
            statusCode: 301
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: harbor
spec:
  parentRefs:
    - name: rmpk-gateway
      sectionName: harbor-https
      namespace: rackspace-system
  hostnames:
    - {{ (index .OpenCenter.Services "harbor").Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /api/
      backendRefs:
        - name: harbor-core
          port: 80
    - matches:
        - path:
            type: PathPrefix
            value: /service/
      backendRefs:
        - name: harbor-core
          port: 80
    - matches:
        - path:
            type: PathPrefix
            value: /v2/
      backendRefs:
        - name: harbor-core
          port: 80
    - matches:
        - path:
            type: PathPrefix
            value: /c/
      backendRefs:
        - name: harbor-core
          port: 80
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: harbor-portal
          port: 80
