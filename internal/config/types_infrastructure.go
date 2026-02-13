package config

// Infrastructure represents the infrastructure configuration block.
type Infrastructure struct {
	Provider            string        `yaml:"provider" json:"provider" validate:"required,oneof=openstack aws gcp azure baremetal vsphere vmware"`
	Cloud               CloudConfig   `yaml:"cloud" json:"cloud" validate:"required"`
	SSHUser             string        `yaml:"ssh_user" json:"ssh_user" validate:"required"`
	SSHKeyPath          string        `yaml:"ssh_key_path,omitempty" json:"ssh_key_path,omitempty" jsonschema:"description=Path to SSH private key for cluster access"`
	OSVersion           string        `yaml:"os_version" json:"os_version" validate:"required"`
	ServerGroupAffinity []string      `yaml:"server_group_affinity" json:"server_group_affinity" validate:"dive,oneof=affinity anti-affinity soft-affinity soft-anti-affinity"`
	NodeNaming          NodeNaming    `yaml:"node_naming" json:"node_naming" validate:"required"`
	Bastion             BastionConfig `yaml:"bastion" json:"bastion" validate:"required"`
	K8sAPIIP            string        `yaml:"k8s_api_ip" json:"k8s_api_ip" validate:"omitempty,ipv4"`
}

// NodeNaming represents node naming conventions
type NodeNaming struct {
	Worker        string `yaml:"worker" json:"worker" validate:"required"`
	Master        string `yaml:"master" json:"master" validate:"required"`
	WorkerWindows string `yaml:"worker_windows" json:"worker_windows"`
}

// BastionConfig represents bastion host configuration
type BastionConfig struct {
	Address string `yaml:"address" json:"address" validate:"required,hostname|ipv4"`
}

// CloudConfig represents the cloud configuration within opencenter
type CloudConfig struct {
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
	VMware    VMwareCloud              `yaml:"vmware" json:"vmware"`
}

// Cloud holds provider-specific configuration. Currently, only OpenStack is supported.
type Cloud struct {
	Provider  string         `yaml:"provider" json:"provider"`
	OpenStack OpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       AWSCloud       `yaml:"aws" json:"aws"`
}

// OpenStackCloud contains options for connecting to an OpenStack deployment.
type OpenStackCloud struct {
	AuthURL                 string `yaml:"auth_url" json:"auth_url"`
	Insecure                bool   `yaml:"insecure" json:"insecure"`
	Region                  string `yaml:"region" json:"region"`
	UserName                string `yaml:"user_name" json:"user_name"`
	UserPassword            string `yaml:"user_password" json:"user_password"`
	AdminPassword           string `yaml:"admin_password" json:"admin_password"`
	ProjectDomainName       string `yaml:"project_domain_name" json:"project_domain_name"`
	UserDomainName          string `yaml:"user_domain_name" json:"user_domain_name"`
	TenantName              string `yaml:"tenant_name" json:"tenant_name"`
	AvailabilityZone        string `yaml:"availability_zone" json:"availability_zone"`
	FloatingIPPool          string `yaml:"floatingip_pool" json:"floatingip_pool"`
	RouterExternalNetworkID string `yaml:"router_external_network_id" json:"router_external_network_id"`
	DisableBastion          bool   `yaml:"disable_bastion" json:"disable_bastion"`
	CA                      string `yaml:"ca" json:"ca"`
	ExternalNetwork         string `yaml:"external_network" json:"external_network"`
	UseOctavia              bool   `yaml:"use_octavia" json:"use_octavia"`
	VRRPIP                  string `yaml:"vrrp_ip" json:"vrrp_ip"`
}

// AWSCloud contains options for connecting to AWS environments.
type AWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
}

