---
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
namespace: cert-manager
resources:
  - "rackspace-selfsigned-issuer.yaml"
  - "rackspace-selfsigned-ca.yaml"
  - "rackspace-ca-issuer.yaml"
  - "letsencrypt-issuer.yaml"
  - "{{ .OpenCenter.Cluster.ClusterName }}-aws-credentials-secret.yaml"
secretGenerator:
  - name: cert-manager-values-override
    type: Opaque
    files: [override.yaml=helm-values/override-values.yaml]
    options:
      disableNameSuffixHash: true
