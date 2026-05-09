package services

import (
	"github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
)

// MetalLBConfig extends BaseConfig with MetalLB-specific configuration.
type MetalLBConfig struct {
	BaseConfig `yaml:",inline"`

	IPAddressPools []IPAddressPool `yaml:"ip_address_pools,omitempty" json:"ip_address_pools,omitempty" jsonschema:"description=List of IP address pools for MetalLB"`
}

// IPAddressPool represents a MetalLB IP address pool.
type IPAddressPool struct {
	Name       string   `yaml:"name" json:"name" jsonschema:"description=Name of the IP address pool,required"`
	Addresses  []string `yaml:"addresses" json:"addresses" jsonschema:"description=List of IP address ranges (e.g. 172.23.0.6-172.23.0.8),required"`
	AutoAssign bool     `yaml:"auto_assign,omitempty" json:"auto_assign,omitempty" jsonschema:"description=Automatically assign IPs from this pool,default=true"`
}

func init() {
	registry.RegisterServiceConfig("metallb", MetalLBConfig{})
}
