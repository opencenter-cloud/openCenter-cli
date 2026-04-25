Feature: Organization-based cluster initialization
  As a user, I want to initialize clusters within organization-based directory structures
  so that I can organize my clusters by team or environment.

  Background:
    Given an empty directory "<<tmp>>/conf"

  Scenario: Init cluster with organization creates organization-based directory structure
    When I run "opencenter cluster init web-app --org dev-team --config-dir <<tmp>>/conf"
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
    When I run "opencenter cluster init api-service --org prod-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/prod-team/.api-service-config.yaml" should exist
    And the cluster configuration "api-service" should have "opencenter.meta.organization" set to "prod-team"
    And the cluster configuration "api-service" should have "opencenter.gitops.git_dir" containing "clusters/prod-team"

  Scenario: Init cluster with organization generates SOPS key in organization structure
    When I run "opencenter cluster init database --org data-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/data-team/secrets/age/keys/database-key.txt" should exist
    And the file "<<tmp>>/conf/clusters/data-team/secrets/age/keys/database-key.txt" should contain "AGE-SECRET-KEY-1"
    And a file "<<tmp>>/conf/clusters/data-team/.sops.yaml" should exist
    And the file "<<tmp>>/conf/clusters/data-team/.sops.yaml" should contain "creation_rules:"
    And the cluster configuration "database" should have "secrets.sops_age_key_file" containing "data-team/secrets/age/keys/database-key.txt"

  Scenario: Init cluster without organization uses cluster name as organization
    When I run "opencenter cluster init legacy-app --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/legacy-app.yaml" should exist

  Scenario: Init multiple clusters in same organization share GitOps root
    When I run "opencenter cluster init frontend --org web-team --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init backend --org web-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/frontend" should exist
    And a directory "<<tmp>>/conf/clusters/web-team/infrastructure/clusters/backend" should exist
    And a file "<<tmp>>/conf/clusters/web-team/.frontend-config.yaml" should exist
    And a file "<<tmp>>/conf/clusters/web-team/.backend-config.yaml" should exist
    And the cluster configuration "frontend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"
    And the cluster configuration "backend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"

  Scenario: Init cluster with organization and force flag overwrites existing
    When I run "opencenter cluster init test-service --org qa-team --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init test-service --org qa-team --force --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/qa-team/.test-service-config.yaml" should exist
    And the cluster configuration "test-service" should have "opencenter.meta.organization" set to "qa-team"

  Scenario: Init cluster with organization fails when cluster exists without force
    When I run "opencenter cluster init existing-service --org ops-team --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init existing-service --org ops-team --config-dir <<tmp>>/conf"
    Then the exit code should be 1
    And stderr should contain "already exists in organization 'ops-team'"

  Scenario: Init cluster with organization creates separate SOPS keys per cluster
    When I run "opencenter cluster init service-a --org shared-team --config-dir <<tmp>>/conf"
    And I run "opencenter cluster init service-b --org shared-team --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-a-key.txt" should exist
    And a file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-b-key.txt" should exist
    And the file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-a-key.txt" should contain "AGE-SECRET-KEY-1"
    And the file "<<tmp>>/conf/clusters/shared-team/secrets/age/keys/service-b-key.txt" should contain "AGE-SECRET-KEY-1"

  Scenario: Init cluster with organization and no-sops-keygen flag skips key generation
    When I run "opencenter cluster init no-sops-service --org security-team --no-sops-keygen --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/security-team/infrastructure/clusters/no-sops-service" should exist
    And the file "<<tmp>>/conf/clusters/security-team/secrets/age/keys/no-sops-service-key.txt" should not exist
    And the cluster configuration "no-sops-service" should have "secrets.sops_age_key_file" set to ""