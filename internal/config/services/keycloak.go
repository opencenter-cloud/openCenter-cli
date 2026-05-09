package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// KeycloakConfig extends BaseConfig with Keycloak-specific configuration.
type KeycloakConfig struct {
	BaseConfig `yaml:",inline"`

	// Access
	Hostname    string `yaml:"hostname,omitempty" json:"hostname,omitempty" jsonschema:"description=Keycloak external hostname"`
	FrontendURL string `yaml:"frontend_url,omitempty" json:"frontend_url,omitempty" jsonschema:"description=Keycloak frontend URL"`

	// Realm
	Realm              string   `yaml:"realm,omitempty" json:"realm,omitempty" jsonschema:"description=Keycloak realm name"`
	ClientID           string   `yaml:"client_id,omitempty" json:"client_id,omitempty" jsonschema:"description=Keycloak client ID,default=opencenter"`
	RealmImportEnabled bool     `yaml:"realm_import_enabled,omitempty" json:"realm_import_enabled,omitempty" jsonschema:"description=Enable automatic realm import,default=true"`
	RealmGroups        []string `yaml:"realm_groups,omitempty" json:"realm_groups,omitempty" jsonschema:"description=Additional realm groups to create"`
	RealmAdminEmail    string   `yaml:"realm_admin_email,omitempty" json:"realm_admin_email,omitempty" jsonschema:"description=Admin user email address"`

	// Runtime
	StartOptimized bool   `yaml:"start_optimized,omitempty" json:"start_optimized,omitempty" jsonschema:"description=Enable production-optimized startup,default=true"`
	CacheEnabled   bool   `yaml:"cache_enabled,omitempty" json:"cache_enabled,omitempty" jsonschema:"description=Enable distributed caching,default=true"`
	CacheStack     string `yaml:"cache_stack,omitempty" json:"cache_stack,omitempty" jsonschema:"description=Cache stack (kubernetes or ispn),default=kubernetes"`

	// Resources
	ResourceRequestsCPU    string `yaml:"resource_requests_cpu,omitempty" json:"resource_requests_cpu,omitempty" jsonschema:"description=CPU requests,default=2"`
	ResourceRequestsMemory string `yaml:"resource_requests_memory,omitempty" json:"resource_requests_memory,omitempty" jsonschema:"description=Memory requests,default=1250M"`
	ResourceLimitsCPU      string `yaml:"resource_limits_cpu,omitempty" json:"resource_limits_cpu,omitempty" jsonschema:"description=CPU limits,default=6"`
	ResourceLimitsMemory   string `yaml:"resource_limits_memory,omitempty" json:"resource_limits_memory,omitempty" jsonschema:"description=Memory limits,default=2250M"`

	// Scaling
	Instances   int `yaml:"instances,omitempty" json:"instances,omitempty" jsonschema:"description=Number of Keycloak instances,default=3"`
	MinReplicas int `yaml:"min_replicas,omitempty" json:"min_replicas,omitempty" jsonschema:"description=Minimum replicas for autoscaling,default=3"`
	MaxReplicas int `yaml:"max_replicas,omitempty" json:"max_replicas,omitempty" jsonschema:"description=Maximum replicas for autoscaling,default=10"`

	// Database
	DatabaseHost      string `yaml:"database_host,omitempty" json:"database_host,omitempty" jsonschema:"description=External database host"`
	DatabasePort      int    `yaml:"database_port,omitempty" json:"database_port,omitempty" jsonschema:"description=External database port,default=5432"`
	DatabaseName      string `yaml:"database_name,omitempty" json:"database_name,omitempty" jsonschema:"description=External database name"`
	DatabaseUser      string `yaml:"database_user,omitempty" json:"database_user,omitempty" jsonschema:"description=External database user"`
	DBPoolMinSize     int    `yaml:"db_pool_min_size,omitempty" json:"db_pool_min_size,omitempty" jsonschema:"description=Minimum database connection pool size,default=30"`
	DBPoolInitialSize int    `yaml:"db_pool_initial_size,omitempty" json:"db_pool_initial_size,omitempty" jsonschema:"description=Initial database connection pool size,default=30"`
	DBPoolMaxSize     int    `yaml:"db_pool_max_size,omitempty" json:"db_pool_max_size,omitempty" jsonschema:"description=Maximum database connection pool size,default=30"`

	// Observability
	MetricsEnabled      bool   `yaml:"metrics_enabled,omitempty" json:"metrics_enabled,omitempty" jsonschema:"description=Enable Prometheus metrics,default=true"`
	EventMetricsEnabled bool   `yaml:"event_metrics_enabled,omitempty" json:"event_metrics_enabled,omitempty" jsonschema:"description=Enable event metrics,default=true"`
	HealthEnabled       bool   `yaml:"health_enabled,omitempty" json:"health_enabled,omitempty" jsonschema:"description=Enable health endpoints,default=true"`
	LogLevel            string `yaml:"log_level,omitempty" json:"log_level,omitempty" jsonschema:"description=Log level,enum=INFO,enum=DEBUG,enum=WARN,enum=ERROR,default=INFO"`
	LogFormat           string `yaml:"log_format,omitempty" json:"log_format,omitempty" jsonschema:"description=Log format,enum=default,enum=json,default=json"`

	// TLS
	TLSSecretName string `yaml:"tls_secret_name,omitempty" json:"tls_secret_name,omitempty" jsonschema:"description=TLS secret name,default=keycloak-tls-secret"`
	TLSEnabled    bool   `yaml:"tls_enabled,omitempty" json:"tls_enabled,omitempty" jsonschema:"description=Enable TLS,default=true"`

	// Backup
	BackupEnabled  bool   `yaml:"backup_enabled,omitempty" json:"backup_enabled,omitempty" jsonschema:"description=Enable automated realm backups,default=true"`
	BackupSchedule string `yaml:"backup_schedule,omitempty" json:"backup_schedule,omitempty" jsonschema:"description=Backup cron schedule,default=0 2 * * *"`

	// SMTP
	SMTPHost     string `yaml:"smtp_host,omitempty" json:"smtp_host,omitempty" jsonschema:"description=SMTP server host"`
	SMTPPort     int    `yaml:"smtp_port,omitempty" json:"smtp_port,omitempty" jsonschema:"description=SMTP server port,default=587"`
	SMTPFrom     string `yaml:"smtp_from,omitempty" json:"smtp_from,omitempty" jsonschema:"description=SMTP from address"`
	SMTPStartTLS bool   `yaml:"smtp_starttls,omitempty" json:"smtp_starttls,omitempty" jsonschema:"description=Enable STARTTLS for SMTP,default=true"`
}

func init() {
	registry.RegisterServiceConfig("keycloak", KeycloakConfig{})
}
