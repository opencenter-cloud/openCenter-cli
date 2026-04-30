// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

const (
	inventorySourceConfig            = "config"
	inventorySourceOpenTofuState     = "opentofu_state"
	inventorySourceKubernetesRefresh = "kubernetes_refresh"

	inventoryNodeRoleController    = "controller"
	inventoryNodeRoleWorker        = "worker"
	inventoryNodeRoleWindowsWorker = "windows-worker"
)

type clusterInventory struct {
	Source    string                  `json:"source" yaml:"source"`
	StatePath string                  `json:"state_path,omitempty" yaml:"state_path,omitempty"`
	Network   clusterInventoryNetwork `json:"network" yaml:"network"`
	Nodes     []clusterInventoryNode  `json:"nodes" yaml:"nodes"`
	Warnings  []string                `json:"warnings" yaml:"warnings"`
}

type clusterInventoryNetwork struct {
	APIEndpoint       string `json:"api_endpoint,omitempty" yaml:"api_endpoint,omitempty"`
	APIVIP            string `json:"api_vip,omitempty" yaml:"api_vip,omitempty"`
	InternalVIP       string `json:"internal_vip,omitempty" yaml:"internal_vip,omitempty"`
	LoadBalancer      string `json:"load_balancer,omitempty" yaml:"load_balancer,omitempty"`
	FloatingIPPool    string `json:"floating_ip_pool,omitempty" yaml:"floating_ip_pool,omitempty"`
	BastionFloatingIP string `json:"bastion_floating_ip,omitempty" yaml:"bastion_floating_ip,omitempty"`
}

type clusterInventoryNode struct {
	Name       string `json:"name" yaml:"name"`
	Role       string `json:"role" yaml:"role"`
	InternalIP string `json:"internal_ip,omitempty" yaml:"internal_ip,omitempty"`
	ExternalIP string `json:"external_ip,omitempty" yaml:"external_ip,omitempty"`
	Source     string `json:"source" yaml:"source"`
	Ready      *bool  `json:"ready,omitempty" yaml:"ready,omitempty"`
}

func buildClusterInventory(ctx context.Context, cfg *v2.Config, infraDir, kubeconfigPath string, refresh bool) clusterInventory {
	inventory := configuredClusterInventory(cfg)
	statePath, stateWarning := openTofuLocalStatePath(cfg, infraDir)
	if statePath != "" {
		inventory.StatePath = statePath
	}
	if stateWarning != "" {
		inventory.Warnings = append(inventory.Warnings, stateWarning)
	}

	if statePath != "" {
		stateInventory, err := readOpenTofuStateInventory(statePath)
		if err != nil {
			inventory.Warnings = append(inventory.Warnings, err.Error())
		} else if stateInventory.hasProvisionedData() {
			mergeInventory(&inventory, stateInventory)
		} else {
			inventory.Warnings = append(inventory.Warnings, "opentofu state does not contain recognized inventory values")
		}
	}

	if refresh {
		refreshed, err := collectKubernetesRefreshInventory(ctx, kubeconfigPath)
		if err != nil {
			inventory.Warnings = append(inventory.Warnings, err.Error())
		} else if refreshed.hasProvisionedData() {
			mergeInventory(&inventory, refreshed)
		}
	}

	return inventory
}

func configuredClusterInventory(cfg *v2.Config) clusterInventory {
	inventory := clusterInventory{Source: inventorySourceConfig}
	if cfg == nil {
		return inventory
	}

	network := cfg.OpenCenter.Infrastructure.Networking
	apiVIP := strings.TrimSpace(cfg.OpenCenter.Infrastructure.K8sAPIIP)
	if apiVIP == "" {
		apiVIP = strings.TrimSpace(network.VRRPIP)
	}

	inventory.Network = clusterInventoryNetwork{
		APIVIP:         apiVIP,
		InternalVIP:    strings.TrimSpace(network.VRRPIP),
		LoadBalancer:   strings.TrimSpace(network.LoadbalancerProvider),
		FloatingIPPool: configuredFloatingIPPool(cfg),
	}
	return inventory
}

func configuredFloatingIPPool(cfg *v2.Config) string {
	if cfg == nil || cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		return ""
	}
	openstack := cfg.OpenCenter.Infrastructure.Cloud.OpenStack
	if openstack.Networking != nil {
		if value := strings.TrimSpace(openstack.Networking.FloatingIPPool); value != "" {
			return value
		}
	}
	return strings.TrimSpace(openstack.FloatingIPPool)
}

