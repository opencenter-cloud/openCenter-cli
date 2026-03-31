apiVersion: v1
kind: Secret
metadata:
  name: http-route-fqdn-secret
type: Opaque
data:
  http_route_fqdn: {{ (index .OpenCenter.ManagedService "alert-proxy").HTTPRouteFQDN | b64enc }}
