---
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: kafka-cluster
  namespace: flux-system
spec:
  dependsOn:
    - name: strimzi-kafka-operator-base
      namespace: flux-system
  interval: 15m
  retryInterval: 1m
  timeout: 10m
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
  path: ./applications/overlays/{{ .OpenCenter.Cluster.ClusterName }}/services/kafka-cluster
  targetNamespace: kafka-system
  prune: true
  wait: true
  commonMetadata:
    labels:
      app.kubernetes.io/part-of: kafka-cluster
      app.kubernetes.io/managed-by: flux
      opencenter/managed-by: opencenter