func openTofuLocalStatePath(cfg *v2.Config, infraDir string) (string, string) {
	if cfg == nil || !cfg.OpenTofu.Enabled {
		return "", ""
	}

	backendType := strings.ToLower(strings.TrimSpace(cfg.OpenTofu.Backend.Type))
	if backendType != "" && backendType != "local" {
		return "", fmt.Sprintf("opentofu backend %q cannot be inspected offline", backendType)
	}

	statePath := ""
	if cfg.OpenTofu.Backend.Local != nil {
		statePath = strings.TrimSpace(cfg.OpenTofu.Backend.Local.Path)
	}
	if statePath == "" {
		statePath = fmt.Sprintf(".opentofu-local-%s/terraform.tfstate", cfg.ClusterName())
	}
	if !filepath.IsAbs(statePath) {
		statePath = filepath.Join(infraDir, statePath)
	}
	if !pathExists(statePath) {
		return statePath, fmt.Sprintf("opentofu state missing: %s", statePath)
	}
	return statePath, ""
}

func mergeInventory(base *clusterInventory, incoming clusterInventory) {
	if base == nil {
		return
	}
	base.Source = incoming.Source
	if incoming.StatePath != "" {
		base.StatePath = incoming.StatePath
	}
	if incoming.Network.APIEndpoint != "" {
		base.Network.APIEndpoint = incoming.Network.APIEndpoint
	}
	if incoming.Network.APIVIP != "" {
		base.Network.APIVIP = incoming.Network.APIVIP
	}
	if incoming.Network.InternalVIP != "" {
		base.Network.InternalVIP = incoming.Network.InternalVIP
	}
	if incoming.Network.LoadBalancer != "" {
		base.Network.LoadBalancer = incoming.Network.LoadBalancer
	}
	if incoming.Network.FloatingIPPool != "" {
		base.Network.FloatingIPPool = incoming.Network.FloatingIPPool
	}
	if incoming.Network.BastionFloatingIP != "" {
		base.Network.BastionFloatingIP = incoming.Network.BastionFloatingIP
	}
	if len(incoming.Nodes) > 0 {
		base.Nodes = incoming.Nodes
	}
	base.Warnings = append(base.Warnings, incoming.Warnings...)
}

func (i clusterInventory) hasProvisionedData() bool {
	return len(i.Nodes) > 0 ||
		i.Network.APIEndpoint != "" ||
		i.Network.APIVIP != "" ||
		i.Network.InternalVIP != "" ||
		i.Network.BastionFloatingIP != ""
}

func readOpenTofuStateInventory(statePath string) (clusterInventory, error) {
	inventory := clusterInventory{
		Source:    inventorySourceOpenTofuState,
		StatePath: statePath,
	}

	data, err := os.ReadFile(statePath)
	if err != nil {
		return inventory, fmt.Errorf("read opentofu state: %w", err)
	}

	var state map[string]any
	if err := json.Unmarshal(data, &state); err != nil {
		return inventory, fmt.Errorf("decode opentofu state: %w", err)
	}

	if outputs, ok := state["outputs"].(map[string]any); ok {
		mergeInventoryValues(&inventory, outputs)
	}
	if values, ok := state["values"].(map[string]any); ok {
		parseOpenTofuValuesModule(&inventory, values)
	}
	if resources, ok := state["resources"].([]any); ok {
		parseOpenTofuResources(&inventory, resources)
	}

	return inventory, nil
}

func parseOpenTofuValuesModule(inventory *clusterInventory, module map[string]any) {
	if rootModule, ok := module["root_module"].(map[string]any); ok {
		parseOpenTofuValuesModule(inventory, rootModule)
		return
	}
	if outputs, ok := module["outputs"].(map[string]any); ok {
		mergeInventoryValues(inventory, outputs)
	}
	if resources, ok := module["resources"].([]any); ok {
		parseOpenTofuResourceValues(inventory, resources)
	}
	if childModules, ok := module["child_modules"].([]any); ok {
		for _, rawChild := range childModules {
			child, ok := rawChild.(map[string]any)
			if !ok {
				continue
			}
			parseOpenTofuValuesModule(inventory, child)
		}
	}
}

