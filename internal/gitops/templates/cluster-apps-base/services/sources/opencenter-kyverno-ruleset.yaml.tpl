---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-kyverno-ruleset
  namespace: flux-system
spec:
  interval: 10m
  {{- $service := index .OpenCenter.Services "kyverno" }}
  url: {{ $service.Uri | default .OpenCenter.GitOps.GitURL }}
  ref:
    branch: {{ $service.Branch | default .OpenCenter.GitOps.GitBranch | default "main" }}
  secretRef:
    name: flux-system
  include:
    - repository:
        name: opencenter-kyverno
      fromPath: applications/base/services/kyverno
      toPath: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/base/kyverno/
