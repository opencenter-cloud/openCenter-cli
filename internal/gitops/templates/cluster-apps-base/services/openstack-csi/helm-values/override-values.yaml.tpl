secret:
  enabled: true
  hostMount: true
  create: true
  filename: cloud.conf
  name: cinder-csi-cloud-config
  data:
    cloud.conf: |-
      [Global]
      auth-url = https://keystone.api.sjc3.rackspacecloud.com/v3
      application-credential-id = a04defc3cbfe4ca7ba71af21bb34c07e
      application-credential-secret = ivPUe6npxzaP3eKzd5tw4DiTSrhbrGgd8pZveXG8UZ_5Iu44Qt3HnJh9VSe_p4RrtYAzDnHJJmls6LIvSkGAIw
      domain-name = rackspace_cloud_domain
      region = SJC3
      tenant-name = 981977_Flex
      tls-insecure =  false
