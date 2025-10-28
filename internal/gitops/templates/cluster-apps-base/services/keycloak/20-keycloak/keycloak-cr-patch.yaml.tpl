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
  image: quay.io/keycloak/keycloak:26.3.0 # Keycloak version 26.3.0

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
    httpEnabled: true # Enable HTTP (TLS terminated at gateway)

  # Hostname and proxy configuration
  hostname:
    hostname: https://auth.{{ .ClusterName }}.k8s.opencenter.cloud # Public hostname with HTTPS
    strict: false
    backchannelDynamic: false # Enable dynamic backchannel URLs

  # Additional Keycloak server options
  additionalOptions:
    - name: hostname-url
      value: "https://auth.{{ .ClusterName }}.k8s.opencenter.cloud/"
    - name: metrics-enabled # Enable Prometheus metrics
      value: "true"
    - name: spi-connections-http-client-default-connection-timeout-millis # HTTP client timeout
      value: "60000" # 60 second timeout


  # RECOMMENDED: Add health check configuration for production
  # unsupported:
  #   podTemplate:
  #     spec:
  #       containers:
  #         - name: keycloak
  #           readinessProbe:
  #             httpGet:
  #               path: /health/ready
  #               port: 8080
  #             initialDelaySeconds: 30
  #             periodSeconds: 10
  #           livenessProbe:
  #             httpGet:
  #               path: /health/live
  #               port: 8080
  #             initialDelaySeconds: 60
  #             periodSeconds: 30