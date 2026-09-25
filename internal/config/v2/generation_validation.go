package v2

import (
	"fmt"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
)

// GenerationValidationReport contains deterministic offline findings that must
// be resolved before generation can safely mutate the target GitOps tree.
type GenerationValidationReport struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues,omitempty"`
}

// ValidateGeneration combines the native v2 validation layers with generation-
// specific offline readiness checks. It performs no cloud, Kubernetes, or Git
// network calls.
func ValidateGeneration(cfg *Config) GenerationValidationReport {
	report := GenerationValidationReport{Valid: true}
	if cfg == nil {
		return generationReportFromReadiness(ValidateReadiness(nil))
	}

	if err := NewValidator().Validate(cfg); err != nil {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Category: CategorySchema,
			Message:  err.Error(),
		})
	}

	serviceMap := make(map[string]any, len(cfg.OpenCenter.ManagedServices)+len(cfg.OpenCenter.Services))
	for name, service := range cfg.OpenCenter.ManagedServices {
		serviceMap[name] = service
	}
	for name, service := range cfg.OpenCenter.Services {
		serviceMap[name] = service
	}

	dependencyValidator := services.NewDependencyValidator()
	for _, message := range dependencyValidator.ValidateDependencies(serviceMap) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Category: CategoryServices,
			Message:  message,
		})
	}
	for _, message := range dependencyValidator.ValidateHeadlampOIDC(serviceMap) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Category: CategoryServices,
			Message:  message,
		})
	}

	readiness := ValidateReadiness(cfg)
	for _, issue := range readiness.Issues {
		report.addIssue(issue)
	}
	return report
}

// ValidateForGeneration returns a compact error for callers that only need a
// blocking gate. Use ValidateGeneration when structured findings are needed.
func ValidateForGeneration(cfg *Config) error {
	report := ValidateGeneration(cfg)
	if report.Valid {
		return nil
	}

	problems := make([]string, 0, len(report.Issues))
	for _, issue := range report.Issues {
		if issue.Severity != SeverityError {
			continue
		}
		message := issue.Message
		if issue.Path != "" {
			message = issue.Path + ": " + message
		}
		problems = append(problems, message)
	}
	return fmt.Errorf("offline generation validation failed with %d error(s):\n  - %s", len(problems), strings.Join(problems, "\n  - "))
}

func generationReportFromReadiness(readiness ReadinessReport) GenerationValidationReport {
	return GenerationValidationReport(readiness)
}

func (r *GenerationValidationReport) addIssue(issue ValidationIssue) {
	for _, existing := range r.Issues {
		if existing.Severity == issue.Severity && existing.Category == issue.Category && existing.Path == issue.Path && existing.Message == issue.Message {
			return
		}
	}
	if issue.Severity == SeverityError {
		r.Valid = false
	}
	r.Issues = append(r.Issues, issue)
}
