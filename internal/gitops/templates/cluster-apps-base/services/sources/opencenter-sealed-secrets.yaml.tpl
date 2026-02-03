---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-sealed-secrets
  namespace: flux-system
spec:
  interval: 10m
  {{- $service := index .OpenCenter.Services "sealed-secrets" }}
  url: {{ $service.Uri | default .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
  {{- if $service.Release }}
  tag: {{ $service.Release }}
  {{- else if .OpenCenter.GitOps.GitOpsBaseRelease }}
  tag: {{ .OpenCenter.GitOps.GitOpsBaseRelease }}
  {{- else }}
  branch: {{ $service.Branch | default .OpenCenter.GitOps.GitOpsBranch | default "main" }}
  {{- end }}
  secretRef:
  name: opencenter-base
