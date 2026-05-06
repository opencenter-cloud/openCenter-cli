package config

// Deployment represents deployment behavior configuration
type Deployment struct {
	AutoDeploy bool   `yaml:"auto_deploy" json:"auto_deploy"`
	Method     string `yaml:"method,omitempty" json:"method,omitempty" validate:"required,oneof=kubespray kamaji eks gke aks cluster-api"`
}
