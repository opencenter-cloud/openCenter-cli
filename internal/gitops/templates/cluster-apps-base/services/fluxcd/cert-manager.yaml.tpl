---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cert-manager-base
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-cert-manager
    namespace: flux-system
  path: applications/base/services/cert-manager
  targetNamespace: cert-manager
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: cert-manager
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: cert-manager-override
  namespace: flux-system
spec:
  dependsOn:
    - name: cert-manager-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/cert-manager
  targetNamespace: cert-manager
  prune: true
  wait: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: cert-manager
      namespace: cert-manager
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: cert-manager
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