func parseOpenTofuResources(inventory *clusterInventory, resources []any) {
	for _, rawResource := range resources {
		resource, ok := rawResource.(map[string]any)
		if !ok {
			continue
		}
		instances, ok := resource["instances"].([]any)
		if !ok {
			continue
		}
		for _, rawInstance := range instances {
			instance, ok := rawInstance.(map[string]any)
			if !ok {
				continue
			}
			attrs, ok := instance["attributes"].(map[string]any)
			if !ok {
				continue
			}
			mergeInventoryValues(inventory, attrs)
		}
	}
}

func parseOpenTofuResourceValues(inventory *clusterInventory, resources []any) {
	for _, rawResource := range resources {
		resource, ok := rawResource.(map[string]any)
		if !ok {
			continue
		}
		values, ok := resource["values"].(map[string]any)
		if !ok {
			continue
		}
		mergeInventoryValues(inventory, values)
	}
}

func mergeInventoryValues(inventory *clusterInventory, values map[string]any) {
	if inventory == nil || len(values) == 0 {
		return
	}

	for _, key := range []string{"k8s_api_ip", "api_vip"} {
		if raw, ok := values[key]; ok && inventory.Network.APIVIP == "" {
			inventory.Network.APIVIP = stringValue(terraformValue(raw))
		}
	}
	for _, key := range []string{"k8s_internal_ip", "internal_vip", "vrrp_ip"} {
		if raw, ok := values[key]; ok && inventory.Network.InternalVIP == "" {
			inventory.Network.InternalVIP = stringValue(terraformValue(raw))
		}
	}
	for _, key := range []string{"bastion_floating_ip", "address_bastion"} {
		if raw, ok := values[key]; ok && inventory.Network.BastionFloatingIP == "" {
			inventory.Network.BastionFloatingIP = stringValue(terraformValue(raw))
		}
	}

	appendInventoryNodes(inventory, inventoryNodeRoleController, terraformValue(values["master_nodes"]))
	appendInventoryNodes(inventory, inventoryNodeRoleWorker, terraformValue(values["worker_nodes"]))
	appendInventoryNodes(inventory, inventoryNodeRoleWindowsWorker, terraformValue(values["windows_nodes"]))
	appendInventoryNodes(inventory, inventoryNodeRoleWorker, terraformValue(values["additional_worker_pools_nodes"]))
	appendInventoryNodes(inventory, inventoryNodeRoleWindowsWorker, terraformValue(values["additional_worker_pools_windows_nodes"]))
}

func terraformValue(raw any) any {
	mapped, ok := raw.(map[string]any)
	if !ok {
		return raw
	}
	if value, ok := mapped["value"]; ok {
		return value
	}
	return raw
}

func appendInventoryNodes(inventory *clusterInventory, role string, raw any) {
	if inventory == nil || raw == nil {
		return
	}
	for _, node := range nodesFromValue(raw, role) {
		appendInventoryNode(inventory, node)
	}
}

func nodesFromValue(raw any, role string) []clusterInventoryNode {
	switch value := raw.(type) {
	case []any:
		nodes := make([]clusterInventoryNode, 0, len(value))
		for _, item := range value {
			nodes = append(nodes, nodesFromValue(item, role)...)
		}
		return nodes
	case map[string]any:
		if looksLikeNodeMap(value) {
			if node, ok := nodeFromMap(value, role, inventorySourceOpenTofuState); ok {
				return []clusterInventoryNode{node}
			}
			return nil
		}
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var nodes []clusterInventoryNode
		for _, key := range keys {
			nodes = append(nodes, nodesFromValue(value[key], role)...)
		}
		return nodes
	default:
		return nil
	}
}

func looksLikeNodeMap(value map[string]any) bool {
	for _, key := range []string{
		"name", "id", "hostname",
		"access_ip_v4", "internal_ip", "fixed_ip", "ip", "private_ip", "address",
		"external_ip", "floating_ip", "public_ip",
	} {
		if _, ok := value[key]; ok {
			return true
		}
	}
	return false
}

func nodeFromMap(value map[string]any, role, source string) (clusterInventoryNode, bool) {
	node := clusterInventoryNode{
		Role:   role,
		Source: source,
	}
	node.Name = firstString(value, "name", "id", "hostname")
	node.InternalIP = firstString(value, "access_ip_v4", "internal_ip", "fixed_ip", "ip", "private_ip", "address")
	node.ExternalIP = firstString(value, "external_ip", "floating_ip", "public_ip")
	if node.Name == "" && node.InternalIP != "" {
		node.Name = node.InternalIP
	}
	if node.Name == "" && node.InternalIP == "" && node.ExternalIP == "" {
		return clusterInventoryNode{}, false
	}
	return node, true
}

