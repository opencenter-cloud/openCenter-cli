---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: keycloak-postgres
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: postgres-operator-base
      namespace: flux-system
    - name: postgres-operator-override
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-keycloak-config
    namespace: flux-system
  path: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/keycloak/00-postgres
  targetNamespace: keycloak
  prune: true
  wait: true
  healthCheckExprs:
    - apiVersion: apps/v1
      kind: StatefulSet
      current: spec.replicas == status.availableReplicas
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: keycloak
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: keycloak-operator
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: keycloak-postgres
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-keycloak-config
    namespace: flux-system
  path: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/keycloak/10-operator
  targetNamespace: keycloak
  prune: true
  healthChecks:
    - apiVersion: apps/v1
      kind: Deployment
      name: keycloak-operator
      namespace: keycloak
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: keycloak
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: keycloak-cr
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: keycloak-postgres
      namespace: flux-system
    - name: keycloak-operator
      namespace: flux-system
    - name: envoy-gateway-api-base
      namespace: flux-system
    - name: envoy-gateway-api-override
      namespace: flux-system
    - name: gateway
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-keycloak-config
    namespace: flux-system
  path: applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/keycloak/20-keycloak
  targetNamespace: keycloak
  prune: true
  healthChecks:
    - apiVersion: apps/v1
      kind: StatefulSet
      name: keycloak
      namespace: keycloak
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: keycloak
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: oidc-rbac
  namespace: flux-system
spec:
  dependsOn:
    - name: sources
      namespace: flux-system
    - name: rbac-manager-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: opencenter-keycloak
    namespace: flux-system
  path: applications/base/services/keycloak/30-oidc-rbac
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: keycloak
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
