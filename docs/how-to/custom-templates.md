# Customizing GitOps Templates

**doc_type**: how-to  
**priority**: 3  
**audience**: Platform engineers customizing cluster deployments  
**related_docs**:
- [Template System Reference](../reference/template-system.md)
- [GitOps Setup Guide](./gitops-setup.md)
- [Service Configuration](./service-configuration.md)

## Overview

This guide shows you how to customize the GitOps templates that openCenter uses to generate cluster manifests. You'll learn how to modify existing templates, create custom templates, and integrate them into your cluster setup workflow.

## Prerequisites

- openCenter CLI installed and configured
- Basic understanding of Go templates
- Familiarity with Kubernetes manifests
- A cluster configuration file

## Understanding the Template System

openCenter uses Go's `text/template` package with Sprig functions to render GitOps manifests. Templates are embedded in the binary and rendered during `cluster setup`.

### Template Types

1. **Base Templates** (`.tpl` files): Always rendered, extension stripped
2. **Customizable Templates** (`.tmpl` files): Rendered when `--render` flag is used
3. **Static Files**: Copied as-is without rendering

### Template Locations

```
internal/gitops/
├── gitops-base-dir/          # Base GitOps structure
│   ├── README.md.tpl
│   ├── Makefile.tpl
│   └── .gitignore
└── templates/
    ├── cluster-apps-base/    # Application manifests
    │   ├── services/
    │   └── managed-services/
    └── infrastructure-cluster-template/  # Infrastructure configs
        ├── main.tf.tpl
        └── variables.tf.tpl
```

## Task 1: Customize Existing Templates

### Step 1: Generate GitOps Repository

First, generate the GitOps repository with unrendered templates:

```bash
# Initialize cluster configuration
mise run build
./bin/openCenter cluster init my-cluster

# Edit configuration as needed
vim ~/.config/openCenter/clusters/opencenter/my-cluster/.my-cluster-config.yaml

# Generate GitOps repo without rendering .tmpl files
./bin/openCenter cluster setup my-cluster
```

### Step 2: Locate Template Files

Navigate to your GitOps directory and find `.tmpl` files:

```bash
cd ~/gitops/my-cluster
find . -name "*.tmpl"
```

Common customization targets:
- `applications/overlays/my-cluster/services/*/values.yaml.tmpl`
- `infrastructure/clusters/my-cluster/variables.tf.tmpl`

### Step 3: Edit Template Files

Edit templates using your preferred editor. Templates have access to the full cluster configuration:

```yaml
# Example: applications/overlays/my-cluster/services/cert-manager/values.yaml.tmpl
global:
  leaderElection:
    namespace: {{ .OpenCenter.Services.CertManager.Namespace }}

installCRDs: true

# Access nested configuration
prometheus:
  enabled: {{ .OpenCenter.Services.KubePrometheusStack.Enabled }}
  servicemonitor:
    enabled: {{ .OpenCenter.Services.KubePrometheusStack.Enabled }}
    namespace: {{ .OpenCenter.Services.KubePrometheusStack.Namespace }}

# Use Sprig functions
replicaCount: {{ default 2 .OpenCenter.Services.CertManager.ReplicaCount }}
```

### Step 4: Render Templates

After editing, render templates to generate final manifests:

```bash
# Re-run setup with --render flag
./bin/openCenter cluster setup my-cluster --render
```

### Step 5: Validate Changes

Verify the rendered output:

```bash
# Check rendered files (no .tmpl extension)
cat applications/overlays/my-cluster/services/cert-manager/values.yaml

# Validate Kubernetes manifests
kubectl --dry-run=client -f applications/overlays/my-cluster/services/cert-manager/
```

## Task 2: Create Custom Service Templates

### Step 1: Create Service Directory Structure

```bash
cd ~/gitops/my-cluster/applications/overlays/my-cluster/services
mkdir -p my-custom-service/{base,overlays}
```

### Step 2: Create Service Manifest Template

Create `my-custom-service/base/deployment.yaml.tmpl`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .OpenCenter.Services.MyCustomService.Name | default "my-custom-service" }}
  namespace: {{ .OpenCenter.Services.MyCustomService.Namespace | default "default" }}
  labels:
    app: {{ .OpenCenter.Services.MyCustomService.Name | default "my-custom-service" }}
    cluster: {{ .ClusterName }}
spec:
  replicas: {{ .OpenCenter.Services.MyCustomService.Replicas | default 1 }}
  selector:
    matchLabels:
      app: {{ .OpenCenter.Services.MyCustomService.Name | default "my-custom-service" }}
  template:
    metadata:
      labels:
        app: {{ .OpenCenter.Services.MyCustomService.Name | default "my-custom-service" }}
    spec:
      containers:
      - name: app
        image: {{ .OpenCenter.Services.MyCustomService.Image }}:{{ .OpenCenter.Services.MyCustomService.Tag | default "latest" }}
        ports:
        - containerPort: {{ .OpenCenter.Services.MyCustomService.Port | default 8080 }}
        env:
        {{- range $key, $value := .OpenCenter.Services.MyCustomService.Env }}
        - name: {{ $key }}
          value: {{ $value | quote }}
        {{- end }}
