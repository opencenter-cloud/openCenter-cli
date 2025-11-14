config:
  baseURL: ""
  oidc:
    clientID: opencenter
    clientSecret: f8V0we25ajxjm9OMpFz9BsYObGTYKM4Y
    issuerURL: https://auth.dev.sjc3.rmpk.dev/realms/opencenter
    scopes: email,profile
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
