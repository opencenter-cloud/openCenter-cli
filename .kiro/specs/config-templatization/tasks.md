# Implementation Plan

This document outlines the implementation tasks for the configuration schema enhancements and template updates. Tasks are organized into discrete, incremental steps that build upon each other.

## Task Organization

- Top-level tasks represent major implementation phases
- Sub-tasks are specific, actionable coding steps
- Tasks marked with `*` are optional (testing, documentation)
- Each task references specific requirements from requirements.md

---

- [x] 1. Update Config Struct Definitions
  - Create new type definitions in `internal/config/config.go`
  - Add all new fields to existing structs
  - Remove deprecated fields
  - _Requirements: 1.1, 1.2, 1.3, 2.1, 2.2_

- [x] 1.1 Add ClusterConfig fields
  - Add `BaseDomain string` field with jsonschema tag
  - Add `ClusterFQDN string` field with jsonschema tag
  - Add `AdminEmail string` field with jsonschema tag
  - Remove `FluxNamespace`, `ObservabilityNS`, `RackspaceSystemNS`, `GatewayName` if they exist
  - _Requirements: 1.1_

- [x] 1.2 Add GitOpsConfig fields
  - Add `GitOpsBaseRepo string` field with jsonschema tag
  - Add `GitOpsBaseRelease string` field with jsonschema tag
  - Add `GitOpsBranch string` field with jsonschema tag and default value
  - _Requirements: 1.1_

- [x] 1.3 Create StorageConfig type
  - Define `StorageConfig` struct with `DefaultStorageClass string` field
  - Add jsonschema tags with description and default value
  - Add to `SimplifiedOpenCenter` struct
  - _Requirements: 1.1_

- [x] 1.4 Enhance ServiceCfg struct
  - Add common fields: `Namespace`, `Hostname`, `ImageRepository`, `ImageTag`
  - Add cert-manager field: `LetsEncryptServer`
  - Add Loki fields: `SwiftAuthURL`, `SwiftUsername`, `SwiftProjectName`, `SwiftRegion`, `SwiftDomainName`, `LokiBucketName`, `LokiVolumeSize`, `LokiStorageClass`
  - Add Velero fields: `VeleroBackupBucket`, `VeleroRegion`
  - Add Keycloak fields: `KeycloakRealm`, `KeycloakFrontendURL`, `KeycloakClientID`
  - Add Grafana/Prometheus fields: `GrafanaVolumeSize`, `GrafanaStorageClass`, `PrometheusVolumeSize`, `PrometheusStorageClass`, `AlertmanagerVolumeSize`, `AlertmanagerStorageClass`
  - Add Headlamp fields: `HeadlampOIDCIssuerURL`, `HeadlampOIDCClientID`
  - Add Calico field: `CalicoKubeAPIServer`
  - Remove all secret fields: `AWSAccessKey`, `AWSSecretAccessKey`, `SwiftPassword`, `KeycloakClientSecret`, `KeycloakAdminPassword`, `GrafanaAdminPassword`, `WeaveGitOpsPassword`, `WeaveGitOpsPasswordHash`, `HeadlampOIDCClientSecret`
  - _Requirements: 1.2, 2.1_

- [x] 1.5 Update ManagedServiceCfg struct
  - Remove secret fields: `CoreDeviceId`, `AccountServiceToken`, `CoreAccountNumber`
  - Keep non-secret fields: `AlertManagerBaseUrl`, `HTTPRouteFQDN`, `ImageTag`, `ImageRepository`, `GitOpsSourceRepo`, `GitOpsSourceRelease`, `GitOpsSourceBranch`
  - _Requirements: 1.2, 2.2_

- [x] 1.6 Create Secrets section types
  - Define `CertManagerSecrets` struct with `AWSAccessKey` and `AWSSecretAccessKey` fields
  - Define `LokiSecrets` struct with `SwiftPassword` field
  - Define `KeycloakSecrets` struct with `ClientSecret` and `AdminPassword` fields
  - Define `HeadlampSecrets` struct with `OIDCClientSecret` field
  - Define `WeaveGitOpsSecrets` struct with `Password` and `PasswordHash` fields
  - Define `GrafanaSecrets` struct with `AdminPassword` field
  - Define `AlertProxySecrets` struct with `CoreDeviceId`, `AccountServiceToken`, and `CoreAccountNumber` fields
  - Add all secret types to `Secrets` struct
  - Mark all secret fields with `jsonschema:"secret=true"` tag
  - _Requirements: 1.3, 2.3_

