package pulumi

import (
	"errors"
	"fmt"
)

var (
	// ErrInvalidConfig indicates invalid Pulumi configuration.
	ErrInvalidConfig = errors.New("invalid Pulumi configuration")

	// ErrStackNotFound indicates the Pulumi stack does not exist.
	ErrStackNotFound = errors.New("Pulumi stack not found")

	// ErrBackendUnavailable indicates the Swift backend is not accessible.
	ErrBackendUnavailable = errors.New("Swift backend unavailable")

	// ErrSecretsPassphraseMissing indicates the secrets passphrase is not configured.
	ErrSecretsPassphraseMissing = errors.New("secrets passphrase missing")

	// ErrOperationFailed indicates a Pulumi operation failed.
	ErrOperationFailed = errors.New("Pulumi operation failed")

	// ErrDriftDetected indicates configuration drift was detected.
	ErrDriftDetected = errors.New("configuration drift detected")
)

// ConfigError represents a configuration validation error.
type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return fmt.Sprintf("configuration error in field %s: %s", e.Field, e.Message)
}

// OperationError represents an error during a Pulumi operation.
type OperationError struct {
	Operation string
	Cause     error
	Details   string
}

func (e *OperationError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("operation %s failed: %s (%v)", e.Operation, e.Details, e.Cause)
	}
	return fmt.Sprintf("operation %s failed: %v", e.Operation, e.Cause)
}

func (e *OperationError) Unwrap() error {
	return e.Cause
}

// BackendError represents an error with the Swift backend.
type BackendError struct {
	Container string
	Cause     error
}

func (e *BackendError) Error() string {
	return fmt.Sprintf("Swift backend error for container %s: %v", e.Container, e.Cause)
}

func (e *BackendError) Unwrap() error {
	return e.Cause
}
