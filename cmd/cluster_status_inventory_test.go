package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReadOpenTofuStateInventoryExtractsRootOutputs(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "terraform.tfstate")
	state := `{
  "version": 4,
  "outputs": {
    "master_nodes": {
      "value": [
        {"name": "cp-1", "access_ip_v4": "10.2.128.11"},
        {"name": "cp-2", "internal_ip": "10.2.128.12", "floating_ip": "203.0.113.12"}
      ]
    },
    "worker_nodes": {
      "value": [
        {"id": "worker-id-1", "fixed_ip": "10.2.128.21", "external_ip": "203.0.113.21"}
      ]
    },
    "additional_worker_pools_nodes": {
      "value": {
        "gpu": [
          {"name": "gpu-1", "ip": "10.2.128.31"}
        ]
      }
    },
    "k8s_api_ip": {"value": "203.0.113.20"},
    "k8s_internal_ip": {"value": "10.2.128.5"},
    "bastion_floating_ip": {"value": "198.51.100.10"}
  }
}`
	if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	inventory, err := readOpenTofuStateInventory(statePath)
	if err != nil {
		t.Fatalf("readOpenTofuStateInventory() error = %v", err)
	}

	if inventory.Source != inventorySourceOpenTofuState {
		t.Fatalf("source = %q, want %q", inventory.Source, inventorySourceOpenTofuState)
	}
	if inventory.StatePath != statePath {
		t.Fatalf("state path = %q, want %q", inventory.StatePath, statePath)
	}
	if inventory.Network.APIVIP != "203.0.113.20" {
		t.Fatalf("api vip = %q", inventory.Network.APIVIP)
	}
	if inventory.Network.InternalVIP != "10.2.128.5" {
		t.Fatalf("internal vip = %q", inventory.Network.InternalVIP)
	}
	if inventory.Network.BastionFloatingIP != "198.51.100.10" {
		t.Fatalf("bastion floating ip = %q", inventory.Network.BastionFloatingIP)
	}
	if len(inventory.Nodes) != 4 {
		t.Fatalf("nodes = %#v, want 4", inventory.Nodes)
	}
	if inventory.Nodes[0].Role != inventoryNodeRoleController || inventory.Nodes[0].InternalIP != "10.2.128.11" {
		t.Fatalf("first node = %#v", inventory.Nodes[0])
	}
	if inventory.Nodes[1].ExternalIP != "203.0.113.12" {
		t.Fatalf("second node external ip = %#v", inventory.Nodes[1])
	}
	if inventory.Nodes[2].Name != "worker-id-1" || inventory.Nodes[2].Role != inventoryNodeRoleWorker {
		t.Fatalf("worker node = %#v", inventory.Nodes[2])
	}
	if inventory.Nodes[3].Name != "gpu-1" || inventory.Nodes[3].InternalIP != "10.2.128.31" {
		t.Fatalf("additional worker node = %#v", inventory.Nodes[3])
	}
}

func TestReadOpenTofuStateInventoryExtractsResourceAttributes(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "terraform.tfstate")
	state := `{
  "version": 4,
  "resources": [
    {
      "module": "module.openstack-nova",
      "mode": "managed",
      "type": "test",
      "name": "inventory",
      "instances": [
        {
          "attributes": {
            "master_nodes": [{"name": "cp-resource", "access_ip_v4": "10.2.128.41"}],
            "worker_nodes": [{"name": "wn-resource", "access_ip_v4": "10.2.128.51"}],
            "k8s_api_ip": "203.0.113.40",
            "k8s_internal_ip": "10.2.128.40"
          }
        }
      ]
    }
  ]
}`
	if err := os.WriteFile(statePath, []byte(state), 0o600); err != nil {
		t.Fatalf("write state: %v", err)
	}

	inventory, err := readOpenTofuStateInventory(statePath)
	if err != nil {
		t.Fatalf("readOpenTofuStateInventory() error = %v", err)
	}

	if inventory.Network.APIVIP != "203.0.113.40" {
		t.Fatalf("api vip = %q", inventory.Network.APIVIP)
	}
	if inventory.Network.InternalVIP != "10.2.128.40" {
		t.Fatalf("internal vip = %q", inventory.Network.InternalVIP)
	}
	if len(inventory.Nodes) != 2 {
		t.Fatalf("nodes = %#v, want 2", inventory.Nodes)
	}
}
