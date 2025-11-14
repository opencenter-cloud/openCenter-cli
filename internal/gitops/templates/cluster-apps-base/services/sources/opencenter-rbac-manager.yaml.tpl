---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-rbac-manager
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
    {{- if .OpenCenter.GitOps.GitOpsBaseRelease }}
    tag: {{ .OpenCenter.GitOps.GitOpsBaseRelease }}
    {{- else }}
    branch: {{ .OpenCenter.GitOps.GitOpsBranch | default "oidc-support" }}
    {{- end }}
  secretRef:
    name: opencenter-base
