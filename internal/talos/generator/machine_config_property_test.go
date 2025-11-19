package generator

import (
	"context"
	"strings"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/rackerlabs/openCenter-cli/internal/config"
	"github.com/rackerlabs/openCenter-cli/internal/talos"
	"gopkg.in/yaml.v3"
)

// Feature: talos-openstack-provider, Property 4: Security hardening completeness
// For any generated Talos machine configuration, the configuration should contain
// enabled settings for AppArmor, Seccomp, hardened sysctls, KubePrism, and disk encryption.
// Validates: Requirements 2.1, 2.2, 2.3, 2.4, 2.5
func TestProperty_SecurityHardeningCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("all generated Talos configs have security hardening",
		prop.ForAll(
			func(nodeType talos.NodeType, clusterName string) bool {
				// Create a generator with minimal config
				cfg := &config.Config{
					OpenCenter: config.SimplifiedOpenCenter{
						Meta: config.ClusterMeta{
							Name: clusterName,
						},
					},
				}
				g := New(cfg)

				// Generate machine config
				machineConfigBytes, err := g.GenerateTalosMachineConfig(context.Background(), nodeType)
				if err != nil {
					t.Logf("Failed to generate machine config: %v", err)
					return false
				}

				// Parse the YAML to verify security features
				var machineConfig map[string]interface{}
				if err := yaml.Unmarshal(machineConfigBytes, &machineConfig); err != nil {
					t.Logf("Failed to unmarshal machine config: %v", err)
					return false
				}

				// Verify all security features are present
				hasAppArmor := containsAppArmor(machineConfig)
				hasSeccomp := containsSeccomp(machineConfig)
				hasHardenedSysctls := containsHardenedSysctls(machineConfig)
				hasKubePrism := containsKubePrism(machineConfig)
				hasDiskEncryption := containsDiskEncryption(machineConfig)

				if !hasAppArmor {
					t.Logf("Missing AppArmor configuration")
				}
				if !hasSeccomp {
					t.Logf("Missing Seccomp configuration")
				}
				if !hasHardenedSysctls {
					t.Logf("Missing hardened sysctls")
				}
				if !hasKubePrism {
					t.Logf("Missing KubePrism configuration")
				}
				if !hasDiskEncryption {
					t.Logf("Missing disk encryption configuration")
				}

				return hasAppArmor && hasSeccomp && hasHardenedSysctls && hasKubePrism && hasDiskEncryption
			},
			genNodeType(),
			gen.Identifier(),
		))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// genNodeType generates random node types for property testing.
func genNodeType() gopter.Gen {
	return gen.OneConstOf(
		talos.NodeTypeControlPlane,
		talos.NodeTypeWorker,
		talos.NodeTypeBastion,
	)
}

// containsAppArmor checks if the machine config has AppArmor enabled.
func containsAppArmor(config map[string]interface{}) bool {
	machine, ok := config["machine"].(map[string]interface{})
	if !ok {
		return false
	}

	features, ok := machine["features"].(map[string]interface{})
	if !ok {
		return false
	}

	apparmor, ok := features["apparmor"].(map[string]interface{})
	if !ok {
		return false
	}

	enabled, ok := apparmor["enabled"].(bool)
	return ok && enabled
}

// containsSeccomp checks if the machine config has Seccomp enabled.
func containsSeccomp(config map[string]interface{}) bool {
	machine, ok := config["machine"].(map[string]interface{})
	if !ok {
		return false
	}

	features, ok := machine["features"].(map[string]interface{})
	if !ok {
		return false
	}

	seccomp, ok := features["seccomp"].(map[string]interface{})
	if !ok {
		return false
	}

	enabled, ok := seccomp["enabled"].(bool)
	return ok && enabled
}

// containsHardenedSysctls checks if the machine config has hardened sysctls.
func containsHardenedSysctls(config map[string]interface{}) bool {
	machine, ok := config["machine"].(map[string]interface{})
	if !ok {
		return false
	}

	sysctlsRaw, ok := machine["sysctls"]
	if !ok {
		return false
	}

	// Handle both map[string]string and map[string]interface{}
	var sysctls map[string]interface{}
	switch v := sysctlsRaw.(type) {
	case map[string]string:
		sysctls = make(map[string]interface{})
		for k, val := range v {
			sysctls[k] = val
		}
	case map[string]interface{}:
		sysctls = v
	default:
		return false
	}

	// Check for key hardening sysctls
	requiredSysctls := []string{
		"net.ipv4.conf.all.rp_filter",
		"net.ipv4.tcp_syncookies",
		"kernel.kptr_restrict",
		"kernel.dmesg_restrict",
		"fs.protected_hardlinks",
		"fs.protected_symlinks",
	}

	for _, sysctl := range requiredSysctls {
		if _, exists := sysctls[sysctl]; !exists {
			return false
		}
	}

	return true
}

// containsKubePrism checks if the machine config has KubePrism enabled.
func containsKubePrism(config map[string]interface{}) bool {
	machine, ok := config["machine"].(map[string]interface{})
	if !ok {
		return false
	}

	features, ok := machine["features"].(map[string]interface{})
	if !ok {
		return false
	}

	kubePrism, ok := features["kubePrism"].(map[string]interface{})
	if !ok {
		return false
	}

	enabled, ok := kubePrism["enabled"].(bool)
	return ok && enabled
}

// containsDiskEncryption checks if the machine config has disk encryption configured.
func containsDiskEncryption(config map[string]interface{}) bool {
	machine, ok := config["machine"].(map[string]interface{})
	if !ok {
		return false
	}

	encryption, ok := machine["systemDiskEncryption"].(map[string]interface{})
	if !ok {
		return false
	}

	// Check for state partition encryption
	state, ok := encryption["state"].(map[string]interface{})
	if !ok {
		return false
	}

	stateProvider, ok := state["provider"].(string)
	if !ok || !strings.Contains(stateProvider, "luks") {
		return false
	}

	// Check for ephemeral partition encryption
	ephemeral, ok := encryption["ephemeral"].(map[string]interface{})
	if !ok {
		return false
	}

	ephemeralProvider, ok := ephemeral["provider"].(string)
	if !ok || !strings.Contains(ephemeralProvider, "luks") {
		return false
	}

	return true
}