func appendInventoryNode(inventory *clusterInventory, node clusterInventoryNode) {
	if node.Source == "" {
		node.Source = inventorySourceOpenTofuState
	}
	key := node.Role + "\x00" + node.Name + "\x00" + node.InternalIP + "\x00" + node.ExternalIP
	for _, existing := range inventory.Nodes {
		existingKey := existing.Role + "\x00" + existing.Name + "\x00" + existing.InternalIP + "\x00" + existing.ExternalIP
		if existingKey == key {
			return
		}
	}
	inventory.Nodes = append(inventory.Nodes, node)
}

func collectKubernetesRefreshInventory(ctx context.Context, kubeconfigPath string) (clusterInventory, error) {
	inventory := clusterInventory{Source: inventorySourceKubernetesRefresh}
	if !pathExists(kubeconfigPath) {
		return inventory, fmt.Errorf("refresh kubeconfig missing: %s", kubeconfigPath)
	}

	statusCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	runner := security.GetDefaultCommandRunner()
	cmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "get", "nodes", "-o", "json")
	if err != nil {
		return inventory, fmt.Errorf("prepare kubectl node refresh: %w", err)
	}
	output, err := cmd.Output()
	if err != nil {
		return inventory, fmt.Errorf("kubectl node refresh failed: %w", err)
	}

	var nodes kubernetesNodeList
	if err := json.Unmarshal(output, &nodes); err != nil {
		return inventory, fmt.Errorf("decode kubectl nodes response: %w", err)
	}

	for _, item := range nodes.Items {
		ready := item.ready()
		node := clusterInventoryNode{
			Name:       strings.TrimSpace(item.Metadata.Name),
			Role:       kubernetesNodeRole(item.Metadata.Labels),
			InternalIP: item.address("InternalIP"),
			ExternalIP: item.address("ExternalIP"),
			Source:     inventorySourceKubernetesRefresh,
			Ready:      &ready,
		}
		if node.Name == "" {
			node.Name = node.InternalIP
		}
		if node.Name == "" && node.InternalIP == "" && node.ExternalIP == "" {
			continue
		}
		inventory.Nodes = append(inventory.Nodes, node)
	}

	endpointCmd, err := runner.PrepareCommandContext(statusCtx, "kubectl", "--kubeconfig", kubeconfigPath, "config", "view", "--minify", "-o", "jsonpath={.clusters[0].cluster.server}")
	if err != nil {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("prepare kubectl endpoint refresh: %v", err))
		return inventory, nil
	}
	endpointOutput, err := endpointCmd.Output()
	if err != nil {
		inventory.Warnings = append(inventory.Warnings, fmt.Sprintf("kubectl endpoint refresh failed: %v", err))
		return inventory, nil
	}
	inventory.Network.APIEndpoint = strings.TrimSpace(string(endpointOutput))

	return inventory, nil
}

type kubernetesNodeList struct {
	Items []kubernetesNode `json:"items"`
}

type kubernetesNode struct {
	Metadata struct {
		Name   string            `json:"name"`
		Labels map[string]string `json:"labels"`
	} `json:"metadata"`
	Status struct {
		Addresses []struct {
			Type    string `json:"type"`
			Address string `json:"address"`
		} `json:"addresses"`
		Conditions []struct {
			Type   string `json:"type"`
			Status string `json:"status"`
		} `json:"conditions"`
	} `json:"status"`
}

func (n kubernetesNode) address(addressType string) string {
	for _, address := range n.Status.Addresses {
		if address.Type == addressType {
			return strings.TrimSpace(address.Address)
		}
	}
	return ""
}

func (n kubernetesNode) ready() bool {
	for _, condition := range n.Status.Conditions {
		if condition.Type == "Ready" {
			return strings.EqualFold(condition.Status, "True")
		}
	}
	return false
}

func kubernetesNodeRole(labels map[string]string) string {
	if labels == nil {
		return inventoryNodeRoleWorker
	}
	if _, ok := labels["node-role.kubernetes.io/control-plane"]; ok {
		return inventoryNodeRoleController
	}
	if _, ok := labels["node-role.kubernetes.io/master"]; ok {
		return inventoryNodeRoleController
	}
	return inventoryNodeRoleWorker
}

