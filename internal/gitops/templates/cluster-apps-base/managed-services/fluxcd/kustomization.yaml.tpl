---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
{{- $hasManagedServices := false }}
{{- range $name, $service := .OpenCenter.ManagedService }}
  {{- if $service.Enabled }}
    {{- $hasManagedServices = true }}
  {{- end }}
{{- end }}
{{- if $hasManagedServices }}
  - ./sources.yaml
{{- end }}
{{- if (index .OpenCenter.ManagedService "alert-proxy").Enabled }}
  - ./alert-proxy.yaml
{{- end }}
