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

	// Lifecycle contract v2: the CLI deploy path always renders the
	// infrastructure in cli lifecycle mode, which requires the kubespray
	// module to support inventory_output_dir. A config whose module slots pin
	// a pre-contract-2 gitops-base ref renders the inventory inside the
	// GitOps worktree, and `cluster deploy` then fails in kubespray-prepare
	// with "inventory rendered outside the cluster state zone". Block
	// generation with the exact repair command instead.
	if RequiresKubesprayModuleContract2Upgrade(cfg) {
		report.addIssue(ValidationIssue{
			Severity: SeverityError,
			Category: CategorySchema,
			Path:     "deployment.kubespray",
			Message: fmt.Sprintf(
				"Kubespray module source pins a ref older than lifecycle contract 2 (inventory_output_dir); run: opencenter cluster set %s deployment.kubespray.kubespray_cluster.source=%q deployment.kubespray.modules.kubespray.source=%q",
				cfg.ClusterName(), DefaultKubesprayModuleSourceURL, DefaultKubesprayModuleSourceURL),
		})
	}

	// Contract v3 is advisory, never blocking: a v2 module deploys and runs
	// fine, it just exposes no opencenter_network_id/opencenter_subnet_id, so
	// `cluster observe` cannot promote a tofu-created network and the CCM
	// [LoadBalancer] subnet-id stays blank on a tofu-managed network with
	// use_octavia=false. Warn with the repair command rather than refusing to
	// generate for every cluster that predates the v3 module.
	if ReportsKubesprayModuleContract3Gap(cfg) {
		report.addIssue(ValidationIssue{
			Severity: SeverityWarning,
			Category: CategorySchema,
			Path:     "deployment.kubespray",
			Message: fmt.Sprintf(
				"Kubespray module source predates lifecycle contract 3 (network/subnet outputs); `cluster observe` cannot recover a network OpenTofu created and the CCM [LoadBalancer] subnet-id stays blank when use_octavia=false. Repair: opencenter cluster set %s deployment.kubespray.kubespray_cluster.source=%q deployment.kubespray.modules.kubespray.source=%q",
				cfg.ClusterName(), DefaultKubesprayModuleSourceURLContract3, DefaultKubesprayModuleSourceURLContract3),
		})
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
