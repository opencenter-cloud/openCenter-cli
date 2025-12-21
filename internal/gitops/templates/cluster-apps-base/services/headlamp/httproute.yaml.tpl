---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: headlamp
  namespace: headlamp
spec:
  parentRefs:
    - name: rmpk-gateway
      sectionName: headlamp-https
      namespace: rackspace-system
  hostnames:
    - "headlamp.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud"
  rules:
    - backendRefs:
        - name: headlamp
          port: 80
      matches:
        - path:
            type: PathPrefix
            value: /
