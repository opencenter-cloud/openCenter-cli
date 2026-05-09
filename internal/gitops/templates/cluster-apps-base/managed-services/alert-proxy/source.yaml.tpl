---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: alert-proxy
  namespace: flux-system
spec:
  interval: 1h
  url: {{ (index .OpenCenter.ManagedServices "alert-proxy").Source.Repo | default (index .OpenCenter.ManagedServices "alert-proxy").Source.Repo }}
  ref:
    branch: {{ (index .OpenCenter.ManagedServices "alert-proxy").Source.Branch | default (index .OpenCenter.ManagedServices "alert-proxy").Source.Source.Branch | default "main" }}
