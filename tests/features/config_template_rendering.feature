Feature: Configuration-driven template rendering
  Verifies that cluster generate renders service-specific manifests with
  correct values, file structure, and YAML content.

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/gitops-repo"

  # ---------------------------------------------------------------------------
  # cert-manager
  # ---------------------------------------------------------------------------

  @template @cert-manager @secrets
  Scenario: Render cert-manager with custom AWS credentials
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          admin_email: admin@example.com
          cluster_fqdn: test-cluster.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          cert-manager:
            enabled: true
            region: us-west-2
            letsencrypt_server: "https://acme-staging-v02.api.letsencrypt.org/directory"
      secrets:
        cert_manager:
          aws_access_key: "AKIAEXAMPLE123"
          aws_secret_access_key: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/kustomization.yaml" should exist
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/kustomization.yaml" should contain "cert-manager-values-override"
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/kustomization.yaml" should contain "opencenter-aws-credentials-secret.yaml"
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/letsencrypt-issuer.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/cert-manager.yaml" should exist
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/cert-manager.yaml" should contain "cert-manager-base"
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/cert-manager.yaml" should contain "cert-manager-override"

  @template @cert-manager @letsencrypt
  Scenario: Render cert-manager with LetsEncrypt configuration
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          admin_email: devops@example.com
          cluster_fqdn: prod.example.com
          base_domain: example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          cert-manager:
            enabled: true
            region: eu-west-1
            letsencrypt_server: "https://acme-v02.api.letsencrypt.org/directory"
      secrets:
        cert_manager:
          aws_access_key: "AKIAEXAMPLE456"
          aws_secret_access_key: "secretkey789"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/letsencrypt-issuer.yaml" should exist
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/cert-manager/letsencrypt-issuer.yaml" should contain "acme-v02.api.letsencrypt.org"

  # ---------------------------------------------------------------------------
  # alert-proxy (managed service)
  # ---------------------------------------------------------------------------

  @template @alert-proxy @secrets
  Scenario: Render alert-proxy secrets with custom values
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
        managed_services:
          alert-proxy:
            enabled: true
      secrets:
        alert_proxy:
          core_device_id: "device-123"
          account_service_token: "token-456"
          core_account_number: "account-789"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/managed-services/alert-proxy/kustomization.yaml" should exist

  @template @alert-proxy @configuration
  Scenario: Render alert-proxy configuration with custom image tag
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
        managed_services:
          alert-proxy:
            enabled: true
            image_tag: "v1.2.3"
            alert_manager_base_url: "https://alertmanager.example.com"
            http_route_fqdn: "alerts.example.com"
      secrets:
        alert_proxy:
          core_device_id: "device-123"
          account_service_token: "token-456"
          core_account_number: "account-789"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/managed-services/fluxcd/alert-proxy.yaml" should exist

  # ---------------------------------------------------------------------------
  # loki
  # ---------------------------------------------------------------------------

  @template @loki @swift
  Scenario: Render Loki with custom Swift credentials
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          loki:
            enabled: true
            loki_bucket_name: "my-loki-bucket"
            loki_volume_size: 50
            loki_storage_class: "fast-ssd"
            swift_auth_url: "https://keystone.example.com/v3/"
            swift_username: "loki-user"
            swift_project_name: "my-project"
            swift_region: "US-EAST-1"
            swift_domain_name: "default"
      secrets:
        loki:
          swift_password: "my-secure-password"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/loki/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/loki.yaml" should exist

  # ---------------------------------------------------------------------------
  # velero
  # ---------------------------------------------------------------------------

  @template @velero @backup
  Scenario: Render Velero with custom backup bucket
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          velero:
            enabled: true
            velero_backup_bucket: "my-backup-bucket"
            velero_region: "us-west-1"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/velero/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/velero.yaml" should exist

  # ---------------------------------------------------------------------------
  # keycloak
  # ---------------------------------------------------------------------------

  @template @keycloak @oidc
  Scenario: Render Keycloak with custom OIDC configuration
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          admin_email: admin@example.com
          cluster_fqdn: test.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          keycloak:
            enabled: true
            keycloak_realm: "my-realm"
            keycloak_frontend_url: "https://auth.example.com"
            keycloak_client_id: "my-client"
            hostname: "auth.example.com"
      secrets:
        keycloak:
          client_secret: "secret123"
          admin_password: "admin123"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/keycloak.yaml" should exist
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/keycloak.yaml" should contain "keycloak"

  # ---------------------------------------------------------------------------
  # headlamp
  # ---------------------------------------------------------------------------

  @template @headlamp @oidc
  Scenario: Render Headlamp with OIDC integration
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          cluster_fqdn: test.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          headlamp:
            enabled: true
            headlamp_oidc_client_id: "headlamp-client"
            headlamp_oidc_issuer_url: "https://auth.example.com/realms/my-realm"
            hostname: "headlamp.example.com"
      secrets:
        headlamp:
          oidc_client_secret: "headlamp-secret"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/headlamp/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/headlamp/httproute.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/headlamp.yaml" should exist

  # ---------------------------------------------------------------------------
  # weave-gitops
  # ---------------------------------------------------------------------------

  @template @weave-gitops @password
  Scenario: Render Weave GitOps with custom password
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          cluster_fqdn: test.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          weave-gitops:
            enabled: true
            hostname: "gitops.example.com"
      secrets:
        weave_gitops:
          password: "mypassword"
          password_hash: "$2a$10$abcdefghijklmnopqrstuv"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/weave-gitops.yaml" should exist

  # ---------------------------------------------------------------------------
  # grafana / kube-prometheus-stack
  # ---------------------------------------------------------------------------

  @template @grafana @storage
  Scenario: Render Grafana with custom storage configuration
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
        storage:
          default_storage_class: "csi-cinder-sc-delete"
        services:
          kube-prometheus-stack:
            enabled: true
            grafana_volume_size: 20
            grafana_storage_class: "fast-ssd"
            prometheus_volume_size: 100
            prometheus_storage_class: "fast-ssd"
      secrets:
        grafana:
          admin_password: "grafana-admin-pass"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/kube-prometheus-stack/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/fluxcd/kube-prometheus-stack.yaml" should exist

  # ---------------------------------------------------------------------------
  # HTTPRoute hostname generation
  # ---------------------------------------------------------------------------

  @template @httproute @hostname
  Scenario: HTTPRoute hostname generation from cluster FQDN
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          cluster_fqdn: prod.k8s.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          keycloak:
            enabled: true
          headlamp:
            enabled: true
          weave-gitops:
            enabled: true
      secrets:
        keycloak:
          client_secret: "secret"
          admin_password: "password"
        headlamp:
          oidc_client_secret: "secret"
        weave_gitops:
          password: "password"
          password_hash: "hash"
        grafana:
          admin_password: "password"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/headlamp/httproute.yaml" should exist

  @template @httproute @custom-hostname
  Scenario: HTTPRoute with custom hostname overrides
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
          cluster_fqdn: test.example.com
        gitops:
          git_dir: <<tmp>>/gitops-repo
        services:
          keycloak:
            enabled: true
            hostname: "custom-auth.example.com"
          headlamp:
            enabled: true
            hostname: "custom-ui.example.com"
      secrets:
        keycloak:
          client_secret: "secret"
          admin_password: "password"
        headlamp:
          oidc_client_secret: "secret"
        grafana:
          admin_password: "password"
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/headlamp/httproute.yaml" should exist
    And the file "<<tmp>>/gitops-repo/applications/overlays/test-cluster/services/headlamp/httproute.yaml" should contain "custom-ui.example.com"

  # ---------------------------------------------------------------------------
  # Default values and fallback
  # ---------------------------------------------------------------------------

  @template @defaults @fallback
  Scenario: Template rendering with default values produces base structure
    Given a file "<<tmp>>/conf/test-cluster.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: test-cluster
        gitops:
          git_dir: <<tmp>>/gitops-repo
      """
    When I run "opencenter cluster use test-cluster --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the directory "<<tmp>>/gitops-repo/applications" should exist
    And the directory "<<tmp>>/gitops-repo/infrastructure" should exist
    And a file "<<tmp>>/gitops-repo/README.md" should exist
    And a file "<<tmp>>/gitops-repo/.gitignore" should exist

  # ---------------------------------------------------------------------------
  # Full integration rendering
  # ---------------------------------------------------------------------------

  @integration @full-rendering
  Scenario: Full cluster generation with all services renders complete structure
    Given a file "<<tmp>>/conf/test-integration.yaml" with content:
      """
      opencenter:
        meta:
          name: test-integration
          env: staging
          region: dfw3
          organization: test-org
        cluster:
          cluster_name: test-integration
          base_domain: k8s.test.example.com
          cluster_fqdn: test-integration.dfw3.k8s.test.example.com
          admin_email: admin@test.example.com
        storage:
          default_storage_class: csi-cinder-sc-delete
        gitops:
          git_dir: <<tmp>>/gitops-repo
          gitops_base_repo: ssh://git@github.com/opencenter-cloud/opencenter-gitops-base.git
          gitops_base_release: v0.2.0
          gitops_branch: main
        services:
          cert-manager:
            enabled: true
            region: us-east-1
            letsencrypt_server: https://acme-staging-v02.api.letsencrypt.org/directory
          loki:
            enabled: true
            loki_bucket_name: test-integration-loki
            loki_volume_size: 50
            loki_storage_class: csi-cinder-sc-delete
            swift_auth_url: https://keystone.api.test.example.com/v3/
            swift_username: loki-user
            swift_project_name: test-project
            swift_region: US-EAST-1
            swift_domain_name: default
          velero:
            enabled: true
            velero_backup_bucket: test-integration-backups
            velero_region: us-east-1
          keycloak:
            enabled: true
            keycloak_realm: opencenter
            keycloak_client_id: opencenter
            hostname: auth.test-integration.dfw3.k8s.test.example.com
          headlamp:
            enabled: true
            headlamp_oidc_client_id: opencenter
            hostname: headlamp.test-integration.dfw3.k8s.test.example.com
          weave-gitops:
            enabled: true
            hostname: gitops.test-integration.dfw3.k8s.test.example.com
          kube-prometheus-stack:
            enabled: true
            grafana_volume_size: 20
            prometheus_volume_size: 100
            alertmanager_volume_size: 10
        managed_services:
          alert-proxy:
            enabled: true
            alert_manager_base_url: https://alertmanager.test-integration.dfw3.k8s.test.example.com
            http_route_fqdn: alerts.test-integration.dfw3.k8s.test.example.com
            image_tag: v1.2.3
      secrets:
        cert_manager:
          aws_access_key: AKIATEST123456789ABC
          aws_secret_access_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYTESTKEY123
        loki:
          swift_password: test-swift-password-secure-123
        keycloak:
          client_secret: f8V0we25ajxjm9OMpFz9BsYObGTYKM4Y
          admin_password: SecureKeycloakAdminPassword123!
        headlamp:
          oidc_client_secret: headlamp-oidc-secret-abc123xyz
        weave_gitops:
          password: WeaveGitOpsPassword123!
          password_hash: $2a$10$abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKLMNOP
        grafana:
          admin_password: GrafanaAdminPassword123!
        alert_proxy:
          core_device_id: device-test-integration-12345
          account_service_token: token-test-integration-67890
          core_account_number: account-test-integration-11111
      """
    When I run "opencenter cluster use test-integration --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    # Base structure
    And a file "<<tmp>>/gitops-repo/README.md" should exist
    And a file "<<tmp>>/gitops-repo/.gitignore" should exist
    And the directory "<<tmp>>/gitops-repo/applications" should exist
    And the directory "<<tmp>>/gitops-repo/infrastructure" should exist
    # Service kustomizations rendered in overlay
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/cert-manager/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/loki/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/velero/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/kube-prometheus-stack/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/headlamp/kustomization.yaml" should exist
    # FluxCD kustomizations rendered for each service
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/cert-manager.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/loki.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/velero.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/keycloak.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/headlamp.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/weave-gitops.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/services/fluxcd/kube-prometheus-stack.yaml" should exist
    # Managed services rendered
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/managed-services/alert-proxy/kustomization.yaml" should exist
    And a file "<<tmp>>/gitops-repo/applications/overlays/test-integration/managed-services/fluxcd/alert-proxy.yaml" should exist
