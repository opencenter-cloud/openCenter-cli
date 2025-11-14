apiVersion: "acid.zalan.do/v1"
kind: postgresql
metadata:
  name: postgres-cluster
  namespace: keycloak
spec:
  teamId: "acid"
  numberOfInstances: 3
  postgresql:
    version: "17"
    parameters:
      shared_buffers: "2GB"
      max_connections: "1024"
      log_statement: "all"
  volume:
    size: 20Gi
    storageClass: csi-cinder-sc-delete
  databases:
    keycloak: keycloak
  users:
    keycloak:
      - superuser
      - createdb
