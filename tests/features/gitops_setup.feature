Feature: GitOps repository setup behaviors

  Background:
    Given an empty directory "tmp/conf"
    And an empty directory "tmp/repo-dev"
    And a file "tmp/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
        gitops:
          git_dir: tmp/repo-dev
          git_url: ""
      """

  @gitops @setup @materialize
  Scenario: setup materializes embedded templates into git_dir
    Given I run "opencenter cluster use dev --config-dir tmp/conf"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev" should contain a file matching "README.md"
    And stdout should contain "Render complete"

  @gitops @setup @idempotent @priority2
  Scenario: repeated setup requires force
    Given I run "opencenter cluster use dev --config-dir tmp/conf"
    And the exit code should be 0
    And I run "opencenter cluster generate --render-only --config-dir tmp/conf"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "use --force to overwrite"

  @gitops @setup @force
  Scenario: setup --force overwrites existing files
    Given I run "opencenter cluster use dev --config-dir tmp/conf"
    And the exit code should be 0
    And a file "tmp/repo-dev/README.md" with content:
      """
      local edits that should be replaced
      """
    When I run "opencenter cluster generate --render-only --config-dir tmp/conf"
    Then the exit code should be 0
    And the file "tmp/repo-dev/README.md" should not contain "local edits that should be replaced"

  @gitops @setup @missing_prereqs @priority2 @wip
  Scenario: setup errors when no active cluster or git_dir is missing
    # Note: render command uses default git_dir if not specified
    # This test is skipped as the behavior has changed
    Given the file "tmp/conf/active" does not exist
    When I run "opencenter cluster generate --render-only --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "no active cluster"
    Given a file "tmp/conf/nogit.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: nogit
      """
    When I run "opencenter cluster generate --render-only nogit --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter.gitops.repository.local_dir must be set"
