---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-kyverno
  namespace: flux-system
spec:
  interval: 15m
  {{- $service := index .OpenCenter.Services "kyverno" }}
  url: {{ $service.Uri | default .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
  {{- if $service.Release }}
  tag: {{ $service.Release }}
  {{- else if .OpenCenter.GitOps.GitOpsBaseRelease }}
  tag: {{ .OpenCenter.GitOps.GitOpsBaseRelease }}
  {{- else }}
  branch: {{ $service.Branch | default .OpenCenter.GitOps.GitOpsBranch | default "kyverno" }}
  {{- end }}
  secretRef:
  name: opencenter-base