func renderClusterInventoryText(w io.Writer, inventory clusterInventory) {
	renderNetworkText(w, inventory)
	renderNodesText(w, inventory)

	fmt.Fprintln(w, "\nInventory:")
	fmt.Fprintf(w, "  Source: %s\n", inventorySourceLabel(inventory.Source))
	if inventory.StatePath != "" {
		fmt.Fprintf(w, "  State:  %s\n", inventory.StatePath)
	}
	if len(inventory.Warnings) > 0 {
		fmt.Fprintln(w, "  Warnings:")
		for _, warning := range inventory.Warnings {
			fmt.Fprintf(w, "    - %s\n", warning)
		}
	}
}

func renderNetworkText(w io.Writer, inventory clusterInventory) {
	fmt.Fprintln(w, "\nNetwork:")
	configSuffix := ""
	if inventory.Source == inventorySourceConfig {
		configSuffix = " (configured)"
	}
	if inventory.Network.APIEndpoint != "" {
		fmt.Fprintf(w, "  API endpoint:         %s\n", inventory.Network.APIEndpoint)
	}
	if inventory.Network.APIVIP != "" {
		fmt.Fprintf(w, "  API VIP:              %s%s\n", inventory.Network.APIVIP, configSuffix)
	}
	if inventory.Network.InternalVIP != "" {
		fmt.Fprintf(w, "  Internal VIP:         %s%s\n", inventory.Network.InternalVIP, configSuffix)
	}
	if inventory.Network.LoadBalancer != "" {
		fmt.Fprintf(w, "  Load balancer:        %s\n", inventory.Network.LoadBalancer)
	}
	if inventory.Network.FloatingIPPool != "" {
		fmt.Fprintf(w, "  Floating IP pool:     %s\n", inventory.Network.FloatingIPPool)
	}
	if inventory.Network.BastionFloatingIP != "" {
		fmt.Fprintf(w, "  Bastion Floating IP:  %s\n", inventory.Network.BastionFloatingIP)
	}
}

func renderNodesText(w io.Writer, inventory clusterInventory) {
	fmt.Fprintln(w, "\nNodes:")
	controllers := inventory.nodesByRole(inventoryNodeRoleController)
	workers := inventory.nodesByRole(inventoryNodeRoleWorker)
	windowsWorkers := inventory.nodesByRole(inventoryNodeRoleWindowsWorker)

	if len(controllers) == 0 && len(workers) == 0 && len(windowsWorkers) == 0 {
		fmt.Fprintln(w, "  Controller IPs: unavailable until OpenTofu provisioning completes")
		fmt.Fprintln(w, "  Worker IPs:     unavailable until OpenTofu provisioning completes")
		return
	}

	renderNodeGroup(w, "Controllers", controllers)
	renderNodeGroup(w, "Workers", workers)
	renderNodeGroup(w, "Windows Workers", windowsWorkers)
}

func (i clusterInventory) nodesByRole(role string) []clusterInventoryNode {
	nodes := make([]clusterInventoryNode, 0)
	for _, node := range i.Nodes {
		if node.Role == role {
			nodes = append(nodes, node)
		}
	}
	return nodes
}

func renderNodeGroup(w io.Writer, label string, nodes []clusterInventoryNode) {
	if len(nodes) == 0 {
		return
	}
	fmt.Fprintf(w, "  %s:\n", label)
	for _, node := range nodes {
		if node.ExternalIP != "" {
			fmt.Fprintf(w, "    %s  %s  %s\n", node.Name, node.InternalIP, node.ExternalIP)
			continue
		}
		fmt.Fprintf(w, "    %s  %s\n", node.Name, node.InternalIP)
	}
}

func inventorySourceLabel(source string) string {
	switch source {
	case inventorySourceOpenTofuState:
		return "OpenTofu state"
	case inventorySourceKubernetesRefresh:
		return "Kubernetes refresh"
	case inventorySourceConfig:
		return "configuration"
	default:
		if strings.TrimSpace(source) == "" {
			return "configuration"
		}
		return source
	}
}

func firstString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringValue(values[key]); value != "" {
			return value
		}
	}
	return ""
}

func stringValue(raw any) string {
	switch value := raw.(type) {
	case string:
		return strings.TrimSpace(value)
	case fmt.Stringer:
		return strings.TrimSpace(value.String())
	default:
		return ""
	}
}
