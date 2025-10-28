---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: flux-system
resources:
{{- if (index .OpenCenter.Services "gateway-api").Enabled }}
  - ./opencenter-gateway-api.yaml
{{- end }}
{{- if (index .OpenCenter.Services "cert-manager").Enabled }}
  - ./opencenter-cert-manager.yaml
{{- end }}
{{- if (index .OpenCenter.Services "olm").Enabled }}
  - ./opencenter-olm.yaml
  - ./opencenter-olm-config.yaml
{{- end }}
{{- if (index .OpenCenter.Services "velero").Enabled }}
  - ./opencenter-velero.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kube-prometheus-stack").Enabled }}
  - ./opencenter-kube-prometheus-stack.yaml
{{- end }}
{{- if (index .OpenCenter.Services "openstack-ccm").Enabled }}
  - ./opencenter-openstack-ccm.yaml
{{- end }}
{{- if (index .OpenCenter.Services "openstack-csi").Enabled }}
  - ./opencenter-openstack-csi.yaml
{{- end }}
{{- if (index .OpenCenter.Services "weave-gitops").Enabled }}
  - ./opencenter-weave-gitops.yaml
{{- end }}
{{- if (index .OpenCenter.Services "external-snapshotter").Enabled }}
  - ./opencenter-external-snapshotter.yaml
{{- end }}
{{- if (index .OpenCenter.Services "rbac-manager").Enabled }}
  - ./opencenter-rbac-manager.yaml
{{- end }}
{{- if (index .OpenCenter.Services "kyverno").Enabled }}
  - ./opencenter-kyverno.yaml
{{- end }}
{{- if (index .OpenCenter.Services "headlamp").Enabled }}
  - ./opencenter-headlamp.yaml
{{- end }}
{{- if (index .OpenCenter.Services "keycloak").Enabled }}
  - ./opencenter-keycloak.yaml
  - ./opencenter-keycloak-config.yaml
{{- end }}
{{- if (index .OpenCenter.Services "postgres-operator").Enabled }}
  - ./opencenter-postgres-operator.yaml
{{- end }}
