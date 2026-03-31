---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-strimzi-kafka-operator
  namespace: flux-system
spec:
  interval: 15m
  url: {{ .OpenCenter.GitOps.GitOpsBaseRepo }}
  ref:
    branch: {{ .OpenCenter.GitOps.GitOpsBranch | default "main" }}
  secretRef:
    name: opencenter-base
