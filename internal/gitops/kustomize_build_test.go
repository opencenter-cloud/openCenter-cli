package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

func TestRenderMetalLBOverlayBuildsWithKustomize(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("kustomize-metallb")
	useMetalLBTestOwnership(&cfg)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{
		BaseConfig: services.BaseConfig{Enabled: true, Namespace: "metallb-system"},
		IPAddressPools: []services.IPAddressPool{{
			Name:      "public-pool",
			Addresses: []string{"10.0.0.1/32"},
		}},
		L2Advertisements: []services.L2Advertisement{{
			Name:           "public-pool-l2",
			IPAddressPools: []string{"public-pool"},
		}},
	}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}

	serviceRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
	output := runKustomizeBuild(t, serviceRoot)
	if !strings.Contains(output, "kind: IPAddressPool") || !strings.Contains(output, "kind: L2Advertisement") {
		t.Fatalf("kustomize output missing MetalLB resources:\n%s", output)
	}
	t.Logf("asserted kustomize output snippet:\n%s", kustomizeKinds(output, "IPAddressPool", "L2Advertisement"))
}

func TestServiceOverlayWithoutCustomResourcesBuilds(t *testing.T) {
	repo := t.TempDir()
	cfg := newDefault("kustomize-empty-custom")
	useMetalLBTestOwnership(&cfg)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	cfg.OpenCenter.Services = cloneServiceMap(cfg.OpenCenter.Services)
	cfg.OpenCenter.Services["metallb"] = &services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: true}}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render: %v", err)
	}

	serviceRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName(), "services", "metallb")
	kustomization, err := os.ReadFile(filepath.Join(serviceRoot, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("read service kustomization: %v", err)
	}
	if !strings.Contains(string(kustomization), "custom/") {
		t.Fatalf("service kustomization does not reference custom layer:\n%s", kustomization)
	}
	if _, err := os.Stat(filepath.Join(serviceRoot, "custom", "kustomization.yaml")); err != nil {
		t.Fatalf("custom layer seed missing: %v", err)
	}
	output := runKustomizeBuild(t, serviceRoot)
	t.Logf("kustomize output for service without custom resources:\n%s", output)
}

func TestGeneratedDefaultOverlayKustomizeFailureMatrix(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		if _, err := exec.LookPath("kubectl"); err != nil {
			t.Skip("kustomize build skipped: neither kustomize nor kubectl is installed")
		}
	}

	repo := t.TempDir()
	cfg := newDefault("kustomize-default-matrix")
	cfg.OpenCenter.GitOps.Repository.LocalDir = repo
	// Secret manifests are materialized by secrets sync; the default overlay
	// smoke test uses the empty optional Grafana block to avoid a dangling ref.
	cfg.Secrets.Grafana = v2.GrafanaSecrets{}
	cfg.Secrets.CertManager = v2.CertManagerSecrets{}
	cfg.Secrets.Loki = v2.LokiSecrets{}
	cfg.Secrets.Keycloak = v2.KeycloakSecrets{}
	cfg.Secrets.Headlamp = v2.HeadlampSecrets{}
	cfg.Secrets.WeaveGitOps = v2.WeaveGitOpsSecrets{}
	cfg.Secrets.Tempo = v2.TempoSecrets{}
	cfg.Secrets.AlertProxy = v2.AlertProxySecrets{}
	cfg.Secrets.VSphereCsi = v2.VSphereCsiSecrets{}
	if err := RenderClusterApps(cfg); err != nil {
		t.Fatalf("render default overlay: %v", err)
	}
	// OLM's base is materialized by Flux's GitRepository include rather than
	// generated locally. Seed the v0.46-shaped source artifact so this matrix
	// validates the composed overlay instead of failing on an absent include.
	seedOCTR631OLMBaseFixture(t, repo, cfg.ClusterName())

	// The root overlay kustomization references ./flux-system, which is written
	// by `flux bootstrap` at deploy time (not by generate). Seed a stub so the
	// root overlay builds locally, mirroring the post-bootstrap layout.
	seedFluxSystemFixture(t, repo, cfg.ClusterName())

	overlayRoot := filepath.Join(repo, "applications", "overlays", cfg.ClusterName())
	var roots []string
	err := filepath.WalkDir(overlayRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && entry.Name() == "kustomization.yaml" {
			roots = append(roots, filepath.Dir(path))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("discover generated kustomizations: %v", err)
	}

	failures := make(map[string]string)
	for _, root := range roots {
		relative, err := filepath.Rel(overlayRoot, root)
		if err != nil {
			t.Fatalf("relative kustomization directory %s: %v", root, err)
		}
		output, buildErr := runKustomizeBuildResult(root)
		if buildErr != nil {
			failures[filepath.ToSlash(relative)] = fmt.Sprintf("%v\n%s", buildErr, output)
		}
	}

	for relative := range failures {
		t.Errorf("generated-overlay kustomize failure in %s:\n%s", relative, failures[relative])
	}
}

