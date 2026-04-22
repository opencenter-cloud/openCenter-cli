package importer

import (
	"time"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type ConfidenceLevel string

const (
	ConfidenceHigh     ConfidenceLevel = "high"
	ConfidenceMedium   ConfidenceLevel = "medium"
	ConfidenceLow      ConfidenceLevel = "low"
	ConfidenceConflict ConfidenceLevel = "conflict"
)

type FieldOrigin string

const (
	FieldOriginGitOps  FieldOrigin = "gitops"
	FieldOriginLive    FieldOrigin = "live"
	FieldOriginDefault FieldOrigin = "default"
	FieldOriginManual  FieldOrigin = "manual"
)

type EvidenceRef struct {
	Source string `json:"source,omitempty"`
	Path   string `json:"path,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type FieldInferenceResult struct {
	Path       string          `json:"path"`
	Value      any             `json:"value,omitempty"`
	Confidence ConfidenceLevel `json:"confidence"`
	Origin     FieldOrigin     `json:"origin"`
	Evidence   []EvidenceRef   `json:"evidence,omitempty"`
}

type FieldConflict struct {
	Path           string        `json:"path"`
	GitOpsValue    any           `json:"gitops_value,omitempty"`
	LiveValue      any           `json:"live_value,omitempty"`
	Recommended    string        `json:"recommended,omitempty"`
	Evidence       []EvidenceRef `json:"evidence,omitempty"`
	ProtectedField bool          `json:"protected_field,omitempty"`
}

type SkippedField struct {
	Path       string          `json:"path"`
	Reason     string          `json:"reason"`
	Confidence ConfidenceLevel `json:"confidence,omitempty"`
}

type ServiceInferenceResult struct {
	ServiceName  string                       `json:"service_name"`
	Namespaces   []string                     `json:"namespaces,omitempty"`
	Enabled      *bool                        `json:"enabled,omitempty"`
	AdoptionMode *configservices.AdoptionMode `json:"adoption_mode,omitempty"`
	Fields       []FieldInferenceResult       `json:"fields,omitempty"`
	Conflicts    []FieldConflict              `json:"conflicts,omitempty"`
	Skipped      []SkippedField               `json:"skipped,omitempty"`
}

type ClusterSources struct {
	RepoPath         string   `json:"repo_path"`
	ClusterName      string   `json:"cluster_name"`
	ClusterDir       string   `json:"cluster_dir,omitempty"`
	OverlayDir       string   `json:"overlay_dir,omitempty"`
	LegacyConfigPath string   `json:"legacy_config_path,omitempty"`
	ReadmePath       string   `json:"readme_path,omitempty"`
	KubeconfigPaths  []string `json:"kubeconfig_paths,omitempty"`
}

type ClusterImportResult struct {
	ClusterName    string                   `json:"cluster_name"`
	Organization   string                   `json:"organization,omitempty"`
	Sources        ClusterSources           `json:"sources"`
	ProposedConfig *v2.Config               `json:"proposed_config,omitempty"`
	FieldResults   []FieldInferenceResult   `json:"field_results,omitempty"`
	ServiceResults []ServiceInferenceResult `json:"service_results,omitempty"`
	Conflicts      []FieldConflict          `json:"conflicts,omitempty"`
	SkippedFields  []SkippedField           `json:"skipped_fields,omitempty"`
	ExistingConfig string                   `json:"existing_config,omitempty"`
	ProposedPatch  string                   `json:"proposed_patch,omitempty"`
	Warnings       []string                 `json:"warnings,omitempty"`
	Errors         []string                 `json:"errors,omitempty"`
}

type ImportSummary struct {
	ClustersDiscovered int `json:"clusters_discovered"`
	ClustersWithErrors int `json:"clusters_with_errors,omitempty"`
	ConflictCount      int `json:"conflict_count,omitempty"`
	SkippedFieldCount  int `json:"skipped_field_count,omitempty"`
}

type ImportScanResult struct {
	RepoPath  string                `json:"repo_path"`
	ScannedAt time.Time             `json:"scanned_at"`
	Clusters  []ClusterImportResult `json:"clusters"`
	Summary   ImportSummary         `json:"summary"`
	Warnings  []string              `json:"warnings,omitempty"`
}

type SavedArtifact struct {
	RepoHash string            `json:"repo_hash"`
	Path     string            `json:"path"`
	SavedAt  time.Time         `json:"saved_at"`
	Result   *ImportScanResult `json:"result,omitempty"`
}
