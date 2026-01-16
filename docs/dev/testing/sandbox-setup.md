# sandbox deployment

```
openCenter cluster init sandbox.opencenter.sjc3 \
  --org opencenter \
  --opencenter.meta.env=sandbox \
  --opencenter.meta.region=sjc3 \
  --opencenter.infrastructure.provider=openstack \
  --opencenter.infrastructure.cloud.openstack.auth_url="https://keystone.api.sjc3.rackspacecloud.com/v3/" \
  --opencenter.infrastructure.cloud.openstack.region=SJC3 \
  --opencenter.infrastructure.cloud.openstack.tenant_name="4c07654c099f59021ac0166a84648742" \
  --opencenter.infrastructure.cloud.openstack.domain="rackspace_cloud_domain" \
  --opencenter.cluster.kubernetes.version="1.32.8" \
  --opencenter.cluster.kubernetes.master_count=3 \
  --opencenter.cluster.kubernetes.flavor_master="gp.0.4.8" \
  --opencenter.cluster.kubernetes.worker_count=4 \
  --opencenter.cluster.kubernetes.flavor_worker="gp.0.4.16" \
  --opencenter.cluster.kubernetes.subnet_pods="10.42.0.0/16" \
  --opencenter.cluster.kubernetes.subnet_services="10.43.0.0/16" \
  --networking.subnet_nodes="10.2.128.0/22" \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.0.name=compute \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.0.worker_count=3 \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.0.flavor_worker=gp.0.4.16 \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.0.node_worker=-compute \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.1.name=gpu \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.1.worker_count=3 \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.1.flavor_worker=gpu.0.2.196 \
  --opencenter.cluster.kubernetes.additional_server_pools_worker.1.node_worker=-gpu
```

openCenter cluster init sandbox \
  --org rmpk.dev \
  --opencenter.meta.env=dev \
