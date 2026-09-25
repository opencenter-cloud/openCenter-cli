package gitops

import (
	"path/filepath"
	"testing"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

func TestKubePrometheusStackOverlayRendersRoutesWithPinnedChartBackends(t *testing.T) {
	cfg := newDefault("monitoring-routes")
	stack := cfg.OpenCenter.Services["kube-prometheus-stack"].(*configservices.PrometheusStackConfig)
	stack.PrometheusHostname = "prometheus.example.test"
	stack.AlertmanagerHostname = "alerts.example.test"
	stack.GrafanaHostname = "grafana.example.test"

	files, err := kubePrometheusStackOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	for filename, expected := range map[string][]string{
		"prometheus-http-route.yaml":   {"name: prometheus-gateway-route", `"prometheus.example.test"`, "sectionName: prometheus-https", "name: kube-prometheus-stack-prometheus", "port: 9090"},
		"alertmanager-http-route.yaml": {"name: alertmanager-gateway-route", `"alerts.example.test"`, "sectionName: alertmanager-https", "name: kube-prometheus-stack-alertmanager", "port: 9093"},
		"grafana-http-route.yaml":      {"name: grafana-gateway-route", `"grafana.example.test"`, "sectionName: grafana-https", "name: kube-prometheus-stack-grafana", "port: 80"},
	} {
		content, found := files[filename]
		require.Truef(t, found, "missing generated %s", filename)
		require.Contains(t, content, "namespace: observability")
		require.Contains(t, content, "namespace: rackspace-system")
		for _, want := range expected {
			require.Containsf(t, content, want, "%s must contain %q", filename, want)
		}
	}
}

func TestKubePrometheusStackContractAlignsDefaultValuesPatchAndRoutes(t *testing.T) {
	cfg := newDefault("monitoring-contract-default")

	contract, err := resolveKubePrometheusStackContract(cfg)
	require.NoError(t, err)
	require.Equal(t, kubePrometheusStackDefaultReleaseName, contract.ReleaseName)
	require.Equal(t, "observability", contract.Namespace)
	require.Equal(t, "kube-prometheus-stack-prometheus", contract.PrometheusServiceName)
	require.Equal(t, "kube-prometheus-stack-alertmanager", contract.AlertmanagerServiceName)
	require.Equal(t, "kube-prometheus-stack-grafana", contract.GrafanaServiceName)

	values := renderOverrideValues(t, cfg, "kube-prometheus-stack")
	require.Contains(t, values, "fullnameOverride: "+contract.ReleaseName)
	require.Contains(t, values, "fullnameOverride: "+contract.GrafanaServiceName)

	files, err := kubePrometheusStackOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.Len(t, files, 3)
	require.Contains(t, files["prometheus-http-route.yaml"], "namespace: "+contract.Namespace)
	require.Contains(t, files["prometheus-http-route.yaml"], "name: "+contract.PrometheusServiceName)
	require.Contains(t, files["alertmanager-http-route.yaml"], "name: "+contract.AlertmanagerServiceName)
	require.Contains(t, files["grafana-http-route.yaml"], "name: "+contract.GrafanaServiceName)
	patch, err := kubePrometheusStackBaseKustomizationPatchesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, patch, "releaseName: "+contract.ReleaseName)
	require.Contains(t, patch, "targetNamespace: "+contract.Namespace)
}

