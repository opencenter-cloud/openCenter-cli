apiVersion: v1
kind: Secret
metadata:
  name: {{ .OpenCenter.GitOps.OverlayUnits.CustomerManaged.SecretName | default .OpenCenter.GitOps.OverlayUnits.CustomerManaged.RepositoryName }}
  namespace: flux-system
type: Opaque
data:
  identity: {{ .Secrets.OverlayUnits.CustomerManaged.Identity | b64enc }}
  identity.pub: {{ .Secrets.OverlayUnits.CustomerManaged.IdentityPub | b64enc }}
  known_hosts: {{ .Secrets.OverlayUnits.CustomerManaged.KnownHosts | b64enc }}
