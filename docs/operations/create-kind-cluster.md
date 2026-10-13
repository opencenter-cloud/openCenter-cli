---
id: create-kind-cluster
title: Create a Kind cluster
sidebar_label: Create a Kind cluster
description: Create and deploy a Kind cluster through the openCenter command path.
doc_type: how-to
audience: openCenter operators
tags: [kind, clusters]
last_updated: 2026-09-25
---
# Create a Kind cluster

This is the short command path for a local Kind configuration. It intentionally
does not claim a fixed node count, service set, host IP, or bootstrap duration;
those values come from the generated configuration and local runtime.

```bash
opencenter cluster init local-demo --org local --type kind
opencenter cluster validate local/local-demo
opencenter cluster generate local/local-demo
opencenter cluster deploy local/local-demo --container-runtime docker
```

Use `--container-runtime podman` when Podman is the selected runtime. The flag
is accepted only for the Kind deployment path.

## Inspect and resume

```bash
opencenter cluster describe local/local-demo
opencenter cluster deploy local/local-demo --dry-run
opencenter cluster deploy local/local-demo --restart
opencenter cluster deploy local/local-demo --from-step STEP_ID
```

After deployment has produced a kubeconfig, use its reported path for live
inspection with tools outside this CLI page.

## Access services from the host

On Docker Desktop for macOS, containers run in a Linux VM and the Docker
network used by Kind is not routable from the host. MetalLB addresses such as
`172.21.255.200` answer from inside a node container but time out from the
host. On Linux hosts the Kind subnet is routable and this section is not needed.

`kubectl port-forward` reaches a service without using the MetalLB address.
The command stays in the foreground and the forward ends when it exits.

```bash
kubectl port-forward -n NAMESPACE svc/SERVICE 8080:SERVICE_PORT
```

Find the namespace and port with `kubectl get svc -A`. The service is then
available at `http://localhost:8080`. For example, to reach Headlamp:

```bash
kubectl port-forward -n headlamp svc/headlamp 8080:80
```

To go through the gateway instead, forward the Envoy Gateway service and
send the route hostname with `--resolve`. Ports below 1024 need `sudo`.

```bash
kubectl get svc -n envoy-gateway-system
sudo kubectl port-forward -n envoy-gateway-system svc/ENVOY_SERVICE 443:443
curl -k --resolve app.example.test:443:127.0.0.1 https://app.example.test/
```

Browsers need a matching `/etc/hosts` entry that maps the hostname to
`127.0.0.1`. The `-k` flag is required while the gateway serves a certificate
that the host does not trust.

## Cleanup

```bash
opencenter cluster destroy local/local-demo --force
```

`--remove-files` additionally removes local configuration and GitOps files;
`--skip-infrastructure --remove-files` skips infrastructure destruction.

## Evidence

- Kind flags: `cmd/cluster_init.go`, `cmd/cluster_deploy.go`
- Destroy flags: `cmd/cluster_destroy.go`
- Kind lifecycle: `internal/cluster/kind_bootstrap_provider.go`
- Workflow limitation: `tests/features/workflow.feature:1-8`
