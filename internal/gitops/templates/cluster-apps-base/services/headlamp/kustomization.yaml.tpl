---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: headlamp
resources:
  - "./httproute.yaml"
secretGenerator:
  - name: headlamp-values-override
    namespace: headlamp
    type: Opaque
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
