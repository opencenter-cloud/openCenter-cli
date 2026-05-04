# Talos OpenStack External Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use `superpowers:subagent-driven-development` or `superpowers:executing-plans` to implement task-by-task. When execution is allowed, save this plan to `docs/superpowers/plans/2026-05-04-talos-openstack-external-management.md` before editing code.

**Goal:** Make Talos OpenStack deploy work from outside OpenStack by using Kubernetes API on `443` and direct per-node Talos management access on `50000`, restricted to configured CIDRs.

**Architecture:** Add `deployment.talos.network.management_cidrs`, enforce Talos OpenStack API endpoint `443`, render per-node management floating IP/security resources into OpenStack Talos Terraform, and split Talos client behavior into insecure direct initial apply plus authenticated direct post-apply operations.

**Tech Stack:** Go, Talos machinery client, OpenTofu/Terraform OpenStack provider, v2 config/schema generator, existing Go tests.

---

## Key Interface Changes

- Add `ManagementCIDRs []string yaml:"management_cidrs,omitempty" json:"management_cidrs,omitempty" validate:"omitempty,dive,cidrv4"` to `v2.TalosNetworkConfig`.
- Talos OpenStack configs must set `deployment.talos.network.management_cidrs`; no auto-detection of operator public IP.
- `ApplyTalosDeploymentDefaults` must set Kubernetes API port to `443` for Talos deployments.
- Inventory validation must require `cluster.endpoint` to be explicit `https://...:443`.
- Talos client wrapper must never use Talos `WithNodes` proxy metadata for node-targeted OpenStack bootstrap operations.

## Implementation Tasks

### Task 1: Config, Defaults, Schema

- Update v2 config types/defaults/validation:
  - Add `management_cidrs` to `TalosNetworkConfig`.
  - Include `management_cidrs` in `cluster init --deployment talos` generated config map as an empty list.
  - Set `cfg.OpenCenter.Cluster.Kubernetes.APIPort = 443` inside `ApplyTalosDeploymentDefaults`.
  - In `TalosDeployment.ValidateConfig`, reject:
    - empty `management_cidrs`
    - explicit `deployment.talos.endpoint` that is not HTTPS with explicit port `443`
    - Talos OpenStack configs where `opencenter.cluster.kubernetes.api_port != 443`
- Update generated schema so `deployment.talos.network.management_cidrs` appears as an array of IPv4 CIDRs.
- Tests:
  - Add/update `internal/config/v2/talos_deployment_test.go` cases for required CIDRs, invalid CIDR, endpoint `:6443` rejection, endpoint `:443` acceptance, and API port forced to `443`.
  - Update schema tests or regenerate `schema/opencenter-v2.schema.json`.
- Verify:
  - `go test ./internal/config/v2 ./internal/config/v2schema`

### Task 2: Inventory Contract

- Update Talos inventory validation:
  - Add `validateClusterEndpoint(path, endpoint string) error`.
  - Parse with `net/url`; require scheme `https`, host present, explicit port `443`.
  - Keep `cluster.talos_api_port` range `1..65535`.
- Add endpoint helpers:
  - `func (i *Inventory) ControlPlaneEndpoints() []string`
  - `func (i *Inventory) AllNodeEndpoints() []string`
  - `func talosEndpoint(address string, port int) string`
  - Use `net.JoinHostPort`; preserve input if it already has a port.
- Replace existing `EndpointIPs()` callers with endpoint helpers.
- Tests:
  - Update inventory fixtures from `:6443` to `:443`.
  - Add rejection tests for `http://...:443`, `https://...:6443`, and `https://host-without-port`.
  - Add helper tests for `198.51.100.11` plus port `50000` returning `198.51.100.11:50000`.
- Verify:
  - `go test ./internal/deployment/talos`

### Task 3: Talos Client Modes

- Refactor `internal/deployment/talos/client.go`:
  - Add `withAuthenticatedClient(ctx, endpoint string, fn ...)`.
  - Add `withMaintenanceClient(ctx, endpoint string, fn ...)` using `talosclient.WithTLSConfig(&tls.Config{InsecureSkipVerify: true})` and `talosclient.WithEndpoints(endpoint)`.
  - Make `ApplyMachineConfig` use maintenance client and no `WithNodes`.
  - Make `Bootstrap`, `Kubeconfig`, and per-node health checks use authenticated direct endpoint and no `WithNodes`.
  - Make `Health` loop endpoints and call `Version(ctx)` on a direct client per endpoint.
