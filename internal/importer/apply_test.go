package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSelectApprovedFieldsSkipsProtectedLowConfidenceAndConflicts(t *testing.T) {
	cluster := ClusterImportResult{
		FieldResults: []FieldInferenceResult{
			{
				Path:       "opencenter.meta.region",
				Value:      "ord1",
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginGitOps,
			},
			{
				Path:       "opencenter.secrets.keycloak.client_secret",
				Value:      "top-secret",
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginGitOps,
			},
			{
				Path:       "opencenter.cluster.kubernetes.version",
				Value:      "1.32.8",
				Confidence: ConfidenceMedium,
				Origin:     FieldOriginLive,
			},
		},
		Conflicts: []FieldConflict{
			{Path: "opencenter.infrastructure.compute.worker_count"},
		},
		ServiceResults: []ServiceInferenceResult{
			{
				ServiceName: "velero",
				Fields: []FieldInferenceResult{
					{
						Path:       "opencenter.services.velero.enabled",
						Value:      true,
						Confidence: ConfidenceHigh,
						Origin:     FieldOriginGitOps,
					},
				},
			},
			{
				ServiceName: "keycloak",
				Fields: []FieldInferenceResult{
					{
						Path:       "opencenter.services.keycloak.hostname",
						Value:      "auth.example.com",
						Confidence: ConfidenceLow,
						Origin:     FieldOriginLive,
					},
				},
			},
		},
	}

	approved, skipped := SelectApprovedFields(cluster)

	if len(approved) != 2 {
		t.Fatalf("expected 2 approved fields, got %d", len(approved))
	}
	if approved[0].Path != "opencenter.meta.region" {
		t.Fatalf("unexpected first approved field %q", approved[0].Path)
	}
	if approved[1].Path != "opencenter.services.velero.enabled" {
		t.Fatalf("unexpected second approved field %q", approved[1].Path)
	}

	reasons := make(map[string]string, len(skipped))
	for _, skippedField := range skipped {
		reasons[skippedField.Path] = skippedField.Reason
	}

	if reasons["opencenter.secrets.keycloak.client_secret"] != "protected field" {
		t.Fatalf("expected protected field skip, got %#v", reasons)
	}
	if reasons["opencenter.cluster.kubernetes.version"] != "requires high confidence" {
		t.Fatalf("expected medium-confidence skip, got %#v", reasons)
	}
	if reasons["opencenter.services.keycloak.hostname"] != "requires high confidence" {
		t.Fatalf("expected low-confidence service skip, got %#v", reasons)
	}
}

func TestPatchYAMLFileAppliesApprovedFieldsAndPreservesUnrelatedContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cluster.yaml")
	original := `schema_version: "2.0"
opencenter:
  meta:
    organization: example-inc
    region: iad3
  infrastructure:
    compute:
      worker_count: 2
  services:
    velero:
      enabled: false
      namespace: velero
custom_note: keep-me
`
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatalf("write original yaml: %v", err)
	}

	patched, err := PatchYAMLFile(path, []FieldInferenceResult{
		{
			Path:       "opencenter.meta.region",
			Value:      "ord1",
			Confidence: ConfidenceHigh,
			Origin:     FieldOriginGitOps,
		},
		{
			Path:       "opencenter.infrastructure.compute.worker_count",
			Value:      3,
			Confidence: ConfidenceHigh,
			Origin:     FieldOriginLive,
		},
		{
			Path:       "opencenter.services.velero.enabled",
			Value:      true,
			Confidence: ConfidenceHigh,
			Origin:     FieldOriginGitOps,
		},
	})
	if err != nil {
		t.Fatalf("PatchYAMLFile() error = %v", err)
	}

	updatedBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read patched yaml: %v", err)
	}
	updated := string(updatedBytes)

	if !strings.Contains(updated, "region: ord1") {
		t.Fatalf("expected patched region in yaml, got:\n%s", updated)
	}
	if !strings.Contains(updated, "worker_count: 3") {
		t.Fatalf("expected patched worker count in yaml, got:\n%s", updated)
	}
	if !strings.Contains(updated, "enabled: true") {
		t.Fatalf("expected velero enabled in yaml, got:\n%s", updated)
	}
	if !strings.Contains(updated, "custom_note: keep-me") {
		t.Fatalf("expected unrelated content to be preserved, got:\n%s", updated)
	}
	if !strings.Contains(patched.Diff, "-    region: iad3") || !strings.Contains(patched.Diff, "+    region: ord1") {
		t.Fatalf("expected diff to include region change, got:\n%s", patched.Diff)
	}
}
