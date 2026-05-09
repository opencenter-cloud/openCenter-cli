package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// OpenTelemetryConfig extends BaseConfig with OpenTelemetry-specific configuration.
type OpenTelemetryConfig struct {
	BaseConfig `yaml:",inline"`

	CollectorMode     string         `yaml:"collector_mode,omitempty" json:"collector_mode,omitempty" jsonschema:"description=Collector deployment mode,enum=deployment,enum=daemonset,enum=statefulset,default=deployment"`
	CollectorReplicas int            `yaml:"collector_replicas,omitempty" json:"collector_replicas,omitempty" jsonschema:"description=Number of collector replicas,default=1"`
	Exporters         []OTelExporter `yaml:"exporters,omitempty" json:"exporters,omitempty" jsonschema:"description=List of exporters"`
	Processors        []string       `yaml:"processors,omitempty" json:"processors,omitempty" jsonschema:"description=List of processor names"`
}

// OTelExporter represents an OpenTelemetry exporter configuration.
type OTelExporter struct {
	Name     string            `yaml:"name" json:"name" jsonschema:"description=Exporter name,required"`
	Type     string            `yaml:"type" json:"type" jsonschema:"description=Exporter type,enum=otlp,enum=prometheus,enum=jaeger,required"`
	Endpoint string            `yaml:"endpoint" json:"endpoint" jsonschema:"description=Exporter endpoint URL,required"`
	Headers  map[string]string `yaml:"headers,omitempty" json:"headers,omitempty" jsonschema:"description=Additional headers for the exporter"`
}

func init() {
	registry.RegisterServiceConfig("opentelemetry-kube-stack", OpenTelemetryConfig{})
}
