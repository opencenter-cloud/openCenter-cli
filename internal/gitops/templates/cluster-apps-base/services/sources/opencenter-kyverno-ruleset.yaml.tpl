---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-kyverno-ruleset
  namespace: flux-system
spec:
  interval: 15m
  {{- $service := index .OpenCenter.Services "kyverno" }}
  url: {{ $service.Source.Repo | default .OpenCenter.GitOps.Repository.URL }}
  ref:
    branch: {{ $service.Source.Branch | default .OpenCenter.GitOps.Repository.Branch | default "main" }}
  secretRef:
    name: flux-system
  include:
    - repository:
        name: opencenter-kyverno
      fromPath: applications/base/services/kyverno
      toPath: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/base/kyverno/
