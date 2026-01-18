Feature: Cluster initialisation
  As a user, I want to initialise a new cluster configuration using
  the `init` command, so that I can start defining my cluster layout.

  @priority8
  Scenario: Initialise a new cluster with default settings
    When I run "openCenter cluster init test-cluster"
    Then a cluster configuration "test-cluster" should exist
    And the cluster configuration "test-cluster" should have "opencenter.cluster.cluster_name" set to "test-cluster"
    And the file should not contain "local."

  Scenario: Initialise a cluster and override string settings from flags
    When I run "openCenter cluster init test-cluster --opencenter.gitops.git_dir=/opt/openCenter/test-cluster --opencenter.cluster.kubernetes.master_count=5"
    Then a cluster configuration "test-cluster" should exist
    And the cluster configuration "test-cluster" should have "opencenter.gitops.git_dir" set to "/opt/openCenter/test-cluster"
    And the cluster configuration "test-cluster" should have "opencenter.cluster.kubernetes.master_count" set to "5"

  # iac.* internals are not settable via flags in the new model (only iac.main_tf).

  # Removed: legacy IAC fields (counts, networking) no longer exist.

  Scenario: Init generates a SOPS key when not provided
    When I run "openCenter cluster init demo --opencenter.gitops.git_dir=<<tmp>>/repo-demo"
    Then a file "~/.config/openCenter/clusters/opencenter/secrets/age/keys/demo-key.txt" should exist
    And the file "~/.config/openCenter/clusters/opencenter/secrets/age/keys/demo-key.txt" should contain "AGE-SECRET-KEY-1"

  Scenario: Init does not generate a SOPS key when disabled
    When I run "openCenter cluster init demo2 --opencenter.gitops.git_dir=<<tmp>>/repo-demo2 --no-sops-keygen"
    Then the file "~/.config/openCenter/clusters/opencenter/secrets/age/keys/demo2-key.txt" should not exist
    And the cluster configuration "demo2" should have "secrets.sops_age_key_file" set to ""

  @priority8
  Scenario: Init with full schema includes local references
    When I run "openCenter cluster init full-one --full-schema"
    Then a cluster configuration "full-one" should exist
    And the file should contain "local."

  # New directory structure behavior tests

  Scenario: Init creates clusters subdirectory and cluster directory structure
    When I run "openCenter cluster init new-cluster"
    Then a directory "~/.config/openCenter/clusters" should exist
    And a directory "~/.config/openCenter/clusters/opencenter" should exist
    And a directory "~/.config/openCenter/clusters/opencenter/infrastructure/clusters/new-cluster" should exist
    And a file "~/.config/openCenter/clusters/opencenter/.new-cluster-config.yaml" should exist

  Scenario: Init creates cluster-specific secrets directory structure
    When I run "openCenter cluster init secrets-test"
    Then a directory "~/.config/openCenter/clusters/opencenter/secrets" should exist
    And a directory "~/.config/openCenter/clusters/opencenter/secrets/age" should exist
    And a directory "~/.config/openCenter/clusters/opencenter/secrets/age/keys" should exist
    And a file "~/.config/openCenter/clusters/opencenter/secrets/age/keys/secrets-test-key.txt" should exist

  Scenario: Force flag overwrites existing cluster directory
    When I run "openCenter cluster init force-test"
    And I run "openCenter cluster init force-test --force"
    Then the command should succeed
    And a cluster configuration "force-test" should exist

  Scenario: Init fails when cluster directory exists without force flag
    When I run "openCenter cluster init existing-test"
    And I run "openCenter cluster init existing-test"
    Then exit code should be 1
    And stderr should contain "already exists"

  Scenario: Configuration loading works with new directory structure only
    When I run "openCenter cluster init load-test"
    And I run "openCenter cluster select load-test"
    Then the active cluster should be "load-test"
    And the command should succeed

  Scenario: SOPS key generation uses cluster-specific directory
    When I run "openCenter cluster init sops-dir-test"
    Then a file "~/.config/openCenter/clusters/opencenter/secrets/age/keys/sops-dir-test-key.txt" should exist
    And the cluster configuration "sops-dir-test" should have "secrets.sops_age_key_file" containing "clusters/opencenter/secrets/age/keys/sops-dir-test-key.txt"

  Scenario: Cluster directory creation with special characters in name
    When I run "openCenter cluster init test-cluster-123"
    Then a directory "~/.config/openCenter/clusters/opencenter/infrastructure/clusters/test-cluster-123" should exist
    And a file "~/.config/openCenter/clusters/opencenter/.test-cluster-123-config.yaml" should exist

  # Organization-based cluster initialization tests

  Scenario: Init cluster with organization creates organization-based directory structure
    When I run "openCenter cluster init web-app --opencenter.meta.organization=dev-team"
    Then a directory "~/.config/openCenter/clusters/dev-team" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/infrastructure" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/infrastructure/clusters" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/infrastructure/clusters/web-app" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/applications" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/applications/overlays" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/applications/overlays/web-app" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/secrets" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/secrets/age" should exist
    And a directory "~/.config/openCenter/clusters/dev-team/secrets/age/keys" should exist

  Scenario: Init cluster with organization creates cluster configuration in correct location
    When I run "openCenter cluster init api-service --opencenter.meta.organization=prod-team"
    Then a file "~/.config/openCenter/clusters/prod-team/.api-service-config.yaml" should exist
    And the cluster configuration "api-service" should have "opencenter.meta.organization" set to "prod-team"
    And the cluster configuration "api-service" should have "opencenter.gitops.git_dir" containing "clusters/prod-team"

  Scenario: Init cluster with organization generates SOPS key in organization structure
    When I run "openCenter cluster init database --opencenter.meta.organization=data-team"
    Then a file "~/.config/openCenter/clusters/data-team/secrets/age/keys/database-key.txt" should exist
    And the file "~/.config/openCenter/clusters/data-team/secrets/age/keys/database-key.txt" should contain "AGE-SECRET-KEY-1"
    And a file "~/.config/openCenter/clusters/data-team/.sops.yaml" should exist
    And the file "~/.config/openCenter/clusters/data-team/.sops.yaml" should contain "creation_rules:"
    And the cluster configuration "database" should have "secrets.sops_age_key_file" containing "data-team/secrets/age/keys/database-key.txt"

  Scenario: Init cluster without organization uses opencenter organization
    When I run "openCenter cluster init legacy-app"
    Then a directory "~/.config/openCenter/clusters/opencenter" should exist
    And a directory "~/.config/openCenter/clusters/opencenter/infrastructure/clusters/legacy-app" should exist
    And a file "~/.config/openCenter/clusters/opencenter/.legacy-app-config.yaml" should exist
    And the cluster configuration "legacy-app" should have "opencenter.meta.organization" set to "opencenter"

  Scenario: Init multiple clusters in same organization share GitOps root
    When I run "openCenter cluster init frontend --opencenter.meta.organization=web-team"
    And I run "openCenter cluster init backend --opencenter.meta.organization=web-team"
    Then a directory "~/.config/openCenter/clusters/web-team/infrastructure/clusters/frontend" should exist
    And a directory "~/.config/openCenter/clusters/web-team/infrastructure/clusters/backend" should exist
    And a file "~/.config/openCenter/clusters/web-team/.frontend-config.yaml" should exist
    And a file "~/.config/openCenter/clusters/web-team/.backend-config.yaml" should exist
    And the cluster configuration "frontend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"
    And the cluster configuration "backend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"

  Scenario: Init cluster with organization and force flag overwrites existing
    When I run "openCenter cluster init test-service --opencenter.meta.organization=qa-team"
    And I run "openCenter cluster init test-service --opencenter.meta.organization=qa-team --force"
    Then the command should succeed
    And a file "~/.config/openCenter/clusters/qa-team/.test-service-config.yaml" should exist
    And the cluster configuration "test-service" should have "opencenter.meta.organization" set to "qa-team"

  Scenario: Init cluster with organization fails when cluster exists without force
    When I run "openCenter cluster init existing-service --opencenter.meta.organization=ops-team"
    And I run "openCenter cluster init existing-service --opencenter.meta.organization=ops-team"
    Then exit code should be 1
    And stderr should contain "already exists in organization 'ops-team'"

  Scenario: Init cluster with organization creates separate SOPS keys per cluster
    When I run "openCenter cluster init service-a --opencenter.meta.organization=shared-team"
    And I run "openCenter cluster init service-b --opencenter.meta.organization=shared-team"
    Then a file "~/.config/openCenter/clusters/shared-team/secrets/age/keys/service-a-key.txt" should exist
    And a file "~/.config/openCenter/clusters/shared-team/secrets/age/keys/service-b-key.txt" should exist
    And the file "~/.config/openCenter/clusters/shared-team/secrets/age/keys/service-a-key.txt" should contain "AGE-SECRET-KEY-1"
    And the file "~/.config/openCenter/clusters/shared-team/secrets/age/keys/service-b-key.txt" should contain "AGE-SECRET-KEY-1"

  Scenario: Init cluster with organization and no-sops-keygen flag skips key generation
    When I run "openCenter cluster init no-sops-service --opencenter.meta.organization=security-team --no-sops-keygen"
    Then a directory "~/.config/openCenter/clusters/security-team/infrastructure/clusters/no-sops-service" should exist
    And the file "~/.config/openCenter/clusters/security-team/secrets/age/keys/no-sops-service-key.txt" should not exist
    And the cluster configuration "no-sops-service" should have "secrets.sops_age_key_file" set to ""

  Scenario: Init cluster with organization validates organization name in config
    When I run "openCenter cluster init validation-test --opencenter.meta.organization=validation-team --strict"
    Then the command should succeed
    And the cluster configuration "validation-test" should have "opencenter.meta.organization" set to "validation-team"
