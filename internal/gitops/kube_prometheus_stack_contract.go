package gitops

import (
	"fmt"
	"strings"

	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const kubePrometheusStackDefaultReleaseName = configservices.DefaultPrometheusStackReleaseName

// kubePrometheusStackRenderContract is the single resolved identity contract
// used by every generated artifact for kube-prometheus-stack. In particular,
// route backends must not independently recreate the names produced by Helm.
type kubePrometheusStackRenderContract struct {
	ReleaseName             string
	Namespace               string
	PrometheusServiceName   string
	AlertmanagerServiceName string
	GrafanaServiceName      string
}

// resolveKubePrometheusStackContract resolves the Helm release identity and
// component Service names once. ReleaseName is intentionally defaulted here as
// well as in config defaults so older in-memory configs remain compatible.
func resolveKubePrometheusStackContract(cfg v2.Config) (kubePrometheusStackRenderContract, error) {
	service, ok := cfg.OpenCenter.Services["kube-prometheus-stack"].(*configservices.PrometheusStackConfig)
	if !ok || service == nil {
		return kubePrometheusStackRenderContract{}, fmt.Errorf("kube-prometheus-stack service configuration is missing")
	}

	releaseName := strings.TrimSpace(service.ReleaseName)
	if releaseName == "" {
		releaseName = kubePrometheusStackDefaultReleaseName
	}
	namespace := strings.TrimSpace(service.Namespace)
	if namespace == "" {
		namespace = "observability"
	}

	return kubePrometheusStackRenderContract{
		ReleaseName:             releaseName,
		Namespace:               namespace,
		PrometheusServiceName:   releaseName + "-prometheus",
		AlertmanagerServiceName: releaseName + "-alertmanager",
		GrafanaServiceName:      releaseName + "-grafana",
	}, nil
}

// kubePrometheusStackBaseKustomizationPatchesRenderer emits the narrow inline
// patch on the Flux base Kustomization. The patch must be applied while Flux
// builds the upstream base, before the separately reconciled service overlay.
func kubePrometheusStackBaseKustomizationPatchesRenderer(cfg v2.Config) (string, error) {
	contract, err := resolveKubePrometheusStackContract(cfg)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf(`  patches:
    - patch: |-
        apiVersion: helm.toolkit.fluxcd.io/v2
        kind: HelmRelease
        metadata:
          name: kube-prometheus-stack
        spec:
          releaseName: %s
          targetNamespace: %s
          chart:
            spec:
              sourceRef:
                namespace: %s
      target:
        group: helm.toolkit.fluxcd.io
        version: v2
        kind: HelmRelease
        name: kube-prometheus-stack
`, contract.ReleaseName, contract.Namespace, contract.Namespace), nil
}
