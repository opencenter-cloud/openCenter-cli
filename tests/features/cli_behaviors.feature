# tests/features/cli_behaviors.feature
# End-to-end CLI behaviors:
# - listing clusters
# - setting the active cluster (by name & interactive)
# - describe (active & named) with --output json and --validate
# - init (non-interactive) incl. --strict failures
# - generate (materialization and forced overwrite)
# - deploy (git init/commit/remote/push)
# - validation rules (Octavia/VRRP/Designate/counts/flavors/git_dir)

Feature: CLI core flows and validations

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-dev"
    And an empty directory "<<tmp>>/repo-prod"
    And a file "<<tmp>>/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
          kubernetes:
            master_count: 1
            worker_count: 1
            subnet_pods: "10.42.0.0/16"
            subnet_services: "10.43.0.0/16"
            loadbalancer_provider: octavia
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: ""
        infrastructure:
          provider: openstack
          cloud:
            openstack:
              region: "regionOne"
      """
    And a file "<<tmp>>/conf/prod.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: prod
          kubernetes:
            master_count: 3
            worker_count: 6
            subnet_pods: "10.42.0.0/16"
            subnet_services: "10.43.0.0/16"
            loadbalancer_provider: octavia
        gitops:
          git_dir: "<<tmp>>/repo-prod"
          git_url: ""
        infrastructure:
          provider: openstack
          cloud:
            openstack:
              region: "regionOne"
      """

  # ---------------------------------------------------------------------------
  # LIST
  # ---------------------------------------------------------------------------
  @list
  Scenario: Listing clusters shows names without .yaml
    When I run "opencenter cluster list"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "prod"
    And stdout should not contain ".yaml"

  @list @json
  Scenario: Listing clusters as JSON
    When I run "opencenter cluster ls --output json"
    Then the exit code should be 0
    And stdout should contain '["dev","prod"]'

  # ---------------------------------------------------------------------------
  # SELECT (by name & interactive)
  # ---------------------------------------------------------------------------
  @select @by_name
  Scenario: Selecting a cluster by name
    When I run "opencenter cluster use dev"
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^dev$"

  @select @interactive
  Scenario: Selecting a cluster interactively
    When I run interactively "opencenter cluster use"
    And I choose "prod" from the prompt
    Then the exit code should be 0
    And the file "<<tmp>>/conf/.active" should match regex "^prod$"

  # ---------------------------------------------------------------------------
  # DESCRIBE (active & named) with --output json and --validate
  # ---------------------------------------------------------------------------
  @info @active
  Scenario: Showing info for the active cluster
    Given I run "opencenter cluster use dev"
    And the exit code should be 0
    When I run "opencenter cluster describe"
    Then the exit code should be 0
    And stdout should contain "dev"
    And stdout should contain "git_dir: <<tmp>>/repo-dev"

  @info @json
  Scenario: Showing info for a named cluster with JSON output
    When I run "opencenter cluster describe prod --output json"
    Then the exit code should be 0
    And stdout should contain '"cluster_name": "prod"'
    And stdout should contain '"git_dir": "<<tmp>>/repo-prod"'

  @info @validate
  Scenario: Validating configuration with --validate
    When I run "opencenter cluster describe dev --validate"
    Then the exit code should be 0
    And stdout should contain "Validation successful"
    And stdout should not contain "ERROR"

  # ---------------------------------------------------------------------------
  # INIT (non-interactive), STRICT validation
  # ---------------------------------------------------------------------------
  @init @non_interactive
  Scenario: Non-interactive init creates a minimal skeleton
    When I run "opencenter cluster init test-nonint --force"
    Then the exit code should be 0
    And a file "<<tmp>>/conf/test-nonint.yaml" should exist

  @init @strict @wip
  Scenario: Non-interactive init fails with --strict when required values missing
    # Note: init --strict currently does not validate required fields
    # This test is skipped until validation is implemented
    When I run "opencenter cluster init bad --strict"
    Then the exit code should not be 0
    And stderr should contain "validation failed"

  # ---------------------------------------------------------------------------
  # SETUP (materialization, idempotency, forced overwrite)
  # ---------------------------------------------------------------------------
  @setup @materialize
  Scenario: Setup materializes GitOps template into git_dir
    Given I run "opencenter cluster use dev"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only"
    Then the exit code should be 0
    And the directory "<<tmp>>/repo-dev" should contain a file matching "README.md"
    And the directory "<<tmp>>/repo-dev" should contain a directory "applications"

  @setup @idempotent
  Scenario: Running generate again requires force
    Given I run "opencenter cluster use dev"
    And the exit code should be 0
    And I run "opencenter cluster generate --render-only"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only"
    Then the exit code should not be 0
    And stderr should contain "use --force to overwrite"

  @setup @force
  Scenario: Forced setup overwrites existing files
    Given I run "opencenter cluster use dev"
    And the exit code should be 0
    And a file "<<tmp>>/repo-dev/README.md" with content:
      """
      manual edit that should be replaced
      """
    When I run "opencenter cluster generate --render-only"
    Then the exit code should be 0
    And the file "<<tmp>>/repo-dev/README.md" should not contain "manual edit that should be replaced"

  # ---------------------------------------------------------------------------
  # BOOTSTRAP (git init/commit/remote/push)
  # ---------------------------------------------------------------------------
  @bootstrap @priority5 @wip
  Scenario: Bootstrap pushes the local repo to a remote
    Given a bare git repository exists at "<<tmp>>/remote.git"
    And I update the YAML "<<tmp>>/conf/dev.yaml" to set:
      """
      opencenter:
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: "<<tmp>>/remote.git"
      """
    And I run "opencenter cluster use dev"
    And the exit code should be 0
    And I run "opencenter cluster generate --render-only"
    And the exit code should be 0
    When I run "opencenter cluster deploy"
    Then the exit code should be 0
    And the bare repo "<<tmp>>/remote.git" should have branch "main"

  # (validation scenarios moved to validation.feature)

  @validate @git_dir_missing @priority2
  Scenario: opencenter.gitops.git_dir missing -> error on setup
    Given a file "<<tmp>>/conf/no-gitdir.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: no-gitdir
        gitops:
          git_dir: ""
      """
    When I run "opencenter cluster generate --render-only no-gitdir"
    Then the exit code should not be 0
    And stderr should contain "opencenter.gitops.repository.local_dir must be set"
