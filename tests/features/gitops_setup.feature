Feature: GitOps repository setup behaviors

  Background:
    Given an empty directory "tmp/conf"
    And an empty directory "tmp/repo-dev"
    And a file "tmp/conf/dev.yaml" with content:
      """
      cluster_name: dev
      gitops:
        git_dir: tmp/repo-dev
        git_url: ""
      """

  @gitops @setup @materialize
  Scenario: setup materializes embedded templates into git_dir
    Given I run "openCenter cluster select dev --config-dir tmp/conf"
    And the exit code should be 0
    When I run "openCenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev" should contain a file matching "README.md"
    And stdout should contain "Created GitOps repo"
    And stdout should contain "tmp/repo-dev"

  @gitops @setup @idempotent
  Scenario: setup is idempotent when run repeatedly
    Given I run "openCenter cluster select dev --config-dir tmp/conf"
    And the exit code should be 0
    And I run "openCenter cluster setup --config-dir tmp/conf"
    And the exit code should be 0
    When I run "openCenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And stdout should contain "already initialized"

  @gitops @setup @force
  Scenario: setup --force overwrites existing files
    Given I run "openCenter cluster select dev --config-dir tmp/conf"
    And the exit code should be 0
    And a file "tmp/repo-dev/README.md" with content:
      """
      local edits that should be replaced
      """
    When I run "openCenter cluster setup --force --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/repo-dev/README.md" should not contain "local edits that should be replaced"

  @gitops @setup @missing_prereqs
  Scenario: setup errors when no active cluster or git_dir is missing
    Given the file "tmp/conf/active" does not exist
    When I run "openCenter cluster setup --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "no active cluster"
    Given a file "tmp/conf/nogit.yaml" with content:
      """
      cluster_name: nogit
      iac:
        networking:
          use_designate: false
      """
    When I run "openCenter cluster setup nogit --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "git_dir"
    And stderr should contain "must be set"
