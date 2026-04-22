package importer

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	configregistry "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

type LiveSnapshot struct {
	KubernetesVersion string
	MasterCount       int
	WorkerCount       int
	Namespaces        map[string]struct{}
}

type nodeListResponse struct {
	Items []struct {
		Metadata struct {
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Status struct {
			NodeInfo struct {
				KubeletVersion string `json:"kubeletVersion"`
			} `json:"nodeInfo"`
		} `json:"status"`
	} `json:"items"`
}

type namespaceListResponse struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	} `json:"items"`
}

func CollectLiveSnapshot(ctx context.Context, kubeconfigPath string) (*LiveSnapshot, error) {
	nodesJSON, err := runKubectlJSON(ctx, kubeconfigPath, "get", "nodes", "-o", "json")
	if err != nil {
		return nil, err
	}

	var nodes nodeListResponse
	if err := json.Unmarshal(nodesJSON, &nodes); err != nil {
		return nil, fmt.Errorf("decode kubectl nodes response: %w", err)
	}

	snapshot := &LiveSnapshot{
		Namespaces: make(map[string]struct{}),
	}

	for _, node := range nodes.Items {
		if snapshot.KubernetesVersion == "" {
			snapshot.KubernetesVersion = strings.TrimPrefix(strings.TrimSpace(node.Status.NodeInfo.KubeletVersion), "v")
		}
		if isControlPlaneNode(node.Metadata.Labels) {
			snapshot.MasterCount++
		} else {
			snapshot.WorkerCount++
		}
	}

	namespacesJSON, err := runKubectlJSON(ctx, kubeconfigPath, "get", "namespaces", "-o", "json")
	if err == nil {
		var namespaces namespaceListResponse
		if decodeErr := json.Unmarshal(namespacesJSON, &namespaces); decodeErr == nil {
			for _, namespace := range namespaces.Items {
				name := strings.TrimSpace(namespace.Metadata.Name)
				if name != "" {
					snapshot.Namespaces[name] = struct{}{}
				}
			}
		}
	}

	return snapshot, nil
}

func BuildLiveImportResult(ctx context.Context, cfg *v2.Config, kubeconfigPath string) (ClusterImportResult, error) {
	if cfg == nil {
		return ClusterImportResult{}, fmt.Errorf("configuration cannot be nil")
	}

	snapshot, err := CollectLiveSnapshot(ctx, kubeconfigPath)
	if err != nil {
		return ClusterImportResult{}, err
	}

	result := ClusterImportResult{
		ClusterName:    cfg.ClusterName(),
		Organization:   cfg.OpenCenter.Meta.Organization,
		ProposedConfig: cfg,
		Sources: ClusterSources{
			KubeconfigPaths: []string{kubeconfigPath},
		},
	}

	appendField := func(path string, value any, evidenceDetail string) {
		result.FieldResults = append(result.FieldResults, FieldInferenceResult{
			Path:       path,
			Value:      value,
			Confidence: ConfidenceHigh,
			Origin:     FieldOriginLive,
			Evidence: []EvidenceRef{
				{Source: "kubectl", Path: kubeconfigPath, Detail: evidenceDetail},
			},
		})
	}

	if snapshot.KubernetesVersion != "" && snapshot.KubernetesVersion != strings.TrimSpace(cfg.OpenCenter.Cluster.Kubernetes.Version) {
		appendField("opencenter.cluster.kubernetes.version", snapshot.KubernetesVersion, "nodes")
	}
	if snapshot.MasterCount > 0 && snapshot.MasterCount != cfg.OpenCenter.Infrastructure.Compute.MasterCount {
		appendField("opencenter.infrastructure.compute.master_count", snapshot.MasterCount, "nodes")
	}
	if snapshot.WorkerCount > 0 && snapshot.WorkerCount != cfg.OpenCenter.Infrastructure.Compute.WorkerCount {
		appendField("opencenter.infrastructure.compute.worker_count", snapshot.WorkerCount, "nodes")
	}

	registry := NewNamespaceRegistry()
	for _, serviceName := range configregistry.GetRegisteredServices() {
		namespaces := registry.NamespacesFor(serviceName)
		if !snapshotHasAnyNamespace(snapshot, namespaces) {
			continue
		}

		service, path, ok := serviceConfigTarget(cfg, serviceName)
		if !ok {
			continue
		}

		serviceResult := ServiceInferenceResult{
			ServiceName: serviceName,
			Namespaces:  namespaces,
		}

		if !serviceEnabledValue(service) {
			enabled := true
			serviceResult.Enabled = &enabled
			serviceResult.Fields = append(serviceResult.Fields, FieldInferenceResult{
				Path:       path + ".enabled",
				Value:      true,
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginLive,
				Evidence: []EvidenceRef{
					{Source: "kubectl", Path: kubeconfigPath, Detail: "namespace detected"},
				},
			})
		}

		if base := baseConfigPointer(service); base != nil && strings.TrimSpace(base.Namespace) == "" && len(namespaces) > 0 {
			serviceResult.Fields = append(serviceResult.Fields, FieldInferenceResult{
				Path:       path + ".namespace",
				Value:      namespaces[0],
				Confidence: ConfidenceHigh,
				Origin:     FieldOriginDefault,
				Evidence: []EvidenceRef{
					{Source: "namespace-registry", Detail: serviceName},
				},
			})
		}

		if len(serviceResult.Fields) > 0 {
			result.ServiceResults = append(result.ServiceResults, serviceResult)
		}
	}

	return result, nil
}

func runKubectlJSON(ctx context.Context, kubeconfigPath string, args ...string) ([]byte, error) {
	baseArgs := make([]string, 0, len(args)+2)
	if kubeconfigPath != "" {
		baseArgs = append(baseArgs, "--kubeconfig", kubeconfigPath)
	}
	baseArgs = append(baseArgs, args...)

	cmd := exec.CommandContext(ctx, "kubectl", baseArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("kubectl %s failed: %w: %s", strings.Join(baseArgs, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func isControlPlaneNode(labels map[string]string) bool {
	if len(labels) == 0 {
		return false
	}
	if _, ok := labels["node-role.kubernetes.io/control-plane"]; ok {
		return true
	}
	if _, ok := labels["node-role.kubernetes.io/master"]; ok {
		return true
	}
	return false
}

func snapshotHasAnyNamespace(snapshot *LiveSnapshot, namespaces []string) bool {
	if snapshot == nil || len(snapshot.Namespaces) == 0 {
		return false
	}
	for _, namespace := range namespaces {
		if _, ok := snapshot.Namespaces[namespace]; ok {
			return true
		}
	}
	return false
}