```

### Step 3: Create Kustomization

Create `my-custom-service/base/kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - deployment.yaml
  - service.yaml

namespace: my-custom-service
```

### Step 4: Add Configuration to Cluster Config

Edit your cluster configuration:

```yaml
# ~/.config/openCenter/clusters/opencenter/my-cluster/.my-cluster-config.yaml
opencenter:
  services:
    my-custom-service:
      enabled: true
      name: my-custom-service
      namespace: my-custom-service
      image: myregistry/my-app
      tag: 1.0.0
      replicas: 3
      port: 8080
      env:
        LOG_LEVEL: info
        DATABASE_URL: postgres://db:5432/myapp
```

### Step 5: Render and Deploy

```bash
# Render templates
./bin/openCenter cluster setup my-cluster --render

# Verify rendered manifest
cat applications/overlays/my-cluster/services/my-custom-service/base/deployment.yaml

# Apply to cluster
kubectl apply -k applications/overlays/my-cluster/services/my-custom-service/base/
```

## Task 3: Use Advanced Template Features

### Conditionals

```yaml
{{- if .OpenCenter.Services.MyService.Enabled }}
apiVersion: v1
kind: Service
metadata:
  name: {{ .OpenCenter.Services.MyService.Name }}
spec:
  {{- if eq .OpenCenter.Services.MyService.Type "LoadBalancer" }}
  type: LoadBalancer
  {{- else }}
  type: ClusterIP
  {{- end }}
{{- end }}
```

### Loops

```yaml
{{- range $index, $replica := .OpenCenter.Services.MyService.Replicas }}
---
apiVersion: v1
kind: Pod
metadata:
  name: {{ $.OpenCenter.Services.MyService.Name }}-{{ $index }}
  labels:
    app: {{ $.OpenCenter.Services.MyService.Name }}
    replica: "{{ $index }}"
{{- end }}
```

### Sprig Functions

```yaml
# String manipulation
name: {{ .OpenCenter.Services.MyService.Name | lower | replace "-" "_" }}

# Default values
replicas: {{ .OpenCenter.Services.MyService.Replicas | default 1 }}

# Date formatting
timestamp: {{ now | date "2006-01-02T15:04:05Z07:00" }}

# Base64 encoding
secret: {{ .OpenCenter.Services.MyService.Secret | b64enc }}

# List operations
{{- $services := list "api" "worker" "scheduler" }}
{{- range $services }}
- {{ . }}
{{- end }}
```

### Template Variables

```yaml
{{- $serviceName := .OpenCenter.Services.MyService.Name }}
{{- $namespace := .OpenCenter.Services.MyService.Namespace }}

apiVersion: v1
kind: Service
metadata:
  name: {{ $serviceName }}
  namespace: {{ $namespace }}
spec:
  selector:
    app: {{ $serviceName }}
```

## Task 4: Override Infrastructure Templates

### Step 1: Customize Terraform Templates

Edit infrastructure templates for provider-specific customization:

```hcl
# infrastructure/clusters/my-cluster/main.tf.tmpl
terraform {
  required_version = ">= 1.0"
  
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> {{ .OpenCenter.Infrastructure.TerraformProviderVersion | default "1.54.0" }}"
    }
  }
}

# Custom resource definitions
resource "openstack_compute_instance_v2" "control_plane" {
  count = {{ .OpenCenter.Infrastructure.ControlPlane.Count }}
  
  name            = "{{ .ClusterName }}-cp-${count.index}"
  flavor_name     = "{{ .OpenCenter.Infrastructure.ControlPlane.Flavor }}"
  image_name      = "{{ .OpenCenter.Infrastructure.ControlPlane.Image }}"
  key_pair        = "{{ .ClusterName }}-keypair"
  security_groups = {{ .OpenCenter.Infrastructure.ControlPlane.SecurityGroups | toJson }}
  
  {{- if .OpenCenter.Infrastructure.ControlPlane.UserData }}
  user_data = <<-EOF
{{ .OpenCenter.Infrastructure.ControlPlane.UserData | indent 4 }}
  EOF
  {{- end }}
  
  network {
    name = "{{ .OpenCenter.Infrastructure.Network.Name }}"
  }
  
  metadata = {
    cluster = "{{ .ClusterName }}"
    role    = "control-plane"
    index   = "${count.index}"
  }
}
```

### Step 2: Add Custom Variables

Create `infrastructure/clusters/my-cluster/custom-variables.tf.tmpl`:

```hcl
variable "custom_tags" {
  description = "Custom tags for all resources"
  type        = map(string)
  default = {
    {{- range $key, $value := .OpenCenter.Infrastructure.CustomTags }}
    {{ $key }} = "{{ $value }}"
    {{- end }}
  }
}

