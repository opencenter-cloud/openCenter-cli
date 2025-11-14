---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - "../base/olm/"
  - "./netpol.yaml"
patchesStrategicMerge:
  - "./patches.yaml"
