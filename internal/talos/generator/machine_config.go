package generator

import (
	"context"
	"fmt"

	"github.com/rackerlabs/openCenter-cli/internal/talos"
	"gopkg.in/yaml.v3"
)

// GenerateTalosMachineConfig creates Talos machine configurations with security hardening.
func (g *generator) GenerateTalosMachineConfig(ctx context.Context, nodeType talos.NodeType) ([]byte, error) {
	if g.config == nil {
		return nil, talos.NewConfigurationError(
			"MISSING_CONFIG",
			"generator config is nil",
			nil,
		)
	}

	// Get Talos configuration from the main config
	// For now, we'll use default values since the config integration is in task 2
	talosConfig := talos.DefaultTalosConfig()

	// Build machine configuration based on node type
	machineConfig := g.buildMachineConfig(nodeType, talosConfig)

	// Marshal to YAML
	data, err := yaml.Marshal(machineConfig)
	if err != nil {
		return nil, talos.NewConfigurationError(
			"MARSHAL_ERROR",
			"failed to marshal machine config to YAML",
			err,
		)
	}

	return data, nil
}

// buildMachineConfig constructs the machine configuration structure.
func (g *generator) buildMachineConfig(nodeType talos.NodeType, talosConfig *talos.TalosConfig) map[string]interface{} {
	config := map[string]interface{}{
		"version": "v1alpha1",
		"machine": g.buildMachineSection(nodeType, talosConfig),
		"cluster": g.buildClusterSection(nodeType, talosConfig),
	}

	return config
}

// buildMachineSection creates the machine section of the configuration.
func (g *generator) buildMachineSection(nodeType talos.NodeType, talosConfig *talos.TalosConfig) map[string]interface{} {
	machine := map[string]interface{}{
		"type": g.getMachineType(nodeType),
	}

	// Add security features
	machine["features"] = g.buildSecurityFeatures(talosConfig)

	// Add sysctls for hardening
	machine["sysctls"] = g.buildHardenedSysctls()

	// Add disk encryption configuration
	if talosConfig.MachineConfig.DiskEncryption {
		machine["systemDiskEncryption"] = g.buildDiskEncryption(talosConfig)
	}

	// Add logging configuration
	if talosConfig.MachineConfig.LogDestination != "" {
		machine["logging"] = map[string]interface{}{
			"destinations": []map[string]interface{}{
				{
					"endpoint": talosConfig.MachineConfig.LogDestination,
					"format":   "json_lines",
				},
			},
		}
	}

	// Add network configuration
	machine["network"] = g.buildNetworkConfig(talosConfig)

	return machine
}

// buildSecurityFeatures creates the security features configuration.
func (g *generator) buildSecurityFeatures(talosConfig *talos.TalosConfig) map[string]interface{} {
	features := make(map[string]interface{})

	// Enable AppArmor
	if talosConfig.MachineConfig.AppArmorEnabled {
		features["apparmor"] = map[string]interface{}{
			"enabled": true,
		}
	}

	// Enable Seccomp
	if talosConfig.MachineConfig.SeccompEnabled {
		features["seccomp"] = map[string]interface{}{
			"enabled": true,
		}
	}

	// Enable KubePrism for internal load balancing
	if talosConfig.MachineConfig.KubePrismEnabled {
		features["kubePrism"] = map[string]interface{}{
			"enabled": true,
			"port":    7445,
		}
	}

	return features
}

// buildHardenedSysctls returns hardened kernel parameters.
func (g *generator) buildHardenedSysctls() map[string]string {
	return map[string]string{
		// Network security
		"net.ipv4.conf.all.rp_filter":                    "1",
		"net.ipv4.conf.default.rp_filter":                "1",
		"net.ipv4.conf.all.accept_source_route":          "0",
		"net.ipv4.conf.default.accept_source_route":      "0",
		"net.ipv4.conf.all.accept_redirects":             "0",
		"net.ipv4.conf.default.accept_redirects":         "0",
		"net.ipv4.conf.all.secure_redirects":             "0",
		"net.ipv4.conf.default.secure_redirects":         "0",
		"net.ipv4.conf.all.send_redirects":               "0",
		"net.ipv4.conf.default.send_redirects":           "0",
		"net.ipv4.icmp_echo_ignore_broadcasts":           "1",
		"net.ipv4.icmp_ignore_bogus_error_responses":     "1",
		"net.ipv4.tcp_syncookies":                        "1",
		"net.ipv4.conf.all.log_martians":                 "1",
		"net.ipv4.conf.default.log_martians":             "1",
		"net.ipv6.conf.all.accept_redirects":             "0",
		"net.ipv6.conf.default.accept_redirects":         "0",
		"net.ipv6.conf.all.accept_source_route":          "0",
		"net.ipv6.conf.default.accept_source_route":      "0",
		// Kernel hardening
		"kernel.kptr_restrict":                           "2",
		"kernel.dmesg_restrict":                          "1",
		"kernel.perf_event_paranoid":                     "3",
		"kernel.unprivileged_bpf_disabled":               "1",
		"kernel.yama.ptrace_scope":                       "1",
		// File system hardening
		"fs.protected_hardlinks":                         "1",
		"fs.protected_symlinks":                          "1",
		"fs.suid_dumpable":                               "0",
	}
}

