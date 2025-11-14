apiVersion: rbacmanager.reactiveops.io/v1beta1
kind: RBACDefinition
metadata:
  name: rbac-manager-users
rbacBindings:
  - name: cluster-admin
    subjects:
      - kind: Group
        name: "oidc:admins"
    clusterRoleBindings:
      - clusterRole: cluster-admin
  - name: readonly
    subjects:
      - kind: Group
        name: "oidc:viewers"
    clusterRoleBindings:
      - clusterRole: view