- [x] 2. Update Default Configuration Function
  - Modify `defaultConfig()` to populate all new fields
  - Initialize all secret structs as empty
  - _Requirements: 1.4, 17.1, 17.2, 17.3, 17.4, 17.5, 17.6_

- [x] 2.1 Update ClusterConfig defaults
  - Set `BaseDomain` to `"k8s.opencenter.cloud"`
  - Set `ClusterFQDN` to `fmt.Sprintf("%s.sjc3.k8s.opencenter.cloud", name)`
  - Set `AdminEmail` to `"admin@example.com"`
  - _Requirements: 17.1, 17.2_

- [x] 2.2 Update GitOpsConfig defaults
  - Set `GitOpsBaseRepo` to `"ssh://git@github.com/rackerlabs/openCenter-gitops-base.git"`
  - Set `GitOpsBaseRelease` to `"v0.1.0"`
  - Set `GitOpsBranch` to `"main"`
  - _Requirements: 17.3_

- [x] 2.3 Update StorageConfig defaults
  - Set `DefaultStorageClass` to `"csi-cinder-sc-delete"`
  - _Requirements: 17.5_

- [x] 2.4 Update Services defaults
  - Set cert-manager `LetsEncryptServer` to `"https://acme-v02.api.letsencrypt.org/directory"`
  - Set cert-manager `Region` to `"us-east-1"`
  - Set loki `LokiVolumeSize` to `20`
  - Set loki `LokiStorageClass` to `"csi-cinder-sc-delete"`
  - Set kube-prometheus-stack volume sizes: Prometheus=50, Grafana=10, Alertmanager=10
  - Set kube-prometheus-stack storage classes to `"csi-cinder-sc-delete"`
  - _Requirements: 17.4, 17.5_

- [x] 2.5 Initialize Secrets section
  - Initialize all secret structs as empty (CertManager, Loki, Keycloak, Headlamp, WeaveGitOps, Grafana, AlertProxy)
  - Add comment indicating secrets must be provided by user
  - _Requirements: 17.6_

- [x] 3. Update Alert-Proxy Templates
  - Replace all hardcoded values with config references
  - Use Secrets section for sensitive data
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [x] 3.1 Update account-service-token-secret.yaml.tpl
  - Replace hardcoded base64 value with `{{ .Secrets.AlertProxy.AccountServiceToken | b64enc }}`
  - _Requirements: 3.1_

- [x] 3.2 Update core-account-id-secret.yaml.tpl
  - Replace hardcoded base64 value with `{{ .Secrets.AlertProxy.CoreAccountNumber | b64enc }}`
  - _Requirements: 3.2_

- [x] 3.3 Update overseer-core-device-id-secret.yaml.tpl
  - Replace hardcoded base64 value with `{{ .Secrets.AlertProxy.CoreDeviceId | b64enc }}`
  - _Requirements: 3.3_

- [x] 3.4 Update alert-manager-url-secret.yaml.tpl
  - Replace hardcoded base64 value with `{{ .OpenCenter.ManagedService.alert-proxy.AlertManagerBaseUrl | b64enc }}`
  - _Requirements: 3.4_

- [x] 3.5 Update http-route-fqdn-secret.yaml.tpl
  - Replace hardcoded base64 value with `{{ .OpenCenter.ManagedService.alert-proxy.HTTPRouteFQDN | b64enc }}`
  - _Requirements: 3.5_

- [x] 3.6 Update alert-proxy helm-values/override-values.yaml.tpl
  - Replace hardcoded image tag with `{{ .OpenCenter.ManagedService.alert-proxy.ImageTag | default "latest" }}`
  - _Requirements: 3.6_

- [x] 4. Update Cert-Manager Templates
  - Replace hardcoded AWS credentials and configuration
  - Use Secrets section for credentials
  - _Requirements: 4.1, 4.2, 4.3_

- [x] 4.1 Update cluster_name-aws-credentials-secret.yaml.tpl
  - Replace hardcoded access-key-id with `{{ .Secrets.CertManager.AWSAccessKey | b64enc }}`
  - Replace hardcoded secret-access-key with `{{ .Secrets.CertManager.AWSSecretAccessKey | b64enc }}`
  - Replace hardcoded secret name with `{{ .OpenCenter.Cluster.ClusterName }}-aws-credentials-secret`
  - _Requirements: 4.1_

