// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
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

package cloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rackerlabs/opencenter-cli/internal/config"
)

// mockProvider is a mock implementation of CloudProvider for testing
type mockProvider struct {
	name string
}

func (m *mockProvider) GetCurrentState(ctx context.Context, cfg config.Config) (*InfrastructureState, error) {
	return &InfrastructureState{}, nil
}

func (m *mockProvider) DetectDrift(ctx context.Context, desired, actual *InfrastructureState) (*DriftReport, error) {
	return &DriftReport{}, nil
}

func (m *mockProvider) ReconcileDrift(ctx context.Context, drift *DriftReport) error {
	return nil
}

func TestNewCloudProviderFactory(t *testing.T) {
	factory := NewCloudProviderFactory()
	require.NotNil(t, factory)
	assert.NotNil(t, factory.providers)
	assert.Empty(t, factory.providers)
}

func TestCloudProviderFactory_RegisterProvider(t *testing.T) {
	factory := NewCloudProviderFactory()
	provider := &mockProvider{name: "test"}

	factory.RegisterProvider("test", provider)

	assert.Len(t, factory.providers, 1)
	assert.Equal(t, provider, factory.providers["test"])
}

func TestCloudProviderFactory_GetProvider_Success(t *testing.T) {
	factory := NewCloudProviderFactory()
	provider := &mockProvider{name: "test"}
	factory.RegisterProvider("test", provider)

	result, err := factory.GetProvider("test")

	require.NoError(t, err)
	assert.Equal(t, provider, result)
}

func TestCloudProviderFactory_GetProvider_NotFound(t *testing.T) {
	factory := NewCloudProviderFactory()

	result, err := factory.GetProvider("nonexistent")

	require.Error(t, err)
	assert.Nil(t, result)

	var unsupportedErr *UnsupportedProviderError
	require.ErrorAs(t, err, &unsupportedErr)
	assert.Equal(t, "nonexistent", unsupportedErr.Provider)
	assert.Empty(t, unsupportedErr.SupportedProviders)
}

func TestCloudProviderFactory_GetProvider_ErrorMessage(t *testing.T) {
	factory := NewCloudProviderFactory()
	factory.RegisterProvider("openstack", &mockProvider{name: "openstack"})
	factory.RegisterProvider("aws", &mockProvider{name: "aws"})

	_, err := factory.GetProvider("azure")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported cloud provider: azure")
	assert.Contains(t, err.Error(), "supported:")
}

func TestCloudProviderFactory_MultipleProviders(t *testing.T) {
	factory := NewCloudProviderFactory()
	
	openstackProvider := &mockProvider{name: "openstack"}
	awsProvider := &mockProvider{name: "aws"}
	
	factory.RegisterProvider("openstack", openstackProvider)
	factory.RegisterProvider("aws", awsProvider)

	// Get OpenStack provider
	result1, err := factory.GetProvider("openstack")
	require.NoError(t, err)
	assert.Equal(t, openstackProvider, result1)

	// Get AWS provider
	result2, err := factory.GetProvider("aws")
	require.NoError(t, err)
	assert.Equal(t, awsProvider, result2)

	// Verify they're different
	assert.NotEqual(t, result1, result2)
}

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		name     string
		severity Severity
		expected string
	}{
		{
			name:     "info severity",
			severity: SeverityInfo,
			expected: "info",
		},
		{
			name:     "warning severity",
			severity: SeverityWarning,
			expected: "warning",
		},
		{
			name:     "critical severity",
			severity: SeverityCritical,
			expected: "critical",
		},
		{
			name:     "unknown severity",
			severity: Severity(999),
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.severity.String()
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUnsupportedProviderError_Error(t *testing.T) {
	err := &UnsupportedProviderError{
		Provider:           "azure",
		SupportedProviders: []string{"openstack", "aws"},
	}

	message := err.Error()
	assert.Contains(t, message, "unsupported cloud provider: azure")
	assert.Contains(t, message, "openstack")
	assert.Contains(t, message, "aws")
}
