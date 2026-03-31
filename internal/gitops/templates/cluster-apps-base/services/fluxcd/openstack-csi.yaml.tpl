---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: openstack-csi-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: opencenter-openstack-csi
    namespace: flux-system
  path: applications/base/services/openstack-csi
  targetNamespace: openstack-csi
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: openstack-cinder-csi
      namespace: openstack-csi
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: openstack-csi
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: openstack-csi-override
  namespace: flux-system
spec:
  interval: 15m
  retryInterval: 1m
  timeout: 5m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/openstack-csi
  targetNamespace: openstack-csi
  prune: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: openstack-csi
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
