---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
{{- if (index .OpenCenter.Services "sources").Enabled }}
  - ./sources.yaml
{{- end }}
{{- if (index .OpenCenter.Services "gateway-api").Enabled }}
  - ./gateway-api.yaml
{{- end }}
{{- if (index .OpenCenter.Services "cert-manager").Enabled }}
  - ./cert-manager.yaml
{{- end }}
{{- if (index .OpenCenter.Services "olm").Enabled }}
  - ./olm.yaml
{{- end }}
{{- if (index .OpenCenter.Services "gateway").Enabled }}
  - ./gateway.yaml
{{- end }}
{{- if (index .OpenCenter.Services "velero").Enabled }}
  - ./velero.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kube-prometheus-stack").Enabled }}
  - ./kube-prometheus-stack.yaml
{{- end }}
{{- if (index .OpenCenter.Services "openstack-ccm").Enabled }}
  - ./openstack-ccm.yaml
{{- end }}
{{- if (index .OpenCenter.Services "openstack-csi").Enabled }}
  - ./openstack-csi.yaml
{{- end }}
{{- if (index .OpenCenter.Services "weave-gitops").Enabled }}
  - ./weave-gitops.yaml
{{- end }}
{{- if (index .OpenCenter.Services "external-snapshotter").Enabled }}
  - ./external-snapshotter.yaml
{{- end }}
{{- if (index .OpenCenter.Services "rbac-manager").Enabled }}
  - ./rbac-manager.yaml
{{- end }}
{{- if (index .OpenCenter.Services "headlamp").Enabled }}
  - ./headlamp.yaml
{{- end }}
{{- if (index .OpenCenter.Services "keycloak").Enabled }}
  - ./keycloak.yaml
{{- end }}
{{- if (index .OpenCenter.Services "postgres-operator").Enabled }}
  - ./postgres-operator.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kyverno").Enabled }}
  - ./kyverno.yaml
{{- end }}
