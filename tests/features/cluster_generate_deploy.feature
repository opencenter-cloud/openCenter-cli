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

  @generate @provisioning
  Scenario: Generate with provisioning creates Terraform files
    Given a file "<<tmp>>/conf/dev.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: dev
        gitops:
          git_dir: "<<tmp>>/repo-dev"
        infrastructure:
          cloud:
            openstack:
              image_id: "test-image-id"
      """
    When I run "opencenter cluster generate --render-only dev --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/repo-dev/infrastructure/clusters/dev/main.tf" should exist
    And a file "<<tmp>>/repo-dev/infrastructure/clusters/dev/provider.tf" should exist

  @generate @metallb
  Scenario: Generate MetalLB pool and interface-scoped L2 advertisement
    Given a file "<<tmp>>/conf/metallb-test.yaml" with content:
      """
      opencenter:
        cluster:
          cluster_name: metallb-test
        gitops:
          git_dir: <<tmp>>/repo-dev
        services:
          metallb:
            enabled: true
            namespace: metallb-system
            ip_address_pools:
              - name: public-pool
                addresses:
                  - 72.4.119.48/28
            l2_advertisements:
              - name: public-pool-l2
                ip_address_pools:
                  - public-pool
                interfaces:
                  - metal.105
      """
    When I run "opencenter cluster use metallb-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    When I run "opencenter cluster generate --skip-validation --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/ipaddresspool.yaml" should exist
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/ipaddresspool.yaml" should contain "kind: IPAddressPool"
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/ipaddresspool.yaml" should contain "name: public-pool"
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/ipaddresspool.yaml" should contain "72.4.119.48/28"
    And a file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/l2advertisement.yaml" should exist
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/l2advertisement.yaml" should contain "kind: L2Advertisement"
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/l2advertisement.yaml" should contain "ipAddressPools:"
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/l2advertisement.yaml" should contain "interfaces:"
    And the file "<<tmp>>/repo-dev/applications/overlays/metallb-test/services/metallb/l2advertisement.yaml" should contain "metal.105"

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
    When I update the YAML "<<tmp>>/conf/clusters/blueprints/gitops-org/gitops-test/gitops-test-config.yaml" to set:
      """
      opencenter:
        infrastructure:
          bastion:
            image: "test-bastion-image"
          cloud:
            openstack:
              image_id: "test-image-id"
      """
    When I run "opencenter cluster generate --render-only gitops-test --config-dir <<tmp>>/conf"
    Then the exit code should be 0
    And a directory "<<tmp>>/conf/clusters/gitops-org" should exist
    And a directory "<<tmp>>/conf/clusters/gitops-org/applications" should exist
    And a directory "<<tmp>>/conf/clusters/gitops-org/infrastructure" should exist
