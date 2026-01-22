# Developing Custom Plugins


## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Understanding Plugin Types](#understanding-plugin-types)
- [Task 1: Create an External Command Plugin](#task-1-create-an-external-command-plugin)
- [Task 2: Create a Service Plugin](#task-2-create-a-service-plugin)
- [Task 3: Add Plugin Lifecycle Hooks](#task-3-add-plugin-lifecycle-hooks)
- [Task 4: Create Plugin Manifest](#task-4-create-plugin-manifest)
- [Task 5: Test and Debug Plugins](#task-5-test-and-debug-plugins)
- [Task 6: Package and Distribute Plugin](#task-6-package-and-distribute-plugin)
- [Best Practices](#best-practices)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)
**doc_type**: how-to  
**priority**: 3  
**audience**: Developers extending openCenter functionality  
**related_docs**:
- [Plugin System Reference](../reference/plugin-system.md)
- [Service Configuration](./service-configuration.md)
- [Custom Templates](./custom-templates.md)

## Overview

This guide shows you how to develop custom plugins for openCenter. You'll learn how to create external command plugins, service plugins, and integrate them into the CLI workflow.

## Prerequisites

- Go 1.25.2 or later installed
- openCenter CLI installed
- Basic understanding of Go programming
- Familiarity with Cobra CLI framework

## Understanding Plugin Types

openCenter supports two types of plugins:

1. **External Command Plugins**: Standalone executables that extend CLI commands
2. **Service Plugins**: Go packages that define custom services with lifecycle management

## Task 1: Create an External Command Plugin

External plugins are executables prefixed with `openCenter-` that are automatically discovered and loaded.

### Step 1: Set Up Plugin Project

```bash
# Create plugin directory
mkdir -p ~/projects/openCenter-hello
cd ~/projects/openCenter-hello

# Initialize Go module
go mod init github.com/yourusername/openCenter-hello

# Create main.go
touch main.go
```

### Step 2: Implement Plugin Command

Create a simple plugin that adds a `hello` command:

```go
// main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	var name string

	rootCmd := &cobra.Command{
		Use:   "hello",
		Short: "Greet the user",
		Long:  "A simple plugin that greets the user by name",
		RunE: func(cmd *cobra.Command, args []string) error {
			if name == "" {
				name = "World"
			}
			fmt.Printf("Hello, %s! This is a custom openCenter plugin.\n", name)
			return nil
		},
	}

	rootCmd.Flags().StringVarP(&name, "name", "n", "", "Name to greet")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
```

### Step 3: Build and Install Plugin

```bash
# Build the plugin
go build -o openCenter-hello

# Make it executable
chmod +x openCenter-hello

# Install to plugins directory
mkdir -p ~/.config/openCenter/plugins
cp openCenter-hello ~/.config/openCenter/plugins/

# Or install to PATH
sudo cp openCenter-hello /usr/local/bin/
```

### Step 4: Test Plugin

```bash
# Plugin is automatically discovered
openCenter hello

# Use plugin flags
openCenter hello --name "Platform Engineer"
```

### Step 5: Add Subcommands

Extend your plugin with subcommands:

```go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "hello",
		Short: "Greeting commands",
	}

	// Add greet subcommand
	greetCmd := &cobra.Command{
		Use:   "greet [name]",
		Short: "Greet someone",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "World"
			if len(args) > 0 {
				name = args[0]
			}
			fmt.Printf("Hello, %s!\n", name)
			return nil
		},
	}

	// Add goodbye subcommand
	goodbyeCmd := &cobra.Command{
		Use:   "goodbye [name]",
		Short: "Say goodbye",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := "World"
			if len(args) > 0 {
				name = args[0]
			}
			fmt.Printf("Goodbye, %s!\n", name)
			return nil
		},
	}

	rootCmd.AddCommand(greetCmd, goodbyeCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
```

Rebuild and test:

```bash
go build -o openCenter-hello
cp openCenter-hello ~/.config/openCenter/plugins/

openCenter hello greet Alice
openCenter hello goodbye Bob
```

## Task 2: Create a Service Plugin

Service plugins integrate with openCenter's service management system.

### Step 1: Understand Service Plugin Interface

Service plugins must implement the `ServicePlugin` interface:

```go
type ServicePlugin interface {
	Name() string
	Type() ServiceType
	Validate(config interface{}) error
	Render(ctx context.Context, config interface{}, workspace interface{}) error
	Status(config interface{}) ServiceStatus
}
```

### Step 2: Create Plugin Package

```bash
# In openCenter-cli repository
mkdir -p internal/services/plugins/myservice
cd internal/services/plugins/myservice
touch myservice.go
```

### Step 3: Implement Service Plugin

```go
// internal/services/plugins/myservice/myservice.go
package myservice

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/config/services"
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
)

// MyServicePlugin implements the ServicePlugin interface
type MyServicePlugin struct{}

// NewMyServicePlugin creates a new instance
func NewMyServicePlugin() svc.ServicePlugin {
	return &MyServicePlugin{}
}

// Name returns the service name
func (p *MyServicePlugin) Name() string {
	return "myservice"
}

// Type returns the service type
func (p *MyServicePlugin) Type() svc.ServiceType {
	return svc.ServiceTypeCustom
}

// Validate validates the service configuration
func (p *MyServicePlugin) Validate(config interface{}) error {
	cfg, ok := config.(*services.MyServiceConfig)
	if !ok {
		return fmt.Errorf("invalid config type: expected *MyServiceConfig")
	}

	// Validate required fields
	if cfg.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}

	// Validate port range
	if cfg.Port < 1 || cfg.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}

	return nil
}

// Render renders the service templates
func (p *MyServicePlugin) Render(ctx context.Context, config interface{}, workspace interface{}) error {
	cfg, ok := config.(*services.MyServiceConfig)
	if !ok {
		return fmt.Errorf("invalid config type")
	}

	if !cfg.Enabled {
		return nil // Skip rendering if disabled
	}

	// Template rendering is handled by the gitops package
	// This method can perform additional rendering logic if needed
	return nil
}

// Status returns the current status
func (p *MyServicePlugin) Status(config interface{}) svc.ServiceStatus {
	cfg, ok := config.(*services.MyServiceConfig)
	if !ok {
		return svc.ServiceStatus{
			State:   "failed",
			Message: "Invalid configuration type",
		}
	}

	if !cfg.Enabled {
		return svc.ServiceStatus{
			State:   "disabled",
			Message: "Service is disabled",
		}
	}

	return svc.ServiceStatus{
		State:   "pending",
		Message: "MyService is configured",
		Details: map[string]interface{}{
			"endpoint": cfg.Endpoint,
			"port":     cfg.Port,
		},
	}
}
```

### Step 4: Define Service Configuration

```go
// internal/config/services/myservice.go
package services

// MyServiceConfig represents the configuration for MyService
type MyServiceConfig struct {
	Enabled  bool   `yaml:"enabled" json:"enabled"`
	Endpoint string `yaml:"endpoint" json:"endpoint"`
	Port     int    `yaml:"port" json:"port"`
	Replicas int    `yaml:"replicas" json:"replicas"`
}

// IsEnabled returns whether the service is enabled
func (c *MyServiceConfig) IsEnabled() bool {
	return c.Enabled
}

// GetStatus returns the service status
func (c *MyServiceConfig) GetStatus() string {
	if c.Enabled {
		return "configured"
	}
	return "disabled"
}
```

### Step 5: Register Plugin

Add your plugin to the registry:

```go
// internal/services/plugins/registry.go
package plugins

import (
	svc "github.com/rackerlabs/openCenter-cli/internal/services"
	"github.com/rackerlabs/openCenter-cli/internal/services/plugins/myservice"
)

// RegisterDefaultServices registers all built-in service plugins
func RegisterDefaultServices(registry svc.ServiceRegistry) error {
	// ... existing registrations ...

	// Register MyService
	myServicePlugin := myservice.NewMyServicePlugin()
	if err := registry.RegisterService(svc.ServiceDefinition{
		Name:         "myservice",
		Type:         svc.ServiceTypeCustom,
		Version:      "1.0.0",
		Description:  "Custom service plugin",
		Dependencies: []string{}, // Add dependencies if needed
		Plugin:       myServicePlugin,
	}); err != nil {
		return err
	}

	return nil
}
```

### Step 6: Create Service Templates

Create templates in the GitOps structure:

```bash
mkdir -p internal/gitops/templates/cluster-apps-base/services/myservice
```

Create deployment template:

```yaml
# internal/gitops/templates/cluster-apps-base/services/myservice/deployment.yaml.tmpl
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myservice
  namespace: {{ .OpenCenter.Services.MyService.Namespace | default "default" }}
spec:
  replicas: {{ .OpenCenter.Services.MyService.Replicas | default 1 }}
  selector:
    matchLabels:
      app: myservice
  template:
    metadata:
      labels:
        app: myservice
    spec:
      containers:
      - name: myservice
        image: myregistry/myservice:latest
        ports:
        - containerPort: {{ .OpenCenter.Services.MyService.Port }}
        env:
        - name: ENDPOINT
          value: {{ .OpenCenter.Services.MyService.Endpoint | quote }}
```

### Step 7: Test Service Plugin

```bash
# Build CLI with new plugin
mise run build

# Create test configuration
cat > test-myservice.yaml <<EOF
opencenter:
  cluster_name: test-cluster
  services:
    myservice:
      enabled: true
      endpoint: https://api.example.com
      port: 8080
      replicas: 3
EOF

# Validate configuration
./bin/openCenter config validate --config test-myservice.yaml

# Generate GitOps repo
./bin/openCenter cluster setup test-cluster --config test-myservice.yaml --render
```

## Task 3: Add Plugin Lifecycle Hooks

Lifecycle hooks allow plugins to execute custom logic during service lifecycle events.

### Step 1: Define Lifecycle Hooks

```go
// internal/services/plugins/myservice/lifecycle.go
package myservice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// PreInstall runs before service installation
func (p *MyServicePlugin) PreInstall(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running pre-install checks...")
	
	// Example: Verify prerequisites
	// Check if required secrets exist
	// Validate network connectivity
	
	return nil
}

// PostInstall runs after service installation
func (p *MyServicePlugin) PostInstall(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running post-install tasks...")
	
	// Example: Initialize service
	// Create default resources
	// Run database migrations
	
	return nil
}

// PreUpdate runs before service update
func (p *MyServicePlugin) PreUpdate(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running pre-update checks...")
	
	// Example: Backup current state
	// Validate update compatibility
	
	return nil
}

// PostUpdate runs after service update
func (p *MyServicePlugin) PostUpdate(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running post-update tasks...")
	
	// Example: Verify update success
	// Clean up old resources
	
	return nil
}

// PreRemove runs before service removal
func (p *MyServicePlugin) PreRemove(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running pre-remove tasks...")
	
	// Example: Backup data
	// Notify dependent services
	
	return nil
}

// PostRemove runs after service removal
func (p *MyServicePlugin) PostRemove(ctx context.Context, config interface{}) error {
	fmt.Println("MyService: Running post-remove cleanup...")
	
	// Example: Clean up persistent data
	// Remove external resources
	
	return nil
}
```

### Step 2: Register Lifecycle Hooks

```go
// internal/services/plugins/registry.go
if err := registry.RegisterService(svc.ServiceDefinition{
	Name:         "myservice",
	Type:         svc.ServiceTypeCustom,
	Version:      "1.0.0",
	Description:  "Custom service plugin",
	Dependencies: []string{},
	Plugin:       myServicePlugin,
	Lifecycle: svc.ServiceLifecycle{
		PreInstall:  myServicePlugin.PreInstall,
		PostInstall: myServicePlugin.PostInstall,
		PreUpdate:   myServicePlugin.PreUpdate,
		PostUpdate:  myServicePlugin.PostUpdate,
		PreRemove:   myServicePlugin.PreRemove,
		PostRemove:  myServicePlugin.PostRemove,
	},
}); err != nil {
	return err
}
```

## Task 4: Create Plugin Manifest

For external service plugins, create a manifest file:

```yaml
# ~/.config/openCenter/plugins/myservice.yaml
name: myservice
version: 1.0.0
type: custom
description: Custom service for specialized workloads

dependencies:
  - cert-manager
  - kube-prometheus-stack

templates:
  - name: deployment
    path: services/myservice/deployment.yaml.tmpl
  - name: service
    path: services/myservice/service.yaml.tmpl
  - name: ingress
    path: services/myservice/ingress.yaml.tmpl
    condition:
      enabled: true

config:
  schema:
    endpoint:
      type: string
      required: true
    port:
      type: integer
      default: 8080
    replicas:
      type: integer
      default: 1
  
  defaults:
    enabled: false
    port: 8080
    replicas: 1
  
  required:
    - endpoint
  
  validation:
    - field: port
      type: range
      operator: between
      value: "1-65535"
      message: "Port must be between 1 and 65535"
    - field: replicas
      type: range
      operator: gte
      value: "1"
      message: "Replicas must be at least 1"

metadata:
  author: Your Name
  homepage: https://github.com/yourusername/myservice
  license: Apache-2.0
  tags:
    - custom
    - api
```

## Task 5: Test and Debug Plugins

### Step 1: Enable Debug Logging

```bash
# Set debug environment variable
export OPENCENTER_DEBUG=true

# Run command with verbose output
openCenter hello --name Test
```

### Step 2: Test Plugin Discovery

```bash
# List discovered plugins
openCenter --help | grep -A 100 "Available Commands"

# Check plugin location
which openCenter-hello

# Verify plugin is executable
ls -la ~/.config/openCenter/plugins/openCenter-hello
```

### Step 3: Test Service Plugin

```bash
# Build with test
mise run build
mise run test

# Run specific service tests
go test ./internal/services/plugins/myservice/... -v

# Test with BDD scenarios
mise run godog
```

### Step 4: Validate Plugin Integration

```bash
# Test full workflow
./bin/openCenter cluster init test-plugin
./bin/openCenter cluster setup test-plugin --render
./bin/openCenter cluster validate test-plugin
```

## Task 6: Package and Distribute Plugin

### Step 1: Create Release Build

```bash
# Build for multiple platforms
GOOS=linux GOARCH=amd64 go build -o openCenter-hello-linux-amd64
GOOS=darwin GOARCH=amd64 go build -o openCenter-hello-darwin-amd64
GOOS=darwin GOARCH=arm64 go build -o openCenter-hello-darwin-arm64
GOOS=windows GOARCH=amd64 go build -o openCenter-hello-windows-amd64.exe
```

### Step 2: Create Installation Script

```bash
#!/bin/bash
# install.sh

set -e

PLUGIN_NAME="openCenter-hello"
VERSION="1.0.0"
INSTALL_DIR="${HOME}/.config/openCenter/plugins"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
esac

BINARY="${PLUGIN_NAME}-${OS}-${ARCH}"

echo "Installing ${PLUGIN_NAME} v${VERSION} for ${OS}/${ARCH}..."

# Create plugins directory
mkdir -p "${INSTALL_DIR}"

# Download binary
curl -L "https://github.com/yourusername/${PLUGIN_NAME}/releases/download/v${VERSION}/${BINARY}" \
    -o "${INSTALL_DIR}/${PLUGIN_NAME}"

# Make executable
chmod +x "${INSTALL_DIR}/${PLUGIN_NAME}"

echo "✓ ${PLUGIN_NAME} installed successfully!"
echo "Run 'openCenter hello' to test the plugin."
```

### Step 3: Create README

```markdown
# openCenter Hello Plugin

A simple greeting plugin for openCenter CLI.

## Installation

### Quick Install

```bash
curl -sSL https://raw.githubusercontent.com/yourusername/openCenter-hello/main/install.sh | bash
```

### Manual Install

1. Download the binary for your platform from [releases](https://github.com/yourusername/openCenter-hello/releases)
2. Rename to `openCenter-hello`
3. Make executable: `chmod +x openCenter-hello`
4. Move to plugins directory: `mv openCenter-hello ~/.config/openCenter/plugins/`

## Usage

```bash
# Basic greeting
openCenter hello

# Greet specific person
openCenter hello --name "Alice"

# Use subcommands
openCenter hello greet Bob
openCenter hello goodbye Charlie
```

## Development

See [DEVELOPMENT.md](DEVELOPMENT.md) for development instructions.

## License

Apache License 2.0
```

## Best Practices

1. **Follow Naming Convention**: Prefix external plugins with `openCenter-`
2. **Use Cobra Framework**: Maintain consistency with main CLI
3. **Implement Error Handling**: Return meaningful errors with context
4. **Add Help Text**: Provide clear usage documentation
5. **Test Thoroughly**: Write unit tests and integration tests
6. **Version Your Plugin**: Use semantic versioning
7. **Document Dependencies**: List required services in manifest
8. **Validate Configuration**: Check inputs before processing
9. **Handle Lifecycle**: Implement appropriate lifecycle hooks
10. **Provide Examples**: Include usage examples in documentation

## Troubleshooting

### Plugin Not Discovered

**Problem**: Plugin doesn't appear in `openCenter --help`

**Solutions**:
- Verify plugin name starts with `openCenter-`
- Check plugin is executable: `chmod +x openCenter-myplugin`
- Ensure plugin is in PATH or `~/.config/openCenter/plugins/`
- Set `OPENCENTER_PLUGINS_DIR` environment variable if using custom location

### Plugin Execution Fails

**Problem**: `Error: plugin exited with code 1`

**Solutions**:
- Test plugin directly: `./openCenter-myplugin --help`
- Check plugin logs and error messages
- Verify plugin dependencies are installed
- Enable debug mode: `export OPENCENTER_DEBUG=true`

### Service Plugin Not Registered

**Problem**: Service doesn't appear in cluster configuration

**Solutions**:
- Verify plugin is registered in `RegisterDefaultServices()`
- Check plugin implements all required interface methods
- Rebuild CLI: `mise run build`
- Validate configuration schema matches plugin expectations

## Next Steps

- [Custom Templates](./custom-templates.md) - Create custom service templates
- [Service Configuration](./service-configuration.md) - Configure service plugins
- [Plugin System Reference](../reference/plugin-system.md) - Complete plugin API documentation
- [CI/CD Integration](./cicd-integration.md) - Automate plugin testing and deployment
