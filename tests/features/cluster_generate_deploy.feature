Feature: GitOps generation and deployment
  Verifies cluster generate (template materialisation), deploy (git push),
  and destroy lifecycle commands.

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-dev"
    And a file "<<tmp>>/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: ""
      """

  # ---------------------------------------------------------------------------
  # Generate (materialise templates into git_dir)
  # ---------------------------------------------------------------------------

  @generate @materialize
  Scenario: Generate materialises embedded templates into git_dir
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the directory "<<tmp>>/repo-dev" should contain a file matching "README.md"
    And the directory "<<tmp>>/repo-dev" should contain a directory "applications"
    And stdout should contain "Render complete"

  @generate @idempotent @priority2
  Scenario: Repeated generate requires force
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I run "opencenter cluster generate --render-only --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "use --force to overwrite"

  @generate @force
  Scenario: Forced generate overwrites existing files
    Given I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And a file "<<tmp>>/repo-dev/README.md" with content:
      """
      manual edit that should be replaced
      """
    When I run "opencenter cluster generate --render-only --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the file "<<tmp>>/repo-dev/README.md" should not contain "manual edit that should be replaced"

  @generate @provisioning
  Scenario: Generate with provisioning creates Terraform files
    Given a file "<<tmp>>/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
        gitops:
          git_dir: "<<tmp>>/repo-dev"
      opentofu:
        enabled: true
      """
    When I run "opencenter cluster generate --render-only dev --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/repo-dev/infrastructure/clusters/dev/main.tf" should exist
    And a file "<<tmp>>/repo-dev/infrastructure/clusters/dev/provider.tf" should exist

  # ---------------------------------------------------------------------------
  # Validation errors during generate
  # ---------------------------------------------------------------------------

  @generate @validate @priority2
  Scenario: Generate fails when git_dir is missing
    Given a file "<<tmp>>/conf/no-gitdir.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: no-gitdir
        gitops:
          git_dir: ""
      """
    When I run "opencenter cluster generate --render-only no-gitdir --config-dir <<tmp>>/conf"
    Then the exit code should not be 0
    And stderr should contain "opencenter.gitops.repository.local_dir must be set"

  # ---------------------------------------------------------------------------
  # Deploy (git init/commit/remote/push)
  # ---------------------------------------------------------------------------

  # @wip triage (2026-04-26): Deploy triggers infrastructure provisioning which
  # requires OpenStack credentials. Kept as @wip until a mock provider or
  # --skip-infrastructure flag is available for deploy.
  @deploy @wip
  Scenario: Deploy pushes the local repo to a remote
    Given a file "<<tmp>>/conf/dev-deploy.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev-deploy
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: ""
      """
    And a bare git repository exists at "<<tmp>>/remote.git"
    And I update the YAML "<<tmp>>/conf/dev-deploy.yaml" to set:
      """
      opencenter:
        gitops:
          git_dir: "<<tmp>>/repo-dev"
          git_url: "file://<<tmp>>/remote.git"
      """
    And I run "opencenter cluster use dev-deploy --config-dir <<tmp>>/conf"
    And the exit code should be 0
    And I run "opencenter cluster generate --render-only --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster deploy --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And the bare repo "<<tmp>>/remote.git" should have branch "main"

  # ---------------------------------------------------------------------------
  # Destroy
  # ---------------------------------------------------------------------------

  @destroy @priority7
  Scenario: Destroy removes config and GitOps directory
    Given an empty directory "<<tmp>>/opencenter-demo"
    And a file "<<tmp>>/conf/demo.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: demo
        gitops:
          git_dir: "<<tmp>>/opencenter-demo"
      """
    When I run "opencenter cluster destroy demo --force --skip-infrastructure --remove-files --config-dir <<tmp>>/conf"
    Then the command should succeed
    And a file "<<tmp>>/conf/demo.yaml" should not exist
    And a directory "<<tmp>>/opencenter-demo" should not exist

  # ---------------------------------------------------------------------------
  # Organization-based generate
  # ---------------------------------------------------------------------------

  @generate @org
  Scenario: Generate works with organization-based structure
    Given I run "opencenter cluster init gitops-test --org gitops-org --config-dir <<tmp>>/conf"
    And the exit code should be 0
    When I run "opencenter cluster generate --render-only gitops-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/gitops-org" should exist
    And a directory "<<tmp>>/conf/clusters/gitops-org/applications" should exist
    And a directory "<<tmp>>/conf/clusters/gitops-org/infrastructure" should exist
