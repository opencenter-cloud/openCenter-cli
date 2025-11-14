collectors:
  daemon:
    config:
      exporters:
        otlphttp/loki:
          endpoint: http://observability-loki-gateway.observability.svc.cluster.local/otlp
          headers:
            X-Scope-OrgID: "default"
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
        otlp/tempo:
          endpoint: observability-tempo-distributor.observability.svc.cluster.local:4317
          headers:
            X-Scope-OrgID: "default"
          tls:
            insecure: true
          compression: gzip
          timeout: 30s
          retry_on_failure:
            enabled: true
            initial_interval: 1s
            max_interval: 10s
            max_elapsed_time: 0s
          sending_queue:
            enabled: true
            num_consumers: 10
            queue_size: 2000
