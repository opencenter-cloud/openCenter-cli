---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-alert-proxy
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
    {{- if .OpenCenter.GitOps.GitOpsBaseRelease }}
    tag: {{ .OpenCenter.GitOps.GitOpsBaseRelease }}
    {{- else }}
    branch: {{ .OpenCenter.GitOps.GitOpsBranch | default "main" }}
    {{- end }}
  secretRef:
    name: opencenter-base
