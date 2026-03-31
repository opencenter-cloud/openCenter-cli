---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: alert-proxy
  namespace: flux-system
spec:
  interval: 1h
  url: {{ (index .OpenCenter.ManagedService "alert-proxy").Uri | default (index .OpenCenter.ManagedService "alert-proxy").GitOpsSourceRepo }}
  ref:
    branch: {{ (index .OpenCenter.ManagedService "alert-proxy").Branch | default (index .OpenCenter.ManagedService "alert-proxy").GitOpsSourceBranch | default "main" }}
