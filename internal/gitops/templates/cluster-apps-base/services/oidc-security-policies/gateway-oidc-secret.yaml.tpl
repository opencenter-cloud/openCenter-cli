{{- if and (index .OpenCenter.Services "keycloak").Enabled (index .OpenCenter.Services "kube-prometheus-stack").Enabled .OpenCenter.Identity.OIDC.Enabled oidcClientSecretsConfigured }}
apiVersion: v1
kind: Secret
metadata:
  name: {{ dedicatedSecretFilename "gateway-oidc" | trimSuffix ".yaml" }}
  namespace: {{ (kubePrometheusStack).Namespace }}
type: Opaque
stringData:
  client-secret: {{ .Secrets.Keycloak.ClientSecret | quote }}
{{- end }}
