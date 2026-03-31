---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: harbor
secretGenerator:
  - name: harbor-values-override
    namespace: harbor
    type: Opaque
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
resources:
{{- if (index .OpenCenter.Services "harbor").EmitCertificate }}
  - "certificate.yaml"
{{- end }}
  - "httproute.yaml"
