---
title: Go Package API Reference
doc_type: reference
category: reference
weight: 10
---

# Go Package API Reference


## Table of Contents

- [Package Overview](#package-overview)
- [config Package](#config-package)
- [gitops Package](#gitops-package)
- [sops Package](#sops-package)
- [template Package](#template-package)
- [services Package](#services-package)
- [util Package](#util-package)
- [Usage Examples](#usage-examples)
- [See Also](#see-also)
This document provides reference documentation for the internal Go packages in openCenter CLI. These packages form the core implementation of the CLI tool and can be used for extending functionality or understanding the codebase.

## Package Overview

### Core Packages

- **config**: Configuration management, validation, and schema handling
- **gitops**: GitOps repository scaffolding and template rendering
- **sops**: Secrets management with SOPS and Age encryption
- **template**: Template engine for rendering configuration files
- **services**: Service configuration definitions

### Supporting Packages

- **cloud**: Cloud provider integrations (OpenStack, AWS, VMware)
- **provision**: Infrastructure provisioning (Terraform/OpenTofu, Ansible, Pulumi)
- **util**: Utility functions (crypto, errors, files, paths, security)

---

## config Package

The `config` package provides configuration management, validation, and schema handling for cluster configurations.

### Key Types

#### Config

The root configuration structure representing a complete cluster configuration.

```go
type Config struct {
    SchemaVersion string               `yaml:"schema_version,omitempty"`
    OpenCenter    SimplifiedOpenCenter `yaml:"opencenter"`
    OpenTofu      SimplifiedOpenTofu   `yaml:"opentofu"`
    Secrets       Secrets              `yaml:"secrets"`
    Networking    Networking           `yaml:"networking,omitempty"`
    Deployment    Deployment           `yaml:"deployment,omitempty"`
    Overrides     map[string]any       `yaml:"overrides,omitempty"`
    Metadata      ConfigMetadata       `yaml:"metadata,omitempty"`
}
```

**Key Methods:**
- `ClusterName() string` - Returns the cluster name
- `GitOps() GitOpsConfig` - Returns GitOps configuration
- `ToJSON() ([]byte, error)` - Marshals configuration to JSON

#### SimplifiedOpenCenter

Contains all openCenter-specific configuration including cluster, infrastructure, services, and GitOps settings.

```go
type SimplifiedOpenCenter struct {
    Meta           ClusterMeta
    Secrets        OpenCenterSecrets
    Infrastructure Infrastructure
    Cluster        ClusterConfig
    GitOps         GitOpsConfig
    Storage        StorageConfig
    Talos          *TalosConfig
    ManagedService ServiceMap
    Services       ServiceMap
}
```

#### ClusterConfig

Kubernetes cluster configuration including node counts, networking, and service settings.

```go
type ClusterConfig struct {
    ClusterName        string
    BaseDomain         string
    ClusterFQDN        string
    AdminEmail         string
    SSHAuthorizedKeys  []string
    Networking         ClusterNetworkingConfig
    Kubernetes         KubernetesConfig
}
```

### Core Functions

#### Configuration Loading

```go
// Load reads and unmarshals a YAML configuration file
func Load(name string) (Config, error)

// NewDefault returns a Config initialized with default values
func NewDefault(name string) Config

// GenerateCompleteConfig generates a complete configuration by merging
// schema defaults with the actual cluster configuration
func GenerateCompleteConfig(name string) (Config, error)
```

#### Configuration Saving

```go
// Save writes the configuration to a YAML file
func Save(cfg Config) error

// SaveWithOmitEmpty writes the configuration omitting empty fields
func SaveWithOmitEmpty(cfg Config) error
```

#### Path Resolution

```go
// ResolveConfigDir resolves the configuration directory
func ResolveConfigDir() (string, error)

// ConfigPath returns the absolute path to a cluster's configuration file
func ConfigPath(name string) (string, error)

// ClusterDirectoryPath returns the absolute path to a cluster's directory
func ClusterDirectoryPath(name string) (string, error)

// ClusterSecretsPath returns the absolute path to a cluster's secrets directory
func ClusterSecretsPath(name string) (string, error)
```

#### Cluster Management

```go
// List returns a sorted list of cluster names
func List() ([]string, error)

// SetActive writes the given cluster name as the active cluster
func SetActive(name string) error

// GetActive reads the active cluster name
func GetActive() (string, error)
```

#### Validation

```go
// Validate performs invariant checks on the configuration
func Validate(cfg Config) []string

// ValidateClusterName validates and sanitizes a cluster name
func ValidateClusterName(name string) error

// ParseClusterIdentifier parses a cluster identifier (org/cluster format)
func ParseClusterIdentifier(identifier string) (organization string, clusterName string, err error)
```

#### Schema Management

```go
// GenerateSchema returns a JSON schema describing the configuration structure
func GenerateSchema(pretty bool) ([]byte, error)

// GetSchemaVersion returns the current schema version
func GetSchemaVersion() string

// DetectSchemaMigrationNeeded checks if a configuration needs schema migration
func DetectSchemaMigrationNeeded(config Config) (bool, string, error)
```

### ConfigLoader

Implements loading configurations from various sources.

```go
type ConfigLoader struct {
    pathResolver PathResolverInterface
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(pathResolver PathResolverInterface) *ConfigLoader
```

**Methods:**
- `LoadFromFile(ctx context.Context, filePath string) (*Config, error)`
- `LoadFromBytes(ctx context.Context, data []byte, clusterName string) (*Config, error)`
- `LoadDefault(ctx context.Context, clusterName string) (*Config, error)`
- `SaveToFile(ctx context.Context, config *Config, filePath string) error`
- `ValidateFile(ctx context.Context, filePath string) error`

### ConfigValidator

Provides comprehensive configuration validation.

```go
type ClusterConfigValidator struct {
    autoRepair       bool
    pipelineAdapter  *PipelineAdapter
    suggestionEngine *SuggestionEngine
}

// NewConfigValidator creates a new configuration validator
func NewConfigValidator(autoRepair bool) *ClusterConfigValidator
```

**Methods:**
- `Validate(ctx context.Context, config *Config) *ConfigValidationResult`
- `ValidateStructure(ctx context.Context, config *Config) *ConfigValidationResult`
- `ValidateSemantics(ctx context.Context, config *Config) *ConfigValidationResult`
- `ValidateNetworking(ctx context.Context, config *Config) *ConfigValidationResult`
- `ValidateCloudProvider(ctx context.Context, config *Config) *ConfigValidationResult`

---

## gitops Package

The `gitops` package handles GitOps repository scaffolding and template rendering.

### Key Functions

#### Repository Initialization

```go
// IsGitOpsInitialized checks if a GitOps directory has been initialized
func IsGitOpsInitialized(gitDir string) (bool, error)

// CopyBase copies or renders embedded files from gitops-base-dir
func CopyBase(cfg config.Config, render bool) error

// CopyBaseAtomic copies files using atomic operations
func CopyBaseAtomic(cfg config.Config, render bool, workspace *GitOpsWorkspace) error
```

#### Template Rendering

```go
// RenderClusterApps renders cluster-apps-base template to applications/overlays/<cluster-name>/
func RenderClusterApps(cfg config.Config) error

// RenderInfrastructureCluster renders infrastructure-cluster-template to infrastructure/clusters/<cluster-name>/
func RenderInfrastructureCluster(cfg config.Config) error

// RenderSingleService renders only the specified service
func RenderSingleService(cfg config.Config, serviceName string, isManaged bool) error
```

#### Atomic Operations

```go
// RenderClusterAppsAtomic renders cluster apps using atomic file operations
func RenderClusterAppsAtomic(cfg config.Config, workspace *GitOpsWorkspace) error

// RenderInfrastructureClusterAtomic renders infrastructure using atomic operations
func RenderInfrastructureClusterAtomic(cfg config.Config, workspace *GitOpsWorkspace) error
```

### GitOpsWorkspace

Provides atomic file operations for GitOps repositories.

```go
type GitOpsWorkspace struct {
    RootDir string
    // Internal fields for transaction management
}
```

### AtomicWriter

Handles atomic file writes within a workspace.

```go
type AtomicWriter struct {
    workspace *GitOpsWorkspace
}

// NewAtomicWriter creates a new atomic writer
func NewAtomicWriter(workspace *GitOpsWorkspace) *AtomicWriter
```

**Methods:**
- `WriteFile(relPath string, data []byte, perm os.FileMode) error`
- `WriteFileString(relPath string, content string, perm os.FileMode) error`

---

## sops Package

The `sops` package provides secrets management with SOPS and Age encryption.

### SOPSManager Interface

```go
type SOPSManager interface {
    GetKeyManager() crypto.KeyManager
    GetEncryptor() Encryptor
    GetValidator() Validator
    EncryptOverlayFiles(ctx context.Context, overlayPath string, cfg *config.Config) error
    CreateSOPSConfig(overlayPath string, cfg *config.Config) error
    ValidateEncryption(overlayPath string, cfg *config.Config) error
    CreateSampleEncryptedSecrets(ctx context.Context, repoPath string, ageKey string) error
    EncryptRepositorySecrets(ctx context.Context, repoPath string, ageKey string) error
    CheckSOPSVersion(ctx context.Context) (string, error)
}
```

### DefaultSOPSManager

Default implementation of SOPSManager.

```go
type DefaultSOPSManager struct {
    keyManager crypto.KeyManager
    encryptor  Encryptor
    validator  Validator
    logger     *slog.Logger
}

// NewSOPSManager creates a new SOPS manager with default implementations
func NewSOPSManager() *DefaultSOPSManager

// NewDefaultSOPSManager creates a new SOPS manager with dependency injection
func NewDefaultSOPSManager(keyManager crypto.KeyManager, encryptor Encryptor, validator Validator, logger *slog.Logger) *DefaultSOPSManager
```

### Encryptor Interface

```go
type Encryptor interface {
    EncryptFile(ctx context.Context, filePath string, config EncryptionConfig) error
    DecryptFile(ctx context.Context, filePath string) ([]byte, error)
    IsFileEncrypted(filePath string) (bool, error)
}
```

### EncryptionConfig

Configuration for encryption operations.

```go
type EncryptionConfig struct {
    AgeKeys []string
    InPlace bool
    Verbose bool
}
```

### Key Management

```go
// NewKeyManager creates a new key manager
func NewKeyManager(keyDir string) crypto.KeyManager

// SetupSOPSEnvironment sets up the SOPS environment for a specific key
func SetupSOPSEnvironment(keyManager crypto.KeyManager, keyName string) error

// CheckSOPSInstallation checks if SOPS is properly installed
func CheckSOPSInstallation(ctx context.Context) error

// ValidateSOPSKeyAccess validates that a key can be used for SOPS operations
func ValidateSOPSKeyAccess(keyManager crypto.KeyManager, keyName string) error
```

---

## template Package

The `template` package provides a template engine for rendering Go templates with caching and validation.

### TemplateEngine Interface

```go
type TemplateEngine interface {
    Render(ctx context.Context, templatePath string, data interface{}) ([]byte, error)
    RenderString(ctx context.Context, templateName, templateContent string, data interface{}) ([]byte, error)
    RenderToWriter(ctx context.Context, templatePath string, data interface{}, w io.Writer) error
    ValidateTemplate(templatePath string) error
    RegisterFunction(name string, fn interface{})
    RegisterFunctions(funcs template.FuncMap)
    SetCacheEnabled(enabled bool)
    ClearCache()
    LoadFromFS(fsys fs.FS, pattern string) error
    LoadFromFile(path string) error
    ExecuteTemplate(templateName string, data interface{}) ([]byte, error)
    ExecuteTemplateToWriter(templateName string, data interface{}, w io.Writer) error
    GetTemplate(name string) (*template.Template, error)
}
```

### GoTemplateEngine

Default implementation using Go's text/template package.

```go
type GoTemplateEngine struct {
    funcMap      template.FuncMap
    cache        map[string]*template.Template
    cacheEnabled bool
    sandbox      *DefaultTemplateSandbox
    sandboxed    bool
}

// NewGoTemplateEngine creates a new Go template engine with default settings
func NewGoTemplateEngine() *GoTemplateEngine
```

**Methods:**
- `EnableSandbox()` - Enables template sandboxing for secure rendering
- `DisableSandbox()` - Disables template sandboxing
- `IsSandboxed() bool` - Returns whether sandboxing is enabled

### TemplateContext

Provides context information for template rendering.

```go
type TemplateContext struct {
    Config    interface{}
    Metadata  map[string]interface{}
    Functions template.FuncMap
}

// NewTemplateContext creates a new template context
func NewTemplateContext(config interface{}) *TemplateContext
```

**Methods:**
- `WithMetadata(key string, value interface{}) *TemplateContext`
- `WithFunction(name string, fn interface{}) *TemplateContext`
- `ToMap() map[string]interface{}`

### Template Functions

The template engine includes all [Sprig functions](http://masterminds.github.io/sprig/) by default, providing:

- String manipulation: `trim`, `upper`, `lower`, `replace`, etc.
- Math operations: `add`, `sub`, `mul`, `div`, etc.
- Date/time: `now`, `date`, `dateModify`, etc.
- Lists and dictionaries: `list`, `dict`, `merge`, etc.
- Type conversion: `toString`, `toInt`, `toBool`, etc.
- Encoding: `b64enc`, `b64dec`, `sha256sum`, etc.

---

## services Package

The `services` package defines service configuration structures.

### BaseConfig

Common fields for all services.

```go
type BaseConfig struct {
    Enabled   bool   `yaml:"enabled"`
    Status    string `yaml:"status,omitempty"`
    Namespace string `yaml:"namespace,omitempty"`
    Hostname  string `yaml:"hostname,omitempty"`
    
    // Image configuration
    ImageRepository string `yaml:"image_repository,omitempty"`
    ImageTag        string `yaml:"image_tag,omitempty"`
    
    // Version control fields
    Release string `yaml:"release,omitempty"`
    Branch  string `yaml:"branch,omitempty"`
    Uri     string `yaml:"uri,omitempty"`
    
    // GitOps source fields
    GitOpsSourceRepo    string `yaml:"gitops_source_repo,omitempty"`
    GitOpsSourceRelease string `yaml:"gitops_source_release,omitempty"`
    GitOpsSourceBranch  string `yaml:"gitops_source_branch,omitempty"`
}
```

**Methods:**
- `IsEnabled() bool` - Returns true if the service is enabled
- `GetStatus() string` - Returns the status of the service

### ServiceConfig Interface

All service configurations implement this interface.

```go
type ServiceConfig interface {
    IsEnabled() bool
    GetStatus() string
}
```

### Service-Specific Configurations

Each service has its own configuration type that embeds `BaseConfig`:

- `CalicoConfig` - Calico CNI configuration
- `CertManagerConfig` - Cert-manager configuration
- `EtcdBackupConfig` - Etcd backup configuration
- `HeadlampConfig` - Headlamp dashboard configuration
- `KeycloakConfig` - Keycloak OIDC configuration
- `PrometheusStackConfig` - Prometheus stack configuration
- `LokiConfig` - Loki logging configuration
- `VeleroConfig` - Velero backup configuration
- `WeaveGitOpsConfig` - Weave GitOps configuration

---

## util Package

The `util` package provides utility functions organized by domain.

### crypto Subpackage

Key management and cryptographic operations.

```go
// KeyManager interface for managing Age encryption keys
type KeyManager interface {
    GenerateAgeKey(name string) (*AgeKeyPair, error)
    LoadAgeKey(name string) (*AgeKeyPair, error)
    SaveAgeKey(name string, keyPair *AgeKeyPair) error
    DeleteAgeKey(name string) error
    ListAgeKeys() ([]string, error)
    GetKeyInfo(name string) (*KeyInfo, error)
    ValidateAgeKey(key string) error
    GenerateFallbackKey() (*AgeKeyPair, error)
}

// AgeKeyPair represents an Age encryption key pair
type AgeKeyPair struct {
    PublicKey  string
    PrivateKey string
}

// NewDefaultKeyManager creates a new key manager
func NewDefaultKeyManager(keyDir string) KeyManager
```

### errors Subpackage

Structured error handling.

```go
// StructuredError provides detailed error information
type StructuredError struct {
    Type        ErrorType
    Field       string
    Message     string
    Cause       error
    Suggestions []string
}

// Error types
const (
    ValidationError ErrorType = "validation"
    ConfigError     ErrorType = "config"
    FileError       ErrorType = "file"
    NetworkError    ErrorType = "network"
    SOPSError       ErrorType = "sops"
    TemplateError   ErrorType = "template"
    SystemError     ErrorType = "system"
)
```

### files Subpackage

File operations with atomic writes and backups.

```go
// AtomicWrite writes data to a file atomically
func AtomicWrite(path string, data []byte, perm os.FileMode) error

// BackupFile creates a backup of a file
func BackupFile(path string) (string, error)

// EnsureDir ensures a directory exists
func EnsureDir(path string, perm os.FileMode) error
```

### paths Subpackage

Path resolution and validation.

```go
// ExpandPath expands environment variables and tilde in paths
func ExpandPath(path string) string

// ValidatePath validates that a path is safe and accessible
func ValidatePath(path string) error

// ResolvePath resolves a path relative to a base directory
func ResolvePath(base, path string) (string, error)
```

---

## Usage Examples

### Loading and Validating Configuration

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

// Load configuration
cfg, err := config.Load("my-cluster")
if err != nil {
    log.Fatal(err)
}

// Validate configuration
validator := config.NewConfigValidator(false)
result := validator.Validate(context.Background(), &cfg)
if !result.IsValid {
    for _, err := range result.Errors {
        log.Printf("Validation error: %s", err.Message)
    }
}
```

### Rendering GitOps Templates

```go
import (
    "github.com/rackerlabs/openCenter-cli/internal/config"
    "github.com/rackerlabs/openCenter-cli/internal/gitops"
)

// Load configuration
cfg, err := config.Load("my-cluster")
if err != nil {
    log.Fatal(err)
}

// Render GitOps repository
if err := gitops.CopyBase(cfg, true); err != nil {
    log.Fatal(err)
}

if err := gitops.RenderClusterApps(cfg); err != nil {
    log.Fatal(err)
}

if err := gitops.RenderInfrastructureCluster(cfg); err != nil {
    log.Fatal(err)
}
```

### Managing Secrets with SOPS

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/sops"
    "github.com/rackerlabs/openCenter-cli/internal/config"
)

// Create SOPS manager
manager := sops.NewSOPSManager()

// Load configuration
cfg, err := config.Load("my-cluster")
if err != nil {
    log.Fatal(err)
}

// Encrypt overlay files
ctx := context.Background()
overlayPath := "/path/to/overlay"
if err := manager.EncryptOverlayFiles(ctx, overlayPath, &cfg); err != nil {
    log.Fatal(err)
}

// Create SOPS configuration
if err := manager.CreateSOPSConfig(overlayPath, &cfg); err != nil {
    log.Fatal(err)
}
```

### Using the Template Engine

```go
import (
    "context"
    "github.com/rackerlabs/openCenter-cli/internal/template"
)

// Create template engine
engine := template.NewGoTemplateEngine()

// Register custom function
engine.RegisterFunction("myFunc", func(s string) string {
    return strings.ToUpper(s)
})

// Render template
ctx := context.Background()
data := map[string]interface{}{
    "ClusterName": "my-cluster",
    "Region":      "us-east-1",
}

result, err := engine.Render(ctx, "template.yaml", data)
if err != nil {
    log.Fatal(err)
}

fmt.Println(string(result))
```

---

## See Also

- [Configuration Reference](configuration.md) - Complete configuration schema
- [Secrets Management Reference](secrets.md) - Secrets configuration details
- [Template System Reference](templates.md) - Template functions and structure
- [CLI Commands Reference](cli-commands.md) - Command-line interface
