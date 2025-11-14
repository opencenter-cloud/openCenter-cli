apiVersion: k8s.keycloak.org/v2alpha1
kind: Keycloak
metadata:
  name: keycloak
  namespace: keycloak
spec:
  startOptimized: false
  instances: 3
  image: quay.io/keycloak/keycloak:26.3.0
  db:
    vendor: postgres
    usernameSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do
      key: username
    passwordSecret:
      name: keycloak.postgres-cluster.credentials.postgresql.acid.zalan.do
      key: password
    url: jdbc:postgresql://postgres-cluster.keycloak.svc.cluster.local:5432/keycloak
  http:
    httpEnabled: true
  hostname:
    hostname: https://auth.dev.sjc3.rmpk.dev
    strict: false
    backchannelDynamic: true
  proxy:
    headers: xforwarded
  additionalOptions:
    - name: proxy
      value: "edge"
    - name: metrics-enabled
      value: "true"
    - name: spi-connections-http-client-default-connection-timeout-millis
      value: "60000"
