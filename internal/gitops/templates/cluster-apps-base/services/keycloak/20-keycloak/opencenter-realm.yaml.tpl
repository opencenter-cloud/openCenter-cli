apiVersion: k8s.keycloak.org/v2alpha1
kind: KeycloakRealmImport
metadata:
  name: opencenter-realm
  namespace: keycloak
spec:
  keycloakCRName: keycloak
  realm:
    id: opencenter
    realm: opencenter
    enabled: true
    displayName: opencenter
    sslRequired: external
    bruteForceProtected: true
    passwordPolicy: length(12) and upperCase(1) and lowerCase(1) and digits(1) and specialChars(1)
    attributes:
      frontendUrl: https://auth.oc-poc2.td2.k8s.rmpk.dev
    clients:
      - clientId: opencenter
        name: openCenter Cluster
        description: OIDC client for Kubernetes API authentication
        enabled: true
        publicClient: true
        standardFlowEnabled: true
        directAccessGrantsEnabled: true
        protocol: openid-connect
        attributes:
          pkce.code.challenge.method: ""
        redirectUris:
          - http://*
          - https://*
        webOrigins:
          - http://*
          - https://*
        protocolMappers:
          - name: groups
            protocol: openid-connect
            protocolMapper: oidc-group-membership-mapper
            consentRequired: false
            config:
              claim.name: groups
              full.path: "false"
              id.token.claim: "true"
              access.token.claim: "true"
              userinfo.token.claim: "true"
          - name: audience
            protocol: openid-connect
            protocolMapper: oidc-audience-mapper
            consentRequired: false
            config:
              included.client.audience: kubernetes
              id.token.claim: "true"
              access.token.claim: "true"
          - name: email
            protocol: openid-connect
            protocolMapper: oidc-usermodel-property-mapper
            consentRequired: false
            config:
              usermodel.attribute: email
              claim.name: email
              id.token.claim: "true"
              access.token.claim: "true"
    groups:
      - name: admins
        path: /admins
      - name: developers
        path: /developers
      - name: monitoring-users
        path: /monitoring-users
      - name: observability-team
        path: /observability-team
      - name: platform-team
        path: /platform-team
      - name: security-team
        path: /security-team
      - name: viewers
        path: /viewers
      - name: development-developers
        path: /development-developers
      - name: staging-developers
        path: /staging-developers
      - name: production-developers
        path: /production-developers
    defaultGroups:
      - /viewers
    users:
      - username: oc.admin
        enabled: true
        email: mpk-support@rackspace.com
        emailVerified: true
        firstName: Admin
        lastName: User
        groups:
          - /admins
        credentials:
          - type: password
            temporary: true
            value: n5rUVHYhhrmchH!@#
