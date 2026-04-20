---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-alert-proxy
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.BaseRepo.URL }}
  ref:
    branch: {{ .OpenCenter.GitOps.Repository.Branch | default "main" }}
  secretRef:
    name: opencenter-base
