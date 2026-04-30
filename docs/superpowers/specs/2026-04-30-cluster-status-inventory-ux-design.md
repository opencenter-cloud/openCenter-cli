# Cluster Status Inventory UX Design

## Context

`opencenter cluster status` currently shows cluster metadata, lifecycle state, file path readiness, and provider readiness checks for Kind and OpenStack. Operators also need the command to answer the practical question: "Where is this cluster?" That includes controller IPs, worker IPs, Kubernetes API VIPs, internal VIPs, and related floating IP or load balancer details.

The command must remain safe and predictable by default. A plain `cluster status` should not contact Kubernetes or cloud provider APIs. It should read local configuration and local OpenTofu state. Live refresh behavior belongs behind an explicit `--refresh` flag.

## Goals

- Show controller, worker, API VIP, internal VIP, and load balancer or floating IP details when available.
- Prefer OpenTofu state as the source of provisioned infrastructure inventory after provisioning has completed.
- Keep default status output offline, fast, and script-safe.
- Use `--refresh` for live lookups only.
- Expose the same inventory data through JSON/YAML output.
- Keep `--quiet` behavior unchanged.

## Non-Goals

- Do not persist refreshed live data back into the cluster config.
- Do not require root OpenTofu outputs to exist before the UX works.
- Do not call OpenStack, Kubernetes, or other provider APIs unless `--refresh` is set.
- Do not redesign `cluster describe` or the `cluster status --sync` mode.

## Data Sources

Default `cluster status` should merge data from:

- cluster configuration
- resolved cluster paths
- local OpenTofu state, when enabled and present

`--refresh` may additionally query:

- Kubernetes nodes through the cluster-owned kubeconfig
- existing provider readiness checks that currently run during status

The implementation should treat OpenTofu data as optional. If the state file is absent, unreadable, remote-only, or does not contain recognizable inventory values, the command should still render useful configured values and clearly mark provisioned inventory as unavailable.

## OpenTofu Inventory Extraction

The first implementation should support local state extraction from Terraform/OpenTofu state JSON. It should search both root outputs and module outputs/resource attributes for these known logical values:

- `master_nodes`
- `worker_nodes`
- `windows_nodes`
- `additional_worker_pools_nodes`
- `additional_worker_pools_windows_nodes`
- `k8s_api_ip`
- `k8s_internal_ip`
- `bastion_floating_ip`

Node values should normalize into a small internal model:

- name
- role: controller, worker, or windows-worker
- internal IP
- external IP, if available
- source

The reader should tolerate common field names such as `name`, `id`, `access_ip_v4`, `ip`, `fixed_ip`, `internal_ip`, `external_ip`, and `floating_ip`. Unknown shapes should be ignored rather than failing the command.

## Refresh Behavior

`--refresh` should be explicit. Without it, `cluster status` must not run `kubectl cluster-info`, `kubectl get nodes`, OpenStack CLI/API checks, or Kind API readiness checks.

With `--refresh`, the command may:

- check API readiness
- read the live API endpoint from kubeconfig
- collect node names, roles, InternalIP, ExternalIP, and readiness from `kubectl get nodes -o json`

Live node data should override stale state data for display in the same run, but the structured output should preserve source metadata so consumers can tell whether a value came from config, OpenTofu state, or refresh.

## Human Output

The default text output should keep the current top metadata block and add compact inventory sections.

Example with state-backed inventory:

```text
Cluster: acme/prod
  Active:       yes
  Name:         prod
  Environment:  production
  Region:       iad3
  Stage:        bootstrap
  Status:       success
  Organization: acme
  Provider:     openstack

Network:
  API endpoint:     https://203.0.113.20:6443
  API VIP:          203.0.113.20
  Internal VIP:     10.2.128.5
  Load balancer:    ovn
  Floating IP pool: PUBLICNET

Nodes:
  Controllers:
    prod-cp-1  10.2.128.11
    prod-cp-2  10.2.128.12
    prod-cp-3  10.2.128.13
  Workers:
    prod-wn-1  10.2.128.21
    prod-wn-2  10.2.128.22

Inventory:
  Source: OpenTofu state
  State:  /path/to/infrastructure/clusters/prod/.opentofu-local-prod/terraform.tfstate
```

Example before provisioning:

```text
Network:
  API VIP:          10.2.128.5 (configured)
  Internal VIP:     10.2.128.5 (configured)
  Load balancer:    ovn

Nodes:
  Controller IPs: unavailable until OpenTofu provisioning completes
  Worker IPs:     unavailable until OpenTofu provisioning completes
```

When `--refresh` is used, the inventory source line should make that visible:

```text
Inventory:
  Source: Kubernetes refresh
  State:  /path/to/terraform.tfstate
```

## Structured Output

`clusterStatusOutput` should gain an `inventory` object rather than placing node data inside provider-specific status maps. The shape should be stable and provider-neutral:

```json
{
  "inventory": {
    "source": "opentofu_state",
    "state_path": "/path/to/terraform.tfstate",
    "network": {
      "api_endpoint": "https://203.0.113.20:6443",
      "api_vip": "203.0.113.20",
      "internal_vip": "10.2.128.5",
      "load_balancer": "ovn",
      "floating_ip_pool": "PUBLICNET"
    },
    "nodes": [
      {
        "name": "prod-cp-1",
        "role": "controller",
        "internal_ip": "10.2.128.11",
        "external_ip": "",
        "source": "opentofu_state"
      }
    ],
    "warnings": []
  }
}
```

Warnings should be machine-readable strings for absent state, unsupported remote backend inspection, malformed state snippets, or refresh failures.

## Error Handling

Inventory extraction should not make `cluster status` fail unless the base config load already fails. Recoverable inventory problems should become warnings in structured output and short explanatory text in human output.

Examples:

- local state path does not exist
- state JSON cannot be decoded
- expected module outputs are missing
- `--refresh` cannot find kubeconfig
- `kubectl get nodes` fails

## Testing

Tests should cover:

- text output shows OpenTofu-derived controller, worker, API VIP, internal VIP, and floating IP/load balancer details
- default status does not run live checks
- `--refresh` runs live node collection and shows refreshed node IPs
- JSON output includes `inventory.network`, `inventory.nodes`, source, state path, and warnings
- missing state keeps command successful and shows configured VIP values plus unavailable node messaging
- `--quiet` remains only the cluster name

## Documentation

Update generated/reference documentation for:

- the new `--refresh` flag
- default offline/OpenTofu-first behavior
- examples for state-backed inventory and refreshed live inventory
