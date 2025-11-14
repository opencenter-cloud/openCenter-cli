apiVersion: v1
kind: Secret
metadata:
  name: velero-credentials
  namespace: velero
type: Opaque
stringData:
  OS_AUTH_URL: https://keystone.api.sjc3.rackspacecloud.com/v3
  OS_PROJECT_NAME: 981977_Flex
  OS_APPLICATION_CREDENTIAL_ID: a04defc3cbfe4ca7ba71af21bb34c07e
  OS_APPLICATION_CREDENTIAL_SECRET: ivPUe6npxzaP3eKzd5tw4DiTSrhbrGgd8pZveXG8UZ_5Iu44Qt3HnJh9VSe_p4RrtYAzDnHJJmls6LIvSkGAIw
  OS_REGION_NAME: SJC3
  OS_DOMAIN_NAME: rackspace_cloud_domain
