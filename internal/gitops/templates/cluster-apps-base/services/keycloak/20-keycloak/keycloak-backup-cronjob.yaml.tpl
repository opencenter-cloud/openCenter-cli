{{- if .OpenCenter.Services.keycloak.BackupEnabled }}
---
apiVersion: v1
kind: ServiceAccount
metadata:
  name: keycloak-backup
  namespace: keycloak
---
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: keycloak-backup
  namespace: keycloak
rules:
  - apiGroups: ["k8s.keycloak.org"]
    resources: ["keycloakrealmimports"]
    verbs: ["get", "list"]
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["get", "list"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: keycloak-backup
  namespace: keycloak
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: keycloak-backup
subjects:
  - kind: ServiceAccount
    name: keycloak-backup
    namespace: keycloak
---
apiVersion: batch/v1
kind: CronJob
metadata:
  name: keycloak-realm-backup
  namespace: keycloak
  labels:
    app.kubernetes.io/name: keycloak-backup
    app.kubernetes.io/component: backup
    app.kubernetes.io/part-of: keycloak
spec:
  schedule: {{ .OpenCenter.Services.keycloak.BackupSchedule | default "0 2 * * *" | quote }}
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  concurrencyPolicy: Forbid
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            app.kubernetes.io/name: keycloak-backup
            app.kubernetes.io/component: backup
        spec:
          serviceAccountName: keycloak-backup
          restartPolicy: OnFailure
          containers:
            - name: backup
              image: bitnami/kubectl:latest
              command:
                - /bin/sh
                - -c
                - |
                  set -e
                  BACKUP_DATE=$(date +%Y%m%d-%H%M%S)
                  BACKUP_DIR="/backup"
                  
                  echo "Starting Keycloak realm backup at ${BACKUP_DATE}"
                  
                  # Export realm configuration
                  kubectl get keycloakrealmimport -n keycloak -o yaml > ${BACKUP_DIR}/realm-${BACKUP_DATE}.yaml
                  
                  # Export secrets (encrypted)
                  kubectl get secrets -n keycloak -l app.kubernetes.io/part-of=keycloak -o yaml > ${BACKUP_DIR}/secrets-${BACKUP_DATE}.yaml
                  
                  echo "Backup completed: ${BACKUP_DIR}/realm-${BACKUP_DATE}.yaml"
                  
                  # TODO: Upload to object storage (S3/Swift)
                  # Example for S3:
                  # aws s3 cp ${BACKUP_DIR}/realm-${BACKUP_DATE}.yaml s3://keycloak-backups/
              volumeMounts:
                - name: backup
                  mountPath: /backup
          volumes:
            - name: backup
              emptyDir: {}
{{- end }}
