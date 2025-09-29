---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: gitops
  namespace: flux-system
spec:
  parentRefs:
    - name: rmpk-gateway
      sectionName: gitops-https
      namespace: rackspace-system
  hostnames:
    - "gitops.{{ .ClusterName }}.k8s.opencenter.cloud"
  rules:
    - backendRefs:
        - name: weave-gitops
          port: 9001
