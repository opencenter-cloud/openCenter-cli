package services

import (
	"github.com/rackerlabs/opencenter-cli/internal/config/registry"
)

// KeycloakConfig extends BaseConfig with Keycloak-specific configuration
type KeycloakConfig struct {
	BaseConfig `yaml:",inline"`

	Realm       string `yaml:"keycloak_realm" json:"keycloak_realm,omitempty" jsonschema:"description=Keycloak realm name"`
	FrontendURL string `yaml:"keycloak_frontend_url" json:"keycloak_frontend_url,omitempty" jsonschema:"description=Keycloak frontend URL"`
	ClientID    string `yaml:"keycloak_client_id" json:"keycloak_client_id,omitempty" jsonschema:"description=Keycloak client ID,default=opencenter"`

	// Production configuration
	StartOptimized bool   `yaml:"start_optimized" json:"start_optimized,omitempty" jsonschema:"description=Enable production-optimized startup,default=true"`
	CacheEnabled   bool   `yaml:"cache_enabled" json:"cache_enabled,omitempty" jsonschema:"description=Enable distributed caching,default=true"`
	CacheStack     string `yaml:"cache_stack" json:"cache_stack,omitempty" jsonschema:"description=Cache stack (kubernetes or ispn),default=kubernetes"`

	// Resource configuration
	ResourceRequestsCPU    string `yaml:"resource_requests_cpu" json:"resource_requests_cpu,omitempty" jsonschema:"description=CPU requests,default=2"`
	ResourceRequestsMemory string `yaml:"resource_requests_memory" json:"resource_requests_memory,omitempty" jsonschema:"description=Memory requests,default=1250M"`
	ResourceLimitsCPU      string `yaml:"resource_limits_cpu" json:"resource_limits_cpu,omitempty" jsonschema:"description=CPU limits,default=6"`
	ResourceLimitsMemory   string `yaml:"resource_limits_memory" json:"resource_limits_memory,omitempty" jsonschema:"description=Memory limits,default=2250M"`

	// Scaling configuration
	Instances   int `yaml:"instances" json:"instances,omitempty" jsonschema:"description=Number of Keycloak instances,default=3"`
	MinReplicas int `yaml:"min_replicas" json:"min_replicas,omitempty" jsonschema:"description=Minimum replicas for autoscaling,default=3"`
	MaxReplicas int `yaml:"max_replicas" json:"max_replicas,omitempty" jsonschema:"description=Maximum replicas for autoscaling,default=10"`

	// Database configuration
	DatabaseHost string `yaml:"database_host" json:"database_host,omitempty" jsonschema:"description=External database host"`
	DatabasePort int    `yaml:"database_port" json:"database_port,omitempty" jsonschema:"description=External database port,default=5432"`
	DatabaseName string `yaml:"database_name" json:"database_name,omitempty" jsonschema:"description=External database name"`
	DatabaseUser string `yaml:"database_user" json:"database_user,omitempty" jsonschema:"description=External database user"`

	// Database connection pool
	DBPoolMinSize     int `yaml:"db_pool_min_size" json:"db_pool_min_size,omitempty" jsonschema:"description=Minimum database connection pool size,default=30"`
	DBPoolInitialSize int `yaml:"db_pool_initial_size" json:"db_pool_initial_size,omitempty" jsonschema:"description=Initial database connection pool size,default=30"`
	DBPoolMaxSize     int `yaml:"db_pool_max_size" json:"db_pool_max_size,omitempty" jsonschema:"description=Maximum database connection pool size,default=30"`

	// Monitoring configuration
	MetricsEnabled      bool   `yaml:"metrics_enabled" json:"metrics_enabled,omitempty" jsonschema:"description=Enable Prometheus metrics,default=true"`
	EventMetricsEnabled bool   `yaml:"event_metrics_enabled" json:"event_metrics_enabled,omitempty" jsonschema:"description=Enable event metrics,default=true"`
	HealthEnabled       bool   `yaml:"health_enabled" json:"health_enabled,omitempty" jsonschema:"description=Enable health endpoints,default=true"`
	LogLevel            string `yaml:"log_level" json:"log_level,omitempty" jsonschema:"description=Log level (INFO|DEBUG|WARN|ERROR),default=INFO"`
	LogFormat           string `yaml:"log_format" json:"log_format,omitempty" jsonschema:"description=Log format (default|json),default=json"`

	// TLS configuration
	TLSSecretName string `yaml:"tls_secret_name" json:"tls_secret_name,omitempty" jsonschema:"description=TLS secret name,default=keycloak-tls-secret"`
	TLSEnabled    bool   `yaml:"tls_enabled" json:"tls_enabled,omitempty" jsonschema:"description=Enable TLS,default=true"`

	// Realm configuration
	RealmImportEnabled bool     `yaml:"realm_import_enabled" json:"realm_import_enabled,omitempty" jsonschema:"description=Enable automatic realm import,default=true"`
	RealmGroups        []string `yaml:"realm_groups" json:"realm_groups,omitempty" jsonschema:"description=Additional realm groups to create"`
	RealmAdminEmail    string   `yaml:"realm_admin_email" json:"realm_admin_email,omitempty" jsonschema:"description=Admin user email address"`

	// Backup configuration
	BackupEnabled  bool   `yaml:"backup_enabled" json:"backup_enabled,omitempty" jsonschema:"description=Enable automated realm backups,default=true"`
	BackupSchedule string `yaml:"backup_schedule" json:"backup_schedule,omitempty" jsonschema:"description=Backup cron schedule,default=0 2 * * *"`

	// SMTP configuration
	SMTPHost     string `yaml:"smtp_host" json:"smtp_host,omitempty" jsonschema:"description=SMTP server host"`
	SMTPPort     int    `yaml:"smtp_port" json:"smtp_port,omitempty" jsonschema:"description=SMTP server port,default=587"`
	SMTPFrom     string `yaml:"smtp_from" json:"smtp_from,omitempty" jsonschema:"description=SMTP from address"`
	SMTPStartTLS bool   `yaml:"smtp_starttls" json:"smtp_starttls,omitempty" jsonschema:"description=Enable STARTTLS for SMTP,default=true"`
}

func init() {
	registry.RegisterServiceConfig("keycloak", KeycloakConfig{})
}
