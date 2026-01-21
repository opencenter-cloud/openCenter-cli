package services

import (
	"github.com/rackerlabs/openCenter-cli/internal/config/registry"
)

// PrometheusStackConfig extends BaseConfig with Prometheus stack configuration
type PrometheusStackConfig struct {
	BaseConfig `yaml:",inline"`

	// Storage configuration for each component
	GrafanaVolumeSize        int    `yaml:"grafana_volume_size" json:"grafana_volume_size,omitempty" jsonschema:"description=Grafana persistent volume size in GB"`
	GrafanaStorageClass      string `yaml:"grafana_storage_class" json:"grafana_storage_class,omitempty" jsonschema:"description=Grafana storage class"`
	PrometheusVolumeSize     int    `yaml:"prometheus_volume_size" json:"prometheus_volume_size,omitempty" jsonschema:"description=Prometheus persistent volume size in GB"`
	PrometheusStorageClass   string `yaml:"prometheus_storage_class" json:"prometheus_storage_class,omitempty" jsonschema:"description=Prometheus storage class"`
	AlertmanagerVolumeSize   int    `yaml:"alertmanager_volume_size" json:"alertmanager_volume_size,omitempty" jsonschema:"description=Alertmanager persistent volume size in GB"`
	AlertmanagerStorageClass string `yaml:"alertmanager_storage_class" json:"alertmanager_storage_class,omitempty" jsonschema:"description=Alertmanager storage class"`
	WebhookURL               string `yaml:"webhook_url" json:"webhook_url,omitempty" jsonschema:"description=Webhook URL for alerting integrations"`
}

func init() {
	registry.RegisterServiceConfig("kube-prometheus-stack", PrometheusStackConfig{})
}
