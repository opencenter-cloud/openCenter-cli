---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-kyverno
  namespace: flux-system
spec:
  interval: 15m
  url: ssh://git@github.com/rackerlabs/openCenter-gitops-base.git
  ref:
    branch: kyverno
  secretRef:
    name: opencenter-base
