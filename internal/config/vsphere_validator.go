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

package config

import (
	"context"

	"github.com/rackerlabs/openCenter-cli/internal/util/errors"
)

// VSphereValidator validates vSphere-specific configuration.
type VSphereValidator struct{}

// NewVSphereValidator creates a new vSphere validator.
func NewVSphereValidator() *VSphereValidator {
	return &VSphereValidator{}
}

// ValidateCredentials validates vSphere credentials.
func (v *VSphereValidator) ValidateCredentials(ctx context.Context, config *Config) []*errors.StructuredError {
	// vSphere is not currently supported in the simplified config structure
	return []*errors.StructuredError{}
}

// ValidateConfiguration validates vSphere configuration.
func (v *VSphereValidator) ValidateConfiguration(ctx context.Context, config *Config) []*errors.StructuredError {
	// vSphere is not currently supported in the simplified config structure
	return []*errors.StructuredError{}
}

// ValidateConnectivity validates connectivity to vSphere services.
func (v *VSphereValidator) ValidateConnectivity(ctx context.Context, config *Config) []*errors.StructuredError {
	// vSphere is not currently supported in the simplified config structure
	return []*errors.StructuredError{}
}

// GetRequiredFields returns the list of required fields for vSphere.
func (v *VSphereValidator) GetRequiredFields() []string {
	// vSphere is not currently supported in the simplified config structure
	return []string{}
}