variable "enable_monitoring" {
  description = "Enable monitoring integration"
  type        = bool
  default     = {{ .OpenCenter.Services.KubePrometheusStack.Enabled }}
}
```

## Task 5: Test Template Changes

### Step 1: Validate Template Syntax

Use the template engine's validation:

```bash
# Build with validation enabled
mise run build

# Validate specific template
./bin/openCenter cluster setup my-cluster --validate-only
```

### Step 2: Dry Run Rendering

Test template rendering without applying:

```bash
# Render to temporary directory
./bin/openCenter cluster setup my-cluster --render --output /tmp/test-render

# Compare with existing
diff -r ~/gitops/my-cluster /tmp/test-render
```

### Step 3: Test with Different Configurations

Create test configurations:

```yaml
# test-config-minimal.yaml
opencenter:
  cluster_name: test-minimal
  services:
    cert-manager:
      enabled: true

# test-config-full.yaml
opencenter:
  cluster_name: test-full
  services:
    cert-manager:
      enabled: true
      replicas: 3
      namespace: cert-manager-system
```

Render each configuration:

```bash
for config in test-config-*.yaml; do
  ./bin/openCenter cluster setup $(basename $config .yaml) --config $config --render
done
```

## Task 6: Version Control Template Changes

### Step 1: Track Template Modifications

```bash
cd ~/gitops/my-cluster

# Add custom templates
git add applications/overlays/my-cluster/services/my-custom-service/

# Commit with descriptive message
git commit -m "feat: add custom service template for my-custom-service

- Add deployment and service manifests
- Configure environment variables from cluster config
- Set replica count based on configuration"
```

### Step 2: Document Template Customizations

Create `TEMPLATES.md` in your GitOps repo:

```markdown
# Template Customizations

## Custom Services

### my-custom-service
- **Location**: `applications/overlays/my-cluster/services/my-custom-service/`
- **Purpose**: Custom application deployment
- **Configuration**: See `.my-cluster-config.yaml` under `opencenter.services.my-custom-service`
- **Rendering**: Run `openCenter cluster setup my-cluster --render`

## Modified Templates

### cert-manager values.yaml
- **Changes**: Added custom resource limits
- **Reason**: Optimize for production workload
- **Date**: 2024-01-15
```

## Troubleshooting

### Template Rendering Errors

**Problem**: Template fails to render with syntax error

```
Error: template: main.tf.tmpl:15:23: executing "main.tf.tmpl" at <.OpenCenter.Invalid>: 
can't evaluate field Invalid in type config.Config
```

**Solution**: Check field names match configuration structure:

```bash
# Verify configuration structure
./bin/openCenter config show my-cluster --format json | jq '.opencenter'

# Check available fields in template context
# Templates have access to entire Config struct
```

### Missing Template Variables

**Problem**: Template renders with empty values

**Solution**: Use default values and check configuration:

```yaml
# Use defaults for optional fields
replicas: {{ .OpenCenter.Services.MyService.Replicas | default 1 }}

# Check if field exists before using
{{- if .OpenCenter.Services.MyService.CustomField }}
customField: {{ .OpenCenter.Services.MyService.CustomField }}
{{- end }}
```

### Template Function Not Found

**Problem**: `Error: function "myFunc" not defined`

**Solution**: Use Sprig functions or register custom functions. Available functions:

- All Sprig v3 functions: https://masterminds.github.io/sprig/
- Go template built-ins: `and`, `or`, `not`, `eq`, `ne`, `lt`, `le`, `gt`, `ge`

## Best Practices

1. **Use Defaults**: Always provide default values for optional fields
2. **Validate Early**: Test templates with minimal configurations first
3. **Document Changes**: Keep TEMPLATES.md updated with customizations
4. **Version Control**: Commit template changes with descriptive messages
5. **Test Thoroughly**: Render with multiple configurations before deploying
6. **Preserve Structure**: Keep template organization consistent with defaults
7. **Use Comments**: Add comments explaining complex template logic
8. **Avoid Hardcoding**: Use configuration values instead of hardcoded strings

## Next Steps

- [Configure Services](./service-configuration.md) - Learn service-specific configuration
- [GitOps Workflows](./gitops-workflows.md) - Manage GitOps repository lifecycle
- [Template System Reference](../reference/template-system.md) - Complete template API documentation
- [Plugin Development](./plugin-development.md) - Create custom service plugins
