package cluster

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

// ValidationError represents a single parsed validation error with structured fields.
type ValidationError struct {
	Section    string `json:"section"`
	Field      string `json:"field"`
	YAMLPath   string `json:"yaml_path"`
	Tag        string `json:"tag"`
	Message    string `json:"message"`
	Category   string `json:"category"` // "configuration", "connectivity", "credentials", "provider"
	Severity   string `json:"severity,omitempty"`
	Suggestion string `json:"suggestion,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

// ValidationOutput is the structured JSON output for CI/CD pipelines.
type ValidationOutput struct {
	Valid           bool                         `json:"valid"`
	Target          ValidationTarget             `json:"target"`
	Mode            string                       `json:"mode"`
	Summary         ValidationSummary            `json:"summary"`
	Checks          ValidationCheckSummary       `json:"checks"`
	Details         ValidationDetails            `json:"details"`
	Services        []ValidationServiceReport    `json:"services"`
	GitOps          ValidationGitOpsReport       `json:"gitops"`
	Missing         []ValidationMissing          `json:"missing"`
	ActionItems     []string                     `json:"action_items"`
	Errors          []ValidationError            `json:"errors,omitempty"`
	ErrorsBySection map[string][]ValidationError `json:"errors_by_section,omitempty"`
	Warnings        []string                     `json:"warnings,omitempty"`
	Suggestions     []string                     `json:"suggestions,omitempty"`
	Issues          []v2.ValidationIssue         `json:"issues,omitempty"`
	SchemaVersion   string                       `json:"schema_version,omitempty"`
	DebugConfigPath string                       `json:"debug_config_path,omitempty"`
}

// ValidationSummary provides a quick count overview.
type ValidationSummary struct {
	TotalErrors   int            `json:"total_errors"`
	TotalWarnings int            `json:"total_warnings"`
	BySection     map[string]int `json:"by_section,omitempty"`
}

// ValidationDetails shows pass/fail per validation category.
type ValidationDetails struct {
	Configuration bool `json:"configuration"`
	Connectivity  bool `json:"connectivity"`
	Provider      bool `json:"provider"`
}

// goStructToYAMLPath converts a Go struct path like "Config.OpenCenter.Meta.Env"
// to a YAML config path like "opencenter.meta.env".
var goToYAMLMap = map[string]string{
	"Config":          "",
	"OpenCenter":      "opencenter",
	"Meta":            "meta",
	"Cluster":         "cluster",
	"Infrastructure":  "infrastructure",
	"SSH":             "ssh",
	"Networking":      "networking",
	"Compute":         "compute",
	"Storage":         "storage",
	"Cloud":           "cloud",
	"OpenStack":       "openstack",
	"AWS":             "aws",
	"GCP":             "gcp",
	"Azure":           "azure",
	"VMware":          "vmware",
	"Services":        "services",
	"ManagedServices": "managed_services",
	"GitOps":          "gitops",
	"Deployment":      "deployment",
	"OpenTofu":        "opentofu",
	"Backend":         "backend",
	"Secrets":         "secrets",
	"Security":        "security",
	"Kubernetes":      "kubernetes",
	"NetworkPlugin":   "network_plugin",
	"OIDC":            "oidc",
	"Bastion":         "bastion",
	"NodeNaming":      "node_naming",
	"VLAN":            "vlan",
	"SOPSConfig":      "sops",
	"Global":          "global",
}

// fieldNameToYAML converts a Go field name to its YAML equivalent using common patterns.
func fieldNameToYAML(name string) string {
	if mapped, ok := goToYAMLMap[name]; ok {
		return mapped
	}
	// Convert CamelCase/PascalCase to snake_case
	return camelToSnake(name)
}

// camelToSnake converts CamelCase to snake_case.
func camelToSnake(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// Check if previous char was lowercase or next char is lowercase
			if i+1 < len(s) && s[i+1] >= 'a' && s[i+1] <= 'z' {
				result.WriteByte('_')
			} else if s[i-1] >= 'a' && s[i-1] <= 'z' {
				result.WriteByte('_')
			}
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}

func goPathToYAMLPath(goPath string) string {
	parts := strings.Split(goPath, ".")
	var yamlParts []string
	for _, part := range parts {
		yaml := fieldNameToYAML(part)
		if yaml != "" {
			yamlParts = append(yamlParts, yaml)
		}
	}
	return strings.Join(yamlParts, ".")
}

// sectionFromYAMLPath extracts the top-level section from a YAML path.
// e.g., "opencenter.infrastructure.ssh.authorized_keys" -> "Infrastructure > SSH"
func sectionFromYAMLPath(yamlPath string) string {
	parts := strings.Split(yamlPath, ".")
	if len(parts) < 2 {
		return "General"
	}

	// Skip "opencenter" prefix for section grouping
	start := 0
	if parts[0] == "opencenter" {
		start = 1
	}

	if start >= len(parts) {
		return "General"
	}

	// Build section from the structural parts (not the leaf field)
	sectionParts := parts[start:]
	if len(sectionParts) <= 1 {
		return titleCase(sectionParts[0])
	}

	// Use up to 2 levels for section grouping
	depth := 2
	if len(sectionParts)-1 < depth {
		depth = len(sectionParts) - 1
	}

	var sections []string
	for i := 0; i < depth; i++ {
		sections = append(sections, titleCase(sectionParts[i]))
	}
	return strings.Join(sections, " > ")
}

func titleCase(s string) string {
	s = strings.ReplaceAll(s, "_", " ")
	words := strings.Fields(s)
	for i, w := range words {
		if len(w) > 0 {
			// Keep common acronyms uppercase
			upper := strings.ToUpper(w)
			switch upper {
			case "SSH", "AWS", "GCP", "DNS", "NTP", "VRRP", "OIDC", "API", "VLAN", "SOPS", "FQDN", "VPC", "AMI", "CSI":
				words[i] = upper
			default:
				words[i] = strings.ToUpper(w[:1]) + w[1:]
			}
		}
	}
	return strings.Join(words, " ")
}

// tagToHumanMessage converts a validator tag to a human-readable message.
func tagToHumanMessage(tag, field string) string {
	switch tag {
	case "required":
		return "required, currently empty"
	case "required_if":
		return "conditionally required based on related field"
	case "oneof":
		return oneofMessage(field)
	case "email":
		return "must be a valid email address"
	case "url":
		return "must be a valid URL"
	case "fqdn":
		return "must be a valid fully qualified domain name"
	case "ipv4":
		return "must be a valid IPv4 address"
	case "cidrv4":
		return "must be a valid IPv4 CIDR (e.g., 10.0.0.0/24)"
	case "dns1123":
		return "must be a valid DNS name (lowercase alphanumeric and hyphens)"
	case "semver":
		return "must be a valid semantic version (e.g., 1.28.0)"
	case "min":
		return "value is below minimum"
	case "max":
		return "value exceeds maximum"
	case "eq":
		return "must equal the expected value"
	default:
		return fmt.Sprintf("failed validation: %s", tag)
	}
}

// oneofMessage returns the allowed values for known oneof fields.
func oneofMessage(field string) string {
	knownOneofs := map[string]string{
		"Env":                         "must be one of: dev, staging, production",
		"Provider":                    "must be one of: openstack, aws, gcp, azure, baremetal, vsphere",
		"LoadbalancerProvider":        "must be one of: ovn, octavia, metallb, cloud-native",
		"Type":                        "must be one of: s3, local, remote",
		"WorkerVolumeDestinationType": "must be one of: volume, local",
		"WorkerVolumeSourceType":      "must be one of: image, volume, snapshot",
		"DestinationType":             "must be one of: volume, local",
		"SourceType":                  "must be one of: image, volume, snapshot",
		"Effect":                      "must be one of: NoSchedule, PreferNoSchedule, NoExecute",
		"PodSecurityStandards":        "must be one of: privileged, baseline, restricted",
		"IPIPMode":                    "must be one of: Always, CrossSubnet, Never",
		"VXLANMode":                   "must be one of: Always, CrossSubnet, Never",
		"TunnelMode":                  "must be one of: vxlan, geneve, disabled",
	}
	if msg, ok := knownOneofs[field]; ok {
		return msg
	}
	return "invalid value, check allowed options"
}

// inactiveProviderPrefixes returns YAML path prefixes for providers that are NOT active.
func inactiveProviderPrefixes(activeProvider string) []string {
	allProviders := map[string]string{
		"openstack": "opencenter.infrastructure.cloud.openstack",
		"aws":       "opencenter.infrastructure.cloud.aws",
		"gcp":       "opencenter.infrastructure.cloud.gcp",
		"azure":     "opencenter.infrastructure.cloud.azure",
		"vmware":    "opencenter.infrastructure.cloud.vmware",
	}
	var inactive []string
	for provider, prefix := range allProviders {
		if provider != activeProvider {
			inactive = append(inactive, prefix)
		}
	}
	return inactive
}

// schemaErrorRegex matches individual validation errors from go-playground/validator.
var schemaErrorRegex = regexp.MustCompile(`Key:\s*'([^']+)'\s*Error:Field validation for '([^']+)' failed on the '([^']+)' tag`)

