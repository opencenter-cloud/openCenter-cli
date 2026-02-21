package config

// OpenTofu holds OpenTofu-specific settings.
type OpenTofu struct {
	Enabled bool        `yaml:"enabled" json:"enabled"`
	Path    string      `yaml:"path" json:"path"`
	Backend TofuBackend `yaml:"backend" json:"backend"`
}

// TofuBackend describes the state backend configuration for OpenTofu.
// Type can be "local", "s3", or "aws" (aws is an alias for s3).
// When "local", Backend.Local.Path is used.
// When "s3" or "aws", Backend.S3 fields are used.
type TofuBackend struct {
	Type  string    `yaml:"type" json:"type"`
	Local TofuLocal `yaml:"local" json:"local"`
	S3    TofuS3    `yaml:"s3" json:"s3"`
}

type TofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

type TofuS3 struct {
	Bucket   string `yaml:"bucket" json:"bucket"`
	Key      string `yaml:"key" json:"key"`
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty"`
	Profile  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Encrypt  bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
}

// SimplifiedOpenTofu represents the opentofu section
type SimplifiedOpenTofu struct {
	Enabled bool                  `yaml:"enabled" json:"enabled"`
	Path    string                `yaml:"path" json:"path" validate:"required"`
	Backend SimplifiedTofuBackend `yaml:"backend" json:"backend" validate:"required"`
}

// SimplifiedTofuBackend represents the backend configuration
type SimplifiedTofuBackend struct {
	Type  string              `yaml:"type" json:"type" validate:"required,oneof=local s3 aws"`
	Local SimplifiedTofuLocal `yaml:"local,omitempty" json:"local,omitempty"`
	S3    SimplifiedTofuS3    `yaml:"s3,omitempty" json:"s3,omitempty"`
}

// SimplifiedTofuLocal represents the local backend
type SimplifiedTofuLocal struct {
	Path string `yaml:"path" json:"path"`
}

// SimplifiedTofuS3 represents the S3 backend
type SimplifiedTofuS3 struct {
	Bucket   string `yaml:"bucket" json:"bucket"`
	Key      string `yaml:"key" json:"key"`
	Region   string `yaml:"region" json:"region"`
	Endpoint string `yaml:"endpoint,omitempty" json:"endpoint,omitempty" validate:"omitempty,url"`
	Profile  string `yaml:"profile,omitempty" json:"profile,omitempty"`
	Encrypt  bool   `yaml:"encrypt,omitempty" json:"encrypt,omitempty"`
}
