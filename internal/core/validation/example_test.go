package validation_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// Example_basicValidation demonstrates basic validation usage.
func Example_basicValidation() {
	// Create a simple validator
	validator := validation.NewValidatorFunc("length", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, ok := value.(string)
		if !ok {
			result.AddError("value", "expected string")
			return result, nil
		}
		if len(str) < 3 {
			result.AddError("value", "must be at least 3 characters")
			return result, nil
		}
		return result, nil
	})

	// Create validation engine and register validator
	engine := validation.NewValidationEngine()
	engine.Register(validator)

	// Validate a value
	ctx := context.Background()
	result, err := engine.Validate(ctx, "length", "hello")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("Validation passed")
	} else {
		for _, e := range result.Errors {
			fmt.Printf("Error: %s\n", e.Message)
		}
	}

	// Output:
	// Validation passed
}

// Example_validationWithSuggestions demonstrates validation with suggestions.
func Example_validationWithSuggestions() {
	// Create validator that provides suggestions
	validator := validation.NewValidatorFunc("format", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, ok := value.(string)
		if !ok {
			result.AddError("value", "expected string")
			return result, nil
		}
		if strings.Contains(str, "_") {
			result.AddError("value", "underscores not allowed", "try using hyphens instead")
			return result, nil
		}
		return result, nil
	})

	engine := validation.NewValidationEngine()
	engine.Register(validator)

	ctx := context.Background()
	result, _ := engine.Validate(ctx, "format", "my_value")

	if !result.Valid {
		for _, e := range result.Errors {
			fmt.Printf("Error: %s\n", e.Message)
			if len(e.Suggestions) > 0 {
				fmt.Printf("Suggestion: %s\n", e.Suggestions[0])
			}
		}
	}

	// Output:
	// Error: underscores not allowed
	// Suggestion: try using hyphens instead
}

// Example_multipleValidators demonstrates using multiple validators.
func Example_multipleValidators() {
	engine := validation.NewValidationEngine()

	// Register length validator
	lengthValidator := validation.NewValidatorFunc("length", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)
		if len(str) < 3 {
			result.AddError("value", "too short")
		}
		return result, nil
	})

	// Register format validator
	formatValidator := validation.NewValidatorFunc("format", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)
		if strings.Contains(str, " ") {
			result.AddError("value", "spaces not allowed")
		}
		return result, nil
	})

	engine.Register(lengthValidator)
	engine.Register(formatValidator)

	ctx := context.Background()

	// Validate with length validator
	result1, _ := engine.Validate(ctx, "length", "hello")
	fmt.Printf("Length valid: %v\n", result1.Valid)

	// Validate with format validator
	result2, _ := engine.Validate(ctx, "format", "hello")
	fmt.Printf("Format valid: %v\n", result2.Valid)

	// Output:
	// Length valid: true
	// Format valid: true
}

// Example_validateAll demonstrates validating with multiple validators at once.
func Example_validateAll() {
	engine := validation.NewValidationEngine()

	// Register validators
	engine.Register(validation.NewValidatorFunc("length", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)
		if len(str) < 3 {
			result.AddError("value", "too short")
		}
		return result, nil
	}))

	engine.Register(validation.NewValidatorFunc("format", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)
		if strings.Contains(str, " ") {
			result.AddError("value", "spaces not allowed")
		}
		return result, nil
	}))

	ctx := context.Background()

	// Validate with all validators
	result, err := engine.ValidateAll(ctx, []string{"length", "format"}, "hello")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("All validations passed")
	} else {
		fmt.Printf("Validation failed with %d errors\n", len(result.Errors))
	}

	// Output:
	// All validations passed
}

// EmailValidator is a custom validator for email addresses.
type EmailValidator struct{}

func (v *EmailValidator) Name() string {
	return "email"
}

func (v *EmailValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{Valid: true}

	email, ok := value.(string)
	if !ok {
		result.AddError("email", "value must be a string")
		return result, nil
	}

	// Simple email validation
	if len(email) == 0 || !strings.Contains(email, "@") {
		result.AddError("email", "invalid email format", "email must contain @ symbol")
		return result, nil
	}

	return result, nil
}

