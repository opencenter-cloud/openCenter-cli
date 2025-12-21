---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-k8s-dev
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
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
      hostname: auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: gitops.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
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
      hostname: headlamp.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
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
      hostname: prometheus.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
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
      hostname: alertmanager.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
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
      hostname: grafana.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
      tls:
        mode: Terminate
        certificateRefs:
          - group: ""
            kind: Secret
            name: grafana-tls
    - name: harbor-http
      protocol: HTTP
      port: 80
      hostname: harbor.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
      allowedRoutes:
        namespaces:
          from: All
    - name: harbor-https
      protocol: HTTPS
      port: 443
      hostname: harbor.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: harbor-tls
      allowedRoutes:
        namespaces:
          from: All
