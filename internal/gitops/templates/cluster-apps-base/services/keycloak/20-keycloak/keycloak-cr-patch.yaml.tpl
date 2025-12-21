---
# Keycloak Custom Resource patch configuration
# Defines a highly available Keycloak deployment with PostgreSQL backend
apiVersion: k8s.keycloak.org/v2alpha1
kind: Keycloak
metadata:
  name: keycloak
  namespace: keycloak
spec:
  # Deployment configuration
  startOptimized: false # Start in  mode (not optimized for production)
  #startOptimized: true                             # RECOMMENDED: Enable for production performance
  instances: 3 # High availability with 3 replicas
  # RECOMMENDED: Add resource limits for production
  resources:
    requests:
      memory: "1Gi"
      cpu: "500m"
    limits:
      memory: "2Gi"
      cpu: "1000m"

  # Database configuration (PostgreSQL)
  db:
    vendor: postgres # Use PostgreSQL as backend database
    usernameSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do # DB username from secret
      key: username
    passwordSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do # DB password from secret
      key: password
    url: jdbc:postgresql://postgres-cluster.keycloak.svc.cluster.local:5432/keycloak # JDBC connection URL

  # HTTP configuration
  http:
    httpEnabled: true
  hostname:
    hostname: https://auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
    strict: false
    backchannelDynamic: false
  proxy:
    headers: xforwarded
  additionalOptions:
    - name: proxy
      value: "edge"
    - name: hostname-url
      value: "https://auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud"
    - name: metrics-enabled # Enable Prometheus metrics
      value: "true"
    - name: spi-connections-http-client-default-connection-timeout-millis # HTTP client timeout
      value: "60000" # 60 second timeout

