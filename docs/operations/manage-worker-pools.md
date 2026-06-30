# Manage Worker Pools

> **Purpose:** For operators, shows how to add, scale, and safely remove worker pools (Linux and Windows) using CLI commands.

## Prerequisites

- Existing openCenter cluster configuration
- openCenter CLI installed
- Infrastructure provider access (OpenStack)

## Add a Worker Pool

### Linux pool

```bash
opencenter cluster pool add gpu-pool \
  --count=2 \
  --flavor=gpu.0.4.16 \
  --boot-volume-size=200 \
  --boot-volume-type=HA-Performance \
  --label=workload-type=ml \
  --taint=nvidia.com/gpu=true:NoSchedule
```

### Windows pool

```bash
opencenter cluster pool add win-dotnet \
  --os=windows \
  --count=3 \
  --flavor=gp.0.4.16 \
  --boot-volume-size=100
```

After adding, apply to infrastructure:

```bash
opencenter cluster generate my-cluster
opencenter cluster deploy my-cluster
```

## Scale a Worker Pool

```bash
# Scale up
opencenter cluster pool scale gpu-pool --count=5

# Scale down
opencenter cluster pool scale gpu-pool --count=1

# Apply
opencenter cluster generate my-cluster
opencenter cluster deploy my-cluster
```

## Remove a Worker Pool

Removal requires a scale-to-zero workflow to prevent orphaned infrastructure:

```bash
# Step 1: Scale to zero (marks pool for decommission)
opencenter cluster pool scale gpu-pool --count=0

# Step 2: Apply infrastructure changes (deletes VMs)
opencenter cluster generate my-cluster
opencenter cluster deploy my-cluster

# Step 3: Remove pool definition from config
opencenter cluster pool remove gpu-pool
```

Attempting `pool remove` on a pool with `count > 0` fails with guidance:

```
Error: pool "gpu-pool" has count=2. Scale to zero first to decommission nodes:
  opencenter cluster pool scale gpu-pool --count=0
  opencenter cluster generate
  opencenter cluster deploy
Then re-run: opencenter cluster pool remove gpu-pool
```

Use `--force` to bypass (use only when infrastructure is already cleaned up).

## List Worker Pools

```bash
opencenter cluster pool list
```

Output:

```
NAME                 TYPE       COUNT   FLAVOR               VOLUME     STATUS
default              linux      3       gp.0.4.16            40GB       active
gpu-pool             linux      2       gpu.0.4.16           200GB      active
win-dotnet           windows    0       gp.0.4.16            100GB      draining
```

JSON/YAML output:

```bash
opencenter cluster pool list --output=json
opencenter cluster pool list --output=yaml
```

## Update a Worker Pool

Update flavor, image, or boot volume without changing count:

```bash
opencenter cluster pool update gpu-pool --flavor=gpu.0.8.32 --boot-volume-size=500
```

## Configuration Reference

Worker pools are defined under `opencenter.infrastructure.compute`:

```yaml
opencenter:
  infrastructure:
    compute:
      # Default Linux workers
      worker_count: 3
      flavor_worker: gp.0.4.16

      # Additional Linux pools
      additional_server_pools_worker:
        - name: gpu-pool
          count: 2
          flavor: gpu.0.4.16
          boot_volume:
            size: 200
            type: HA-Performance
          labels:
            workload-type: ml
          taints:
            - key: nvidia.com/gpu
              value: "true"
              effect: NoSchedule

      # Additional Windows pools
      additional_server_pools_worker_windows:
        - name: win-dotnet
          count: 3
          flavor: gp.0.4.16
          image: win-2022-img
          boot_volume:
            size: 100
            type: HA-Standard
          server_group_affinity: anti-affinity
```

## Validation Rules

| Rule | Description |
|------|-------------|
| Unique pool names | Pool names must be unique across Linux and Windows pools |
| Windows image required | `image_id_windows` must be set when any Windows pool has count > 0 without a per-pool image |
| Scale-to-zero for removal | `pool remove` requires count=0 unless `--force` is used |

## Troubleshooting

| Problem | Solution |
|---------|----------|
| "duplicate pool name" error | Pool names must be unique across Linux and Windows pools |
| "image_id_windows required" | Set the Windows image in `infrastructure.cloud.openstack.image_id_windows` or per-pool `image` |
| Pool remove fails | Scale to zero, generate, deploy, then remove |
| Nodes not joining after scale-up | Run `opencenter cluster status --refresh` to check API readiness |