// SimplifiedCloud represents the cloud section
type SimplifiedCloud struct {
	Provider  string                   `yaml:"provider" json:"provider"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
	VMware    VMwareCloud              `yaml:"vmware" json:"vmware"`
}

// SimplifiedOpenStackCloud represents the OpenStack configuration
type SimplifiedOpenStackCloud struct {
	AuthURL                     string                    `yaml:"auth_url" json:"auth_url"`
	Insecure                    bool                      `yaml:"insecure" json:"insecure"`
	Region                      string                    `yaml:"region" json:"region"`
	ApplicationCredentialID     string                    `yaml:"application_credential_id" json:"application_credential_id"`
	ApplicationCredentialSecret string                    `yaml:"application_credential_secret" json:"application_credential_secret"`
	Domain                      string                    `yaml:"domain" json:"domain"`
	TenantName                  string                    `yaml:"tenant_name" json:"tenant_name"`
	AvailabilityZone            string                    `yaml:"availability_zone" json:"availability_zone"`
	ProjectDomainName           string                    `yaml:"project_domain_name" json:"project_domain_name"`
	UserDomainName              string                    `yaml:"user_domain_name" json:"user_domain_name"`
	CA                          string                    `yaml:"ca" json:"ca"`
	ImageID                     string                    `yaml:"image_id" json:"image_id"`
	ImageIDWindows              string                    `yaml:"image_id_windows" json:"image_id_windows"`
	Networking                  OpenStackNetworkingConfig `yaml:"networking" json:"networking"`
	Modules                     OpenStackModulesConfig    `yaml:"modules" json:"modules"`
}

// OpenStackNetworkingConfig represents OpenStack networking configuration
type OpenStackNetworkingConfig struct {
	FloatingIPPool          string          `yaml:"floating_ip_pool" json:"floating_ip_pool"`
	FloatingNetworkId       string          `yaml:"floating_network_id" json:"floating_network_id"`
	NetworkID               string          `yaml:"network_id" json:"network_id"`
	RouterExternalNetworkID string          `yaml:"router_external_network_id" json:"router_external_network_id"`
	SubnetId                string          `yaml:"subnet_id" json:"subnet_id"`
	Designate               DesignateConfig `yaml:"designate" json:"designate"`
	VLAN                    VLAN            `yaml:"vlan" json:"vlan"`
	K8sAPIPortACL           []string        `yaml:"k8s_api_port_acl" json:"k8s_api_port_acl" jsonschema:"description=CIDR blocks allowed to access Kubernetes API server"`
}

// DesignateConfig represents OpenStack Designate DNS configuration
type DesignateConfig struct {
	DNSZoneName string `yaml:"dns_zone_name" json:"dns_zone_name"`
}

// SimplifiedAWSCloud represents the AWS configuration
type SimplifiedAWSCloud struct {
	Profile        string   `yaml:"profile" json:"profile"`
	Region         string   `yaml:"region" json:"region"`
	VPCID          string   `yaml:"vpc_id" json:"vpc_id"`
	PrivateSubnets []string `yaml:"private_subnets" json:"private_subnets"`
	PublicSubnets  []string `yaml:"public_subnets" json:"public_subnets"`
}

// OpenStackModulesConfig represents the OpenStack module configurations
type OpenStackModulesConfig struct {
	OpenstackNova OpenstackNovaModuleConfig `yaml:"openstack_nova" json:"openstack_nova"`
}

// OpenstackNovaModuleConfig represents the openstack-nova module configuration
type OpenstackNovaModuleConfig struct {
	Source string `yaml:"source" json:"source"`
}

// VMwareCloud contains options for VMware vSphere environments.
// VMware is treated as baremetal for deployment purposes - nodes must be pre-provisioned.
type VMwareCloud struct {
	VCenterServer string   `yaml:"vcenter_server" json:"vcenter_server" jsonschema:"description=vCenter server hostname or IP address"`
	Datacenter    string   `yaml:"datacenter" json:"datacenter" jsonschema:"description=VMware datacenter name"`
	Datastore     string   `yaml:"datastore" json:"datastore" jsonschema:"description=Default datastore for persistent volumes"`
	Cluster       string   `yaml:"cluster" json:"cluster" jsonschema:"description=VMware compute cluster name"`
	ResourcePool  string   `yaml:"resource_pool" json:"resource_pool" jsonschema:"description=Resource pool for VMs (optional)"`
	Folder        string   `yaml:"folder" json:"folder" jsonschema:"description=VM folder path (optional)"`
	Network       string   `yaml:"network" json:"network" jsonschema:"description=Network name for VMs"`
	Nodes         []VMNode `yaml:"nodes" json:"nodes" jsonschema:"description=Pre-provisioned VM nodes"`
}

// VMNode represents a pre-provisioned VMware VM node.
type VMNode struct {
	Name       string `yaml:"name" json:"name" validate:"required" jsonschema:"description=Node hostname"`
	IP         string `yaml:"ip" json:"ip" validate:"required,ipv4" jsonschema:"description=Node IP address"`
	Role       string `yaml:"role" json:"role" validate:"required,oneof=master worker" jsonschema:"description=Node role (master or worker)"`
	UUID       string `yaml:"uuid" json:"uuid" jsonschema:"description=VM UUID (optional)"`
	MACAddress string `yaml:"mac_address" json:"mac_address" jsonschema:"description=Primary network interface MAC address (optional)"`
}
