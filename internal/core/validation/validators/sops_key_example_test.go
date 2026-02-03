// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validators_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
	"github.com/rackerlabs/opencenter-cli/internal/util/errors"
	"github.com/rackerlabs/opencenter-cli/internal/util/fs"
)

// ExampleSOPSKeyValidator demonstrates basic SOPS key validation.
func ExampleSOPSKeyValidator() {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "sops-example-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create a valid Age key file
	keyPath := filepath.Join(tmpDir, "age-key.txt")
	validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
	if err := os.WriteFile(keyPath, []byte(validKey), 0600); err != nil {
		fmt.Printf("Error writing key file: %v\n", err)
		return
	}

	// Create validator
	errorHandler := errors.NewDefaultErrorHandler()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validators.NewSOPSKeyValidator(fileSystem)

	// Validate the key
	result, err := validator.Validate(context.Background(), keyPath)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("SOPS key is valid")
	} else {
		fmt.Println("SOPS key validation failed")
		for _, e := range result.Errors {
			fmt.Printf("Error: %s\n", e.Message)
		}
	}

	// Output:
	// SOPS key is valid
}

// ExampleSOPSKeyValidator_missingFile demonstrates validation of a missing key file.
func ExampleSOPSKeyValidator_missingFile() {
	// Create validator
	errorHandler := errors.NewDefaultErrorHandler()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validators.NewSOPSKeyValidator(fileSystem)

	// Validate a non-existent key file
	result, err := validator.Validate(context.Background(), "/nonexistent/key.txt")
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if !result.Valid {
		fmt.Println("Validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("  %s\n", e.Message)
			if len(e.Suggestions) > 0 {
				fmt.Println("  Suggestions:")
				for _, s := range e.Suggestions {
					fmt.Printf("    - %s\n", s)
				}
			}
		}
	}

	// Output:
	// Validation failed:
	//   SOPS key file not found: /nonexistent/key.txt
	//   Suggestions:
	//     - Generate a new Age key: age-keygen -o /nonexistent/key.txt
	//     - Or use: opencenter sops generate-key
	//     - Verify the file path is correct
}

// ExampleSOPSKeyValidator_invalidFormat demonstrates validation of an invalid key format.
func ExampleSOPSKeyValidator_invalidFormat() {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "sops-example-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create an invalid key file
	keyPath := filepath.Join(tmpDir, "invalid-key.txt")
	invalidKey := "INVALID-KEY-FORMAT"
	if err := os.WriteFile(keyPath, []byte(invalidKey), 0600); err != nil {
		fmt.Printf("Error writing key file: %v\n", err)
		return
	}

	// Create validator
	errorHandler := errors.NewDefaultErrorHandler()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validators.NewSOPSKeyValidator(fileSystem)

	// Validate the key
	result, err := validator.Validate(context.Background(), keyPath)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if !result.Valid {
		fmt.Println("Validation failed:")
		for _, e := range result.Errors {
			fmt.Printf("  %s\n", e.Message)
		}
	}

	// Output:
	// Validation failed:
	//   invalid Age key format
}

// ExampleSOPSKeyValidator_insecurePermissions demonstrates validation with insecure permissions.
func ExampleSOPSKeyValidator_insecurePermissions() {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "sops-example-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create a valid key file with insecure permissions
	keyPath := filepath.Join(tmpDir, "age-key.txt")
	validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
	if err := os.WriteFile(keyPath, []byte(validKey), 0644); err != nil {
		fmt.Printf("Error writing key file: %v\n", err)
		return
	}

	// Create validator
	errorHandler := errors.NewDefaultErrorHandler()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validators.NewSOPSKeyValidator(fileSystem)

	// Validate the key
	result, err := validator.Validate(context.Background(), keyPath)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("Key format is valid")
	}

	if result.HasWarnings() {
		fmt.Println("Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("  %s\n", w.Message)
		}
	}

	// Output:
	// Key format is valid
	// Warnings:
	//   insecure key file permissions: 644 (should be 0600)
}

// ExampleSOPSKeyValidator_withEngine demonstrates using the validator with ValidationEngine.
func ExampleSOPSKeyValidator_withEngine() {
	// Create a temporary directory for testing
	tmpDir, err := os.MkdirTemp("", "sops-example-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create a valid Age key file
	keyPath := filepath.Join(tmpDir, "age-key.txt")
	validKey := "AGE-SECRET-KEY-1ZYXWVUTSRQPONMLKJIHGFEDCBA9876543210ZYXWVUTSRQPONMLKJIHGFE"
	if err := os.WriteFile(keyPath, []byte(validKey), 0600); err != nil {
		fmt.Printf("Error writing key file: %v\n", err)
		return
	}

	// Create validation engine
	engine := validation.NewValidationEngine()

	// Register SOPS key validator
	errorHandler := errors.NewDefaultErrorHandler()
	fileSystem := fs.NewDefaultFileSystem(errorHandler)
	validator := validators.NewSOPSKeyValidator(fileSystem)
	if err := engine.Register(validator); err != nil {
		fmt.Printf("Registration error: %v\n", err)
		return
	}

	// Validate using the engine
	result, err := engine.Validate(context.Background(), "sops-key", keyPath)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	if result.Valid {
		fmt.Println("SOPS key validation passed")
	}

	// Output:
	// SOPS key validation passed
}
