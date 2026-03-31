---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: flux-system
resources:
  - "{{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.RepositoryName }}.yaml"
{{- if .OpenCenter.GitOps.OverlayUnits.CustomerManaged.EmitSecret }}
  - "{{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.RepositoryName }}-secret.yaml"
{{- end }}
