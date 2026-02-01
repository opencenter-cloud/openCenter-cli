package config

// ClusterConfig represents the cluster configuration section
type ClusterConfig struct {
	ClusterName        string                  `yaml:"cluster_name" json:"cluster_name" jsonschema:"description=Name of the cluster" validate:"required,dns1123"`
	AWSAccessKey       string                  `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string                  `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
	SSHAuthorizedKeys  []string                `yaml:"ssh_authorized_keys" json:"ssh_authorized_keys" validate:"required,min=1,dive,required"`
	Kubernetes         KubernetesConfig        `yaml:"kubernetes" json:"kubernetes" validate:"required"`
	Networking         ClusterNetworkingConfig `yaml:"networking" json:"networking" validate:"required"`

	// New fields for configuration-driven templates
	BaseDomain  string `yaml:"base_domain,omitempty" json:"base_domain,omitempty" jsonschema:"description=Base domain for the cluster (e.g. k8s.opencenter.cloud)" validate:"required,fqdn"`
	ClusterFQDN string `yaml:"cluster_fqdn,omitempty" json:"cluster_fqdn,omitempty" jsonschema:"description=Fully qualified domain name for the cluster" validate:"required,fqdn"`
	AdminEmail  string `yaml:"admin_email,omitempty" json:"admin_email,omitempty" jsonschema:"description=Administrator email address for certificates and notifications" validate:"omitempty,email"`
}

// ClusterNetworkingConfig represents cluster-level networking configuration
type ClusterNetworkingConfig struct {
	NTPServers     []string              `yaml:"ntp_servers" json:"ntp_servers" validate:"required,min=1,dive,fqdn|ipv4"`
	DNSNameservers []string              `yaml:"dns_nameservers" json:"dns_nameservers" validate:"required,min=1,dive,ipv4"`
	Security       ClusterSecurityConfig `yaml:"security" json:"security"`

	// Network topology settings
	SubnetNodes         string `yaml:"subnet_nodes,omitempty" json:"subnet_nodes,omitempty" jsonschema:"description=CIDR block for Kubernetes node network" validate:"required,cidrv4"`
	AllocationPoolStart string `yaml:"allocation_pool_start,omitempty" json:"allocation_pool_start,omitempty" jsonschema:"description=Start IP for DHCP allocation pool" validate:"omitempty,ipv4"`
	AllocationPoolEnd   string `yaml:"allocation_pool_end,omitempty" json:"allocation_pool_end,omitempty" jsonschema:"description=End IP for DHCP allocation pool" validate:"omitempty,ipv4"`

	// VRRP settings
	VRRPIP      string `yaml:"vrrp_ip,omitempty" json:"vrrp_ip,omitempty" jsonschema:"description=Virtual IP for VRRP (Kubernetes API VIP)" validate:"required_if=VRRPEnabled true,omitempty,ipv4"`
	VRRPEnabled bool   `yaml:"vrrp_enabled" json:"vrrp_enabled" jsonschema:"description=Enable VRRP for HA,default=true"`

	// Load balancer settings
	UseOctavia           bool   `yaml:"use_octavia" json:"use_octavia" jsonschema:"description=Use Octavia load balancer instead of floating IP,default=false"`
	LoadbalancerProvider string `yaml:"loadbalancer_provider,omitempty" json:"loadbalancer_provider,omitempty" jsonschema:"description=Load balancer provider (ovn/octavia/metallb),default=ovn" validate:"required,oneof=ovn octavia metallb cloud-native"`

	// DNS settings
	UseDesignate bool   `yaml:"use_designate" json:"use_designate" jsonschema:"description=Use OpenStack Designate for DNS,default=false"`
	DNSZoneName  string `yaml:"dns_zone_name,omitempty" json:"dns_zone_name,omitempty" jsonschema:"description=DNS zone name for Designate" validate:"omitempty,fqdn"`

	// VLAN settings
	VLAN VLAN `yaml:"vlan,omitempty" json:"vlan,omitempty"`
}

// VLAN describes VLAN settings for the cluster
type VLAN struct {
	ID       string `yaml:"id,omitempty" json:"id,omitempty" jsonschema:"description=VLAN ID"`
	MTU      int    `yaml:"mtu,omitempty" json:"mtu,omitempty" jsonschema:"description=MTU size for VLAN,default=1500" validate:"omitempty,min=68,max=9000"`
	Provider string `yaml:"provider,omitempty" json:"provider,omitempty" jsonschema:"description=Network provider,default=physnet1"`
}
