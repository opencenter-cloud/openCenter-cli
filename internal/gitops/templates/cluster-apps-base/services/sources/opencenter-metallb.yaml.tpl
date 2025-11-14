---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-metallb
  namespace: flux-system
spec:
  interval: 10m
  url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
  ref:
    branch: main
  secretRef:
    name: opencenter-base
