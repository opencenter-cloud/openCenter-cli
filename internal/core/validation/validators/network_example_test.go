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

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
	"github.com/rackerlabs/opencenter-cli/internal/core/validation/validators"
)

// ExampleNetworkValidator demonstrates how to use the NetworkValidator.
func ExampleNetworkValidator() {
	// Create validation engine
	engine := validation.NewValidationEngine()

	// Register network validator
	networkValidator := validators.NewNetworkValidator()
	engine.MustRegister(networkValidator)

	// Create network configuration
	networkConfig := &validators.NetworkConfig{
		SubnetPods:     "10.244.0.0/16",
		SubnetServices: "10.96.0.0/12",
		DNSNameservers: []string{"8.8.8.8", "8.8.4.4"},
	}

	// Validate the configuration
	result, err := engine.Validate(context.Background(), "network", networkConfig)
	if err != nil {
		fmt.Printf("Validation error: %v\n", err)
		return
	}

	// Check validation result
	if result.Valid {
		fmt.Println("Network configuration is valid")
	} else {
		fmt.Println("Network configuration has errors:")
		for _, e := range result.Errors {
			fmt.Printf("  - %s: %s\n", e.Field, e.Message)
		}
	}

	// Output:
	// Network configuration is valid
}

// ExampleNetworkValidator_invalidConfiguration demonstrates validation with errors.
func ExampleNetworkValidator_invalidConfiguration() {
	// Create validator
	validator := validators.NewNetworkValidator()

	// Create invalid configuration (overlapping CIDRs)
	networkConfig := &validators.NetworkConfig{
		SubnetPods:     "10.96.1.0/24",  // Overlaps with service CIDR
		SubnetServices: "10.96.0.0/12",  // Contains pod CIDR
		DNSNameservers: []string{"invalid-ip"},
	}

	// Validate
	result, _ := validator.Validate(context.Background(), networkConfig)

	// Display errors
	fmt.Printf("Valid: %v\n", result.Valid)
	fmt.Printf("Errors: %d\n", len(result.Errors))

	// Output:
	// Valid: false
	// Errors: 2
}

// ExampleNetworkValidator_suggestions demonstrates error suggestions.
func ExampleNetworkValidator_suggestions() {
	validator := validators.NewNetworkValidator()

	// Invalid CIDR format
	networkConfig := &validators.NetworkConfig{
		SubnetPods: "10.244.0.0", // Missing prefix length
	}

	result, _ := validator.Validate(context.Background(), networkConfig)

	if !result.Valid && len(result.Errors) > 0 {
		fmt.Println("Error:", result.Errors[0].Message)
		fmt.Println("Suggestions:")
		for _, s := range result.Errors[0].Suggestions {
			fmt.Printf("  - %s\n", s)
		}
	}

	// Output:
	// Error: invalid CIDR format: 10.244.0.0
	// Suggestions:
	//   - Use CIDR notation: <ip>/<prefix>
	//   - Example: 10.244.0.0/16 or 2001:db8::/32
	//   - Ensure the IP address and prefix length are valid
}
