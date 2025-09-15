Feature: Configuration validation rules

  Background:
    Given an empty directory "<<tmp>>/conf"
    And an empty directory "<<tmp>>/repo-bad"

  @validation @missing_git_dir
  Scenario: missing gitops.git_dir -> error
    Given a file "<<tmp>>/conf/mgd.yaml" with content:
      """
      cluster_name: mgd
      """
    When I run "openCenter cluster info mgd --validate"
    Then the exit code should not be 0
    And stderr should contain "gitops.git_dir must be set"

  @validation @opentofu_s3_requires_creds
  Scenario: OpenTofu S3 backend requires credentials -> error then pass
    Given a file "<<tmp>>/conf/s3.yaml" with content:
      """
      cluster_name: s3
      gitops: { git_dir: "<<tmp>>/repo-bad" }
      opentofu:
        enabled: true
        backend:
          type: s3
          s3:
            bucket: b
            key: k
            region: us-east-1
      """
    When I run "openCenter cluster info s3 --validate"
    Then the exit code should not be 0
    And stderr should contain "opencenter.aws_access_key"
    And stderr should contain "opencenter.aws_secret_access_key"

  @validation @s3_with_creds_ok
  Scenario: OpenTofu S3 backend with credentials -> ok
    Given a file "<<tmp>>/conf/s3ok.yaml" with content:
      """
      cluster_name: s3ok
      gitops: { git_dir: "<<tmp>>/repo-bad" }
      opencenter:
        aws_access_key: AKIA...
        aws_secret_access_key: secret
      opentofu:
        enabled: true
        backend:
          type: s3
          s3:
            bucket: b
            key: k
            region: us-east-1
      """
    When I run "openCenter cluster info s3ok --validate"
    Then the exit code should be 0

  # All other legacy iac.* validations removed in the new model.
