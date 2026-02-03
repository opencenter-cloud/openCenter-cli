// Package validation provides a unified validation engine for all validation operations.
//
// The validation package consolidates validation logic scattered across 15+ packages
// into a single ValidationEngine. This eliminates duplicate code, provides consistent
// error messages with actionable suggestions, and establishes a single source of truth
// for all validation rules.
//
// # Core Components
//
// ValidationEngine: Central registry and executor for all validators
//   - Thread-safe validator registration and lookup
//   - Single validator execution via Validate()
//   - All validators execution via ValidateAll()
//   - Result aggregation from multiple validators
//
// Validator Interface: All validators implement this interface
//   - Name() returns unique validator identifier
//   - Validate() performs validation and returns results
//   - Description() provides human-readable description
//
// ValidationResult: Rich result structure with errors, warnings, and suggestions
//   - Valid boolean indicating overall success
//   - Errors slice containing all validation failures
//   - Warnings slice for non-fatal issues
//   - Suggestions slice with actionable recommendations
//
// # Usage Example
//
//	// Create validation engine
//	errorHandler := errors.NewDefaultErrorHandler()
//	engine := validation.NewValidationEngine(errorHandler)
//
//	// Register validators
//	engine.Register(validators.NewClusterNameValidator(errorHandler))
//	engine.Register(validators.NewNetworkValidator(errorHandler))
//
//	// Validate specific domain
//	result, err := engine.Validate(ctx, "cluster-name", "my-cluster")
//	if err != nil {
//	    log.Fatalf("validation error: %v", err)
//	}
//
//	if !result.Valid {
//	    for _, validationErr := range result.Errors {
//	        fmt.Printf("Error: %s\n", validationErr.Message)
//	        for _, suggestion := range validationErr.Suggestions {
//	            fmt.Printf("  - %s\n", suggestion)
//	        }
//	    }
//	}
//
//	// Validate all domains
//	result, err = engine.ValidateAll(ctx, config)
//	if err != nil {
//	    log.Fatalf("validation error: %v", err)
//	}
//
// # Performance
//
// The ValidationEngine is designed for high performance:
//   - <1ms overhead per individual validation operation
//   - <10ms for full validation (all validators)
//   - Thread-safe concurrent validation operations
//   - Efficient O(1) validator lookup using maps
//
// # Security
//
// Security validators always run and cannot be bypassed:
//   - Path traversal detection
//   - Command injection detection
//   - Input sanitization
//   - Security violations are logged for audit trail
package validation
