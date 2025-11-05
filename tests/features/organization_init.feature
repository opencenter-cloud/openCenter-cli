Feature: Organization-based cluster initialization
  As a user, I want to initialize clusters within organization-based directory structures
  so that I can organize my clusters by team or environment.

  Background:
    Given an empty directory "<<tmp>>/conf"

  Scenario: Init cluster with organization creates organization-based directory structure
    When I run "openCenter cluster init web-app --opencenter.meta.organization=dev-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/dev-team" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/infrastructure" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/infrastructure/clusters" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/infrastructure/clusters/web-app" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/applications" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/applications/overlays" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/applications/overlays/web-app" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/secrets" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/secrets/age" should exist
    And a directory "<<tmp>>/conf/clusters/dev-team/secrets/age/keys" should exist

  Scenario: Init cluster with organization creates cluster configuration in correct location
    When I run "openCenter cluster init api-service --opencenter.meta.organization=prod-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/prod-team/infrastructure/clusters/api-service/.api-service-config.yaml" should exist
    And the cluster configuration "api-service" should have "opencenter.meta.organization" set to "prod-team"
    And the cluster configuration "api-service" should have "opencenter.gitops.git_dir" containing "clusters/prod-team"

  Scenario: Init cluster with organization generates SOPS key in organization structure
    When I run "openCenter cluster init database --opencenter.meta.organization=data-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/data-team/secrets/age/keys/database-key.txt" should exist
    And the file "<<tmp>>/conf/clusters/data-team/secrets/age/keys/database-key.txt" should contain "AGE-SECRET-KEY-1"
    And a file "<<tmp>>/conf/clusters/data-team/secrets/.sops.yaml" should exist
    And the file "<<tmp>>/conf/clusters/data-team/secrets/.sops.yaml" should contain "creation_rules:"
    And the cluster configuration "database" should have "secrets.sops_age_key_file" containing "data-team/secrets/age/keys/database-key.txt"

  Scenario: Init cluster without organization uses opencenter organization
    When I run "openCenter cluster init legacy-app --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/opencenter" should exist
    And a directory "<<tmp>>/conf/clusters/opencenter/infrastructure/clusters/legacy-app" should exist
    And a file "<<tmp>>/conf/clusters/opencenter/infrastructure/clusters/legacy-app/.legacy-app-config.yaml" should exist
    And the cluster configuration "legacy-app" should have "opencenter.meta.organization" set to "opencenter"

  Scenario: Init multiple clusters in same organization share GitOps root
    When I run "openCenter cluster init frontend --opencenter.meta.organization=web-team --config-dir <<tmp>>/conf"
    And I run "openCenter cluster init backend --opencenter.meta.organization=web-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/frontend" should exist
    And a directory "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/backend" should exist
    And a file "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/frontend/.frontend-config.yaml" should exist
    And a file "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/backend/.backend-config.yaml" should exist
    And the cluster configuration "frontend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"
    And the cluster configuration "backend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"

  Scenario: Init cluster with organization and force flag overwrites existing
    When I run "openCenter cluster init test-service --opencenter.meta.organization=qa-team --config-dir <<tmp>>/conf"
    And I run "openCenter cluster init test-service --opencenter.meta.organization=qa-team --force --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/qa-team/infrastructure/clusters/test-service/.test-service-config.yaml" should exist
    And the cluster configuration "test-service" should have "opencenter.meta.organization" set to "qa-team"

  Scenario: Init cluster with organization fails when cluster exists without force
    When I run "openCenter cluster init existing-service --opencenter.meta.organization=ops-team --config-dir <<tmp>>/conf"
    And I run "openCenter cluster init existing-service --opencenter.meta.organization=ops-team --config-dir <<tmp>>/conf"
    Then the exit code should be 1
    And stderr should contain "already exists in organization 'ops-team'"

  Scenario: Init cluster with organization creates separate SOPS keys per cluster
    When I run "openCenter cluster init service-a --opencenter.meta.organization=shared-team --config-dir <<tmp>>/conf"
    And I run "openCenter cluster init service-b --opencenter.meta.organization=shared-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-a-key.txt" should exist
    And a file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-b-key.txt" should exist
    And the file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-a-key.txt" should contain "AGE-SECRET-KEY-1"
    And the file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-b-key.txt" should contain "AGE-SECRET-KEY-1"

  Scenario: Init cluster with organization and no-sops-keygen flag skips key generation
    When I run "openCenter cluster init no-sops-service --opencenter.meta.organization=security-team --no-sops-keygen --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/security-team/infrastructure/clusters/no-sops-service" should exist
    And the file "<<tmp>>/conf/clusters/security-team/secrets/age/keys/no-sops-service-key.txt" should not exist
    And the cluster configuration "no-sops-service" should have "secrets.sops_age_key_file" set to ""