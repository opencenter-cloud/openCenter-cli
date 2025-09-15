Kubespray Provider Configuration Guide

We use OpenTofu to deploy virtual machines on OpenStack, using the outputs of the infra module, we generate the Kubespray YAML manifests, then run the Kubespray playbooks to deploy Kubernetes on the virtual machines.

**NOTE:** These steps will change once automation is built.

The terraform state file will be stored in an S3 bucket

This will configure a Ubuntu 24.04 Linux VM as a deployment node that will be used to bootstrap and deploy openCenter clusters. The VM can go away once the resulting files have been committed to a code repository.
The deployment node could be your laptop or an existing Linux VM. 

- [openCenter Deployment Guide](#opencenter-deployment-guide)
  - [Pre Requisites](#pre-requisites)
    - [Packages](#packages)
      - [Add local binaries to PATH](#add-local-binaries-to-path)
    - [OpenTofu Requirements](#opentofu-requirements)
      - [Create S3 Bucket](#create-s3-bucket)
      - [Create access policy for Bucket](#create-access-policy-for-bucket)
      - [Create AWS User](#create-aws-user)
  - [GitOps Workflow](#gitops-workflow)
    - [Initialize the new cluster OpenTofu files](#initialize-the-new-cluster-opentofu-files)
    - [Configure the OpenTofu files](#configure-the-opentofu-files)
    - [Create application credentials](#create-application-credentials)
    - [Export credentials](#export-credentials)
      - [Option 1 - Application credentials](#option-1---application-credentials)
      - [Option 2 - Username and API Key](#option-2---username-and-api-key)
  - [Deploy Cluster](#deploy-cluster)
  - [Use the cluster](#use-the-cluster)
    - [Kubeconfig](#kubeconfig)
    - [Ansible](#ansible)
  - [Post Deployment Steps](#post-deployment-steps)
    - [Deploy a CNI](#deploy-a-cni)
    - [Complete the Hardening by deploying CSR Approver](#complete-the-hardening-by-deploying-csr-approver)
    - [Bootstrap Flux](#bootstrap-flux)
      - [Steps](#steps)
    - [Commit to Git](#commit-to-git)
  - [Save the SSH Keys to PasswordSafe](#save-the-ssh-keys-to-passwordsafe)
  - [Save the Kubeconfig file in PasswordSsafe](#save-the-kubeconfig-file-in-passwordssafe)
- [Outcome](#outcome)
  - [Virtual Machines](#virtual-machines)
  - [Kubernetes Cluster](#kubernetes-cluster)
- [Infra Module Configuration Options](#infra-module-configuration-options)
    - [openCenter](#opencenter)
    - [gitops](#gitops)
    - [Kubernetes](#kubernetes)
      - [kubespray](#kubespray)
      - [calico](#calico)
      - [cilium](#cilium)
      - [kube-ovn](#kube-ovn)
      - [kube-ovn + celium](#kube-ovn--celium)
    - [infra module](#infra-module)
- [To Do's:](#to-dos)

# openCenter Deployment Guide

## Pre Requisites

### Packages
- Python >=3.10 (Already in Ubuntu 24.04)
- python3.10-venv
- Terraform >=v1.11.1
- kubectl
- unzip
- make

We create a `.bin` directory within each cluster directory that will hold the binaries that are compatible with the current cluster version. This allows to have different clusters at different release versions.

#### Add local binaries to PATH
In order to make it easier to run the local binaries you can add them to your path in the current shell session by running this from the cluster directory.

```
export BIN=${PWD}/.bin
export PATH=${BIN}:${PATH}

```

### OpenTofu Requirements

#### Create S3 Bucket
The S3 bucket will be used to store the OpenTofu state file remotely.

- Give it a unique name
- Enable `Block all public access`
- Enable Bucket Versioning
- Add Tags to know if this is important or not: `production` or `dev`
- Default Encryption: Server-side encryption with Amazon S3 managed keys

#### Create access policy for Bucket

- In the Resource URN replace BUCKET_NAME with the name of the S3 bucket from the previous step
- This policy will allow a single account to access the OpenTofu state file of multiple clusters by allowing a directory structure:
```
├── BUCKET_NAME
│   ├── CLUSTER_NAME
│   │   ├── tfstate
│   │   |	└── terraform.tfstate
│   │   |	└── terraform.tfstate.tflock
```

IAM Policy:

``` json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": "s3:ListBucket",
            "Resource": "arn:aws:s3:::BUCKET_NAME"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject"
            ],
            "Resource": [
                "arn:aws:s3:::BUCKET_NAME/*/tfstate/terraform.tfstate"
            ]
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject"
            ],
            "Resource": [
                "arn:aws:s3:::BUCKET_NAME/*/tfstate/terraform.tfstate.tflock"
            ]
        }
    ]
}
```


#### Create AWS User
- Give it a clear name like "customer name".
- Leave console access unchecked.
- Attach policies directly and pick the policy created above.
- Take note of the AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY of the new user.

## GitOps Workflow

The starting point is to copy the init directory into the new clusters directory

```

├── infrastructure
│   └── init
│       ├── main.tf
│       ├── provider.tf
│       └── variables.tf
│       └── Makefile
│   └── clusters
│       ├── demo-cluster
│       └── production
```


### Initialize the new cluster OpenTofu files

**NOTE:** As of time of writing: The init files are expected to be in customer repo, where we really want them in the openCenter code repo. So for now you are going to have to copy the base terraform files manually from somewhere else or another cluster.

```
# cd /etc/openCenter
# cp -r infrastructure/init infrastructure/clusters/demo-cluster
# mkdir -p applications/overlays/demo-cluster/managed-services/calico/helm-values
# cd infrastructure/clusters/demo-cluster
```

### Configure the OpenTofu files

**provider.tf**

Update the BUCKET_NAME wiht the S3 bucket and CLUSTER_NAME with the unique cluster name.

```
terraform {
  backend "s3" {
    bucket       = "BUCKET_NAME"
    key          = "CLUSTER_NAME/tfstate/terraform.tfstate"
    region       = "us-west-2"
    use_lockfile = true
    encrypt      = true
  }
}
```

**main.tf**

**NOTE:** The main.tf in base is configured to work on Openstack Flex in SJC3 it will use Kube-VIP with a floating IP associated to expose the Kubernetes API publicly. When deploying in another cloud more settings will need to be updated.

These are the minimum required changes;  further configuration options are documented later in this document

Replace the cluster and Tenant names accordignly.
Pick a CIDR that is available for the servers or VMs and reokace it in the subnet_nodes.

```
locals {

  cluster_name                            = "CLUSTER_NAME"
  openstack_tenant_name                   = "TENANT_NAME"
  #CIDR that the openstack VMs will use for K8s nodes
  subnet_nodes                            = "10.2.188.0/22"
  # ==================================== 
  #Kubespray Settings
  kubespray_version                       = "v2.28.1"
  kubernetes_version                      = "1.32.5"
}
```

### Create application credentials
Click on UserCenter > Application Credentials > Create Application Credentials
- Set a unique name
- Set an expiration. Empty doesnt expire.
- Roles
-  - load-balancer_member
- - reader
- - member
- - network_member
- - heat_stack_user
- - creator
Leave Unrestricted unchecked.
Add a Description with the purpose of the credentials.

This will download a json file with credentials:
```json
{
  "id": "f07a1d5ff5254c0b9cda848353169891",
  "description": null,
  "project_id": "4c07654c099f59021ac0166a84648742",
  "expires_at": null,
  "secret": "REDACTED"
}
```

### Export credentials

#### Option 1 - Application credentials
Export the openstack user and S3 credentials
```

export TF_VAR_application_credential_id='longidstring'
export TF_VAR_application_credential_secret='securesecret'
export AWS_ACCESS_KEY_ID=<KEY>
export AWS_SECRET_ACCESS_KEY=<KEY>
```

#### Option 2 - Username and API Key

Export the openstack user and S3 credentials
```

export TF_VAR_openstack_user_password='api-key'
export TF_VAR_openstack_user_name='fanatiguy@rackspace.com'
export AWS_ACCESS_KEY_ID=<KEY>
export AWS_SECRET_ACCESS_KEY=<KEY>
```


## Deploy Cluster

```
# terraform init
```
The terraform init needs to access modules in git which can be done with SSH keys or a Git Token.
If you want to use the SSH Key method each module source will use: `git@github.com:rackerlabs/openCenter.git`
For Token `github.com/rackerlabs/openCenter.git`

If the init succeeds you are good to apply

```
# terraform apply
```

## Use the cluster

### Kubeconfig
A Kubeconfig file will be copied to the local cluster directory during the OpenTofu apply to provide access to the Kubernetes API.
```
export KUBECONFIG=${PWD}/kubeconfig.yaml

kubectl get nodes
```
### Ansible

An ansible inventory file is created in the path `CLUSTER_DIR/inventory/inventory.yaml` that is pre-configured to use the bastion server allowing secure access into the virtual machine servers.



## Post Deployment Steps
### Deploy a CNI

We deploy kubespray without a CNI to allow for the option of deploying any of the supported CNIs.

```
# helm repo add projectcalico https://docs.tigera.io/calico/charts
# helm upgrade --install calico projectcalico/tigera-operator --namespace tigera-operator -f ../../../applications/overlays/dev-cluster/services/calico/helm-values/override_values.yaml --create-namespace
```

### Complete the Hardening by deploying CSR Approver

Part of the hardening configuration is to allow Kubelet to renew its certificates. To allow for the automatic renewal we need to deploy the `kubelet-csr-approver` by setting the hardening value `kubelet_rotate_server_certificates` to `true` in the cluster's `main.tf`

**NOTE:** If the kubelet_rotate_server_certificates is true and the cluster doesnt have a CNI installed, the kubespray ansible playbook run will fail to deploy the `kubelet-csr-approver` helm chart.

Apply terraform to have it update the hardeninig yaml template file.
```
# terraform apply

```
Then

```
export ANSIBLE_INVENTORY=${PWD}/inventory/inventory.yaml
source venv/bin/activate
cd kubespray
ansible-playbook -f 10 -b upgrade-cluster.yml -e "@../inventory/k8s_hardening.yml"

```

### Bootstrap Flux

#### Steps
- A Git repository based on the openCenter-gitops-template.git
- An SSH Key with Read permissions to the repository as deploy keys. Stored in PasswordSafe
- Add public key as a deploy key to the repository
- export KUBECONFIG variable
- Install Flux curl flux.sh | kubectl apply -f -
- Run the flux boostrap git command to initialize the repository using the ssh key
- 

### Commit to Git
We want to make sure we commit and exclude the correct files to the repository

Files need encryption. Generated from the cluster hardening and CA certs.

`inventory/credentials/kube_encrypt_token.creds`

`inventory/credentials/kubeadm_certificate_key.creds`

There is a fix in place so that if we run cluster operations without the files present in the directory structure, the playbook will grab a copy from the masters to avoid creating a new one and drift. But we still need a process where we pull the files from a store and place them in the directory structure before making changes in the cluster

https://github.com/kubernetes-sigs/kubespray/pull/4255

## Save the SSH Keys to PasswordSafe
The files `id_rsa` and `id_rsa.pub` need to be saved to the customer's password safe in https://passwordsafe.corp.rackspace.com/projects/32616

## Save the Kubeconfig file in PasswordSsafe
The file `kubeconfig.yaml` needs to be saved to the customer's password safe in https://passwordsafe.corp.rackspace.com/projects/32616



# Outcome

## Virtual Machines
- Bastion Server
- Control Plane Servers
- Wroker Node Servers

## Kubernetes Cluster

```
# kubectl get nodes
NAME               STATUS   ROLES           AGE   VERSION
demo-cluster-cp0   Ready    control-plane   17h   v1.31.4
demo-cluster-cp1   Ready    control-plane   17h   v1.31.4
demo-cluster-cp2   Ready    control-plane   17h   v1.31.4
demo-cluster-wn0   Ready    <none>          17h   v1.31.4
demo-cluster-wn1   Ready    <none>          17h   v1.31.4
```

# Infra Module Configuration Options

### openCenter
| key | type | default | description |
| --- | --- | --- | --- |
| cluster_name | string | ""  | sets the name of the cluster, openstack project and user. |
| statsu | string | "config"  | sets the name of the cluster, openstack project and user. |

### gitops
| key | type | default | description |
| --- | --- | --- | --- |
| kube | string | ""  | sets the name of the cluster, openstack project and user. |
| statsu | string | "config"  | sets the name of the cluster, openstack project and user. |

### Kubernetes
#### kubespray
| key | type | default | description |
| --- | --- | --- | --- |
| kube | string | ""  | sets the name of the cluster, openstack project and user. |

####  calico
| key | type | default | description |
| --- | --- | --- | --- |
|  |  | ""  | |

####  cilium
| key | type | default | description |
| --- | --- | --- | --- |
|  |  | ""  | |

####  kube-ovn
| key | type | default | description |
| --- | --- | --- | --- |
|  |  | ""  | |

####  kube-ovn + celium
| key | type | default | description |
| --- | --- | --- | --- |
|  |  | ""  | |

### infra module
| key | type | default | description |
| --- | --- | --- | --- |
| cluster_name | string | ""  | sets the name of the cluster, openstack project and user. |
| naming_prefix | string | ""  | Prefix to add to Resource Names |
| openstack_auth_url | string | ""  | Openstack Keystone Endpoint |
| openstack_insecure | bool | false | Trust self-signed SSL certificates. |
| openstack_region | string | "RegionOne" | The region of the OpenStack cloud to use. |
| openstack_user_name | string | ""  | The Openstack Username to create. |
| openstack_user_password | string | ""  | The password to set on the Openstack Username. |
| openstack_admin_password | string | ""  | The password of the Openstack Administrator account. |
| openstack_project_domain_name | string | ""  | The openstack project domain name. |
| openstack_user_domain_name | string | ""  | The openstack user domain name. |
| openstack_tenant_name | string | ""  | The openstack tenant name if it already exists. |
| availability_zone | string | ""  | OpenStack availability zone for resource placement |
| floatingip_pool | string | ""  | The name of the floating IP pool to use for external access. |
| router_external_network_id | string | ""  | The UUID of the openstack network to attach to the router for external access. |
| disable_bastion | bool | false | To disable the bastion set it true. Will open port 22 on nodes |
| vlan_id | string | ""  | If set, it will create a VLAN network for the node network. |
| vlan_mtu | string | "1440" | MTU for the VLAN. If VxLAN it will use the environment's default. |
| network_provider | string | ""  | Network provider for the VLAN network interface. |
| subnet_nodes | string | "10.0.0.0/16" | CIDR for Openstack Network for nodes. |
| allocation_pool_start | string | ""  | Start IP of the DHCP allocation IPs of the subnet_nodes network. |
| allocation_pool_end | string | ""  | End IP of the DHCP allocation IPs of the subnet_nodes network. |
| vrrp_ip | string | ""  | Must be an IP from subnet_nodes and will be used as the internal Kubernetes API VIP. |
| subnet_services | string | "10.43.0.0/16" | CIDR to use for Kubernetes services. |
| subnet_pods | string | "10.42.0.0/16" | CIDR to use for Kubernetes pods. |
| use_octavia | bool | true | Use Octavia Load Balancer for Kubernetes API. If False, the vrrp_ip will se used with keepalived. |
| loadbalancer_provider | string | "amphora" | Openstack Octavia loadbalancer provider. |
| vrrp_enabled | bool | ""  | Will use vrrp_ip as the vip to be used with kube-vip. cannot be set to true if use_octavia is true |
| use_designate | bool | true | Creates a DNS record using the LB floating IP and dns_zone_name |
| dns_zone_name | string | ""  | dns_zone_name is the dns zone to create if use_designate is true. The k8s.dns_zone_name record will be added to the Kubernetes api SSL cert. |
| dns_nameservers | list(string) | \["8.8.8.8", "8.8.4.4"\] | DNS servers to configure on the nodes |
| image_id | string | ""  | Glance ImageID for the node Operating System |
| image_id_windows | string | ""  | Glance ImageID for the node Windows Operating System |
| worker_count | number | 0   | Number of worker node VMs to build |
| worker_count_windows | number | 0   | Number of Windows worker node VMs to build |
| master_count | number | 0   | Number of master node VMs to build |
| node_master | string | "master" | Define the role to customize the hostname for eg. the default role is master |
| node_worker | string | "worker" | Define the role to customize the hostname for eg. the default role is worker |
| node_windows | string | "win_wn" | Define the role to customize the hostname for eg. the default role is windows |
| master_node_bfv_size | number | 100 | boot from volume size for the master nodes |
| master_node_bfv_type | string | local | boot from volume type for the master nodes |
| worker_node_bfv_size | number | 100 | boot from volume size for the worker nodes |
| worker_node_bfv_type | string | local | boot from volume type for the worker nodes |
| ssh_user | string | "ubuntu" | Username with SSH access |
| openstack_ca | string | ""  | Signing CA certificate for TLS Openstack Endpoints |
| ca_certificates | string | ""  | Certificates to add to the node OS trusts |
| k8s_api_port | number | 443 | Port number for Kubernetes API server |
| flavor_bastion | string | ""  | OpenStack flavor for bastion host instances |
| flavor_master | string | ""  | OpenStack flavor for master node instances |
| flavor_worker | string | ""  | OpenStack flavor for worker node instances |
| flavor_worker_windows | string | ""  | OpenStack flavor for Windows worker node instances |
| ssh_authorized_keys | list(string) | \[\] | List of SSH public keys for cluster access |
| ub_version | string | "20" | Ubuntu version to use for nodes |
| windows_user | string | "Administrator" | Username for Windows nodes |
| windows_admin_password | string | ""  | Administrator password for Windows nodes |
| worker_node_bfv_size_windows | number | 0   | Boot from volume size for Windows worker nodes |
| worker_node_bfv_type_windows | string | "local" | Boot from volume type for Windows worker nodes |

# To Do's:
- Add support for app credentials auth
- Document how to switch remote state teraform between S3 and Local
- Review Git Tokens as a method of giving access to customer repo to Flux.
- Document upgrade process
- Add Windows nodes to cluster
