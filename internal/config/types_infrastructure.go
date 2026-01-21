package config

// Infrastructure represents the infrastructure configuration block.
type Infrastructure struct {
	Provider            string         `yaml:"provider" json:"provider"`
	Cloud               CloudConfig    `yaml:"cloud" json:"cloud"`
	SSHUser             string         `yaml:"ssh_user" json:"ssh_user"`
	OSVersion           string         `yaml:"os_version" json:"os_version"`
	ServerGroupAffinity []string       `yaml:"server_group_affinity" json:"server_group_affinity"`
	NodeNaming          NodeNaming     `yaml:"node_naming" json:"node_naming"`
	Bastion             BastionConfig  `yaml:"bastion" json:"bastion"`
	K8sAPIIP            string         `yaml:"k8s_api_ip" json:"k8s_api_ip"`
}

// NodeNaming represents node naming conventions
type NodeNaming struct {
	Worker        string `yaml:"worker" json:"worker"`
	Master        string `yaml:"master" json:"master"`
	WorkerWindows string `yaml:"worker_windows" json:"worker_windows"`
}

// BastionConfig represents bastion host configuration
type BastionConfig struct {
	Address string `yaml:"address" json:"address"`
}

// CloudConfig represents the cloud configuration within opencenter
type CloudConfig struct {
	AWS       SimplifiedAWSCloud       `yaml:"aws" json:"aws"`
	OpenStack SimplifiedOpenStackCloud `yaml:"openstack" json:"openstack"`
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
