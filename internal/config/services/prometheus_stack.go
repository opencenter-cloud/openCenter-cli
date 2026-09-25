package services

import (
	"fmt"
	"regexp"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	"gopkg.in/yaml.v3"
)

const (
	// DefaultPrometheusStackReleaseName is the Helm release name used by the
	// kube-prometheus-stack chart when no release name is configured.
	DefaultPrometheusStackReleaseName = "kube-prometheus-stack"
	// DefaultPrometheusStackNamespace is the deployment namespace used when a
	// legacy Prometheus stack config omits namespace.
	DefaultPrometheusStackNamespace = "observability"

	// PrometheusStackBaseFullnameMaxLength leaves room for the chart's
	// component suffixes while keeping generated Kubernetes names stable.
	PrometheusStackBaseFullnameMaxLength = 26
)

var prometheusStackReleaseNamePattern = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$`)
var prometheusStackNamespacePattern = regexp.MustCompile(`^[a-z0-9](?:[-a-z0-9]*[a-z0-9])?$`)

// PrometheusStackConfig extends BaseConfig with Prometheus stack configuration.
type PrometheusStackConfig struct {
	BaseConfig `yaml:",inline"`

	// ReleaseName is the Helm release name used to derive the chart's resource
	// and component service names. It is intentionally independent from
	// BaseConfig.Namespace, which remains the authoritative deployment
	// namespace.
	ReleaseName string `yaml:"release_name,omitempty" json:"release_name,omitempty" validate:"omitempty,max=26,helmrelease" jsonschema:"description=Helm release name for kube-prometheus-stack,default=kube-prometheus-stack"`

	// Hostname is retained as the Grafana hostname for backward compatibility.
	Hostname             string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Deprecated Grafana external hostname; use grafana_hostname"`
	PrometheusHostname   string `yaml:"prometheus_hostname,omitempty" json:"prometheus_hostname,omitempty" jsonschema:"description=Prometheus external hostname"`
	AlertmanagerHostname string `yaml:"alertmanager_hostname,omitempty" json:"alertmanager_hostname,omitempty" jsonschema:"description=Alertmanager external hostname"`
	GrafanaHostname      string `yaml:"grafana_hostname,omitempty" json:"grafana_hostname,omitempty" jsonschema:"description=Grafana external hostname"`

	// Storage per component
	GrafanaVolumeSize        int    `yaml:"grafana_volume_size,omitempty" json:"grafana_volume_size,omitempty" jsonschema:"description=Grafana persistent volume size in GB"`
	GrafanaStorageClass      string `yaml:"grafana_storage_class,omitempty" json:"grafana_storage_class,omitempty" jsonschema:"description=Grafana storage class"`
	PrometheusVolumeSize     int    `yaml:"prometheus_volume_size,omitempty" json:"prometheus_volume_size,omitempty" jsonschema:"description=Prometheus persistent volume size in GB"`
	PrometheusStorageClass   string `yaml:"prometheus_storage_class,omitempty" json:"prometheus_storage_class,omitempty" jsonschema:"description=Prometheus storage class"`
	AlertmanagerVolumeSize   int    `yaml:"alertmanager_volume_size,omitempty" json:"alertmanager_volume_size,omitempty" jsonschema:"description=Alertmanager persistent volume size in GB"`
	AlertmanagerStorageClass string `yaml:"alertmanager_storage_class,omitempty" json:"alertmanager_storage_class,omitempty" jsonschema:"description=Alertmanager storage class"`

	// Alerting
	WebhookURL string `yaml:"webhook_url,omitempty" json:"webhook_url,omitempty" jsonschema:"description=Webhook URL for alerting integrations"`
}

// ApplyDefaults materializes defaults introduced after the initial v2 config
// shape, keeping legacy configurations usable without changing their
// namespace.
func (c *PrometheusStackConfig) ApplyDefaults() {
	if c == nil {
		return
	}
	if c.ReleaseName == "" {
		c.ReleaseName = DefaultPrometheusStackReleaseName
	}
	if c.Namespace == "" {
		c.Namespace = DefaultPrometheusStackNamespace
	}
}

// UnmarshalYAML applies the release-name default when the field is absent from
// a legacy configuration. Explicit values are left untouched for validation.
func (c *PrometheusStackConfig) UnmarshalYAML(node *yaml.Node) error {
	type plain PrometheusStackConfig
	var decoded plain
	if err := node.Decode(&decoded); err != nil {
		return err
	}
	*c = PrometheusStackConfig(decoded)

	provided := false
	if node.Kind == yaml.MappingNode {
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == "release_name" {
				provided = true
				break
			}
		}
	}
	if !provided {
		c.ReleaseName = DefaultPrometheusStackReleaseName
	}
	if c.Namespace == "" {
		c.Namespace = DefaultPrometheusStackNamespace
	}
	return nil
}

// ValidPrometheusStackReleaseName reports whether name is a stable Helm and
// Kubernetes-compatible release name within the chart-safe base fullname
// limit.
func ValidPrometheusStackReleaseName(name string) bool {
	return len(name) <= PrometheusStackBaseFullnameMaxLength &&
		prometheusStackReleaseNamePattern.MatchString(name)
}

// ValidatePrometheusStackReleaseName validates the configured Helm release
// name and the component service names derived from it.
func ValidatePrometheusStackReleaseName(name string) error {
	if len(name) > PrometheusStackBaseFullnameMaxLength {
		return fmt.Errorf("release_name must be at most %d characters", PrometheusStackBaseFullnameMaxLength)
	}
	if !prometheusStackReleaseNamePattern.MatchString(name) {
		return fmt.Errorf("release_name must be a lowercase Kubernetes/Helm name beginning and ending with an alphanumeric character and containing only lowercase letters, numbers, and hyphens")
	}

	for _, component := range []string{"prometheus", "alertmanager", "grafana"} {
		serviceName := name + "-" + component
		if len(serviceName) > 63 || !prometheusStackReleaseNamePattern.MatchString(serviceName) {
			return fmt.Errorf("generated %s service name %q is not a valid Kubernetes service name", component, serviceName)
		}
	}
	return nil
}

// ValidatePrometheusStackConfig validates the release-name contract for the
// Prometheus stack service.
func ValidatePrometheusStackConfig(c *PrometheusStackConfig) error {
	if c == nil {
		return fmt.Errorf("prometheus stack configuration must not be nil")
	}
	c.ApplyDefaults()
	if err := ValidatePrometheusStackReleaseName(c.ReleaseName); err != nil {
		return err
	}
	return ValidatePrometheusStackNamespace(c.Namespace)
}

// ValidatePrometheusStackNamespace validates the resolved Kubernetes namespace
// used by the Prometheus stack and its component Services.
func ValidatePrometheusStackNamespace(namespace string) error {
	if len(namespace) > 63 {
		return fmt.Errorf("namespace must be at most 63 characters")
	}
	if !prometheusStackNamespacePattern.MatchString(namespace) {
		return fmt.Errorf("namespace must be a valid DNS-1123 label")
	}
	return nil
}

func init() {
	registry.RegisterServiceConfig("kube-prometheus-stack", PrometheusStackConfig{})
}
