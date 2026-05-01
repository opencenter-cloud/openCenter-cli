# OpenStack Network Plugin Install Design

## Context

OpenStack deploy currently relies on OpenTofu to provision infrastructure and run the embedded Kubernetes bootstrap. For Helm-installed CNIs, the generated infrastructure template sets Kubespray's `network_plugin` to `none` so Kubespray does not install a CNI. That leaves an ordering gap: the Kubernetes API can exist, but node networking is incomplete until Calico, Cilium, or Kube-OVN is installed.

The cluster config already models the supported CNI choices under `opencenter.cluster.kubernetes.network_plugin`:

- `calico`
- `cilium`
- `kube-ovn`

The new deploy behavior applies only to OpenStack clusters. Kubespray must not install CNIs for this path.

## Goals

- Install exactly one enabled network plugin during OpenStack deploy.
- Support Calico, Cilium, and Kube-OVN.
- Enforce that only one network plugin is enabled in cluster validation.
- Keep Kubespray out of CNI installation.
- Support the two acceptable install mechanisms:
  - direct Helm CLI
  - Kustomize with Helm chart rendering enabled
- Preserve dry-run planning and resumable step behavior.
- Keep the implementation aligned with the existing provider bootstrap step model.

## Non-Goals

- Do not use Kubespray to install any CNI.
- Do not make Flux the first CNI installer before a working CNI exists.
- Do not redesign service rendering broadly.
- Do not add provider support outside OpenStack in this change.
- Do not support multiple active network plugins.

## Constraints

For OpenStack deploy, the enabled network plugin must install through Helm-backed machinery only. `install_method: kubespray` is invalid for OpenStack and should fail validation before deploy.

The existing user-facing network plugin key is `cilium`. The misspelled key `celium` is not accepted; it should continue to fail YAML/schema validation as an unknown field.

The existing `kube-ovn` YAML key should remain unchanged, matching the current v2 schema and config structs.

## Approaches Considered

### Recommended: Direct Helm Bootstrap Step

Add an OpenStack bootstrap step named `openstack-install-network-plugin` after `openstack-normalize-kubeconfig` and before global readiness polling. The step detects the single enabled network plugin and runs plugin-specific `helm upgrade --install` commands against the cluster-owned kubeconfig.

This is the recommended first implementation because it keeps CNI installation deterministic at the moment the cluster needs networking. It does not depend on Flux controllers becoming healthy before pod networking exists.

### Alternative: Kustomize With Helm

Generate a per-plugin Kustomize overlay that uses Helm chart rendering, then apply it with Kustomize Helm support enabled. This satisfies the acceptable install method constraint and keeps the install declarative, but it adds more toolchain and chart-rendering edge cases.

This should be supported as a second install backend, not the default first path.

### Rejected: Flux as the Initial CNI Installer

Flux can manage the steady-state manifests after the CNI exists, but using Flux as the initial network plugin installer is risky because Flux controllers and source reconciliation may not be healthy before pod networking is installed.

## Configuration Model

The selected plugin is derived from `opencenter.cluster.kubernetes.network_plugin`.

Selection rules:

- `calico.enabled: true` selects Calico.
- `cilium.enabled: true` selects Cilium.
- `kube-ovn.enabled: true` selects Kube-OVN.
- Zero enabled plugins is invalid.
- More than one enabled plugin is invalid.

Install method rules for OpenStack:

- Empty `install_method` means `helm`.
- `install_method: helm` uses direct Helm CLI.
- `install_method: kustomize-helm` uses Kustomize with Helm rendering.
- `install_method: kubespray` is invalid for OpenStack.

The implementation should update v2 schema and struct validation so the OpenStack-supported values are `helm` and `kustomize-helm`. Existing OpenStack configs that set `install_method: kubespray` should fail validation with a migration message to use `helm` or `kustomize-helm`.

## Validation

Validation belongs in the v2 readiness/business-rule path used by `opencenter cluster validate` and deploy configuration loading.

Validation should add clear errors for:

- no network plugin enabled
- more than one network plugin enabled
- OpenStack network plugin using `install_method: kubespray`
- unsupported install method

Example error paths:

- `opencenter.cluster.kubernetes.network_plugin`
- `opencenter.cluster.kubernetes.network_plugin.calico.install_method`
- `opencenter.cluster.kubernetes.network_plugin.cilium.install_method`
- `opencenter.cluster.kubernetes.network_plugin.kube-ovn.install_method`

The error should name the enabled plugins and the accepted methods: `helm` and `kustomize-helm`.

## Deploy Flow

OpenStack bootstrap steps become:

