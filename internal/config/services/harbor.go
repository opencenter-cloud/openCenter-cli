package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"gopkg.in/yaml.v3"
)

// HarborConfig extends BaseConfig with Harbor-specific configuration.
type HarborConfig struct {
	BaseConfig `yaml:",inline"`

	// Access
	Hostname    string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Harbor external hostname"`
	ExternalURL string `yaml:"external_url,omitempty" json:"external_url,omitempty" jsonschema:"description=External URL for Harbor"`

	// IssuerRef names the ClusterIssuer that mints Harbor's certificate.
	// Empty means "same issuer the Gateway listener in front of me uses", which
	// keeps the listener and the leaf certificate in agreement. Pin it only to
	// deliberately use a different CA for this service than the rest of the
	// cluster.
	IssuerRef string `yaml:"issuer_ref,omitempty" json:"issuer_ref,omitempty" jsonschema:"description=ClusterIssuer for this service certificate; defaults to the cluster default issuer"`

	// Storage
	StorageType          string `yaml:"storage_type,omitempty" json:"storage_type,omitempty" validate:"oneof=s3 filesystem" jsonschema:"description=Storage backend type,enum=s3,enum=filesystem,default=s3"`
	RegistryVolumeSize   int    `yaml:"registry_volume_size,omitempty" json:"registry_volume_size,omitempty" validate:"min=1" jsonschema:"description=Registry PVC size in GB; retained for compatibility and required Harbor cache/state,default=100"`
	JobserviceVolumeSize int    `yaml:"jobservice_volume_size,omitempty" json:"jobservice_volume_size,omitempty" validate:"min=1" jsonschema:"description=Harbor jobservice log PVC size in GB (min 10 on Cinder-backed regions e.g. Rackspace SJC3),default=10"`
	DatabaseVolumeSize   int    `yaml:"database_volume_size,omitempty" json:"database_volume_size,omitempty" validate:"min=1" jsonschema:"description=Harbor internal database PVC size in GB,default=10"`
	RedisVolumeSize      int    `yaml:"redis_volume_size,omitempty" json:"redis_volume_size,omitempty" validate:"min=1" jsonschema:"description=Harbor Redis PVC size in GB (min 10 on Cinder-backed regions e.g. Rackspace SJC3),default=10"`
	TrivyVolumeSize      int    `yaml:"trivy_volume_size,omitempty" json:"trivy_volume_size,omitempty" validate:"min=1" jsonschema:"description=Harbor Trivy PVC size in GB (min 10 on Cinder-backed regions e.g. Rackspace SJC3),default=10"`
	StorageClass         string `yaml:"storage_class,omitempty" json:"storage_class,omitempty" jsonschema:"description=Storage class for Harbor PVCs; defaults to infrastructure storage.default_storage_class"`
	S3Bucket             string `yaml:"s3_bucket,omitempty" json:"s3_bucket,omitempty" jsonschema:"description=S3 bucket name for image storage"`
	S3Region             string `yaml:"s3_region,omitempty" json:"s3_region,omitempty" jsonschema:"description=S3 region"`
	S3Endpoint           string `yaml:"s3_endpoint,omitempty" json:"s3_endpoint,omitempty" validate:"omitempty,url" jsonschema:"description=S3-compatible endpoint URL for image storage"`

	// Database
	DatabaseType string `yaml:"database_type,omitempty" json:"database_type,omitempty" jsonschema:"description=Database type,enum=internal,enum=external,default=internal"`
	DatabaseHost string `yaml:"database_host,omitempty" json:"database_host,omitempty" jsonschema:"description=External database host"`
	DatabasePort int    `yaml:"database_port,omitempty" json:"database_port,omitempty" jsonschema:"description=External database port"`
	DatabaseName string `yaml:"database_name,omitempty" json:"database_name,omitempty" jsonschema:"description=External database name"`
	DatabaseUser string `yaml:"database_user,omitempty" json:"database_user,omitempty" jsonschema:"description=External database user"`

	// TLS
	EmitCertificate bool `yaml:"emit_certificate,omitempty" json:"emit_certificate,omitempty" jsonschema:"description=Render the Harbor TLS certificate manifest"`
}

// UnmarshalYAML applies defaults only when Harbor storage fields are omitted.
// An explicitly supplied zero remains zero so runtime validation can reject it.
func (c *HarborConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain HarborConfig
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*c = HarborConfig(decoded)

	provided := make(map[string]bool)
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			provided[node.Content[i].Value] = true
		}
	}
	if !provided["storage_type"] {
		c.StorageType = "s3"
	}
	if !provided["registry_volume_size"] {
		c.RegistryVolumeSize = 100
	}
	if !provided["jobservice_volume_size"] {
		c.JobserviceVolumeSize = 10
	}
	if !provided["database_volume_size"] {
		c.DatabaseVolumeSize = 10
	}
	if !provided["redis_volume_size"] {
		c.RedisVolumeSize = 10
	}
	if !provided["trivy_volume_size"] {
		c.TrivyVolumeSize = 10
	}
	return nil
}
func init() {
	registry.RegisterServiceConfig("harbor", HarborConfig{})
}
