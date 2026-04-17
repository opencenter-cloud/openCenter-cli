{{- $certManager := index .OpenCenter.Services "cert-manager" -}}
{{- $dnsProvider := $certManager.DNSProvider | default "route53" -}}
apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-{{ .OpenCenter.Cluster.ClusterName }}
spec:
  acme:
    server: {{ $certManager.LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}
    email: {{ $certManager.Email | default "mpk-support@rackspace.com" }}
    privateKeySecretRef:
      name: letsencrypt-dns01
    solvers:
{{- if eq $dnsProvider "cloudflare" }}
      - dns01:
          cloudflare:
            apiTokenSecretRef:
              name: "opencenter-cloudflare-credentials-secret"
              key: api-token
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
{{- else if eq $dnsProvider "designate" }}
      - dns01:
          webhook:
            groupName: acme.syseleven.de
            solverName: designatedns
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
{{- else }}
      - dns01:
          route53:
            region: {{ $certManager.Region }}
            accessKeyIDSecretRef:
              name: "opencenter-aws-credentials-secret"
              key: access-key-id
            secretAccessKeySecretRef:
              name: "opencenter-aws-credentials-secret"
              key: secret-access-key
        selector:
          dnsZones:
            - {{ .OpenCenter.Cluster.ClusterFQDN }}
{{- end }}