1. `openstack-preflight`
2. `opentofu-init`
3. `opentofu-apply`
4. `openstack-normalize-kubeconfig`
5. `openstack-install-network-plugin`

The disabled Kubespray step block remains disabled. The generated infrastructure config should continue setting Kubespray `network_plugin = "none"` for Helm-backed CNI installation.

`openstack-install-network-plugin` should:

- resolve the enabled plugin from config
- resolve install method, defaulting to `helm`
- require `opts.KubeconfigPath`
- run the selected installer
- wait for plugin-specific rollout or pods to become ready
- write progress and command output through the existing bootstrap log path
- participate in saved-state resume behavior like existing steps

Dry-run output should include the planned CNI install step and commands. `--step openstack-install-network-plugin` and `--from-step openstack-install-network-plugin` should work through the existing filter logic.

## Helm Installer

The direct Helm backend should use `helm upgrade --install` with explicit namespaces and `--create-namespace` where appropriate.

Calico:

- repository: `https://docs.tigera.io/calico/charts`
- chart: `tigera-operator`
- namespace: `tigera-operator`
- values should include the pod CIDR and service CIDR from cluster config.
- readiness should check the Tigera operator deployment and Calico pods/namespaces.

Cilium:

- repository/chart: `oci://ghcr.io/cilium/charts/cilium`
- namespace: `kube-system`
- values should include cluster pod CIDR, service CIDR, tunnel mode where configured, Hubble where configured, and kube-proxy replacement behavior where configured.
- readiness should check Cilium DaemonSet/Deployment status in `kube-system`.

Kube-OVN:

- repository/chart: `oci://ghcr.io/kubeovn/charts/kube-ovn-v2`
- namespace: `kube-system`, matching the chart default.
- values should include pod/service subnet values and network policy settings where supported.
- readiness should check Kube-OVN controller and node components.

Chart versions should come from the matching plugin config version when set. If no version is configured, the installer should use a conservative default aligned with the current defaults and docs.

## Kustomize-With-Helm Installer

The Kustomize backend should generate or reuse a small temporary overlay for the selected plugin and apply it with Helm rendering enabled.

Acceptable execution forms:

- `kubectl kustomize --enable-helm <overlay> | kubectl apply -f -`
- `kustomize build --enable-helm <overlay> | kubectl apply -f -`

The implementation should avoid ad hoc YAML string assembly where structured templates already exist. Temporary files should live under the bootstrap/runtime area or a safely scoped temp directory and be cleaned up after successful apply.

## GitOps Relationship

The bootstrap step owns making the initial CNI live. The first implementation should not attempt Flux adoption of the CNI release. It should document CNI bootstrap as an imperative deploy step and avoid adding new Flux ownership for Cilium or Kube-OVN in the same change.

For Calico, existing GitOps service templates can remain in place, but the implementation should avoid adding duplicate Calico release definitions during deploy. Cilium and Kube-OVN GitOps descriptors/templates are out of scope for this spec and should be handled by a later design if steady-state Flux ownership is required.

## Error Handling

The install step should fail fast when:

- no plugin is selected
- multiple plugins are selected
- kubeconfig is missing
- required tools are not on `PATH`
- Helm/Kustomize apply fails
- plugin readiness times out

Errors should identify the plugin and command that failed. The bootstrap state file should mark `openstack-install-network-plugin` failed so a rerun resumes after earlier successful infrastructure and kubeconfig steps.

Readiness checks should use bounded polling with clear timeout messaging. The global cluster readiness check should remain unchanged and run after CNI installation.

## Testing

Unit tests should cover:

- exactly one enabled plugin passes validation
- zero enabled plugins fails validation
- multiple enabled plugins fails validation and names the enabled plugins
- OpenStack with `install_method: kubespray` fails validation
- direct Helm produces the expected bootstrap step and command plan for Calico
- direct Helm produces the expected bootstrap step and command plan for Cilium
- direct Helm produces the expected bootstrap step and command plan for Kube-OVN
- dry-run plans include `openstack-install-network-plugin`
- `--step` and `--from-step` work with the new step
- saved-state resume skips completed CNI installation

Integration-style tests can use fake command runners to assert command order without contacting a cluster.

## Documentation

Update operator docs for:

- OpenStack deploy now installs the selected CNI after kubeconfig normalization.
- Kubespray does not install OpenStack CNIs.
- only one network plugin may be enabled.
- accepted install methods are `helm` and `kustomize-helm`.
- examples for Calico, Cilium, and Kube-OVN.

Update developer docs for the OpenStack deploy step sequence and dry-run output.