- [x] 4.2 Update letsencrypt-issuer.yaml.tpl
  - Replace hardcoded server URL with `{{ .OpenCenter.Services.cert-manager.LetsEncryptServer | default "https://acme-v02.api.letsencrypt.org/directory" }}`
  - Replace hardcoded email with `{{ .OpenCenter.Cluster.AdminEmail }}`
  - Replace hardcoded region with `{{ .OpenCenter.Services.cert-manager.Region }}`
  - Replace hardcoded DNS zone with `{{ .OpenCenter.Cluster.ClusterFQDN }}`
  - Update secret references to use `{{ .OpenCenter.Cluster.ClusterName }}`
  - _Requirements: 4.2_

- [x] 4.3 Update rackspace-selfsigned-ca.yaml.tpl
  - Replace hardcoded commonName with `{{ .OpenCenter.Cluster.BaseDomain | default "rmpk.dev" }}`
  - _Requirements: 4.3_

- [x] 5. Update Loki Templates
  - Replace hardcoded Swift credentials and configuration
  - Use Secrets section for password
  - _Requirements: 5.1_

- [x] 5.1 Update loki/helm-values/override-values.yaml.tpl
  - Replace hardcoded bucket names with `{{ .OpenCenter.Services.loki.LokiBucketName | default (printf "%s-loki" .OpenCenter.Cluster.ClusterName) }}`
  - Replace hardcoded auth_url with `{{ .OpenCenter.Services.loki.SwiftAuthURL }}`
  - Replace hardcoded username with `{{ .OpenCenter.Services.loki.SwiftUsername }}`
  - Replace hardcoded password with `{{ .Secrets.Loki.SwiftPassword }}`
  - Replace hardcoded project_name with `{{ .OpenCenter.Services.loki.SwiftProjectName }}`
  - Replace hardcoded region_name with `{{ .OpenCenter.Services.loki.SwiftRegion }}`
  - Replace hardcoded domain names with `{{ .OpenCenter.Services.loki.SwiftDomainName }}`
  - Replace hardcoded container_name with `{{ .OpenCenter.Services.loki.LokiBucketName | default (printf "%s-loki" .OpenCenter.Cluster.ClusterName) }}`
  - Update write/read/backend persistence storageClass with `{{ .OpenCenter.Services.loki.LokiStorageClass | default .OpenCenter.Storage.DefaultStorageClass | default "csi-cinder-sc-delete" }}`
  - Update write/read/backend persistence size with `{{ .OpenCenter.Services.loki.LokiVolumeSize | default 20 }}Gi`
  - _Requirements: 5.1_

- [x] 6. Update Velero Templates
  - Replace hardcoded bucket configuration
  - _Requirements: 6.1, 6.2_

- [x] 6.1 Update velero/helm-values/override-values.yaml.tpl
  - Replace hardcoded bucket name with `{{ .OpenCenter.Services.velero.VeleroBackupBucket | default .OpenCenter.Cluster.ClusterName }}`
  - Add region configuration using `{{ .OpenCenter.Services.velero.VeleroRegion }}`
  - _Requirements: 6.1, 6.2_

- [x] 7. Update Keycloak Templates
  - Replace hardcoded realm configuration and credentials
  - Use Secrets section for passwords
  - _Requirements: 7.1, 7.2_

- [x] 7.1 Update keycloak/20-keycloak/opencenter-realm.yaml.tpl
  - Replace hardcoded frontendUrl with `{{ .OpenCenter.Services.keycloak.KeycloakFrontendURL | default (printf "https://auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}`
  - Replace hardcoded clientId with `{{ .OpenCenter.Services.keycloak.KeycloakClientID | default "opencenter" }}`
  - Replace hardcoded admin email with `{{ .OpenCenter.Cluster.AdminEmail }}`
  - Replace hardcoded admin password with `{{ .Secrets.Keycloak.AdminPassword }}`
  - _Requirements: 7.1_

- [x] 7.2 Update keycloak/20-keycloak/httproute.yaml.tpl
  - Replace hardcoded hostname with `{{ .OpenCenter.Services.keycloak.Hostname | default (printf "auth.%s" .OpenCenter.Cluster.ClusterFQDN) }}`
  - _Requirements: 7.2_