// buildDiskEncryption creates disk encryption configuration.
func (g *generator) buildDiskEncryption(talosConfig *talos.TalosConfig) map[string]interface{} {
	encryption := map[string]interface{}{
		"state": map[string]interface{}{
			"provider": "luks2",
		},
		"ephemeral": map[string]interface{}{
			"provider": "luks2",
		},
	}

	// Use vTPM if enabled, otherwise use Barbican-managed keys
	if talosConfig.SecurityConfig.VTPMEnabled {
		encryption["state"].(map[string]interface{})["keys"] = []map[string]interface{}{
			{
				"slot": 0,
				"tpm":  map[string]interface{}{},
			},
		}
		encryption["ephemeral"].(map[string]interface{})["keys"] = []map[string]interface{}{
			{
				"slot": 0,
				"tpm":  map[string]interface{}{},
			},
		}
	} else if talosConfig.SecurityConfig.BarbicanKeyID != "" {
		// Fallback to Barbican-managed keys
		encryption["state"].(map[string]interface{})["keys"] = []map[string]interface{}{
			{
				"slot":   0,
				"static": map[string]interface{}{
					"keyID": talosConfig.SecurityConfig.BarbicanKeyID,
				},
			},
		}
		encryption["ephemeral"].(map[string]interface{})["keys"] = []map[string]interface{}{
			{
				"slot":   0,
				"static": map[string]interface{}{
					"keyID": talosConfig.SecurityConfig.BarbicanKeyID,
				},
			},
		}
	}

	return encryption
}

// buildNetworkConfig creates network configuration.
func (g *generator) buildNetworkConfig(talosConfig *talos.TalosConfig) map[string]interface{} {
	return map[string]interface{}{
		"hostname": fmt.Sprintf("talos-node"),
		"interfaces": []map[string]interface{}{
			{
				"interface": "eth0",
				"dhcp":      true,
			},
		},
	}
}

// buildClusterSection creates the cluster section of the configuration.
func (g *generator) buildClusterSection(nodeType talos.NodeType, talosConfig *talos.TalosConfig) map[string]interface{} {
	cluster := map[string]interface{}{
		"clusterName": "talos-cluster",
		"network": map[string]interface{}{
			"cni": map[string]interface{}{
				"name": "none", // CNI will be managed by GitOps
			},
		},
	}

	// Add control plane configuration for control plane nodes
	if nodeType == talos.NodeTypeControlPlane {
		cluster["controlPlane"] = map[string]interface{}{
			"endpoint": fmt.Sprintf("https://127.0.0.1:%d", talosConfig.NetworkConfig.TalosAPIPort),
		}

		// Add API server configuration with audit logging
		if talosConfig.SecurityConfig.AuditLogEnabled {
			cluster["apiServer"] = map[string]interface{}{
				"auditPolicy": map[string]interface{}{
					"apiVersion": "audit.k8s.io/v1",
					"kind":       "Policy",
					"rules": []map[string]interface{}{
						{
							"level": "Metadata",
						},
					},
				},
			}
		}

		// Add etcd configuration
		cluster["etcd"] = map[string]interface{}{
			"advertisedSubnets": []string{
				talosConfig.NetworkConfig.ControlSubnet,
			},
		}
	}

	return cluster
}

// getMachineType returns the machine type string for the given node type.
func (g *generator) getMachineType(nodeType talos.NodeType) string {
	switch nodeType {
	case talos.NodeTypeControlPlane:
		return "controlplane"
	case talos.NodeTypeWorker:
		return "worker"
	case talos.NodeTypeBastion:
		return "worker" // Bastion is treated as a worker node
	default:
		return "worker"
	}
}