- Add operation error wrapping helper:
  - Include operation, node name when available, endpoint, and remediation text for timeout/refused/DNS failures.
- Update runtime:
  - Pass normalized `node.talos_api_ip:cluster.talos_api_port` endpoints to client calls.
  - Generate talosconfig endpoint list from control-plane management endpoints, not Kubernetes VIP.
- Tests:
  - Add a fake Talos client factory or option capture test proving initial apply uses maintenance/insecure direct endpoint.
  - Update runtime flow expectations to `198.51.100.x:50000`.
  - Add health test proving each node is checked directly, not via proxy node metadata.
- Verify:
  - `go test ./internal/deployment/talos`

### Task 4: OpenStack Talos Terraform Rendering

- Update `main-openstack-talos.tf.tpl`:
  - Add OpenStack provider to `required_providers`.
  - Set `local.k8s_api_port` default to `443`.
  - Set fallback `local.talos_endpoint` to `https://${module.openstack-nova.k8s_api_ip}:443`.
  - Render `local.talos_management_cidrs` from `deployment.talos.network.management_cidrs`.
  - Add per-node management floating IP resources keyed by module node names:
    - lookup instances by node name with `data "openstack_compute_instance_v2"`.
    - allocate `openstack_networking_floatingip_v2`.
    - associate with `openstack_compute_floatingip_associate_v2`.
  - Add Talos API security group rules for control-plane/master/worker groups using `remote_ip_prefix = each.value.cidr`, `port_range_min/max = local.talos_api_port`, protocol `tcp`.
  - Render inventory:
    - `cluster.endpoint` on `443`
    - `talos_api_ip` from management floating IP address
    - `internal_ip` from module node `access_ip_v4`
    - control-plane cert SANs include Kubernetes API IP, management FIP, and internal IP.
- Tests:
  - Extend `internal/gitops/talos_render_test.go` to assert:
    - no `:6443`
    - `k8s_api_port = 443`
    - `talos_management_cidrs`
    - floating IP resources
    - security group rule resource with `remote_ip_prefix`
    - inventory uses floating IP for `talos_api_ip` and `access_ip_v4` for `internal_ip`.
- Verify:
  - `go test ./internal/gitops`

### Task 5: Bootstrap, Status, Docs

- Update Talos bootstrap provider preflight:
  - Validate `management_cidrs` before OpenTofu runs.
  - Dry-run plan notes should mention Talos management CIDRs and Kubernetes API `443`.
- Update status fixtures and behavior:
  - Existing Talos status fixtures must use inventory endpoint `https://...:443`.
  - Status refresh should pass management endpoints from inventory helpers.
- Update docs:
  - `docs/how-to/create-talos-cluster-openstack.md` should show Kubernetes API `443`, `management_cidrs`, per-node Talos management IPs, and no VIP forwarding of `50000`.
- Tests:
  - `go test ./internal/cluster ./cmd`
  - Add/update status tests for endpoint `443` and node management endpoint display.

### Task 6: Full Verification And Commit

- Run focused tests first:
  - `go test ./internal/config/v2 ./internal/config/v2schema`
  - `go test ./internal/deployment/talos`
  - `go test ./internal/gitops`
  - `go test ./internal/cluster ./cmd`
- Run broad suite:
  - `go test ./...`
- Inspect generated schema and docs diff:
  - `git diff -- schema/opencenter-v2.schema.json docs/how-to/create-talos-cluster-openstack.md`
- Commit in logical chunks:
  - `feat: add talos openstack management cidrs`
  - `feat: render talos external management access`
  - `fix: use direct talos node management clients`
  - `docs: update talos openstack external access guide`

## Assumptions And Defaults

- No automatic operator public IP discovery in v1; users must set `management_cidrs`.
- CIDRs are IPv4 only for this implementation.
- Talos OpenStack Kubernetes API port is always `443`; custom API ports are out of scope.
- The implementation must preserve existing unstaged user changes in the worktree.
- If OpenStack cannot allocate or associate per-node floating IPs, `opentofu-apply` should fail clearly; there is no silent fallback to private IPs for external-first deploys.
