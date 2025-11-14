---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: opencenter-keycloak-config
  namespace: flux-system
spec:
  interval: 10m
  url: ssh://git@github.com/rpc-environments/000000-opencenter-example.git
  ref:
    branch: main
  secretRef:
    name: flux-system
  include:
    - repository:
        name: opencenter-keycloak
      fromPath: applications/base/services/keycloak
      toPath: applications/overlays/stage-cluster/services/base/keycloak/
