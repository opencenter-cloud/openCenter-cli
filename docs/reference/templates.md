---
title: Template System Reference
doc_type: reference
category: reference
weight: 30
---

# Template System Reference


## Table of Contents

- [Overview](#overview)
- [Template Structure](#template-structure)
- [Template Context](#template-context)
- [Template Functions](#template-functions)
- [Common Template Patterns](#common-template-patterns)
- [Template Examples](#template-examples)
- [Custom Templates](#custom-templates)
- [Template Engine API](#template-engine-api)
- [Troubleshooting](#troubleshooting)
- [Best Practices](#best-practices)
- [See Also](#see-also)
This document provides complete reference information for the openCenter CLI template system, including template functions, variables, structure, and customization.

## Overview

openCenter CLI uses Go's `text/template` package enhanced with [Sprig functions](http://masterminds.github.io/sprig/) to render configuration files and GitOps manifests. Templates are embedded in the binary and rendered during cluster setup.

### Template Types

1. **Base Templates** (`.tpl`): Always rendered, extension stripped
2. **Customizable Templates** (`.tmpl`): Rendered when `--render` flag is used
3. **Static Files**: Copied as-is without rendering

---

## Template Structure

### Embedded Templates

Templates are embedded in the binary using Go's `embed` directive.

```
internal/gitops/
├── gitops-base-dir/          # Base repository structure
│   ├── README.md
│   ├── Makefile.tpl
│   └── .gitignore
└── templates/
    ├── cluster-apps-base/    # Application manifests
    │   ├── services/
    │   ├── managed-services/
    │   └── flux-system/
    └── infrastructure-cluster-template/  # Infrastructure configs
        ├── main.tf.tpl
        ├── variables.tf.tpl
        └── outputs.tf.tpl
```

### Template Naming Conventions

- **`.tpl` extension**: Always rendered, extension removed in output
  - Input: `main.tf.tpl` → Output: `main.tf`
  
- **`.tmpl` extension**: Rendered only with `--render` flag
  - Input: `config.yaml.tmpl` → Output: `config.yaml` (when rendered)
  - Input: `config.yaml.tmpl` → Output: `config.yaml.tmpl` (when not rendered)

- **No extension**: Copied as-is
  - Input: `README.md` → Output: `README.md`

### Output Structure

```
<git_dir>/
├── README.md
├── Makefile
├── .gitignore
├── applications/
│   └── overlays/
│       └── <cluster-name>/
│           ├── services/
│           ├── managed-services/
│           └── flux-system/
└── infrastructure/
    └── clusters/
        └── <cluster-name>/
            ├── main.tf
            ├── variables.tf
            └── outputs.tf
```

---

## Template Context

### Root Context

Templates receive the complete `Config` struct as the root context (`.`).

```go
type Config struct {
    SchemaVersion string
    OpenCenter    SimplifiedOpenCenter
    OpenTofu      SimplifiedOpenTofu
    Secrets       Secrets
    Networking    Networking
    Deployment    Deployment
    Overrides     map[string]any
    Metadata      ConfigMetadata
}
```

### Accessing Configuration

```yaml
# Cluster name
{{ .OpenCenter.Cluster.ClusterName }}

# Region
{{ .OpenCenter.Meta.Region }}

# Kubernetes version
{{ .OpenCenter.Cluster.Kubernetes.Version }}

# GitOps directory
{{ .OpenCenter.GitOps.GitDir }}

# Service enabled status
{{ .OpenCenter.Services.calico.Enabled }}
```

---

## Template Functions

### Sprig Functions

All [Sprig functions](http://masterminds.github.io/sprig/) are available. Key categories:

#### String Functions

```yaml
# Trim whitespace
{{ trim "  hello  " }}  # "hello"

# Convert case
{{ upper "hello" }}     # "HELLO"
{{ lower "HELLO" }}     # "hello"
{{ title "hello world" }}  # "Hello World"

# Replace
{{ replace " " "-" "hello world" }}  # "hello-world"

# Substring
{{ substr 0 5 "hello world" }}  # "hello"

# Contains
{{ contains "world" "hello world" }}  # true

# Split and join
{{ split "," "a,b,c" }}  # [a b c]
{{ join "," (list "a" "b" "c") }}  # "a,b,c"

# Repeat
{{ repeat 3 "x" }}  # "xxx"

# Trim prefix/suffix
{{ trimPrefix "hello-" "hello-world" }}  # "world"
{{ trimSuffix "-world" "hello-world" }}  # "hello"
```

#### Math Functions

```yaml
# Basic operations
{{ add 1 2 }}      # 3
{{ sub 5 3 }}      # 2
{{ mul 2 3 }}      # 6
{{ div 10 2 }}     # 5
{{ mod 10 3 }}     # 1

# Min/max
{{ max 1 2 3 }}    # 3
{{ min 1 2 3 }}    # 1

# Rounding
{{ round 3.14159 2 }}  # 3.14
{{ ceil 3.1 }}         # 4
{{ floor 3.9 }}        # 3
```

#### List Functions

```yaml
# Create list
{{ list "a" "b" "c" }}

# Append
{{ append (list "a" "b") "c" }}  # [a b c]

# Prepend
{{ prepend (list "b" "c") "a" }}  # [a b c]

# First/last
{{ first (list "a" "b" "c") }}  # "a"
{{ last (list "a" "b" "c") }}   # "c"

# Has
{{ has "b" (list "a" "b" "c") }}  # true

# Reverse
{{ reverse (list "a" "b" "c") }}  # [c b a]

# Unique
{{ uniq (list "a" "b" "a" "c") }}  # [a b c]

# Compact (remove empty)
{{ compact (list "a" "" "b" "" "c") }}  # [a b c]
```

#### Dictionary Functions

```yaml
# Create dict
{{ dict "key1" "value1" "key2" "value2" }}

# Get value
{{ get (dict "key" "value") "key" }}  # "value"

# Set value
{{ set (dict) "key" "value" }}

# Has key
{{ hasKey (dict "key" "value") "key" }}  # true

# Keys
{{ keys (dict "a" 1 "b" 2) }}  # [a b]

# Values
{{ values (dict "a" 1 "b" 2) }}  # [1 2]

# Merge
{{ merge (dict "a" 1) (dict "b" 2) }}  # {a:1 b:2}

# Omit keys
{{ omit (dict "a" 1 "b" 2 "c" 3) "b" }}  # {a:1 c:3}

# Pick keys
{{ pick (dict "a" 1 "b" 2 "c" 3) "a" "c" }}  # {a:1 c:3}
```

#### Type Conversion

```yaml
# To string
{{ toString 123 }}  # "123"

# To int
{{ toInt "123" }}   # 123

# To float
{{ toFloat64 "3.14" }}  # 3.14

# To bool
{{ toBool "true" }}  # true

# To JSON
{{ toJson (dict "key" "value") }}  # {"key":"value"}

# From JSON
{{ fromJson "{\"key\":\"value\"}" }}  # {key:value}

# To YAML
{{ toYaml (dict "key" "value") }}
# key: value

# From YAML
{{ fromYaml "key: value" }}  # {key:value}
```

#### Date/Time Functions

```yaml
# Current time
{{ now }}

# Format date
{{ now | date "2006-01-02" }}  # 2024-01-15

# Date modify
{{ now | dateModify "+24h" }}  # Tomorrow

# Unix timestamp
{{ now | unixEpoch }}

# Date in zone
{{ now | dateInZone "2006-01-02" "UTC" }}
```

#### Encoding Functions

```yaml
# Base64 encode
{{ b64enc "hello" }}  # aGVsbG8=

# Base64 decode
{{ b64dec "aGVsbG8=" }}  # hello

# URL encode
{{ urlquery "hello world" }}  # hello+world

# SHA256 sum
{{ sha256sum "hello" }}

# UUID
{{ uuidv4 }}  # Random UUID
```

#### Flow Control

```yaml
# Default value
{{ default "default" .Value }}

# Empty check
{{ empty .Value }}

# Ternary
{{ ternary "yes" "no" .Condition }}

# Coalesce (first non-empty)
{{ coalesce .Value1 .Value2 "default" }}
```

---

## Common Template Patterns

### Conditional Rendering

```yaml
{{- if .OpenCenter.Services.calico.Enabled }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: calico-config
data:
  cni_iface: {{ .OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico.CNIIface }}
{{- end }}
```

### Iterating Over Services

```yaml
{{- range $name, $service := .OpenCenter.Services }}
{{- if $service.Enabled }}
---
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: {{ $name }}
  namespace: flux-system
spec:
  interval: 15m
  url: {{ $service.Uri | default "https://github.com/rackerlabs/openCenter-gitops-base.git" }}
  ref:
    {{- if $service.Release }}
    tag: {{ $service.Release }}
    {{- else if $service.Branch }}
    branch: {{ $service.Branch }}
    {{- else }}
    branch: main
    {{- end }}
{{- end }}
{{- end }}
```

### String Manipulation

```yaml
# Convert cluster name to lowercase
bucket_name: {{ .OpenCenter.Cluster.ClusterName | lower }}

# Replace spaces with hyphens
resource_name: {{ .OpenCenter.Cluster.ClusterName | replace " " "-" }}

# Trim and lowercase
normalized_name: {{ .OpenCenter.Cluster.ClusterName | trim | lower }}
```

### Default Values

```yaml
# Provide default if value is empty
region: {{ .OpenCenter.Meta.Region | default "us-east-1" }}

# Coalesce multiple values
api_endpoint: {{ coalesce .OpenCenter.Cluster.APIEndpoint .OpenCenter.Cluster.ClusterFQDN "api.example.com" }}
```

### List Processing

```yaml
# Join list with commas
allowed_cidrs: {{ join "," .OpenCenter.Cluster.Networking.K8sAPIPortACL }}

# Iterate over list
{{- range .OpenCenter.Cluster.SSHAuthorizedKeys }}
  - {{ . }}
{{- end }}

# Filter and process
{{- range .OpenCenter.Cluster.Kubernetes.MasterNodes }}
{{- if .AccessIPv4 }}
  - name: {{ .Name }}
    ip: {{ .AccessIPv4 }}
{{- end }}
{{- end }}
```

### Nested Structures

```yaml
# Access nested configuration
{{- with .OpenCenter.Infrastructure.Cloud.OpenStack }}
auth_url: {{ .AuthURL }}
region: {{ .Region }}
tenant_name: {{ .TenantName }}
{{- end }}

# Check if nested value exists
{{- if .OpenCenter.Talos }}
{{- if .OpenCenter.Talos.Enabled }}
talos_version: {{ .OpenCenter.Talos.Version }}
{{- end }}
{{- end }}
```

### Whitespace Control

```yaml
# Remove leading whitespace
{{- if .Condition }}
content
{{- end }}

# Remove trailing whitespace
{{ if .Condition -}}
content
{{- end }}

# Remove both
{{- if .Condition -}}
content
{{- end -}}
```

---

## Template Examples

### Terraform Variables

```hcl
# variables.tf.tpl
variable "cluster_name" {
  description = "Name of the Kubernetes cluster"
  type        = string
  default     = "{{ .OpenCenter.Cluster.ClusterName }}"
}

variable "region" {
  description = "Deployment region"
  type        = string
  default     = "{{ .OpenCenter.Meta.Region }}"
}

variable "master_count" {
  description = "Number of master nodes"
  type        = number
  default     = {{ .OpenCenter.Cluster.Kubernetes.MasterCount }}
}

variable "worker_count" {
  description = "Number of worker nodes"
  type        = number
  default     = {{ .OpenCenter.Cluster.Kubernetes.WorkerCount }}
}

{{- if .OpenCenter.Infrastructure.Provider | eq "openstack" }}
variable "openstack_auth_url" {
  description = "OpenStack authentication URL"
  type        = string
  default     = "{{ .OpenCenter.Infrastructure.Cloud.OpenStack.AuthURL }}"
}
{{- end }}
```

### Kubernetes Manifest

```yaml
# deployment.yaml.tpl
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .OpenCenter.Cluster.ClusterName }}-app
  namespace: {{ .OpenCenter.Services.myapp.Namespace | default "default" }}
  labels:
    app: {{ .OpenCenter.Cluster.ClusterName }}
    environment: {{ .OpenCenter.Meta.Env | default "production" }}
spec:
  replicas: {{ .OpenCenter.Services.myapp.Replicas | default 3 }}
  selector:
    matchLabels:
      app: {{ .OpenCenter.Cluster.ClusterName }}
  template:
    metadata:
      labels:
        app: {{ .OpenCenter.Cluster.ClusterName }}
    spec:
      containers:
      - name: app
        image: {{ .OpenCenter.Services.myapp.ImageRepository }}:{{ .OpenCenter.Services.myapp.ImageTag }}
        ports:
        - containerPort: 8080
        env:
        - name: CLUSTER_NAME
          value: "{{ .OpenCenter.Cluster.ClusterName }}"
        - name: REGION
          value: "{{ .OpenCenter.Meta.Region }}"
```

### FluxCD GitRepository

```yaml
# gitrepository.yaml.tpl
apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: {{ .OpenCenter.Cluster.ClusterName }}-gitops
  namespace: flux-system
spec:
  interval: {{ .OpenCenter.GitOps.Flux.Interval | default "15m" }}
  url: {{ .OpenCenter.GitOps.GitURL }}
  ref:
    {{- if .OpenCenter.GitOps.Release }}
    tag: {{ .OpenCenter.GitOps.Release }}
    {{- else if .OpenCenter.GitOps.Branch }}
    branch: {{ .OpenCenter.GitOps.Branch }}
    {{- else }}
    branch: main
    {{- end }}
  secretRef:
    name: flux-system
  {{- if .OpenCenter.GitOps.Flux.Prune }}
  prune: true
  {{- end }}
```

### ConfigMap with Multiple Values

```yaml
# configmap.yaml.tpl
apiVersion: v1
kind: ConfigMap
metadata:
  name: cluster-config
  namespace: kube-system
data:
  cluster-name: "{{ .OpenCenter.Cluster.ClusterName }}"
  region: "{{ .OpenCenter.Meta.Region }}"
  environment: "{{ .OpenCenter.Meta.Env | default "production" }}"
  
  # Kubernetes configuration
  k8s-version: "{{ .OpenCenter.Cluster.Kubernetes.Version }}"
  api-port: "{{ .OpenCenter.Cluster.Kubernetes.APIPort }}"
  
  # Network configuration
  pod-subnet: "{{ .OpenCenter.Cluster.Kubernetes.SubnetPods }}"
  service-subnet: "{{ .OpenCenter.Cluster.Kubernetes.SubnetServices }}"
  
  # DNS servers
  dns-servers: |
    {{- range .OpenCenter.Cluster.Networking.DNSNameservers }}
    - {{ . }}
    {{- end }}
  
  # NTP servers
  ntp-servers: |
    {{- range .OpenCenter.Cluster.Networking.NTPServers }}
    - {{ . }}
    {{- end }}
```

---

## Custom Templates

### Adding Custom Templates

1. **Create template file** in your GitOps repository:
   ```bash
   mkdir -p custom-templates
   cat > custom-templates/my-service.yaml.tmpl <<EOF
   apiVersion: v1
   kind: Service
   metadata:
     name: {{ .OpenCenter.Cluster.ClusterName }}-custom
   spec:
     type: LoadBalancer
     ports:
     - port: 80
   EOF
   ```

2. **Render template** using openCenter CLI:
   ```bash
   openCenter cluster setup my-cluster --render
   ```

### Template Validation

```bash
# Validate template syntax
openCenter template validate custom-templates/my-service.yaml.tmpl

# Render template to stdout (dry-run)
openCenter template render custom-templates/my-service.yaml.tmpl --cluster my-cluster

# Render template to file
openCenter template render custom-templates/my-service.yaml.tmpl --cluster my-cluster --output my-service.yaml
```

---

## Template Engine API

### Go API Usage

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/template"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

// Create template engine
engine := template.NewGoTemplateEngine()

// Load configuration
cfg, err := config.Load("my-cluster")
if err != nil {
    log.Fatal(err)
}

// Render template
ctx := context.Background()
result, err := engine.Render(ctx, "template.yaml", cfg)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(result))
```

### Custom Functions

```go
// Register custom function
engine.RegisterFunction("myFunc", func(s string) string {
    return strings.ToUpper(s)
})

// Register multiple functions
engine.RegisterFunctions(template.FuncMap{
    "myFunc1": func(s string) string { return strings.ToUpper(s) },
    "myFunc2": func(i int) int { return i * 2 },
})

// Use in template
// {{ myFunc "hello" }}  -> "HELLO"
// {{ myFunc2 5 }}       -> 10
```

### Template Caching

```go
// Enable caching (default)
engine.SetCacheEnabled(true)

// Disable caching (for development)
engine.SetCacheEnabled(false)

// Clear cache
engine.ClearCache()
```

### Sandboxed Rendering

```go
// Enable sandbox mode (restricts dangerous functions)
engine.EnableSandbox()

// Check if sandboxed
if engine.IsSandboxed() {
    fmt.Println("Running in sandbox mode")
}

// Disable sandbox
engine.DisableSandbox()
```

---

## Troubleshooting

### Common Template Errors

#### Undefined Variable

**Error:**
```
template: main.tf.tpl:5:10: executing "main.tf.tpl" at <.OpenCenter.Cluster.InvalidField>: can't evaluate field InvalidField
```

**Solution:**
- Check field name spelling
- Verify field exists in Config struct
- Use `{{ if .Field }}` to check existence

#### Type Mismatch

**Error:**
```
template: main.tf.tpl:10:5: executing "main.tf.tpl" at <add .StringValue 1>: error calling add: incompatible types for comparison
```

**Solution:**
- Convert types: `{{ add (toInt .StringValue) 1 }}`
- Check value types in configuration

#### Missing Function

**Error:**
```
template: main.tf.tpl:15:8: executing "main.tf.tpl" at <myCustomFunc>: function "myCustomFunc" not defined
```

**Solution:**
- Register custom function before rendering
- Check function name spelling
- Verify Sprig function name

#### Syntax Error

**Error:**
```
template: main.tf.tpl:20:1: unexpected "}" in command
```

**Solution:**
- Check template syntax
- Verify all `{{` have matching `}}`
- Check for unclosed `{{ if }}` or `{{ range }}`

### Debugging Templates

#### Print Variables

```yaml
# Debug: print entire config
{{ . | toYaml }}

# Debug: print specific section
{{ .OpenCenter.Cluster | toYaml }}

# Debug: print variable type
{{ printf "%T" .OpenCenter.Cluster.Kubernetes.MasterCount }}
```

#### Conditional Debugging

```yaml
{{- if eq (env "DEBUG") "true" }}
# Debug information
Cluster: {{ .OpenCenter.Cluster.ClusterName }}
Region: {{ .OpenCenter.Meta.Region }}
{{- end }}
```

#### Template Validation

```bash
# Validate all templates
find . -name "*.tpl" -o -name "*.tmpl" | while read f; do
  echo "Validating $f"
  openCenter template validate "$f" || echo "Failed: $f"
done
```

---

## Best Practices

### Template Organization

1. **Use descriptive names**
   ```
   ✓ kubernetes-deployment.yaml.tpl
   ✗ deploy.tpl
   ```

2. **Group related templates**
   ```
   services/
   ├── calico/
   │   ├── deployment.yaml.tpl
   │   └── configmap.yaml.tpl
   └── cert-manager/
       ├── deployment.yaml.tpl
       └── secret.yaml.tpl
   ```

3. **Separate static and dynamic content**
   - Use `.tpl` for always-rendered templates
   - Use `.tmpl` for user-customizable templates
   - Use no extension for static files

### Template Syntax

1. **Use whitespace control**
   ```yaml
   {{- if .Condition }}  # Remove leading whitespace
   content
   {{- end }}            # Remove trailing whitespace
   ```

2. **Provide defaults**
   ```yaml
   value: {{ .Field | default "default-value" }}
   ```

3. **Check existence before access**
   ```yaml
   {{- if .OpenCenter.Talos }}
   {{- if .OpenCenter.Talos.Enabled }}
   talos_version: {{ .OpenCenter.Talos.Version }}
   {{- end }}
   {{- end }}
   ```

4. **Use with for nested access**
   ```yaml
   {{- with .OpenCenter.Infrastructure.Cloud.OpenStack }}
   auth_url: {{ .AuthURL }}
   region: {{ .Region }}
   {{- end }}
   ```

### Performance

1. **Enable caching in production**
   ```go
   engine.SetCacheEnabled(true)
   ```

2. **Minimize template complexity**
   - Avoid deep nesting
   - Use functions for complex logic
   - Pre-process data when possible

3. **Use efficient functions**
   ```yaml
   # Efficient
   {{ join "," .List }}
   
   # Less efficient
   {{- range $i, $v := .List }}{{ if $i }},{{ end }}{{ $v }}{{ end }}
   ```

---

## See Also

- [Configuration Reference](configuration.md) - Complete configuration schema
- [API Reference](api.md) - Go package documentation
- [Secrets Management Reference](secrets.md) - Secrets configuration
- [Sprig Function Documentation](http://masterminds.github.io/sprig/) - Complete function reference
- [Go Template Documentation](https://pkg.go.dev/text/template) - Go template syntax
