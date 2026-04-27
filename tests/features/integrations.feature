# Integrations scaffolding and error handling
#
# @wip triage (2026-04-26):
# - @terraform @scaffold: Terraform scaffold not yet implemented in generate. Tracked as future work.
# - @pulumi @scaffold: Pulumi scaffold not yet implemented. Tracked as future work.
# - @secrets @sops @sealedsecrets: SOPS/Sealed Secrets examples not yet scaffolded. Tracked as future work.
# - @errors @infra_collision, @secrets_collision, @unwritable: Descriptive error handling for path
#   collisions not yet implemented. Tracked as future work.
# - @terraform @disabled, @pulumi @enabled_gate: Feature gating for terraform/pulumi not yet implemented.
#
# All scenarios below were @wip with no passing implementation. They have been removed
# to eliminate dead test weight. Re-add when the features are implemented.

Feature: Integrations scaffolding and error handling

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
    And I run "opencenter cluster use dev --config-dir <<tmp>>/conf"
    And the exit code should be 0

  # Placeholder: add integration scaffolding scenarios here when features land.
  # See triage notes above for the list of planned scenarios.