// seedFluxSystemFixture writes a minimal flux-system/ overlay directory as
// `flux bootstrap` would, so the root overlay (which references ./flux-system)
// can be built locally without a live bootstrap.
func seedFluxSystemFixture(t *testing.T, repo, clusterName string) {
	t.Helper()
	fluxRoot := filepath.Join(repo, "applications", "overlays", clusterName, "flux-system")
	if err := os.MkdirAll(fluxRoot, 0o755); err != nil {
		t.Fatalf("seed flux-system dir: %v", err)
	}
	kustomization := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - gotk-components.yaml\n  - gotk-sync.yaml\n"
	if err := os.WriteFile(filepath.Join(fluxRoot, "kustomization.yaml"), []byte(kustomization), 0o644); err != nil {
		t.Fatalf("seed flux-system kustomization: %v", err)
	}
	// A tiny valid manifest each; content is irrelevant to buildability.
	components := "apiVersion: v1\nkind: Namespace\nmetadata:\n  name: flux-system\n"
	if err := os.WriteFile(filepath.Join(fluxRoot, "gotk-components.yaml"), []byte(components), 0o644); err != nil {
		t.Fatalf("seed gotk-components: %v", err)
	}
	sync := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: flux-system-sync\n  namespace: flux-system\n"
	if err := os.WriteFile(filepath.Join(fluxRoot, "gotk-sync.yaml"), []byte(sync), 0o644); err != nil {
		t.Fatalf("seed gotk-sync: %v", err)
	}
}

func runKustomizeBuild(t *testing.T, root string) string {
	t.Helper()
	output, err := runKustomizeBuildResult(root)
	if err != nil {
		if _, lookErr := exec.LookPath("kustomize"); lookErr != nil {
			if _, lookErr := exec.LookPath("kubectl"); lookErr != nil {
				t.Skip("kustomize build skipped: neither kustomize nor kubectl is installed")
			}
		}
		t.Fatalf("kustomize build %s: %v\n%s", root, err, output)
	}
	return output
}

func runKustomizeBuildResult(root string) (string, error) {
	var cmd *exec.Cmd
	if _, err := exec.LookPath("kustomize"); err == nil {
		cmd = exec.Command("kustomize", "build", root)
	} else if _, err := exec.LookPath("kubectl"); err == nil {
		cmd = exec.Command("kubectl", "kustomize", root)
	} else {
		return "", fmt.Errorf("neither kustomize nor kubectl is installed")
	}
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func kustomizeKinds(output string, kinds ...string) string {
	lines := strings.Split(output, "\n")
	var snippet []string
	for i, line := range lines {
		for _, kind := range kinds {
			if line == "kind: "+kind {
				start := i - 1
				if start < 0 {
					start = 0
				}
				end := i + 1
				if end >= len(lines) {
					end = len(lines) - 1
				}
				snippet = append(snippet, strings.Join(lines[start:end+1], "\n"))
			}
		}
	}
	return strings.Join(snippet, "\n---\n")
}
