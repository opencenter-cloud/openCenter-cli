package config

// Security represents security-related configuration
type Security struct {
	CACertificates        string   `yaml:"ca_certificates" json:"ca_certificates"`
	K8sHardening          bool     `yaml:"k8s_hardening" json:"k8s_hardening"`
	OSHardening           bool     `yaml:"os_hardening" json:"os_hardening"`
	PodSecurityExemptions []string `yaml:"pod_security_exemptions" json:"pod_security_exemptions"`
}

// ClusterSecurityConfig represents cluster-level security configuration
type ClusterSecurityConfig struct {
	CACertificates string `yaml:"ca_certificates" json:"ca_certificates"`
	OSHardening    bool   `yaml:"os_hardening" json:"os_hardening"`
}

// KubernetesSecurityConfig represents Kubernetes-level security configuration
type KubernetesSecurityConfig struct {
	K8sHardening          bool     `yaml:"k8s_hardening" json:"k8s_hardening"`
	PodSecurityExemptions []string `yaml:"pod_security_exemptions" json:"pod_security_exemptions"`
}