// Example_customValidator demonstrates creating a custom validator.
func Example_customValidator() {
	// Use custom validator
	engine := validation.NewValidationEngine()
	engine.Register(&EmailValidator{})

	ctx := context.Background()
	result, _ := engine.Validate(ctx, "email", "user@example.com")

	fmt.Printf("Email valid: %v\n", result.Valid)

	// Output:
	// Email valid: true
}

// Example_contextCancellation demonstrates using context for cancellation.
func Example_contextCancellation() {
	// Create a validator that checks context
	validator := validation.NewValidatorFunc("slow", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			return &validation.ValidationResult{Valid: true}, nil
		}
	})

	engine := validation.NewValidationEngine()
	engine.Register(validator)

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Validation respects context cancellation
	result, err := engine.Validate(ctx, "slow", "value")

	if err != nil {
		fmt.Println("Validation cancelled")
	} else if result.Valid {
		fmt.Println("Validation passed")
	}

	// Output:
	// Validation cancelled
}

// Example_errorHandling demonstrates comprehensive error handling.
func Example_errorHandling() {
	engine := validation.NewValidationEngine()

	// Register a validator
	validator := validation.NewValidatorFunc("test", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)
		if strings.ToUpper(str) == str {
			result.AddError("value", "uppercase not allowed")
			result.Errors[0].Code = "INVALID_FORMAT"
		}
		return result, nil
	})
	engine.Register(validator)

	ctx := context.Background()

	// Try to use unregistered validator
	_, err := engine.Validate(ctx, "nonexistent", "value")
	if err != nil {
		fmt.Println("Error: validator not found")
	}

	// Validate with invalid input
	result, _ := engine.Validate(ctx, "test", "INVALID")
	if !result.Valid {
		fmt.Printf("Validation failed: %d errors\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s (code: %s)\n", e.Message, e.Code)
		}
	}

	// Output:
	// Error: validator not found
	// Validation failed: 1 errors
	//   - uppercase not allowed (code: INVALID_FORMAT)
}

// Example_multipleErrors demonstrates collecting multiple validation errors.
func Example_multipleErrors() {
	// Create validator that checks multiple conditions
	validator := validation.NewValidatorFunc("multi", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)

		if len(str) < 3 {
			result.AddError("value", "too short")
		}
		if len(str) > 10 {
			result.AddError("value", "too long")
		}
		if strings.Contains(str, " ") {
			result.AddError("value", "spaces not allowed")
		}

		return result, nil
	})

	engine := validation.NewValidationEngine()
	engine.Register(validator)

	ctx := context.Background()
	result, _ := engine.Validate(ctx, "multi", "a ")

	if !result.Valid {
		fmt.Printf("Found %d errors:\n", len(result.Errors))
		for _, e := range result.Errors {
			fmt.Printf("  - %s\n", e.Message)
		}
	}

	// Output:
	// Found 2 errors:
	//   - too short
	//   - spaces not allowed
}

// Example_warnings demonstrates using warnings alongside errors.
func Example_warnings() {
	// Create validator that generates warnings
	validator := validation.NewValidatorFunc("check", func(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
		result := &validation.ValidationResult{Valid: true}
		str, _ := value.(string)

		if len(str) > 50 {
			result.AddWarning("value", "unusually long", "consider shortening")
		}
		if strings.Contains(str, "test") {
			result.AddWarning("value", "contains 'test'", "remove test data before production")
		}

		return result, nil
	})

	engine := validation.NewValidationEngine()
	engine.Register(validator)

	ctx := context.Background()
	result, _ := engine.Validate(ctx, "check", "test-value")

	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Warnings: %d\n", len(result.Warnings))
	for _, w := range result.Warnings {
		fmt.Printf("  - %s\n", w.Message)
	}

	// Output:
	// Valid: true
	// Warnings: 1
	//   - contains 'test'
}
