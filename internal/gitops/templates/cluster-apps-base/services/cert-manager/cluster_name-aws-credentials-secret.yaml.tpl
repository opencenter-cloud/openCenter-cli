apiVersion: v1
data:
  access-key-id: {{ .Secrets.CertManager.AWSAccessKey | b64enc }}
  secret-access-key: {{ .Secrets.CertManager.AWSSecretAccessKey | b64enc }}
kind: Secret
metadata:
  name: {{ .OpenCenter.Cluster.ClusterName }}-aws-credentials-secret
  namespace: cert-manager
type: Opaque
