Feature: Cluster initialisation
  As a user, I want to initialise a new cluster configuration using
  the `init` command, so that I can start defining my cluster layout.

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
    Then a file "~/.config/openCenter/clusters/demo/secrets/age/keys/demo-key.txt" should exist
    And the file "~/.config/openCenter/clusters/demo/secrets/age/keys/demo-key.txt" should contain "AGE-SECRET-KEY-1"

  Scenario: Init does not generate a SOPS key when disabled
    When I run "openCenter cluster init demo2 --opencenter.gitops.git_dir=<<tmp>>/repo-demo2 --no-sops-keygen"
    Then the file "~/.config/openCenter/clusters/demo2/secrets/age/keys/demo2-key.txt" should not exist
    And the cluster configuration "demo2" should have "secrets.sops_age_key_file" set to ""

  Scenario: Init with full schema includes local references
    When I run "openCenter cluster init full-one --full-schema"
    Then a cluster configuration "full-one" should exist
    And the file should contain "local."

  # New directory structure behavior tests

  Scenario: Init creates clusters subdirectory and cluster directory structure
    When I run "openCenter cluster init new-cluster"
    Then a directory "~/.config/openCenter/clusters" should exist
    And a directory "~/.config/openCenter/clusters/new-cluster" should exist
    And a file "~/.config/openCenter/clusters/new-cluster/.new-cluster-config.yaml" should exist

  Scenario: Init creates cluster-specific secrets directory structure
    When I run "openCenter cluster init secrets-test"
    Then a directory "~/.config/openCenter/clusters/secrets-test/secrets" should exist
    And a directory "~/.config/openCenter/clusters/secrets-test/secrets/age" should exist
    And a directory "~/.config/openCenter/clusters/secrets-test/secrets/age/keys" should exist
    And a file "~/.config/openCenter/clusters/secrets-test/secrets/age/keys/secrets-test-key.txt" should exist

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
    Then a file "~/.config/openCenter/clusters/sops-dir-test/secrets/age/keys/sops-dir-test-key.txt" should exist
    And the cluster configuration "sops-dir-test" should have "secrets.sops_age_key_file" containing "clusters/sops-dir-test/secrets/age/keys/sops-dir-test-key.txt"

  Scenario: Cluster directory creation with special characters in name
    When I run "openCenter cluster init test-cluster-123"
    Then a directory "~/.config/openCenter/clusters/test-cluster-123" should exist
    And a file "~/.config/openCenter/clusters/test-cluster-123/.test-cluster-123-config.yaml" should exist
