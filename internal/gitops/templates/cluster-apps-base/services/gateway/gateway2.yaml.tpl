---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway-2
  namespace: test
  annotations:
    cert-manager.io/cluster-issuer: rackspace-ca
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: auth.dev.sjc3.rmpk.dev
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
      hostname: auth.dev.sjc3.rmpk.dev
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: gitops.dev.sjc3.rmpk.dev
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
      hostname: prometheus.dev.sjc3.rmpk.dev
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: prometheus-tls
    - name: prometheus-http
      hostname: prometheus.dev.sjc3.rmpk.dev
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: alertmanager-https
      port: 443
      protocol: HTTPS
      hostname: alertmanager.dev.sjc3.rmpk.dev
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
      hostname: grafana.dev.sjc3.rmpk.dev
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: grafana-tls
