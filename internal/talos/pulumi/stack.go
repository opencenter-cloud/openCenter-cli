package pulumi

import (
	"context"
	"fmt"
	"strings"
)

// StackManager manages Pulumi stack isolation and naming.
type StackManager struct {
	logger Logger
}

// NewStackManager creates a new stack manager.
func NewStackManager(logger Logger) (*StackManager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	return &StackManager{
		logger: logger,
	}, nil
}

// GenerateStackName generates a unique stack name for a cluster.
// Stack names follow the format: <cluster-name>-<environment>
func (s *StackManager) GenerateStackName(clusterName string, environment string) (string, error) {
	s.logger.Debug("generating stack name", "cluster", clusterName, "environment", environment)

	if clusterName == "" {
		return "", &ConfigError{
			Field:   "cluster_name",
			Message: "cluster name is required",
		}
	}

	// Sanitize cluster name for stack naming
	sanitizedCluster := s.sanitizeStackName(clusterName)

	var stackName string
	if environment != "" {
		sanitizedEnv := s.sanitizeStackName(environment)
		stackName = fmt.Sprintf("%s-%s", sanitizedCluster, sanitizedEnv)
	} else {
		stackName = sanitizedCluster
	}

	s.logger.Debug("stack name generated", "stack_name", stackName)
	return stackName, nil
}

// GenerateSwiftPrefix generates a unique Swift prefix for a cluster environment.
// Prefixes follow the format: <environment>/<cluster-name>/
func (s *StackManager) GenerateSwiftPrefix(clusterName string, environment string) (string, error) {
	s.logger.Debug("generating Swift prefix", "cluster", clusterName, "environment", environment)

	if clusterName == "" {
		return "", &ConfigError{
			Field:   "cluster_name",
			Message: "cluster name is required",
		}
	}

	sanitizedCluster := s.sanitizeStackName(clusterName)

	var prefix string
	if environment != "" {
		sanitizedEnv := s.sanitizeStackName(environment)
		prefix = fmt.Sprintf("%s/%s/", sanitizedEnv, sanitizedCluster)
	} else {
		prefix = fmt.Sprintf("%s/", sanitizedCluster)
	}

	s.logger.Debug("Swift prefix generated", "prefix", prefix)
	return prefix, nil
}

// ValidateStackSelection validates that a stack is properly selected before operations.
func (s *StackManager) ValidateStackSelection(ctx context.Context, stackName string) error {
	s.logger.Debug("validating stack selection", "stack", stackName)

	if stackName == "" {
		return &ConfigError{
			Field:   "stack_name",
			Message: "stack name is required",
		}
	}

	// Validate stack name format
	if !s.isValidStackName(stackName) {
		return &ConfigError{
			Field:   "stack_name",
			Message: fmt.Sprintf("invalid stack name format: %s", stackName),
		}
	}

	s.logger.Debug("stack selection validated", "stack", stackName)
	return nil
}

// EnsureStackIsolation ensures that stacks are properly isolated per cluster.
func (s *StackManager) EnsureStackIsolation(ctx context.Context, stack1Config, stack2Config *StackConfig) error {
	s.logger.Debug("ensuring stack isolation")

	if stack1Config == nil || stack2Config == nil {
		return fmt.Errorf("stack configurations cannot be nil")
	}

	// Verify different clusters have different stack names
	if stack1Config.ClusterName != stack2Config.ClusterName {
		if stack1Config.StackName == stack2Config.StackName {
			return fmt.Errorf("different clusters must have different stack names")
		}
	}

	// Verify different clusters have different Swift prefixes
	if stack1Config.ClusterName != stack2Config.ClusterName {
		if stack1Config.SwiftPrefix == stack2Config.SwiftPrefix {
			return fmt.Errorf("different clusters must have different Swift prefixes")
		}
	}

	s.logger.Debug("stack isolation verified")
	return nil
}

// sanitizeStackName sanitizes a name for use in stack naming.
func (s *StackManager) sanitizeStackName(name string) string {
	// Replace invalid characters with hyphens
	sanitized := strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return '-'
	}, name)

	// Remove leading/trailing hyphens
	sanitized = strings.Trim(sanitized, "-")

	// Convert to lowercase
	sanitized = strings.ToLower(sanitized)

	return sanitized
}

// isValidStackName validates a stack name format.
func (s *StackManager) isValidStackName(name string) bool {
	if name == "" {
		return false
	}

	// Stack names must contain only alphanumeric characters and hyphens
	for _, r := range name {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return false
		}
	}

	// Must not start or end with hyphen
	if name[0] == '-' || name[len(name)-1] == '-' {
		return false
	}

	return true
}

// StackConfig holds configuration for a Pulumi stack.
type StackConfig struct {
	ClusterName string
	Environment string
	StackName   string
	SwiftPrefix string
}

// NewStackConfig creates a new stack configuration.
func NewStackConfig(clusterName, environment string) (*StackConfig, error) {
	if clusterName == "" {
		return nil, &ConfigError{
			Field:   "cluster_name",
			Message: "cluster name is required",
		}
	}

	return &StackConfig{
		ClusterName: clusterName,
		Environment: environment,
	}, nil
}

// GenerateNames generates stack name and Swift prefix for this configuration.
func (c *StackConfig) GenerateNames(manager *StackManager) error {
	stackName, err := manager.GenerateStackName(c.ClusterName, c.Environment)
	if err != nil {
		return err
	}
	c.StackName = stackName

	swiftPrefix, err := manager.GenerateSwiftPrefix(c.ClusterName, c.Environment)
	if err != nil {
		return err
	}
	c.SwiftPrefix = swiftPrefix

	return nil
}
