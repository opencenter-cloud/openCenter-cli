apiVersion: v1
kind: Secret
metadata:
    name: cloud-credentials
    namespace: velero
type: Opaque
stringData:
    OS_AUTH_URL: https://keystone.api.{{ .OpenCenter.OpenStack.Region }}.rackspacecloud.com/v3
    OS_PROJECT_NAME: {{ .OpenCenter.OpenStack.ProjectName }}
    OS_APPLICATION_CREDENTIAL_ID: {{ .OpenCenter.Secrets.Velero.AppCredID }}
    OS_APPLICATION_CREDENTIAL_SECRET: {{ .OpenCenter.Secrets.Velero.AppCredSecret }}
    OS_REGION_NAME: {{ .OpenCenter.OpenStack.Region }}
    OS_DOMAIN_NAME: {{ .OpenCenter.OpenStack.DomainName }}
    OS_SWIFT_TEMP_URL_KEY: {{ .OpenCenter.Secrets.Velero.TempURLKey }}
    OS_SWIFT_TEMP_URL_DIGEST: sha256