---
apiVersion: gateway.networking.k8s.io/v1beta1
kind: Gateway
metadata:
  name: rmpk-gateway
  namespace: rackspace-system
  annotations:
    cert-manager.io/cluster-issuer: letsencrypt-{{ .OpenCenter.Cluster.ClusterName }}
spec:
  gatewayClassName: eg
  listeners:
    - name: keycloak-https
      port: 443
      protocol: HTTPS
      hostname: {{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      protocol: HTTP
      port: 80
      allowedRoutes:
        namespaces:
          from: All
    - name: gitops-https
      port: 443
      protocol: HTTPS
      hostname: {{ .OpenCenter.Services.gitops.Hostname | default (printf "gitops.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ .OpenCenter.Services.headlamp.Hostname | default (printf "headlamp.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "prometheus.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "alertmanager.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "grafana.%s" .OpenCenter.Cluster.ClusterFQDN) }}
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
      hostname: {{ .OpenCenter.Services.harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      allowedRoutes:
        namespaces:
          from: All
    - name: harbor-https
      protocol: HTTPS
      port: 443
      hostname: {{ .OpenCenter.Services.harbor.Hostname | default (printf "harbor.%s" .OpenCenter.Cluster.ClusterFQDN) }}
      tls:
        mode: Terminate
        certificateRefs:
          - kind: Secret
            name: harbor-tls
      allowedRoutes:
        namespaces:
          from: All
