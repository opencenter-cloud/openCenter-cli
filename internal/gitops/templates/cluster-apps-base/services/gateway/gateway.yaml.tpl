---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-issuer-prod
spec:
  gatewayClassName: eg
  listeners:
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: gitops.demo.stage.sjc3.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: gitops-tls
    - name: prometheus-https
      port: 443
      protocol: HTTPS
      hostname: prometheus.demo.stage.sjc3.k8s.opencenter.cloud
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
      hostname: alertmanager.demo.stage.sjc3.k8s.opencenter.cloud
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
      hostname: grafana.demo.stage.sjc3.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: grafana-tls
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: auth.demo.stage.sjc3.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: keycloak-tls
    - name: keycloak-http
      port: 80
      protocol: HTTP
      hostname: auth.demo.stage.sjc3.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
