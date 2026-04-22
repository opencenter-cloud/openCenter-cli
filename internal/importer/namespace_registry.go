package importer

import (
	"fmt"
	"strings"
)

type NamespaceRegistry struct {
	namespaces map[string][]string
}

func NewNamespaceRegistry() *NamespaceRegistry {
	defaults := map[string][]string{
		"alert-proxy":              {"alert-proxy"},
		"calico":                   {"calico-system"},
		"cert-manager":             {"cert-manager"},
		"cilium":                   {"kube-system"},
		"etcd-backup":              {"kube-system"},
		"external-snapshotter":     {"kube-system"},
		"fluxcd":                   {"flux-system"},
		"gateway":                  {"gateway-system"},
		"gateway-api":              {"gateway-system"},
		"harbor":                   {"harbor"},
		"headlamp":                 {"headlamp"},
		"kafka-cluster":            {"kafka"},
		"keycloak":                 {"keycloak"},
		"kube-prometheus-stack":    {"monitoring"},
		"kube-ovn":                 {"kube-system"},
		"kyverno":                  {"kyverno"},
		"loki":                     {"monitoring"},
		"longhorn":                 {"longhorn-system"},
		"metallb":                  {"metallb-system"},
		"mimir":                    {"monitoring"},
		"olm":                      {"olm"},
		"openstack-ccm":            {"kube-system"},
		"openstack-csi":            {"kube-system"},
		"opentelemetry-kube-stack": {"monitoring"},
		"postgres-operator":        {"postgres-operator"},
		"rbac-manager":             {"rbac-manager"},
		"sealed-secrets":           {"sealed-secrets"},
		"sources":                  {"flux-system"},
		"tempo":                    {"monitoring"},
		"velero":                   {"velero"},
		"vsphere-csi":              {"vmware-system-csi"},
		"weave-gitops":             {"flux-system"},
	}

	return &NamespaceRegistry{namespaces: defaults}
}

func (r *NamespaceRegistry) ApplyOverrides(overrides []string) error {
	for _, override := range overrides {
		override = strings.TrimSpace(override)
		if override == "" {
			continue
		}

		parts := strings.SplitN(override, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid service namespace override %q", override)
		}

		serviceName := strings.TrimSpace(parts[0])
		if serviceName == "" {
			return fmt.Errorf("service name cannot be empty in override %q", override)
		}

		rawNamespaces := strings.Split(parts[1], ",")
		namespaces := make([]string, 0, len(rawNamespaces))
		for _, ns := range rawNamespaces {
			ns = strings.TrimSpace(ns)
			if ns != "" {
				namespaces = append(namespaces, ns)
			}
		}
		if len(namespaces) == 0 {
			return fmt.Errorf("namespace list cannot be empty in override %q", override)
		}

		r.namespaces[serviceName] = namespaces
	}

	return nil
}

func (r *NamespaceRegistry) NamespacesFor(serviceName string) []string {
	if namespaces, ok := r.namespaces[serviceName]; ok && len(namespaces) > 0 {
		return append([]string(nil), namespaces...)
	}
	return []string{serviceName}
}