func TestKubePrometheusStackContractAlignsCustomValuesPatchAndRoutes(t *testing.T) {
	cfg := newDefault("monitoring-contract-custom")
	stack := cfg.OpenCenter.Services["kube-prometheus-stack"].(*configservices.PrometheusStackConfig)
	stack.ReleaseName = "metrics"
	stack.Namespace = "monitoring"

	contract, err := resolveKubePrometheusStackContract(cfg)
	require.NoError(t, err)
	require.Equal(t, "metrics", contract.ReleaseName)
	require.Equal(t, "monitoring", contract.Namespace)

	values := renderOverrideValues(t, cfg, "kube-prometheus-stack")
	require.Contains(t, values, "fullnameOverride: "+contract.ReleaseName)
	require.Contains(t, values, "fullnameOverride: "+contract.GrafanaServiceName)

	files, err := kubePrometheusStackOverlayFilesRenderer(cfg)
	require.NoError(t, err)
	require.Len(t, files, 3)
	patch, err := kubePrometheusStackBaseKustomizationPatchesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, patch, "releaseName: metrics")
	require.Contains(t, patch, "targetNamespace: monitoring")
	for filename, serviceName := range map[string]string{
		"prometheus-http-route.yaml":   contract.PrometheusServiceName,
		"alertmanager-http-route.yaml": contract.AlertmanagerServiceName,
		"grafana-http-route.yaml":      contract.GrafanaServiceName,
	} {
		require.Contains(t, files[filename], "namespace: monitoring")
		require.Contains(t, files[filename], "name: "+serviceName)
	}
}

func TestKubePrometheusStackGeneratedYAMLUsesContractStructurally(t *testing.T) {
	cfg := newDefault("monitoring-structural")
	stack := cfg.OpenCenter.Services["kube-prometheus-stack"].(*configservices.PrometheusStackConfig)
	stack.ReleaseName = "metrics"
	stack.Namespace = "monitoring"

	contract, err := resolveKubePrometheusStackContract(cfg)
	require.NoError(t, err)
	valuesDocs, err := decodeYAMLDocuments([]byte(renderOverrideValues(t, cfg, "kube-prometheus-stack")))
	require.NoError(t, err)
	require.Len(t, valuesDocs, 1)
	require.Equal(t, contract.ReleaseName, valuesDocs[0]["fullnameOverride"])
	for component := range map[string]struct{}{
		"prometheus":   {},
		"alertmanager": {},
		"grafana":      {},
	} {
		componentValues, ok := valuesDocs[0][component].(map[string]any)
		require.True(t, ok, component)
		require.Equal(t, true, componentValues["enabled"], component)
		serviceValues, ok := componentValues["service"].(map[string]any)
		require.True(t, ok, component)
		require.Equal(t, true, serviceValues["enabled"], component)
	}
	grafanaValues := valuesDocs[0]["grafana"].(map[string]any)
	require.Equal(t, contract.GrafanaServiceName, grafanaValues["fullnameOverride"])
	files, err := kubePrometheusStackOverlayFilesRenderer(cfg)
	require.NoError(t, err)

	for filename, serviceName := range map[string]string{
		"prometheus-http-route.yaml":   contract.PrometheusServiceName,
		"alertmanager-http-route.yaml": contract.AlertmanagerServiceName,
		"grafana-http-route.yaml":      contract.GrafanaServiceName,
	} {
		docs, decodeErr := decodeYAMLDocuments([]byte(files[filename]))
		require.NoError(t, decodeErr, filename)
		require.Len(t, docs, 1, filename)
		metadata, ok := docs[0]["metadata"].(map[string]any)
		require.True(t, ok, filename)
		require.Equal(t, contract.Namespace, metadata["namespace"], filename)

		spec, ok := docs[0]["spec"].(map[string]any)
		require.True(t, ok, filename)
		rules, ok := spec["rules"].([]any)
		require.True(t, ok, filename)
		rule, ok := rules[0].(map[string]any)
		require.True(t, ok, filename)
		backendRefs, ok := rule["backendRefs"].([]any)
		require.True(t, ok, filename)
		backend, ok := backendRefs[0].(map[string]any)
		require.True(t, ok, filename)
		require.Equal(t, serviceName, backend["name"], filename)
	}

	ctx := buildAutoServiceContext("kube-prometheus-stack", &stack.BaseConfig, cfg)
	actions, err := renderAutoServiceActions(ctx, cfg)
	require.NoError(t, err)
	var fluxContent string
	for _, action := range actions {
		if action.Output == "services/fluxcd/kube-prometheus-stack.yaml" {
			fluxContent = action.Content
			break
		}
	}
	require.NotEmpty(t, fluxContent)
	fluxDocs, err := decodeYAMLDocuments([]byte(fluxContent))
	require.NoError(t, err)
	baseKustomization := findFluxKustomization(t, fluxDocs, "kube-prometheus-stack-base")
	baseSpec, ok := baseKustomization["spec"].(map[string]any)
	require.True(t, ok)
	patches, ok := baseSpec["patches"].([]any)
	require.True(t, ok)
	require.Len(t, patches, 1)
	patch, ok := patches[0].(map[string]any)
	require.True(t, ok)
	require.Contains(t, patch["patch"], "releaseName: metrics")
	require.Contains(t, patch["patch"], "targetNamespace: monitoring")
	target, ok := patch["target"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "kube-prometheus-stack", target["name"])

	basePatches, err := kubePrometheusStackBaseKustomizationPatchesRenderer(cfg)
	require.NoError(t, err)
	require.Contains(t, basePatches, "releaseName: metrics")
	require.Contains(t, basePatches, "targetNamespace: monitoring")
	require.Contains(t, basePatches, "namespace: monitoring")
	patchDocs, err := decodeYAMLDocuments([]byte(patch["patch"].(string)))
	require.NoError(t, err)
	require.Len(t, patchDocs, 1)
	patchSpec, ok := patchDocs[0]["spec"].(map[string]any)
	require.True(t, ok)
	chart, ok := patchSpec["chart"].(map[string]any)
	require.True(t, ok)
	chartSpec, ok := chart["spec"].(map[string]any)
	require.True(t, ok)
	sourceRef, ok := chartSpec["sourceRef"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "monitoring", sourceRef["namespace"])
}

