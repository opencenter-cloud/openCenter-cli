---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-{{ .ClusterName }}
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: auth.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: keycloak-tls
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: gitops.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: gitops-tls
    - name: headlamp-https
      port: 443
      protocol: HTTPS
      hostname: headlamp.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: headlamp-tls
    - name: prometheus-https
      port: 443
      protocol: HTTPS
      hostname: prometheus.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: prometheus-tls
    - name: alertmanager-https
      port: 443
      protocol: HTTPS
      hostname: alertmanager.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: alertmanager-tls
    - name: grafana-https
      port: 443
      protocol: HTTPS
      hostname: grafana.{{ .ClusterName }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: grafana-tls
