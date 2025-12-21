config:
    oidc:
        enabled: true
        externalSecret:
            enabled: false
        secret:
            create: true
        clientID: opencenter
        clientSecret: {{ .Secrets.Headlamp.oidc_client_secret }}
        issuerURL: https://auth.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud/realms/opencenter
        scopes: openid profile email groups
        callbackURL: https://headlamp.{{ .OpenCenter.Cluster.ClusterName }}.{{ .OpenCenter.Cluster.ClusterRegion }}.k8s.opencenter.cloud/oidc-callback
    pluginsDir: /build/plugins
initContainers:
    - command:
        - /bin/sh
        - -c
        - mkdir -p /build/plugins && cp -r /plugins/* /build/plugins/ && chown -R 100:101 /build
      image: ghcr.io/headlamp-k8s/headlamp-plugin-flux:latest
      imagePullPolicy: Always
      name: headlamp-plugins
      securityContext:
        runAsNonRoot: false
        privileged: false
        runAsUser: 0
        runAsGroup: 0
      volumeMounts:
        - mountPath: /build/plugins
          name: headlamp-plugins
volumeMounts:
    - mountPath: /build/plugins
      name: headlamp-plugins
volumes:
    - name: headlamp-plugins
      emptyDir: {}
