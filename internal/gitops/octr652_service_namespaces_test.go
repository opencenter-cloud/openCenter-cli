package gitops

import (
	"path/filepath"
	"testing"
)

func TestOCTR652ServiceNamespacesBeforeOverrides(t *testing.T) {
	tests := []struct {
		service               string
		namespaceStage        string
		namespaceDependencies []string
		skipSourcesDependency bool
	}{
		{service: "openstack-ccm", namespaceStage: "openstack-ccm-namespace", skipSourcesDependency: true},
		{service: "openstack-csi", namespaceStage: "openstack-csi-namespace", namespaceDependencies: []string{"sources"}},
		{service: "velero", namespaceStage: "velero-namespace", namespaceDependencies: []string{"sources"}},
	}

	for _, tc := range tests {
		t.Run(tc.service, func(t *testing.T) {
			dst := t.TempDir()
			cfg := newDefault("octr652-" + tc.service)
			cfg.OpenCenter.GitOps.Repository.LocalDir = dst
			for name, service := range cfg.OpenCenter.Services {
				base := extractBaseConfig(service)
				if base != nil {
					base.Enabled = name == tc.service
				}
			}

			if err := RenderClusterApps(cfg); err != nil {
				t.Fatalf("RenderClusterApps() error = %v", err)
			}

			fluxDir := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd")
			aggregate := mustReadFile(t, filepath.Join(fluxDir, "kustomization.yaml"))
			for _, resource := range []string{"./" + tc.namespaceStage + ".yaml", "./" + tc.service + ".yaml"} {
				if !containsLine(aggregate, resource) {
					t.Errorf("services/fluxcd/kustomization.yaml missing %q:\n%s", resource, aggregate)
				}
			}

			namespaceStagePath := filepath.Join(fluxDir, tc.namespaceStage+".yaml")
			namespaceStageDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, namespaceStagePath)))
			if err != nil {
				t.Fatalf("parse %s: %v", namespaceStagePath, err)
			}
			namespaceStageDoc := findFluxKustomization(t, namespaceStageDocs, tc.namespaceStage)
			if len(tc.namespaceDependencies) == 0 {
				if _, ok := nestedValue(namespaceStageDoc, "spec", "dependsOn").([]any); ok {
					t.Errorf("%s must not wait for the aggregate sources gate: %#v", tc.namespaceStage, namespaceStageDoc)
				}
			} else {
				assertFluxDependencies(t, namespaceStageDoc, tc.namespaceStage, tc.namespaceDependencies...)
			}
			wantNamespacePath := "./applications/overlays/" + cfg.ClusterName() + "/services/" + tc.service + "/namespace"
			if got := nestedString(namespaceStageDoc, "spec", "path"); got != wantNamespacePath {
				t.Errorf("%s spec.path = %q, want local namespace path %q", tc.namespaceStage, got, wantNamespacePath)
			}

			namespaceKustomizationPath := filepath.Join(dst, "applications", "overlays", cfg.ClusterName(), "services", tc.service, "namespace", "kustomization.yaml")
			namespaceKustomization := mustReadFile(t, namespaceKustomizationPath)
			if !containsLine(namespaceKustomization, "namespace.yaml") {
				t.Errorf("%s must include only its namespace resource:\n%s", namespaceKustomizationPath, namespaceKustomization)
			}
			namespaceDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, filepath.Join(filepath.Dir(namespaceKustomizationPath), "namespace.yaml"))))
			if err != nil {
				t.Fatalf("parse namespace resource: %v", err)
			}
			if len(namespaceDocs) != 1 || namespaceDocs[0]["kind"] != "Namespace" {
				t.Errorf("namespace stage must render exactly one Namespace, got %#v", namespaceDocs)
			}

			serviceFluxPath := filepath.Join(fluxDir, tc.service+".yaml")
			serviceDocs, err := decodeYAMLDocuments([]byte(mustReadFile(t, serviceFluxPath)))
			if err != nil {
				t.Fatalf("parse %s: %v", serviceFluxPath, err)
			}
			base := findFluxKustomization(t, serviceDocs, tc.service+"-base")
			override := findFluxKustomization(t, serviceDocs, tc.service+"-override")
			overrideDependencies := []string{tc.namespaceStage}
			baseDependencies := []string{tc.service + "-override"}
			if !tc.skipSourcesDependency {
				overrideDependencies = append([]string{"sources"}, overrideDependencies...)
				baseDependencies = append([]string{"sources"}, baseDependencies...)
			}
			assertFluxDependencies(t, override, tc.service+"-override", overrideDependencies...)
			assertFluxDependencies(t, base, tc.service+"-base", baseDependencies...)

			if hasFluxDependency(t, override, tc.service+"-base") || hasFluxDependency(t, base, tc.namespaceStage) {
				t.Fatalf("service topology contains an early-base dependency or cycle: base=%#v override=%#v", base, override)
			}
		})
	}
}

func hasFluxDependency(t *testing.T, doc map[string]any, wanted string) bool {
	t.Helper()
	dependencies, ok := nestedValue(doc, "spec", "dependsOn").([]any)
	if !ok {
		t.Fatalf("Flux Kustomization has no spec.dependsOn list: %#v", doc)
	}
	for _, raw := range dependencies {
		dependency, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("Flux Kustomization has malformed dependency: %#v", raw)
		}
		if name, _ := dependency["name"].(string); name == wanted {
			return true
		}
	}
	return false
}