- [x] 8. Update Headlamp Templates
  - Replace hardcoded OIDC configuration
  - Use Secrets section for client secret
  - _Requirements: 8.1, 8.2_

- [x] 8.1 Update headlamp/helm-values/override-values.yaml.tpl
  - Replace hardcoded clientID with `{{ .OpenCenter.Services.headlamp.HeadlampOIDCClientID | default "opencenter" }}`
  - Replace hardcoded clientSecret with `{{ .Secrets.Headlamp.OIDCClientSecret }}`
  - Replace hardcoded issuerURL with `{{ .OpenCenter.Services.headlamp.HeadlampOIDCIssuerURL | default (printf "https://auth.%s/realms/opencenter" .OpenCenter.Cluster.ClusterFQDN) }}`
  - _Requirements: 8.1_

- [x] 8.2 Update headlamp/httproute.yaml.tpl
  - Replace hardcoded hostname with `{{ .OpenCenter.Services.headlamp.Hostname | default (printf "headlamp.%s" .OpenCenter.Cluster.ClusterFQDN) }}`
  - _Requirements: 8.2_

- [x] 9. Update Weave GitOps Templates
  - Replace hardcoded password hash
  - Use Secrets section
  - _Requirements: 9.1, 9.2_

- [x] 9.1 Update weave-gitops/helm-values/override-values.yaml.tpl
  - Replace hardcoded passwordHash with `{{ .Secrets.WeaveGitOps.PasswordHash }}`
  - _Requirements: 9.1_

- [x] 9.2 Update weave-gitops/httproute.yaml.tpl
  - Replace hardcoded hostname with `{{ .OpenCenter.Services.weave-gitops.Hostname | default (printf "gitops.%s" .OpenCenter.Cluster.ClusterFQDN) }}`
  - _Requirements: 9.2_

- [x] 10. Update Grafana/Prometheus Templates
  - Replace hardcoded admin password
  - Use Secrets section
  - _Requirements: 10.1, 10.2, 10.3_

- [x] 10.1 Update kube-prometheus-stack/grafana-admin-password.yaml.tpl
  - Replace hardcoded admin-password with `{{ .Secrets.Grafana.AdminPassword | b64enc }}`
  - Keep admin-user as `{{ "admin" | b64enc }}`
  - _Requirements: 10.1_

- [x] 10.2 Add Prometheus storage configuration
  - Create or update Prometheus helm-values template
  - Add storageClass: `{{ .OpenCenter.Services.kube-prometheus-stack.PrometheusStorageClass | default .OpenCenter.Storage.DefaultStorageClass }}`
  - Add size: `{{ .OpenCenter.Services.kube-prometheus-stack.PrometheusVolumeSize | default 50 }}Gi`
  - _Requirements: 10.2_

- [x] 10.3 Add Grafana storage configuration
  - Create or update Grafana helm-values template
  - Add storageClass: `{{ .OpenCenter.Services.kube-prometheus-stack.GrafanaStorageClass | default .OpenCenter.Storage.DefaultStorageClass }}`
  - Add size: `{{ .OpenCenter.Services.kube-prometheus-stack.GrafanaVolumeSize | default 10 }}Gi`
  - _Requirements: 10.3_

- [x] 11. Update HTTPRoute Templates
  - Replace hardcoded hostnames with config values
  - Use hardcoded gateway references
  - _Requirements: 11.1, 11.2, 11.3_

- [x] 11.1 Update all service HTTPRoute templates
  - For each HTTPRoute template, replace hardcoded hostname with service-specific config
  - Pattern: `{{ .OpenCenter.Services.{service}.Hostname | default (printf "{service}.%s" .OpenCenter.Cluster.ClusterFQDN) }}`
  - Services: keycloak, headlamp, weave-gitops, longhorn, and any others with HTTPRoutes
  - _Requirements: 11.1_

- [x] 11.2 Verify gateway parentRef references
  - Ensure all HTTPRoute templates use hardcoded gateway name "rmpk-gateway"
  - Ensure all HTTPRoute templates use hardcoded namespace "rackspace-system" for gateway
  - _Requirements: 11.2_

- [x] 11.3 Verify HTTPRoute namespaces
  - Ensure HTTPRoute templates use appropriate hardcoded namespaces
  - flux-system for weave-gitops
  - keycloak for keycloak
  - observability for grafana/prometheus
  - _Requirements: 11.3_

