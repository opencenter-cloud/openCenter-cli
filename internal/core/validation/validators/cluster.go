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

package validators

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/rackerlabs/opencenter-cli/internal/core/validation"
)

// ClusterNameValidator validates cluster names according to Kubernetes naming conventions.
//
// Requirements (from Phase 2 Validation Consolidation):
//   - Length: 1-63 characters
//   - Character set: lowercase alphanumeric and hyphens only
//   - Must not start or end with hyphen
//   - Provides actionable suggestions for common mistakes
//
// Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.10
type ClusterNameValidator struct {
	pattern *regexp.Regexp
}

// NewClusterNameValidator creates a new cluster name validator.
func NewClusterNameValidator() *ClusterNameValidator {
	return &ClusterNameValidator{
		// Pattern: lowercase alphanumeric start/end, hyphens allowed in middle, 1-63 chars
		pattern: regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`),
	}
}

// Name returns the validator name.
func (v *ClusterNameValidator) Name() string {
	return "cluster-name"
}

// Priority returns the validator priority.
// Cluster name validation is fast (format check), so it has high priority.
func (v *ClusterNameValidator) Priority() int {
	return validation.PriorityHigh
}

// Validate validates a cluster name according to Kubernetes naming conventions.
//
// The validator checks:
//   - Name is not empty
//   - Length is between 1 and 63 characters
//   - Contains only lowercase alphanumeric characters and hyphens
//   - Does not start or end with a hyphen
//
// Returns a ValidationResult with errors and actionable suggestions.
func (v *ClusterNameValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := validation.NewValidationResult()

	name, ok := value.(string)
	if !ok {
		result.AddError("cluster_name", "value must be a string",
			"Provide a string value for the cluster name")
		return result, nil
	}

	// Check for empty name
	if name == "" {
		result.AddError("cluster_name", "cluster name is required",
			"Provide a name using the --name flag",
			"Example: opencenter cluster init my-cluster",
			"Name must be lowercase alphanumeric with hyphens")
		return result, nil
	}

	// Check length constraints (1-63 characters)
	if len(name) > 63 {
		result.AddError("cluster_name",
			fmt.Sprintf("cluster name too long: %d characters (max 63)", len(name)),
			"Shorten the cluster name to 63 characters or less",
			"Use abbreviations or shorter identifiers")
		return result, nil
	}

	// Check pattern: lowercase alphanumeric and hyphens only, no leading/trailing hyphens
	if !v.pattern.MatchString(name) {
		var suggestions []string

		// Provide specific suggestions based on the error
		if strings.ToLower(name) != name {
			suggestions = append(suggestions, "Convert name to lowercase")
		}
		if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
			suggestions = append(suggestions, "Remove leading or trailing hyphens")
		}
		if strings.Contains(name, "_") {
			suggestions = append(suggestions, "Replace underscores with hyphens")
		}
		if strings.ContainsAny(name, "./@#$%^&*()+=[]{}|\\:;\"'<>,?/") {
			suggestions = append(suggestions, "Remove special characters")
		}

		// Add general guidance
		suggestions = append(suggestions,
			"Name must contain only lowercase letters, numbers, and hyphens",
			"Name must start and end with alphanumeric character",
			"Example: my-cluster-123")

		result.AddError("cluster_name",
			fmt.Sprintf("invalid cluster name format: %s", name),
			suggestions...)
		return result, nil
	}

	// Add warnings for potentially problematic names
	if strings.Contains(name, "--") {
		result.AddWarning("cluster_name",
			"cluster name contains consecutive hyphens, which may be confusing",
			"Consider using single hyphens for better readability")
	}

	// Check for common reserved names
	reservedNames := []string{"default", "kube-system", "kube-public", "kube-node-lease"}
	for _, reserved := range reservedNames {
		if strings.EqualFold(name, reserved) {
			result.AddWarning("cluster_name",
				fmt.Sprintf("cluster name '%s' conflicts with Kubernetes reserved namespace", name),
				"Consider using a different name to avoid confusion")
			break
		}
	}

	return result, nil
}

// OrganizationNameValidator validates organization names using the same rules as cluster names.
type OrganizationNameValidator struct {
	clusterValidator *ClusterNameValidator
}

// NewOrganizationNameValidator creates a new organization name validator.
func NewOrganizationNameValidator() *OrganizationNameValidator {
	return &OrganizationNameValidator{
		clusterValidator: NewClusterNameValidator(),
	}
}

// Name returns the validator name.
func (v *OrganizationNameValidator) Name() string {
	return "organization-name"
}

// Priority returns the validator priority.
// Organization name validation is fast (format check), so it has high priority.
func (v *OrganizationNameValidator) Priority() int {
	return validation.PriorityHigh
}

// Validate validates an organization name.
func (v *OrganizationNameValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	// Use the same validation logic as cluster names
	result, err := v.clusterValidator.Validate(ctx, value)
	if err != nil {
		return nil, err
	}

	// Update field names in the result
	for _, issue := range result.Errors {
		if issue.Field == "cluster_name" {
			issue.Field = "organization_name"
		}
		issue.Message = strings.ReplaceAll(issue.Message, "cluster name", "organization name")
	}

	for _, issue := range result.Warnings {
		if issue.Field == "cluster_name" {
			issue.Field = "organization_name"
		}
		issue.Message = strings.ReplaceAll(issue.Message, "cluster name", "organization name")
	}

	return result, nil
}
