---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-olm-config
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.GitURL }}
  ref:
    branch: main
  secretRef:
    name: flux-system
  include:
    - repository:
        name: opencenter-olm
      fromPath: applications/base/services/olm
      toPath: applications/overlays/{{ .ClusterName }}/services/base/olm/