- [-] 12. Update GitOps Source Templates
  - Replace hardcoded repository URLs and releases
  - _Requirements: 12.1, 12.2_

- [x] 12.1 Update base GitOps source templates
  - For each source template in `services/sources/`, update:
  - url: `{{ .OpenCenter.GitOps.GitOpsBaseRepo }}`
  - ref.tag: `{{ .OpenCenter.GitOps.GitOpsBaseRelease }}`
  - ref.branch: `{{ .OpenCenter.GitOps.GitOpsBranch }}`
  - _Requirements: 12.1_

- [x] 12.2 Update service-specific source overrides
  - For services with Release/Branch/Uri fields, add conditional logic
  - Pattern: `{{ .OpenCenter.Services.{service}.Release | default .OpenCenter.GitOps.GitOpsBaseRelease }}`
  - _Requirements: 12.2_

- [ ] 13. Update Gateway Templates
  - Verify hardcoded namespace usage
  - _Requirements: 13.1, 13.2_

- [-] 13.1 Update gateway/namespace.yaml.tpl
  - Ensure namespace name is hardcoded as "rackspace-system"
  - _Requirements: 13.1_

- [ ] 13.2 Verify gateway resource names
  - Ensure gateway name is hardcoded as "rmpk-gateway" in all gateway resources
  - _Requirements: 13.2_

- [ ] 14. Update Calico Templates
  - Replace hardcoded network configuration
  - _Requirements: 14.1_

- [ ] 14.1 Update calico/helm-values/override_values.yaml.tpl
  - Replace hardcoded pod CIDR with `{{ .OpenCenter.Cluster.Kubernetes.SubnetPods }}`
  - Replace hardcoded service CIDR with `{{ .OpenCenter.Cluster.Kubernetes.SubnetServices }}`
  - Add Kubernetes API server config: `{{ .OpenCenter.Services.calico.CalicoKubeAPIServer }}`
  - _Requirements: 14.1_

- [ ] 15. Update Infrastructure Templates
  - Replace hardcoded node configurations
  - _Requirements: 15.1, 15.2_

- [ ] 15.1 Update main-baremetal.tf.tpl
  - Replace hardcoded master node IPs with iteration over `{{ range .OpenCenter.Cluster.Kubernetes.MasterNodes }}`
  - Replace hardcoded worker node IPs with iteration over `{{ range .OpenCenter.Cluster.Kubernetes.WorkerNodes }}`
  - Use `{{ .ID }}`, `{{ .Name }}`, `{{ .AccessIPv4 }}` for each node
  - _Requirements: 15.1_

- [ ] 15.2 Update main.tf.tpl
  - Replace hardcoded DNS zone with `{{ .OpenCenter.Cluster.Kubernetes.DNSZoneName }}`
  - _Requirements: 15.2_

- [ ] 16. Update Schema Generation
  - Regenerate JSON schema with new fields
  - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.5_

- [ ] 16.1 Run schema generation
  - Execute `mise run schema` to regenerate JSON schema
  - Verify all new fields are present in generated schema
  - _Requirements: 16.1, 16.2, 16.3, 16.4, 16.5_

- [ ] 16.2 Verify schema includes ClusterConfig fields
  - Check BaseDomain, ClusterFQDN, AdminEmail are in schema with descriptions
  - _Requirements: 16.1_

- [ ] 16.3 Verify schema includes ServiceCfg fields
  - Check all new service-specific fields are in schema with descriptions and defaults
  - _Requirements: 16.2_

- [ ] 16.4 Verify schema includes Secrets fields
  - Check all secret types are in schema
  - Verify secret=true marking is present
  - _Requirements: 16.3_

- [ ] 16.5 Verify schema includes other new types
  - Check ManagedServiceCfg, StorageConfig, GitOpsConfig additions
  - _Requirements: 16.4, 16.5_

- [ ] 17. Add Validation Functions
  - Implement validation for new fields
  - _Requirements: 19.1, 19.2, 19.3, 19.4, 19.5_

- [ ] 17.1 Add email validation
  - Implement `isValidEmail()` function
  - Add validation check in `Validate()` for AdminEmail
  - _Requirements: 19.2_

- [ ] 17.2 Add domain validation
  - Implement `isValidDomain()` function
  - Add validation check in `Validate()` for ClusterFQDN and BaseDomain
  - _Requirements: 19.3_

