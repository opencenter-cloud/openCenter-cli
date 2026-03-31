---
nodeSelector: {}
image:
  tag: {{ (index .OpenCenter.ManagedService "alert-proxy").ImageTag | default "latest" }}

config:
  logging:
    log_level: "DEBUG"
  alert_proxy_config:
    alert_verification: true
    create_ticket: true
