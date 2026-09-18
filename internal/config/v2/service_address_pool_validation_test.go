// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v2

import (
	"strings"
	"testing"

	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	"github.com/stretchr/testify/require"
)

// addressPoolTestConfig builds a minimal config carrying a metallb service and a
// longhorn service, so validateServiceAddressPools can be exercised directly via
// the public ValidateServices entry point. Longhorn is used (rather than harbor)
// because it owns a gateway listener and carries no other enabled-service
// preconditions that would fire before the address-pool check.
func addressPoolTestConfig(metallb *services.MetalLBConfig, longhorn *services.LonghornConfig) *Config {
	svcMap := ServiceMap{}
	if metallb != nil {
		svcMap["metallb"] = metallb
	}
	if longhorn != nil {
		svcMap["longhorn"] = longhorn
	}
	return &Config{OpenCenter: OpenCenterConfig{Services: svcMap}}
}

func enabledMetalLB(pools ...services.IPAddressPool) *services.MetalLBConfig {
	return &services.MetalLBConfig{
		BaseConfig:     services.BaseConfig{Enabled: true},
		IPAddressPools: pools,
	}
}

func TestValidateServiceAddressPoolAcceptsDeclaredPool(t *testing.T) {
	cfg := addressPoolTestConfig(
		enabledMetalLB(services.IPAddressPool{Name: "harbor-pool", Addresses: []string{"10.0.1.0/24"}}),
		&services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true, AddressPool: "harbor-pool"}},
	)
	require.NoError(t, NewValidator().ValidateServices(cfg))
}

func TestValidateServiceAddressPoolRejectsUnknownPool(t *testing.T) {
	cfg := addressPoolTestConfig(
		enabledMetalLB(services.IPAddressPool{Name: "default-pool", Addresses: []string{"10.0.0.0/24"}}),
		&services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true, AddressPool: "does-not-exist"}},
	)
	err := NewValidator().ValidateServices(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "does-not-exist")
	require.Contains(t, err.Error(), "ip_address_pools")
}

func TestValidateServiceAddressPoolRequiresMetalLBEnabled(t *testing.T) {
	// harbor names a pool but metallb is absent entirely.
	cfg := addressPoolTestConfig(
		nil,
		&services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true, AddressPool: "harbor-pool"}},
	)
	err := NewValidator().ValidateServices(cfg)
	require.Error(t, err)
	require.Contains(t, err.Error(), "metallb")
}

func TestValidateServiceAddressPoolRequiresMetalLBEnabledWhenPresentButOff(t *testing.T) {
	// metallb present but disabled: naming a pool is still invalid.
	cfg := addressPoolTestConfig(
		&services.MetalLBConfig{BaseConfig: services.BaseConfig{Enabled: false}},
		&services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true, AddressPool: "harbor-pool"}},
	)
	err := NewValidator().ValidateServices(cfg)
	require.Error(t, err)
	require.Contains(t, strings.ToLower(err.Error()), "metallb")
}

func TestValidateServiceAddressPoolEmptyIsAllowed(t *testing.T) {
	// No service names a pool; metallb may be entirely absent (today's default).
	cfg := addressPoolTestConfig(
		nil,
		&services.LonghornConfig{BaseConfig: services.BaseConfig{Enabled: true}},
	)
	require.NoError(t, NewValidator().ValidateServices(cfg))
}