// parseSchemaErrors parses the raw error string from go-playground/validator into structured errors.
func parseSchemaErrors(rawError string, activeProvider string) []ValidationError {
	matches := schemaErrorRegex.FindAllStringSubmatch(rawError, -1)
	if len(matches) == 0 {
		// Not a schema validation error, return as-is
		return []ValidationError{{
			Section:  "General",
			Field:    "",
			YAMLPath: "",
			Tag:      "",
			Message:  rawError,
			Category: "configuration",
		}}
	}

	inactivePrefixes := inactiveProviderPrefixes(activeProvider)
	var errors []ValidationError

	for _, match := range matches {
		goPath := match[1] // e.g., "Config.OpenCenter.Meta.Env"
		field := match[2]  // e.g., "Env"
		tag := match[3]    // e.g., "oneof"

		yamlPath := goPathToYAMLPath(goPath)

		// Filter out errors for inactive providers
		skip := false
		for _, prefix := range inactivePrefixes {
			if strings.HasPrefix(yamlPath, prefix) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		section := sectionFromYAMLPath(yamlPath)
		message := tagToHumanMessage(tag, field)

		errors = append(errors, ValidationError{
			Section:  section,
			Field:    field,
			YAMLPath: yamlPath,
			Tag:      tag,
			Message:  message,
			Category: "configuration",
		})
	}

	return errors
}

// yamlFieldErrorRegex matches YAML type errors like "line 12: field stage not found in type v2.MetaConfig"
var yamlFieldErrorRegex = regexp.MustCompile(`^line (\d+): field (\S+) not found in type v2\.(\w+)$`)

// configTypeToSection maps v2 Go type names to human-readable sections.
var configTypeToSection = map[string]string{
	"MetaConfig":               "Meta",
	"ClusterConfig":            "Cluster",
	"KubernetesConfig":         "Cluster > Kubernetes",
	"KubernetesSecurityConfig": "Cluster > Kubernetes",
	"CalicoConfig":             "Cluster > Kubernetes",
	"CiliumConfig":             "Cluster > Kubernetes",
	"KubeOVNConfig":            "Cluster > Kubernetes",
	"OIDCConfig":               "Cluster > Kubernetes",
	"InfrastructureConfig":     "Infrastructure",
	"SSHConfig":                "Infrastructure > SSH",
	"NetworkingConfig":         "Infrastructure > Networking",
	"ComputeConfig":            "Infrastructure > Compute",
	"StorageConfig":            "Infrastructure > Storage",
	"StoragePluginConfig":      "Infrastructure > Storage",
	"OpenStackCloudConfig":     "Infrastructure > Cloud",
	"AWSCloudConfig":           "Infrastructure > Cloud",
	"VMwareCloudConfig":        "Infrastructure > Cloud",
	"BastionConfig":            "Infrastructure > Compute",
	"NodeNamingConfig":         "Infrastructure > Compute",
	"GitOpsConfig":             "Gitops",
	"DeploymentConfig":         "Deployment",
	"OpenTofuConfig":           "Opentofu",
	"SecretsConfig":            "Secrets",
	"OpenCenterConfig":         "General",
	"Config":                   "General",
}

// parseYAMLFieldError attempts to parse a single YAML "field not found" error
// into a structured ValidationError. Returns the error and true if parsed.
func parseYAMLFieldError(text string) (ValidationError, bool) {
	m := yamlFieldErrorRegex.FindStringSubmatch(text)
	if m == nil {
		return ValidationError{}, false
	}

	line := m[1]
	field := m[2]
	typeName := m[3]

	section := "General"
	if s, ok := configTypeToSection[typeName]; ok {
		section = s
	}

	return ValidationError{
		Section:  section,
		Field:    field,
		YAMLPath: "",
		Tag:      "unknown_field",
		Message:  fmt.Sprintf("unknown field '%s' (line %s) — not recognized in current schema, may need migration", field, line),
		Category: "configuration",
	}, true
}

// parseRawErrors parses all raw error strings into structured ValidationErrors.
func parseRawErrors(rawErrors []string, activeProvider string) []ValidationError {
	var allErrors []ValidationError

	for _, raw := range rawErrors {
		// Check for category prefix
		category := "configuration"
		errorText := raw

		if strings.HasPrefix(raw, "[validation] ") {
			errorText = strings.TrimPrefix(raw, "[validation] ")
			// Strip the stage prefix if present
			if strings.HasPrefix(errorText, "stage ") {
				if idx := strings.Index(errorText, ": "); idx != -1 {
					errorText = errorText[idx+2:]
				}
			}
		} else if strings.HasPrefix(raw, "[connectivity] ") {
			category = "connectivity"
			errorText = strings.TrimPrefix(raw, "[connectivity] ")
		} else if strings.HasPrefix(raw, "[credentials] ") {
			category = "credentials"
			errorText = strings.TrimPrefix(raw, "[credentials] ")
		} else if strings.HasPrefix(raw, "[") {
			// Provider-specific errors like [openstack], [aws]
			if idx := strings.Index(raw, "] "); idx != -1 {
				category = raw[1:idx]
				errorText = raw[idx+2:]
			}
		}

		// Try to parse as schema validation errors
		if strings.Contains(errorText, "schema validation failed:") {
			parsed := parseSchemaErrors(errorText, activeProvider)
			for i := range parsed {
				if parsed[i].Category == "configuration" {
					parsed[i].Category = category
				}
			}
			allErrors = append(allErrors, parsed...)
		} else if parsed, ok := parseYAMLFieldError(errorText); ok {
			parsed.Category = category
			allErrors = append(allErrors, parsed)
		} else {
			allErrors = append(allErrors, ValidationError{
				Section:  "General",
				Field:    "",
				YAMLPath: "",
				Tag:      "",
				Message:  errorText,
				Category: category,
			})
		}
	}

	return allErrors
}

func validationErrorsForResult(result *ValidationResult, activeProvider string) []ValidationError {
	if len(result.Issues) == 0 {
		return parseRawErrors(result.Errors, activeProvider)
	}

	errors := make([]ValidationError, 0, len(result.Issues))
	for _, issue := range result.Issues {
		if issue.Severity != v2.SeverityError {
			continue
		}
		yamlPath := issue.Path
		section := "General"
		if yamlPath != "" {
			section = sectionFromYAMLPath(yamlPath)
		}
		errors = append(errors, ValidationError{
			Section:    section,
			Field:      fieldFromYAMLPath(yamlPath),
			YAMLPath:   yamlPath,
			Tag:        string(issue.Severity),
			Message:    issue.Message,
			Category:   string(issue.Category),
			Severity:   string(issue.Severity),
			Suggestion: issue.Suggestion,
		})
	}
	return errors
}

func fieldFromYAMLPath(path string) string {
	if path == "" {
		return ""
	}
	parts := strings.Split(path, ".")
	return parts[len(parts)-1]
}

// groupBySection groups errors by their section.
func groupBySection(errors []ValidationError) map[string][]ValidationError {
	grouped := make(map[string][]ValidationError)
	for _, err := range errors {
		grouped[err.Section] = append(grouped[err.Section], err)
	}
	return grouped
}

// sortedSections returns section names in a stable, logical order.
func sortedSections(grouped map[string][]ValidationError) []string {
	// Define preferred order for common sections
	order := map[string]int{
		"General":                     0,
		"Meta":                        1,
		"Cluster":                     2,
		"Cluster > Kubernetes":        3,
		"Infrastructure > SSH":        4,
		"Infrastructure > Networking": 5,
		"Infrastructure > Compute":    6,
		"Infrastructure > Storage":    7,
		"Infrastructure > Cloud":      8,
		"Gitops":                      9,
		"Deployment":                  10,
		"Opentofu":                    11,
		"Opentofu > Backend":          12,
		"Secrets":                     13,
	}

	sections := make([]string, 0, len(grouped))
	for s := range grouped {
		sections = append(sections, s)
	}

	sort.Slice(sections, func(i, j int) bool {
		oi, oki := order[sections[i]]
		oj, okj := order[sections[j]]
		if oki && okj {
			return oi < oj
		}
		if oki {
			return true
		}
		if okj {
			return false
		}
		return sections[i] < sections[j]
	})

	return sections
}

// FormatResultGrouped formats the validation result with grouped, human-readable output.
func (s *ValidateService) FormatResultGrouped(result *ValidationResult, provider string) string {
	var out strings.Builder

	if result.Valid {
		out.WriteString("✓ Validation successful\n")
	} else {
		out.WriteString("✗ Validation failed\n")
	}

	out.WriteString(s.formatOperatorReport(result))

	if result.Valid {
		if len(result.Warnings) > 0 {
			out.WriteString(s.formatWarnings(result.Warnings))
		}
		return out.String()
	}

	if provider == "" {
		provider = result.Provider
	}

	parsed := validationErrorsForResult(result, provider)
	grouped := groupBySection(parsed)
	sections := sortedSections(grouped)

	// Summary counts
	sectionCounts := make(map[string]int)
	for section, errs := range grouped {
		sectionCounts[section] = len(errs)
	}
	out.WriteString(fmt.Sprintf("\n%d error(s), %d warning(s)\n", len(parsed), len(result.Warnings)))

	// Grouped errors
	if len(parsed) > 0 {
		out.WriteString("\nErrors:\n")
		for _, section := range sections {
			errs := grouped[section]
			out.WriteString(fmt.Sprintf("\n  %s: (%d)\n", section, len(errs)))
			for _, e := range errs {
				if e.YAMLPath != "" {
					out.WriteString(fmt.Sprintf("    ✗ %s — %s\n", e.YAMLPath, e.Message))
				} else {
					out.WriteString(fmt.Sprintf("    ✗ %s\n", e.Message))
				}
			}
		}
	}

	// Warnings
	if len(result.Warnings) > 0 {
		out.WriteString(s.formatWarnings(result.Warnings))
	}

	// Suggestions
	if len(result.ActionItems) > 0 {
		out.WriteString("\nAction Items:\n")
		for i, item := range result.ActionItems {
			out.WriteString(fmt.Sprintf("  %d. %s\n", i+1, item))
		}
	} else if len(result.Suggestions) > 0 {
		out.WriteString("\nSuggestions:\n")
		seen := make(map[string]bool)
		for _, suggestion := range result.Suggestions {
			if suggestion != "" && !seen[suggestion] {
				seen[suggestion] = true
				out.WriteString(fmt.Sprintf("  → %s\n", suggestion))
			}
		}
	}

	return out.String()
}

func (s *ValidateService) formatOperatorReport(result *ValidationResult) string {
	var out strings.Builder
	if result.Target.Cluster != "" || result.Target.Organization != "" || result.Target.Provider != "" || result.ValidationMode != "" {
		out.WriteString("\n")
		if result.Target.Cluster != "" {
			clusterName := result.Target.Cluster
			if result.Target.Organization != "" && !strings.Contains(clusterName, "/") {
				clusterName = result.Target.Organization + "/" + clusterName
			}
			out.WriteString(fmt.Sprintf("Cluster: %s\n", clusterName))
		}
		if result.Target.Organization != "" {
			out.WriteString(fmt.Sprintf("Organization: %s\n", result.Target.Organization))
		}
		if firstNonEmptyFormatter(result.Target.Provider, result.Provider) != "" {
			out.WriteString(fmt.Sprintf("Provider: %s\n", firstNonEmptyFormatter(result.Target.Provider, result.Provider)))
		}
		if result.ValidationMode != "" {
			out.WriteString(fmt.Sprintf("Validation mode: %s\n", result.ValidationMode))
		}
	}

	if result.CheckSummary.Total() > 0 {
		out.WriteString(fmt.Sprintf("\nSummary: %s\n", validationSummaryWord(result.Valid)))
		out.WriteString(fmt.Sprintf("Checks: %d passed, %d failed, %d warning, %d skipped\n",
			result.CheckSummary.Passed,
			result.CheckSummary.Failed,
			result.CheckSummary.Warnings,
			result.CheckSummary.Skipped,
		))
	}

	if len(result.ServiceReports) > 0 {
		out.WriteString("\nServices:\n")
		for _, service := range result.ServiceReports {
			line := fmt.Sprintf("  %s %s", checkStatusIcon(service.Status), service.Name)
			if service.Message != "" {
				line += " " + service.Message
			}
			out.WriteString(line + "\n")
		}
	}

	if len(result.GitOpsReport.Checks) > 0 {
		out.WriteString("\nGitOps:\n")
		if result.GitOpsReport.RepositoryURL != "" {
			out.WriteString(fmt.Sprintf("  Repository: %s\n", result.GitOpsReport.RepositoryURL))
		}
		if result.GitOpsReport.LocalPath != "" {
			out.WriteString(fmt.Sprintf("  Local path: %s\n", result.GitOpsReport.LocalPath))
		}
		for _, check := range result.GitOpsReport.Checks {
			line := fmt.Sprintf("  %s %s", checkStatusIcon(check.Status), check.Name)
			switch {
			case check.Message != "":
				line += " " + check.Message
			case check.Detail != "":
				line += " " + check.Detail
			}
			out.WriteString(line + "\n")
		}
	}

	if len(result.Missing) > 0 {
		out.WriteString("\nMissing:\n")
		for _, missing := range result.Missing {
			line := "  - " + missing.Path
			if missing.Message != "" {
				line += " — " + missing.Message
			}
			out.WriteString(line + "\n")
		}
	}

	return out.String()
}

func validationSummaryWord(valid bool) string {
	if valid {
		return "passed"
	}
	return "failed"
}

func checkStatusIcon(status CheckStatus) string {
	switch status {
	case CheckStatusPass:
		return "✓"
	case CheckStatusFail:
		return "✗"
	case CheckStatusWarn:
		return "⚠"
	case CheckStatusSkip:
		return "-"
	default:
		return "-"
	}
}

func firstNonEmptyFormatter(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}


func (s *ValidateService) formatWarnings(warnings []string) string {
	var out strings.Builder
	out.WriteString("\nWarnings:\n")
	for _, w := range warnings {
		out.WriteString(fmt.Sprintf("  ⚠ %s\n", w))
	}
	return out.String()
}

// FormatResultJSON returns the validation result as structured JSON.
func (s *ValidateService) FormatResultJSON(result *ValidationResult, provider string) (string, error) {
	if provider == "" {
		provider = result.Provider
	}

	parsed := validationErrorsForResult(result, provider)
	grouped := groupBySection(parsed)

	sectionCounts := make(map[string]int)
	for section, errs := range grouped {
		sectionCounts[section] = len(errs)
	}

	// Deduplicate suggestions
	var suggestions []string
	seen := make(map[string]bool)
	for _, s := range result.Suggestions {
		if s != "" && !seen[s] {
			seen[s] = true
			suggestions = append(suggestions, s)
		}
	}

	output := ValidationOutput{
		Valid:  result.Valid,
		Target: result.Target,
		Mode:   result.ValidationMode,
		Summary: ValidationSummary{
			TotalErrors:   len(parsed),
			TotalWarnings: len(result.Warnings),
			BySection:     sectionCounts,
		},
		Checks: result.CheckSummary,
		Details: ValidationDetails{
			Configuration: result.ConfigValid,
			Connectivity:  result.ConnectivityValid,
			Provider:      result.ProviderValid,
		},
		Services:        result.ServiceReports,
		GitOps:          result.GitOpsReport,
		Missing:         result.Missing,
		ActionItems:     result.ActionItems,
		Errors:          parsed,
		ErrorsBySection: grouped,
		Warnings:        result.Warnings,
		Suggestions:     suggestions,
		Issues:          result.Issues,
		SchemaVersion:   result.SchemaVersion,
		DebugConfigPath: result.DebugConfigPath,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}
	return string(data) + "\n", nil
}
