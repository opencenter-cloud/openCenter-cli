# tests/features/integrations.feature
# Verifies scaffolding & docs for Terraform, Pulumi, Secrets; and descriptive error handling.

Feature: Integrations scaffolding and error handling

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
    And I run "opencenter cluster select dev --config-dir tmp/conf"
    And the exit code should be 0

  # ---------------------------------------------------------------------------
  # Terraform: scaffold + docs + make/mise tasks
  # ---------------------------------------------------------------------------
  @terraform @scaffold @wip
  Scenario: Setup includes Terraform scaffold under gitops.git_dir/terraform with documented tasks
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev/terraform" should exist
    And the directory "tmp/repo-dev/terraform" should contain a file matching "README.md|main\\.tf|variables\\.tf"
    # Project-level task docs (either Makefile and/or mise tasks)
    And the directory "tmp/repo-dev" should contain a file matching "(^|/)Makefile$"
    And the directory "tmp/repo-dev" should contain a file matching "(^|/)(\\.?mise\\.toml|mise\\.json)$"
    And the file "tmp/repo-dev/README.md" should contain "mise run terraform"
    And the directory "tmp/repo-dev/docs" should contain a file matching "terraform(\\.md|/index\\.md)$"

  # ---------------------------------------------------------------------------
  # Pulumi: optional scaffold + docs for stacks
  # ---------------------------------------------------------------------------
  @pulumi @scaffold @wip
  Scenario: Setup includes optional Pulumi scaffold and stack configuration docs
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev/infra/pulumi" should exist
    And the directory "tmp/repo-dev/infra/pulumi" should contain a file matching "Pulumi\\.yaml"
    And the directory "tmp/repo-dev/infra/pulumi/stacks" should exist
    And the directory "tmp/repo-dev/infra/pulumi/stacks" should contain a file matching "(dev|default)\\.(ya?ml)$"
    And the directory "tmp/repo-dev/infra/pulumi" should contain a file matching "README\\.md"
    And the file "tmp/repo-dev/README.md" should contain "mise run pulumi"
    And the directory "tmp/repo-dev/docs" should contain a file matching "pulumi(\\.md|/index\\.md)$"

  # ---------------------------------------------------------------------------
  # Secrets: SOPS (age) and Sealed Secrets examples
  # ---------------------------------------------------------------------------
  @secrets @sops @sealedsecrets @wip
  Scenario: Setup provides SOPS and Sealed Secrets examples and guidance
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    # SOPS (age) examples
    And the directory "tmp/repo-dev/secrets/sops" should exist
    And the directory "tmp/repo-dev/secrets/sops" should contain a file matching "README\\.md"
    And the directory "tmp/repo-dev/secrets/sops" should contain a file matching "(example|sample).*\\.(ya?ml)$"
    And the file "tmp/repo-dev/secrets/sops/README.md" should contain "age-keygen"
    And the file "tmp/repo-dev/secrets/sops/README.md" should contain "sops --encrypt"
    # Sealed Secrets examples
    And the directory "tmp/repo-dev/secrets/sealed-secrets" should exist
    And the directory "tmp/repo-dev/secrets/sealed-secrets" should contain a file matching "README\\.md"
    And the directory "tmp/repo-dev/secrets/sealed-secrets" should contain a file matching "(sealedsecret|example).*\\.(ya?ml)$"
    And the file "tmp/repo-dev/secrets/sealed-secrets/README.md" should contain "kubeseal"
    And the file "tmp/repo-dev/secrets/sealed-secrets/README.md" should contain "controller"

  # ---------------------------------------------------------------------------
  # Error handling: descriptive messages and non-zero exits on failures
  # ---------------------------------------------------------------------------
  @errors @infra_collision @wip
  Scenario: Setup fails descriptively if an expected directory path is occupied by a file (infra)
    Given a file "tmp/repo-dev/infra" with content:
      """
      I am a file, not a directory.
      """
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "infra"
    And stderr should contain "not a directory"

  @errors @secrets_collision @wip
  Scenario: Setup fails descriptively if 'secrets' path is a file
    Given a file "tmp/repo-dev/secrets" with content:
      """
      I am a file, not a directory.
      """
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "secrets"
    And stderr should contain "not a directory"

  @errors @unwritable @wip
  Scenario: Setup fails with non-zero code and helpful message when git_dir is not writable
    Given I update the YAML "tmp/conf/dev.yaml" to set:
      """
      opencenter:
        gitops:
          git_dir: /root/forbidden-path
      """
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should not be 0
    And stderr should contain "git_dir"
    And stderr should contain "permission"
    And stderr should contain "writable"

  @terraform @disabled @wip
  Scenario: Terraform scaffold is omitted when terraform.enabled is false
    Given I update the YAML "tmp/conf/dev.yaml" to set:
      """
      terraform:
        enabled: false
      """
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev/infra/terraform" should not exist

  @pulumi @enabled_gate @wip
  Scenario: Pulumi scaffold appears only when pulumi.enabled is true
    Given I update the YAML "tmp/conf/dev.yaml" to set:
      """
      pulumi:
        enabled: false
      """
    When I run "opencenter cluster setup --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev/infra/pulumi" should not exist
    When I update the YAML "tmp/conf/dev.yaml" to set:
      """
      pulumi:
        enabled: true
      """
    And I run "opencenter cluster setup --force --config-dir tmp/conf"
    Then the exit code should be 0
    And the directory "tmp/repo-dev/infra/pulumi" should exist
