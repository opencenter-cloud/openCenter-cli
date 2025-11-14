---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-velero
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
  ref:
    branch: main
  secretRef:
    name: opencenter-base
