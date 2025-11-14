---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: flux-system
resources:
  - "opencenter-gateway-api.yaml"
  - "opencenter-cert-manager.yaml"
  - "opencenter-olm.yaml"
  - "opencenter-olm-config.yaml"
  - "opencenter-velero.yaml"
  - "opencenter-openstack-ccm.yaml"
  - "opencenter-openstack-csi.yaml"
  - "opencenter-weave-gitops.yaml"
  - "opencenter-external-snapshotter.yaml"
  - "opencenter-rbac-manager.yaml"
  - "opencenter-kyverno.yaml"
  - "opencenter-keycloak.yaml"
  - "opencenter-keycloak-config.yaml"
  - "opencenter-postgres-operator.yaml"
  - "opencenter-observability.yaml"
