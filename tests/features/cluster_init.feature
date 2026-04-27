Feature: Cluster initialisation
  As a user, I want to initialise a new cluster configuration using
  the `init` command, so that I can start defining my cluster layout.

  # ---------------------------------------------------------------------------
  # Default initialisation (no --org flag)
  # ---------------------------------------------------------------------------

  @init @defaults @priority8
  Scenario: Initialise a new cluster with default settings
    When I run "opencenter cluster init test-cluster"
    Then a cluster configuration "test-cluster" should exist
    And the cluster configuration "test-cluster" should have "opencenter.cluster.cluster_name" set to "test-cluster"
    And the file should not contain "local."

  @init @defaults
  Scenario: Initialise a cluster with default GitOps and compute settings
    When I run "opencenter cluster init test-cluster"
    Then a cluster configuration "test-cluster" should exist
    And the cluster configuration "test-cluster" should have "opencenter.gitops.git_dir" containing "clusters/opencenter"
    And the cluster configuration "test-cluster" should have "opencenter.infrastructure.compute.master_count" set to "3"

  @init @defaults
  Scenario: Init without organization uses opencenter as default organization
    When I run "opencenter cluster init legacy-app"
    Then a directory "~/.config/opencenter/clusters/opencenter" should exist
    And a directory "~/.config/opencenter/clusters/opencenter/infrastructure/clusters/legacy-app" should exist
    And a file "~/.config/opencenter/clusters/opencenter/.legacy-app-config.yaml" should exist
    And the cluster configuration "legacy-app" should have "opencenter.meta.organization" set to "opencenter"

  @init @full_schema @priority8
  Scenario: Init with full schema uses valid v2 template
    When I run "opencenter cluster init full-one --full-schema"
    Then a cluster configuration "full-one" should exist
    And the file should not contain "local."
    And the file should not contain "iac:"

  # ---------------------------------------------------------------------------
  # Directory structure creation
  # ---------------------------------------------------------------------------

  @init @directory_structure
  Scenario: Init creates clusters subdirectory and cluster directory structure
    When I run "opencenter cluster init new-cluster"
    Then a directory "~/.config/opencenter/clusters" should exist
    And a directory "~/.config/opencenter/clusters/opencenter" should exist
    And a directory "~/.config/opencenter/clusters/opencenter/infrastructure/clusters/new-cluster" should exist
    And a file "~/.config/opencenter/clusters/opencenter/.new-cluster-config.yaml" should exist

  @init @directory_structure
  Scenario: Init creates cluster-specific secrets directory structure
    When I run "opencenter cluster init secrets-test"
    Then a directory "~/.config/opencenter/clusters/opencenter/secrets" should exist
    And a directory "~/.config/opencenter/clusters/opencenter/secrets/age" should exist
    And a directory "~/.config/opencenter/clusters/opencenter/secrets/age/keys" should exist
    And a file "~/.config/opencenter/clusters/opencenter/secrets/age/keys/secrets-test-key.txt" should exist

  @init @directory_structure
  Scenario: Cluster directory creation with special characters in name
    When I run "opencenter cluster init test-cluster-123"
    Then a directory "~/.config/opencenter/clusters/opencenter/infrastructure/clusters/test-cluster-123" should exist
    And a file "~/.config/opencenter/clusters/opencenter/.test-cluster-123-config.yaml" should exist

  # ---------------------------------------------------------------------------
  # SOPS key generation
  # ---------------------------------------------------------------------------

  @init @sops
  Scenario: Init generates a SOPS key when not provided
    When I run "opencenter cluster init demo"
    Then a file "~/.config/opencenter/clusters/opencenter/secrets/age/keys/demo-key.txt" should exist
    And the file "~/.config/opencenter/clusters/opencenter/secrets/age/keys/demo-key.txt" should contain "AGE-SECRET-KEY-1"

  @init @sops
  Scenario: Init does not generate a SOPS key when disabled
    When I run "opencenter cluster init demo2 --no-sops-keygen"
    Then the file "~/.config/opencenter/clusters/opencenter/secrets/age/keys/demo2-key.txt" should not exist
    And the cluster configuration "demo2" should have "secrets.sops_age_key_file" set to ""

  @init @sops
  Scenario: SOPS key generation uses cluster-specific directory
    When I run "opencenter cluster init sops-dir-test"
    Then a file "~/.config/opencenter/clusters/opencenter/secrets/age/keys/sops-dir-test-key.txt" should exist
    And the cluster configuration "sops-dir-test" should have "secrets.sops_age_key_file" containing "clusters/opencenter/secrets/age/keys/sops-dir-test-key.txt"

  # ---------------------------------------------------------------------------
  # Force and overwrite behaviour
  # ---------------------------------------------------------------------------

  @init @force
  Scenario: Force flag overwrites existing cluster directory
    When I run "opencenter cluster init force-test"
    And I run "opencenter cluster init force-test --force"
    Then the command should succeed
    And a cluster configuration "force-test" should exist

  @init @force
  Scenario: Init fails when cluster directory exists without force flag
    When I run "opencenter cluster init existing-test"
    And I run "opencenter cluster init existing-test"
    Then exit code should be 1
    And stderr should contain "already exists"

  # ---------------------------------------------------------------------------
  # Configuration loading after init
  # ---------------------------------------------------------------------------

  @init @loading
  Scenario: Configuration loading works with new directory structure
    When I run "opencenter cluster init load-test"
    And I run "opencenter cluster use load-test"
    Then the active cluster should be "load-test"
    And the command should succeed

  # ---------------------------------------------------------------------------
  # Organization-based initialisation (--org flag)
  # ---------------------------------------------------------------------------

  @init @org @directory_structure
  Scenario: Init with organization creates full directory structure
    When I run "opencenter cluster init web-app --org dev-team"
    Then a directory "~/.config/opencenter/clusters/dev-team" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/infrastructure" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/infrastructure/clusters" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/infrastructure/clusters/web-app" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/applications" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/applications/overlays" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/applications/overlays/web-app" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/secrets" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/secrets/age" should exist
    And a directory "~/.config/opencenter/clusters/dev-team/secrets/age/keys" should exist

  @init @org
  Scenario: Init with organization creates config in correct location
    When I run "opencenter cluster init api-service --org prod-team"
    Then a file "~/.config/opencenter/clusters/prod-team/.api-service-config.yaml" should exist
    And the cluster configuration "api-service" should have "opencenter.meta.organization" set to "prod-team"
    And the cluster configuration "api-service" should have "opencenter.gitops.git_dir" containing "clusters/prod-team"

  @init @org @sops
  Scenario: Init with organization generates SOPS key in organization structure
    When I run "opencenter cluster init database --org data-team"
    Then a file "~/.config/opencenter/clusters/data-team/secrets/age/keys/database-key.txt" should exist
    And the file "~/.config/opencenter/clusters/data-team/secrets/age/keys/database-key.txt" should contain "AGE-SECRET-KEY-1"
    And a file "~/.config/opencenter/clusters/data-team/.sops.yaml" should exist
    And the file "~/.config/opencenter/clusters/data-team/.sops.yaml" should contain "creation_rules:"
    And the cluster configuration "database" should have "secrets.sops_age_key_file" containing "data-team/secrets/age/keys/database-key.txt"

  @init @org
  Scenario: Multiple clusters in same organization share GitOps root
    When I run "opencenter cluster init frontend --org web-team"
    And I run "opencenter cluster init backend --org web-team"
    Then a directory "~/.config/opencenter/clusters/web-team/infrastructure/clusters/frontend" should exist
    And a directory "~/.config/opencenter/clusters/web-team/infrastructure/clusters/backend" should exist
    And a file "~/.config/opencenter/clusters/web-team/.frontend-config.yaml" should exist
    And a file "~/.config/opencenter/clusters/web-team/.backend-config.yaml" should exist
    And the cluster configuration "frontend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"
    And the cluster configuration "backend" should have "opencenter.gitops.git_dir" containing "clusters/web-team"

  @init @org @force
  Scenario: Init with organization and force flag overwrites existing
    When I run "opencenter cluster init test-service --org qa-team"
    And I run "opencenter cluster init test-service --org qa-team --force"
    Then the command should succeed
    And a file "~/.config/opencenter/clusters/qa-team/.test-service-config.yaml" should exist
    And the cluster configuration "test-service" should have "opencenter.meta.organization" set to "qa-team"

  @init @org @force
  Scenario: Init with organization fails when cluster exists without force
    When I run "opencenter cluster init existing-service --org ops-team"
    And I run "opencenter cluster init existing-service --org ops-team"
    Then exit code should be 1
    And stderr should contain "already exists in organization 'ops-team'"

  @init @org @sops
  Scenario: Init with organization creates separate SOPS keys per cluster
    When I run "opencenter cluster init service-a --org shared-team"
    And I run "opencenter cluster init service-b --org shared-team"
    Then a file "~/.config/opencenter/clusters/shared-team/secrets/age/keys/service-a-key.txt" should exist
    And a file "~/.config/opencenter/clusters/shared-team/secrets/age/keys/service-b-key.txt" should exist
    And the file "~/.config/opencenter/clusters/shared-team/secrets/age/keys/service-a-key.txt" should contain "AGE-SECRET-KEY-1"
    And the file "~/.config/opencenter/clusters/shared-team/secrets/age/keys/service-b-key.txt" should contain "AGE-SECRET-KEY-1"

  @init @org @sops
  Scenario: Init with organization and no-sops-keygen flag skips key generation
    When I run "opencenter cluster init no-sops-service --org security-team --no-sops-keygen"
    Then a directory "~/.config/opencenter/clusters/security-team/infrastructure/clusters/no-sops-service" should exist
    And the file "~/.config/opencenter/clusters/security-team/secrets/age/keys/no-sops-service-key.txt" should not exist
    And the cluster configuration "no-sops-service" should have "secrets.sops_age_key_file" set to ""

  @init @org
  Scenario: Init with organization validates organization name in config
    When I run "opencenter cluster init validation-test --org validation-team --strict"
    Then the command should succeed
    And the cluster configuration "validation-test" should have "opencenter.meta.organization" set to "validation-team"
