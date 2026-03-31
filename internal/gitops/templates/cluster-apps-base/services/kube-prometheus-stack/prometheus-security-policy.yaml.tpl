---
apiVersion: gateway.envoyproxy.io/v1alpha1
kind: SecurityPolicy
metadata:
  name: prometheus-oidc
  namespace: observability
spec:
  targetRef:
    group: gateway.networking.k8s.io
    kind: HTTPRoute
    name: prometheus-gateway-route
  oidc:
    provider:
      issuer: "https://{{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}/realms/opencenter"
    clientID: "opencenter"  
    clientSecret:
      name: "gateway-oidc-secret" 
    redirectURL: "https://{{ (index .OpenCenter.Services "kube-prometheus-stack").Hostname | default (printf "prometheus.%s" .OpenCenter.Cluster.ClusterFQDN) }}/oauth2/callback"
    scopes:
      - openid
      - profile
      - email
      - roles
    logoutPath: "/logout"
