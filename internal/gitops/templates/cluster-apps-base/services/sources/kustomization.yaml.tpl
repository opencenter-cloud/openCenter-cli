---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: flux-system
resources:
{{- $services := .OpenCenter.Services }}
{{- if (index $services "gateway-api").Enabled }}
  - "opencenter-gateway-api.yaml"
{{- end }}
{{- if (index $services "cert-manager").Enabled }}
  - "opencenter-cert-manager.yaml"
{{- end }}
{{- if (index $services "olm").Enabled }}
  - "opencenter-olm.yaml"
  - "opencenter-olm-config.yaml"
{{- end }}
{{- if (index $services "velero").Enabled }}
  - "opencenter-velero.yaml"
{{- end }}
{{- if (index $services "openstack-ccm").Enabled }}
  - "opencenter-openstack-ccm.yaml"
{{- end }}
{{- if (index $services "openstack-csi").Enabled }}
  - "opencenter-openstack-csi.yaml"
{{- end }}
{{- if (index $services "weave-gitops").Enabled }}
  - "opencenter-weave-gitops.yaml"
{{- end }}
{{- if (index $services "external-snapshotter").Enabled }}
  - "opencenter-external-snapshotter.yaml"
{{- end }}
{{- if (index $services "rbac-manager").Enabled }}
  - "opencenter-rbac-manager.yaml"
{{- end }}
{{- if (index $services "kyverno").Enabled }}
  - "opencenter-kyverno.yaml"
{{- end }}
{{- if (index $services "keycloak").Enabled }}
  - "opencenter-keycloak.yaml"
  - "opencenter-keycloak-config.yaml"
{{- end }}
{{- if (index $services "postgres-operator").Enabled }}
  - "opencenter-postgres-operator.yaml"
{{- end }}
{{- if or (index $services "kube-prometheus-stack").Enabled (index $services "loki").Enabled }}
  - "opencenter-observability.yaml"
{{- end }}
