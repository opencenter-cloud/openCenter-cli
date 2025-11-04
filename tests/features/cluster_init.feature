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
