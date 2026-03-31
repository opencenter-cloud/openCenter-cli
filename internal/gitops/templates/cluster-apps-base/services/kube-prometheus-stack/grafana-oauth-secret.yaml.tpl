apiVersion: v1
kind: Secret
metadata:
  name: grafana-oauth-secret
type: Opaque
stringData:
  client_id: grafana-oauth
  client_secret: {{ .Secrets.Keycloak.ClientSecret | default "PLACEHOLDER-GRAFANA-OAUTH-CLIENT-SECRET" }}
