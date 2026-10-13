package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func TestHeadlampOverlayRendersReachableOIDCPortal(t *testing.T) {
	cfg := newDefault("portal-routes")
	headlamp := cfg.OpenCenter.Services["headlamp"].(*configservices.HeadlampConfig)
	headlamp.Hostname = "dashboard.example.test"

	files, err := headlampOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	route := files["headlamp-http-route.yaml"]
	require.Contains(t, route, "name: headlamp-gateway-route")
	require.Contains(t, route, `"dashboard.example.test"`)
	require.Contains(t, route, "sectionName: headlamp-https")
	require.Contains(t, route, "name: headlamp")
	require.Contains(t, route, "port: 80")
	affinity := files["headlamp-session-affinity.yaml"]
	require.Contains(t, affinity, "kind: BackendTrafficPolicy")
	require.Contains(t, affinity, "name: headlamp-gateway-route")
	require.Contains(t, affinity, "type: ConsistentHash")
	require.Contains(t, affinity, "name: opencenter-headlamp-affinity")
	require.Contains(t, files["headlamp-oidc-ca-certificate.yaml"], "secretName: headlamp-oidc-ca")
	require.Contains(t, files["headlamp-oidc-ca-certificate.yaml"], "name: rackspace-ca")
}

func TestWeaveGitOpsOverlayRendersReachablePortal(t *testing.T) {
	cfg := newDefault("portal-routes")

	files, err := weaveGitOpsOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	route := files["weave-gitops-http-route.yaml"]
	require.Contains(t, route, "name: weave-gitops-gateway-route")
	require.Contains(t, route, `"gitops.`+cfg.OpenCenter.Cluster.ClusterFQDN+`"`)
	require.Contains(t, route, "sectionName: gitops-https")
	require.Contains(t, route, "name: weave-gitops")
	require.Contains(t, route, "port: 9001")
}

func TestPortalRoutesAreIncludedInGeneratedOverlays(t *testing.T) {
	cfg := newDefault("portal-routes")
	cfg.OpenCenter.Services["weave-gitops"].(*configservices.DefaultServiceConfig).Enabled = true
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	overlayRoot := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir,
		"applications", "overlays", cfg.ClusterName(), "services")

	headlampKustomization := mustReadFile(t, filepath.Join(overlayRoot, "headlamp", "kustomization.yaml"))
	require.Contains(t, headlampKustomization, "headlamp-http-route.yaml")
	require.Contains(t, headlampKustomization, "headlamp-session-affinity.yaml")
	require.FileExists(t, filepath.Join(overlayRoot, "headlamp", "headlamp-http-route.yaml"))
	require.FileExists(t, filepath.Join(overlayRoot, "headlamp", "headlamp-session-affinity.yaml"))

	weaveKustomization := mustReadFile(t, filepath.Join(overlayRoot, "weave-gitops", "kustomization.yaml"))
	require.Contains(t, weaveKustomization, "weave-gitops-http-route.yaml")
	require.FileExists(t, filepath.Join(overlayRoot, "weave-gitops", "weave-gitops-http-route.yaml"))
}

func TestHeadlampOverrideWaitsForGatewayAPI(t *testing.T) {
	cfg := newDefault("portal-routes")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	flux := mustReadFile(t, filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir,
		"applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "headlamp.yaml"))
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)
	override := findFluxKustomization(t, docs, "headlamp-override")
	require.True(t, hasFluxDependency(t, override, "headlamp-base"))
	require.True(t, hasFluxDependency(t, override, "envoy-gateway-api-base"))
}
