---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-strimzi-kafka-operator
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.BaseRepo.URL }}
  ref:
    branch: {{ .OpenCenter.GitOps.Repository.Branch | default "main" }}
{{- if not (hasPrefix "https://" .OpenCenter.GitOps.BaseRepo.URL) }}
  secretRef:
    name: opencenter-base
{{- end }}
