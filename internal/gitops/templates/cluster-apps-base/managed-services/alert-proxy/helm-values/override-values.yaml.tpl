---
nodeSelector: {}
image:
  tag: {{ (index .OpenCenter.ManagedServices "alert-proxy").Image.Tag | default "latest" }}

config:
  logging:
    log_level: "DEBUG"
  alert_proxy_config:
    alert_verification: true
    create_ticket: true
