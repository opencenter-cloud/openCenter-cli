---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources: [../../base/keycloak/10-operator]
{{- if eq .OpenCenter.Meta.Region "ord1" }}
patches:
  - path: patch-subscription.yaml
    target:
      kind: Subscription
      name: keycloak-subscription
      namespace: keycloak
{{- end }}
