---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
{{- $certManager := index .OpenCenter.Services "cert-manager" -}}
{{- $dnsProvider := $certManager.DNSProvider | default "route53" -}}
resources:
  - "./rackspace-selfsigned-issuer.yaml"
  - "./rackspace-selfsigned-ca.yaml"
  - "./rackspace-ca-issuer.yaml"
{{- if eq $dnsProvider "route53" }}
  - "./opencenter-aws-credentials-secret.yaml"
{{- end }}
{{- if eq $dnsProvider "cloudflare" }}
  - "./opencenter-cloudflare-credentials-secret.yaml"
{{- end }}
{{- if eq $dnsProvider "designate" }}
  - "./opencenter-openstack-designate-credentials-secret.yaml"
{{- end }}
  - "./letsencrypt-issuer.yaml"
secretGenerator:
  - name: cert-manager-values-override
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
