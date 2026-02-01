// Package validation provides a unified validation system with pluggable validators
// and intelligent suggestion generation.
//
// # Overview
//
// The validation package implements a centralized validation engine that eliminates
// duplicate validation logic across the codebase. It provides:
//
//   - Pluggable validator architecture with registration system
//   - Standard ValidationResult format for consistent error reporting
//   - Intelligent suggestion engine for common mistakes
//   - Context-aware validation with metadata support
//   - Thread-safe operations for concurrent validation
//
// # Architecture
//
// The package consists of four main components:
//
//  1. ValidationEngine: Core validation orchestrator
//  2. Validator Interface: Contract for all validators
//  3. SuggestionEngine: Generates helpful suggestions for errors
//  4. Built-in Validators: Common validation implementations
//
// # Quick Start
//
// Create a validation engine and register validators:
//
//	engine := validation.NewValidationEngine()
//	engine.Register(validators.NewClusterNameValidator())
//	engine.Register(validators.NewConfigValidator())
//
// Validate a value:
//
//	result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
//	if err != nil {
//	    return err
//	}
//	if !result.Valid {
//	    for _, e := range result.Errors {
//	        fmt.Printf("Error: %s\n", e.Message)
//	        if e.Suggestion != "" {
//	            fmt.Printf("Suggestion: %s\n", e.Suggestion)
//	        }
//	    }
//	}
//
// # Validators
//
// The package includes built-in validators for common use cases:
//
//   - ClusterNameValidator: Validates cluster naming conventions
//   - ConfigValidator: Validates configuration structure and values
//   - FileValidator: Validates file paths and permissions
//   - SecurityValidator: Validates security-sensitive inputs
//
// # Custom Validators
//
// Implement the Validator interface to create custom validators:
//
//	type MyValidator struct{}
//
//	func (v *MyValidator) Name() string {
//	    return "my-validator"
//	}
//
//	func (v *MyValidator) Validate(ctx context.Context, value interface{}) ValidationResult {
//	    // Validation logic here
//	    return ValidationResult{Valid: true}
//	}
//
// Register your validator:
//
//	engine.Register(&MyValidator{})
//
// # Suggestion Engine
//
// The suggestion engine automatically enhances validation results with helpful
// suggestions based on:
//
//   - Typo detection using Levenshtein distance
//   - Context-aware recommendations
//   - Common mistake patterns
//
// Suggestions are automatically added to validation errors when the engine
// detects potential fixes.
//
// # Thread Safety
//
// All operations are thread-safe. The engine uses RWMutex for concurrent
// validator registration and validation execution.
//
// # Performance
//
// The validation engine is designed for high performance:
//
//   - Validator lookup: O(1) via map
//   - Single validation: <100μs typical
//   - Parallel validation: Scales with CPU cores
//   - Zero allocations for successful validations
//
// # Error Handling
//
// Validation errors are returned in a structured format:
//
//	type ValidationError struct {
//	    Field      string // Field that failed validation
//	    Message    string // Human-readable error message
//	    Code       string // Machine-readable error code
//	    Suggestion string // Optional suggestion for fixing
//	}
//
// # Context Support
//
// Validators can access context for:
//
//   - Cancellation and timeouts
//   - Request-scoped values
//   - Distributed tracing
//
// Example with timeout:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	result, err := engine.Validate(ctx, "cluster-name", name)
//
// # Best Practices
//
//  1. Register validators once at startup
//  2. Reuse engine instances across requests
//  3. Use context for cancellation and timeouts
//  4. Validate at system boundaries (CLI, API)
//  5. Provide specific error messages with suggestions
//  6. Use ValidateAll for multiple validators
//  7. Keep validators focused on single responsibility
//
// # Migration from Legacy Validation
//
// Replace scattered validation functions:
//
//	// Old approach
//	if err := validateClusterName(name); err != nil {
//	    return err
//	}
//	if err := validateConfig(cfg); err != nil {
//	    return err
//	}
//
//	// New approach
//	result := engine.ValidateAll(ctx, []string{"cluster-name", "config"}, data)
//	if !result.Valid {
//	    return result.Error()
//	}
//
// # Package Structure
//
//	validation/
//	├── doc.go              # Package documentation
//	├── engine.go           # ValidationEngine implementation
//	├── types.go            # Core types and interfaces
//	├── registry.go         # Validator registration
//	├── suggestions.go      # Suggestion engine
//	└── validators/         # Built-in validators
//	    ├── cluster.go      # Cluster name validation
//	    ├── config.go       # Configuration validation
//	    ├── file.go         # File validation
//	    └── security.go     # Security validation
package validation
