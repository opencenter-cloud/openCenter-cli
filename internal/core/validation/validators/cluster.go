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

// ClusterNameValidator validates cluster and organization names.
type ClusterNameValidator struct {
	pattern *regexp.Regexp
}

// NewClusterNameValidator creates a new cluster name validator.
func NewClusterNameValidator() *ClusterNameValidator {
	return &ClusterNameValidator{
		// Pattern: must start with alphanumeric, then alphanumeric/hyphen/underscore/dot, max 63 chars
		pattern: regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`),
	}
}

// Name returns the validator name.
func (v *ClusterNameValidator) Name() string {
	return "cluster-name"
}

// Validate validates a cluster or organization name.
func (v *ClusterNameValidator) Validate(ctx context.Context, value interface{}) (*validation.ValidationResult, error) {
	result := &validation.ValidationResult{
		Valid:    true,
		Errors:   []*validation.ValidationIssue{},
		Warnings: []*validation.ValidationIssue{},
		Info:     []*validation.ValidationIssue{},
	}

	name, ok := value.(string)
	if !ok {
		result.AddError("cluster_name", "value must be a string")
		return result, nil
	}

	// Check for empty name
	if name == "" {
		result.AddError("cluster_name", "cluster name cannot be empty",
			"Provide a non-empty cluster name")
		return result, nil
	}

	// Check for path traversal sequences
	if strings.Contains(name, "..") {
		result.AddError("cluster_name", "cluster name cannot contain path traversal sequences (..)",
			"Remove '..' from the cluster name",
			"Use only alphanumeric characters, hyphens, underscores, and dots")
		return result, nil
	}

	// Check for path separators
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		result.AddError("cluster_name", "cluster name cannot contain path separators (/ or \\)",
			"Remove path separators from the cluster name",
			"Use hyphens (-) instead of slashes for separation")
		return result, nil
	}

	// Check length
	if len(name) > 63 {
		result.AddError("cluster_name",
			fmt.Sprintf("cluster name is too long (%d characters, maximum is 63)", len(name)),
			fmt.Sprintf("Shorten the cluster name to 63 characters or less (currently %d)", len(name)),
			"Consider using abbreviations or removing redundant parts")
		return result, nil
	}

	// Validate against pattern
	if !v.pattern.MatchString(name) {
		suggestions := []string{
			"Cluster name must start with an alphanumeric character",
			"Use only alphanumeric characters, hyphens (-), underscores (_), or dots (.)",
			"Maximum length is 63 characters",
		}

		// Provide specific suggestions based on the error
		if !regexp.MustCompile(`^[a-zA-Z0-9]`).MatchString(name) {
			suggestions = append(suggestions, fmt.Sprintf("'%s' starts with an invalid character", name))
		}

		result.AddError("cluster_name",
			"cluster name format is invalid",
			suggestions...)
		return result, nil
	}

	// Add warnings for potentially problematic names
	if strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") {
		result.AddWarning("cluster_name",
			"cluster name starts or ends with a hyphen, which may cause issues with some systems",
			"Consider removing leading/trailing hyphens")
	}

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
