{{- $customer := .OpenCenter.GitOps.OverlayUnits.CustomerManaged -}}
{{- range $index, $kustomization := $customer.Kustomizations }}
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: {{ $kustomization.Name }}
  namespace: flux-system
spec:
  {{- if $kustomization.DependsOn }}
  dependsOn:
    {{- range $dependency := $kustomization.DependsOn }}
    - name: {{ $dependency }}
      namespace: flux-system
    {{- end }}
  {{- end }}
  interval: {{ $customer.Interval | default "5m" }}
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: {{ $customer.RepositoryName }}
    namespace: flux-system
  path: {{ $kustomization.Path }}
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: customer
{{- end }}
