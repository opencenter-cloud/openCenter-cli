---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-keycloak
  namespace: flux-system
spec:
  interval: 15m
  {{- $service := index .OpenCenter.Services "keycloak" }}
  url: {{ $service.Uri | default .OpenCenter.GitOps.BaseRepo.URL }}
  ref:
    branch: {{ $service.Branch | default .OpenCenter.GitOps.Repository.Branch | default "main" }}
{{- if not (hasPrefix "https://" .OpenCenter.GitOps.BaseRepo.URL) }}
  secretRef:
    name: opencenter-base
{{- end }}
