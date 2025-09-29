---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: alert-proxy-override
  namespace: flux-system
spec:
  dependsOn:
    - name: managed-services-sources
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  decryption:
    provider: sops
    secretRef:
      name: sops-age
  path: ./applications/overlays/{{ .ClusterName }}/managed-services/alert-proxy
  targetNamespace: rackspace
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: alert-proxy
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: alert-proxy-base
  namespace: flux-system
spec:
  dependsOn:
    - name: managed-services-sources
      namespace: flux-system
    - name: alert-proxy-override
      namespace: flux-system
  interval: 5m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-alert-proxy
    namespace: flux-system
  path: applications/base/managed-services/alert-proxy
  targetNamespace: rackspace
  prune: true
  healthChecks:
    - apiVersion: helm.toolkit.fluxcd.io/v2
      kind: HelmRelease
      name: alert-proxy
      namespace: rackspace
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: alert-proxy
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
