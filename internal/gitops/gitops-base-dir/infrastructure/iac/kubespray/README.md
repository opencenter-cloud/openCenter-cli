# Kubespray

| Key | Type | Default| Description |
| --- | --- | --- | --- |
| address_bastion | string | "" | Public IP address of the bastion host for SSH access |
| baremetal_deployment | bool | false | No bastion will be used in inventory and it wont wait for cloud-init to complete |
| cert_manager_enabled | bool | false | Enable the built-in cert-manager in Kubespray |
| cluster_name | string | "" | Name of the Kubernetes cluster |
| cni_iface | string | "eth0" | Network interface for CNI plugin ("eth0") |
| deploy_cluster | bool | false | Whether to run the Kubespray playbooks and create the Kubernetes cluster (true) |
| dns_zone_name | string | "" | DNS name for the Kubernetes API ("k8s.cluster-name.demo.mk8s.net") |
| master_nodes | list(object) | List of objects with id, name and access_ip_v4 | Configuration object for master nodes |
| network_plugin | string | "none" | CNI network plugin to use ("calico"). Set to "none" to deploy the CNI separately |
| k8s_hardening_enabled | bool | false | Enable Kubernetes security hardening. Will include additional hardening manifest. |
| os_hardening_enabled | bool | false | Enable OS security hardening. Will run ansible-hardening playbook on the ansible group k8s_cluster |
| ssh_user | string | "ubuntu" | SSH username for node access |
| subnet_nodes | string | "" | CIDR for node network servers|
| subnet_pods | string | "10.42.0.0/16 | CIDR for pod network |
| subnet_services | string | "10.43.0.0/16" | CIDR for service network |
| kubernetes_version | string | "1.30.4" | Kubernetes version to deploy  |
| kubespray_version | string | "v2.28.1" | Kubespray version to use |
| kube_vip_enabled | bool | false | Enable kube-vip for HA on Kube API Server. Requires vrrp_enabled to true  |
| kube_pod_security_exemptions_namespaces | list(string) | [] | Namespaces exempt from pod security |
| worker_nodes | list(object) | List of objects with id, name and access_ip_v4 | Configuration object for worker nodes |
| k8s_api_ip | string | "" | External IP for Kubernetes API |
| k8s_api_port | number | 6443 | Port for Kubernetes API |
| vrrp_ip | string | "" | VRRP IP for high availability. Used for kube-vip and the internal IP of Octavia LB. Nodes will look for this IP when making requests to Kubernetes API server. |
| vrrp_enabled | bool | "false" | Enable the use of the vrrp_ip port without Octavia. |
| windows_nodes | list(object) | List of objects with id, name and access_ip_v4 | Configuration object for Windows worker nodes |
| use_octavia | bool | false | Use Octavia load balancer. Cannot be used if vrrp_enabled and kube_vip_enabled  |
| kube_oidc_auth_enabled | bool | false | Enable OIDC authentication |
| kube_oidc_url | string | "" | OIDC provider URL |
| kube_oidc_client_id | string | "kubernetes" | OIDC client ID |
| kube_oidc_ca_file | string | "/etc/kubernetes/ssl/ca.pem" | CA file for OIDC provider |
| kube_oidc_username_claim | string | "sub" | JWT claim for username |
| kube_oidc_username_prefix | string | 'oidc:' | Prefix for OIDC usernames |
| kube_oidc_groups_claim | string | "groups" | JWT claim for groups |
| kube_oidc_groups_prefix | string | 'oidc:' | Prefix for OIDC groups |