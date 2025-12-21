---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: grafana-gateway-route
  namespace: observability
spec:
  hostnames:
    - "grafana.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud"
  parentRefs:
    - group: gateway.networking.k8s.io
      kind: Gateway
      name: rmpk-gateway
      namespace: rackspace-system
      sectionName: grafana-https
  rules:
    - backendRefs:
        - group: ""
          kind: Service
          name: observability-kube-prometheus-stack-grafana
          port: 80
          weight: 1
      matches:
        - path:
            type: PathPrefix
            value: /