- [ ] 17.3 Add service-specific validation
  - Implement `validateService()` function
  - Check required fields for each enabled service
  - Check required secrets for each enabled service
  - _Requirements: 19.4, 19.5_

- [ ] 18. Remove Backward Compatibility Code
  - Clean up deprecated fields and logic
  - _Requirements: 20.1, 20.2, 20.3, 20.4_

- [ ] 18.1 Remove deprecated Config fields
  - Remove any old field names that were kept for compatibility
  - Remove any fallback logic for old configurations
  - _Requirements: 20.1, 20.3_

- [ ] 18.2 Remove deprecated template logic
  - Remove any conditional logic checking for old vs new field names
  - Remove any fallback values for old configurations
  - _Requirements: 20.2_

- [ ] 18.3 Update documentation
  - Remove references to old configuration structure
  - Update all examples to use new structure only
  - _Requirements: 20.4_

- [ ] 19. Update Unit Tests
  - Add tests for new configuration fields
  - Add tests for validation functions
  - _Requirements: All requirements_

- [ ] 19.1 Test default configuration
  - Test `defaultConfig()` populates all new fields correctly
  - Test default values match specifications
  - _Requirements: 17.1, 17.2, 17.3, 17.4, 17.5_

- [ ] 19.2 Test configuration validation
  - Test `Validate()` catches missing required fields
  - Test email validation
  - Test domain validation
  - Test service-specific validation
  - _Requirements: 19.1, 19.2, 19.3, 19.4, 19.5_

- [ ] 19.3 Test template rendering
  - Test each updated template renders correctly with test config
  - Test secret values are properly encoded
  - Test default values work correctly
  - Test printf and other Sprig functions work correctly
  - _Requirements: 18.1, 18.2, 18.3, 18.4, 18.5_

- [ ] 20. Add BDD Tests
  - Create Godog scenarios for configuration-driven rendering
  - _Requirements: All requirements_

- [ ] 20.1 Add alert-proxy BDD tests
  - Test rendering with custom alert-proxy configuration
  - Test secret values are correctly rendered
  - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5, 3.6_

- [ ] 20.2 Add cert-manager BDD tests
  - Test rendering with custom cert-manager configuration
  - Test AWS credentials are correctly rendered
  - Test LetsEncrypt configuration
  - _Requirements: 4.1, 4.2, 4.3_

- [ ] 20.3 Add Loki BDD tests
  - Test rendering with custom Loki configuration
  - Test Swift credentials are correctly rendered
  - Test volume configuration
  - _Requirements: 5.1_

- [ ] 20.4 Add service BDD tests
  - Test rendering for Velero, Keycloak, Headlamp, Weave GitOps, Grafana
  - Test HTTPRoute hostname generation
  - Test secret handling
  - _Requirements: 6.1, 7.1, 8.1, 9.1, 10.1_

- [ ] 21. Integration Testing
  - Test full cluster configuration and rendering
  - _Requirements: All requirements_

- [ ] 21.1 Create test cluster configuration
  - Create complete test configuration with all new fields
  - Include all required secrets
  - _Requirements: All_

- [ ] 21.2 Test full rendering
  - Run cluster init with test configuration
  - Verify all templates render correctly
  - Verify no hardcoded values remain
  - _Requirements: All_

- [ ] 21.3 Test validation
  - Test configuration validation catches errors
  - Test missing secrets are detected
  - Test invalid values are rejected
  - _Requirements: 19.1, 19.2, 19.3, 19.4, 19.5_

- [ ] 22. Documentation Updates
  - Update all documentation to reflect new configuration structure
  - _Requirements: 20.4_

- [ ] 22.1 Update CONFIG_SCHEMA_ADDITIONS.md
  - Ensure all new fields are documented
  - Ensure examples are up to date
  - _Requirements: 20.4_

- [ ] 22.2 Update TEMPLATE_ANALYSIS_REPORT.md
  - Mark all templates as updated
  - Update examples to show new template syntax
  - _Requirements: 20.4_

- [ ] 22.3 Create migration guide
  - Document how to migrate from old to new configuration
  - Provide example migration scripts if needed
  - Document breaking changes
  - _Requirements: 20.5_

- [ ] 22.4 Update README and examples
  - Update main README with new configuration structure
  - Update example configurations in testdata/
  - _Requirements: 20.4_
