---
alertmanager:
  enabled: true
  alertmanagerSpec:
    externalUrl: https://alertmanager.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
    storage:
      volumeClaimTemplate:
        spec:
          accessModes: ["ReadWriteOnce"]
          resources:
            requests:
              storage: 50Gi
  config:
    global:
      resolve_timeout: 5m
    inhibit_rules:
      - source_matchers: [severity = critical]
        target_matchers: [severity =~ warning|info]
        equal: [namespace, alertname]
      - source_matchers: [severity = warning]
        target_matchers: [severity = info]
        equal: [namespace, alertname]
      - source_matchers: [alertname = InfoInhibitor]
        target_matchers: [severity = info]
        equal: [namespace]
      - target_matchers: [alertname = InfoInhibitor]
    route:
      group_by: [namespace, alertname]
      group_wait: 30s
      group_interval: 60s
      repeat_interval: 12h
      routes:
        - receiver: "null"
          matchers: [alertname = "Watchdog"]
        - receiver: warning_alerts_receiver
          continue: false
          matchers: [severity =~ "warning"]
        - receiver: alert_proxy_receiver
          continue: false
          matchers: [severity =~ "critical"]
    receivers:
      - name: "null"
      - name: warning_alerts_receiver
        msteamsv2_configs:
          - send_resolved: true
            webhook_url: {{ (index .OpenCenter.Services "kube-prometheus-stack").WebhookURL  }}
      - name: alert_proxy_receiver
        webhook_configs:
          - url: http://rackspace-alert-proxy.rackspace.svc.cluster.local/alert/process
            send_resolved: true
prometheus:
  enabled: true
  prometheusSpec:
    externalUrl: https://prometheus.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud
    externalLabels:
      cluster: k8s-dev
      region: {{ .OpenCenter.Cluster.ClusterRegion }}
      customer: {{ .OpenCenter.Cluster.ClusterOrganization }}
    serviceMonitorSelectorNilUsesHelmValues: false
    podMonitorSelectorNilUsesHelmValues: false
    ruleSelectorNilUsesHelmValues: false

grafana:
  enabled: true
  admin:
    existingSecret: "grafana-admin-password"
    userKey: admin-user
    passwordKey: admin-password
  persistence:
    enabled: true
    type: sts
    accessModes:
      - ReadWriteOnce
    size: 50Gi
    finalizers:
      - kubernetes.io/pvc-protection
  datasources:
    datasources.yaml:
      apiVersion: 1
      datasources:
        - name: Loki
          uid: loki-default
          type: loki
          access: proxy
          url: http://observability-loki-gateway.observability.svc.cluster.local
          isDefault: false
          jsonData:
            httpHeaderName1: "X-Scope-OrgID"
            maxLines: 1000
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true
        - name: Tempo
          uid: tempo-default
          type: tempo
          access: proxy
          url: http://observability-tempo-query-frontend.observability.svc.cluster.local:3200
          isDefault: false
          jsonData:
            httpHeaderName1: x-scope-orgid
            maxLines: 1000
            pdcInjected: false
            tracesToLogsV2:
              customQuery: false
              datasourceUid: loki-default
              filterBySpanID: true
              filterByTraceID: true
            tracesToMetrics:
              datasourceUid: prometheus
          secureJsonData:
            httpHeaderValue1: "default"
          editable: true

