---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-weave-gitops
  namespace: flux-system
spec:
  interval: 15m
  {{- $service := index .OpenCenter.Services "weave-gitops" }}
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
