# tests/features/cli_behaviors.feature
# End-to-end CLI behaviors:
# - listing clusters
# - selecting (by name & interactive)
# - info (active & named) with --json and --validate
# - init (non-interactive) incl. --strict failures
# - setup (materialization, idempotency, forced overwrite)
# - bootstrap (git init/commit/remote/push)
# - validation rules (Octavia/VRRP/Designate/counts/flavors/git_dir)

Feature: CLI core flows and validations

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-dev"
    And an empty directory "<<tmp>>/repo-prod"
    And a file "<<tmp>>/conf/dev.yaml" with content:
      """
      cluster_name: dev
      gitops:
        git_dir: "<<tmp>>/repo-dev"
        git_url: ""
      iac:
        counts: { master: 1, worker: 1 }
        flavors: { master: "m1.large", worker: "m1.large" }
        networking:
          use_octavia: true
          vrrp_enabled: false
          use_designate: false
          dns_nameservers: ["8.8.8.8","8.8.4.4"]
          subnet_nodes: "10.0.0.0/16"
          subnet_pods: "10.42.0.0/16"
          subnet_services: "10.43.0.0/16"
      """
    And a file "<<tmp>>/conf/prod.yaml" with content:
      """
      cluster_name: prod
      gitops:
        git_dir: "<<tmp>>/repo-prod"
        git_url: ""
      iac:
        counts: { master: 3, worker: 6 }
        flavors: { master: "m2.xlarge", worker: "m2.large" }
        networking:
          use_octavia: true
          vrrp_enabled: false
          use_designate: false
          dns_nameservers: ["1.1.1.1","8.8.8.8"]
          subnet_nodes: "10.1.0.0/16"
          subnet_pods: "10.42.0.0/16"
          subnet_services: "10.43.0.0/16"
      """

  # ---------------------------------------------------------------------------
  # LIST
  # ---------------------------------------------------------------------------
  @list
  Scenario: Listing clusters shows names without .yaml
    When I run "openCenter cluster list"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "prod"
    And stdout should not contain ".yaml"

  @list @json
  Scenario: Listing clusters as JSON
    When I run "openCenter cluster ls --json"
    Then the exit code should be 0
    And stdout should contain '["dev","prod"]'

  # ---------------------------------------------------------------------------
  # SELECT (by name & interactive)
  # ---------------------------------------------------------------------------
  @select @by_name
  Scenario: Selecting a cluster by name
    When I run "openCenter cluster select dev"
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^dev$"

  @select @interactive
  Scenario: Selecting a cluster interactively
    When I run interactively "openCenter cluster select"
    And I choose "prod" from the prompt
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^prod$"

  # ---------------------------------------------------------------------------
  # INFO (active & named) with --json and --validate
  # ---------------------------------------------------------------------------
  @info @active
  Scenario: Showing info for the active cluster
    Given I run "openCenter cluster select dev"
    And the exit code should be 0
    When I run "openCenter cluster info"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "git_dir: <<tmp>>/repo-dev"

  @info @json
  Scenario: Showing info for a named cluster with JSON output
    When I run "openCenter cluster info prod --json"
    Then the exit code should be 0
    And stdout should contain '"cluster_name":"prod"'
    And stdout should contain '"git_dir":"<<tmp>>/repo-prod"'

  @info @validate
  Scenario: Validating configuration with --validate
    When I run "openCenter cluster info dev --validate"
    Then the exit code should be 0
    And stdout should contain "Validation successful."
    And stdout should not contain "ERROR"

  # ---------------------------------------------------------------------------
  # INIT (non-interactive), STRICT validation
  # ---------------------------------------------------------------------------
  @init @non_interactive
  Scenario: Non-interactive init creates a minimal skeleton
    When I run "openCenter cluster init test-nonint --force"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/test-nonint.yaml" should exist

  @init @strict
  Scenario: Non-interactive init fails with --strict when required values missing
    When I run "openCenter cluster init bad --strict"
    Then the exit code should not be 0
    And stderr should contain "gitops.git_dir must be set"

  # ---------------------------------------------------------------------------
  # SETUP (materialization, idempotency, forced overwrite)
  # ---------------------------------------------------------------------------
  @setup @materialize
  Scenario: Setup materializes GitOps template into git_dir
    Given I run "openCenter cluster select dev"
    And the exit code should be 0
    When I run "openCenter cluster setup"
    Then the exit code should be 0
    And the directory "<<tmp>>/repo-dev" should contain a file matching "README.md"
    And the directory "<<tmp>>/repo-dev" should contain a directory "applications"

  @setup @idempotent
  Scenario: Running setup again is idempotent
    Given I run "openCenter cluster select dev"
    And the exit code should be 0
    And I run "openCenter cluster setup"
    And the exit code should be 0
    When I run "openCenter cluster setup"
    Then the exit code should be 0
    And stdout should contain "already initialized"

  @setup @force
  Scenario: Forced setup overwrites existing files
    Given I run "openCenter cluster select dev"
    And the exit code should be 0
    And a file "<<tmp>>/repo-dev/README.md" with content:
      """
      manual edit that should be replaced
      """
    When I run "openCenter cluster setup --force"
    Then the exit code should be 0
    And the file "<<tmp>>/repo-dev/README.md" should not contain "manual edit that should be replaced"

  # ---------------------------------------------------------------------------
  # BOOTSTRAP (git init/commit/remote/push)
  # ---------------------------------------------------------------------------
  @bootstrap
  Scenario: Bootstrap pushes the local repo to a remote
    Given a bare git repository exists at "<<tmp>>/remote.git"
    And I update the YAML "<<tmp>>/conf/dev.yaml" to set:
      """
      gitops:
        git_dir: "<<tmp>>/repo-dev"
        git_url: "<<tmp>>/remote.git"
      """
    And I run "openCenter cluster select dev"
    And the exit code should be 0
    And I run "openCenter cluster setup"
    And the exit code should be 0
    When I run "openCenter cluster bootstrap"
    Then the exit code should be 0
    And the bare repo "<<tmp>>/remote.git" should have branch "main"

  # (validation scenarios moved to validation.feature)

  @validate @git_dir_missing
  Scenario: gitops.git_dir missing -> error on setup
    Given a file "<<tmp>>/conf/no-gitdir.yaml" with content:
      """
      cluster_name: no-gitdir
      gitops:
        git_dir: ""
      iac:
        counts: {}
        flavors: {}
        networking: { use_octavia: true, vrrp_enabled: false, use_designate: false }
      """
    When I run "openCenter cluster setup no-gitdir"
    Then the exit code should not be 0
    And stderr should contain "gitops.git_dir must be set"