func TestKubePrometheusStackRoutesAreIncludedAndWaitForGatewayAPI(t *testing.T) {
	cfg := newDefault("monitoring-routes")
	cfg.OpenCenter.GitOps.Repository.LocalDir = t.TempDir()
	require.NoError(t, RenderClusterApps(cfg))

	base := filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services", "kube-prometheus-stack")
	for _, filename := range []string{"prometheus-http-route.yaml", "alertmanager-http-route.yaml", "grafana-http-route.yaml"} {
		require.FileExists(t, filepath.Join(base, filename))
	}
	kustomization := mustReadFile(t, filepath.Join(base, "kustomization.yaml"))
	for _, filename := range []string{"prometheus-http-route.yaml", "alertmanager-http-route.yaml", "grafana-http-route.yaml"} {
		require.Contains(t, kustomization, "- "+filename)
	}
	require.NotContains(t, kustomization, "patches:")

	flux := mustReadFile(t, filepath.Join(cfg.OpenCenter.GitOps.Repository.LocalDir, "applications", "overlays", cfg.ClusterName(), "services", "fluxcd", "kube-prometheus-stack.yaml"))
	docs, err := decodeYAMLDocuments([]byte(flux))
	require.NoError(t, err)
	override := findFluxKustomization(t, docs, "kube-prometheus-stack-override")
	require.True(t, hasFluxDependency(t, override, "envoy-gateway-api-base"))
	baseKustomization := findFluxKustomization(t, docs, "kube-prometheus-stack-base")
	baseSpec, ok := baseKustomization["spec"].(map[string]any)
	require.True(t, ok)
	patches, ok := baseSpec["patches"].([]any)
	require.True(t, ok)
	require.Len(t, patches, 1)
	patch, ok := patches[0].(map[string]any)
	require.True(t, ok)
	require.Contains(t, patch["patch"], "releaseName: kube-prometheus-stack")
	require.Contains(t, patch["patch"], "targetNamespace: observability")
	require.Contains(t, patch["patch"], "namespace: observability")
	target, ok := patch["target"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, "kube-prometheus-stack", target["name"])
}
