package config

// ClusterConfig represents the cluster configuration section
type ClusterConfig struct {
	ClusterName        string                  `yaml:"cluster_name" json:"cluster_name" jsonschema:"description=Name of the cluster"`
	AWSAccessKey       string                  `yaml:"aws_access_key" json:"aws_access_key"`
	AWSSecretAccessKey string                  `yaml:"aws_secret_access_key" json:"aws_secret_access_key"`
	SSHAuthorizedKeys  []string                `yaml:"ssh_authorized_keys" json:"ssh_authorized_keys"`
	Kubernetes         KubernetesConfig        `yaml:"kubernetes" json:"kubernetes"`
	Networking         ClusterNetworkingConfig `yaml:"networking" json:"networking"`

	// New fields for configuration-driven templates
	BaseDomain  string `yaml:"base_domain,omitempty" json:"base_domain,omitempty" jsonschema:"description=Base domain for the cluster (e.g. k8s.opencenter.cloud)"`
	ClusterFQDN string `yaml:"cluster_fqdn,omitempty" json:"cluster_fqdn,omitempty" jsonschema:"description=Fully qualified domain name for the cluster"`
	AdminEmail  string `yaml:"admin_email,omitempty" json:"admin_email,omitempty" jsonschema:"description=Administrator email address for certificates and notifications"`
}

// ClusterNetworkingConfig represents cluster-level networking configuration
type ClusterNetworkingConfig struct {
	K8sAPIPortACL  []string              `yaml:"k8s_api_port_acl" json:"k8s_api_port_acl"`
	NTPServers     []string              `yaml:"ntp_servers" json:"ntp_servers"`
	DNSNameservers []string              `yaml:"dns_nameservers" json:"dns_nameservers"`
	Security       ClusterSecurityConfig `yaml:"security" json:"security"`
}
