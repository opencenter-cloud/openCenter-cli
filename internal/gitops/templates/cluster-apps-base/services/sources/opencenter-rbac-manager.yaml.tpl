---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-rbac-manager
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
  ref:
    branch: oidc-support
  secretRef:
    name: opencenter-base
