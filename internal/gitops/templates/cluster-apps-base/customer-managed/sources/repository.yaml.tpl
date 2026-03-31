---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.RepositoryName }}
  namespace: flux-system
spec:
  interval: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.Interval | default "15m" }}
  url: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.RepositoryURL }}
  ref:
    branch: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.Branch | default "main" }}
  {{- if .OpenCenter.GitOps.OverlayUnits.CustomerManaged.SecretName }}
  secretRef:
    name: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.SecretName }}
  {{- end }}
