apiVersion: v1
data:
    admin-password: {{ .Secrets.Grafana.password | quote }}
    admin-user: {{ .Secrets.Grafana.User | quote }}
kind: Secret
metadata:
    name: grafana-admin-password
    namespace: observability

