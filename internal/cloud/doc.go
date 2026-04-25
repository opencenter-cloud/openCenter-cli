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

// Package cloud provides cloud provider abstractions for infrastructure drift detection.
//
// # Overview
//
// The cloud package defines interfaces and types for interacting with cloud providers
// to detect and reconcile infrastructure drift. It supports multiple cloud providers
// through a factory pattern, allowing opencenter to work with OpenStack, VMware, and
// other cloud platforms.
//
// This package is not the registry for lifecycle deploy providers. Providers used
// by cluster deploy or destroy flows may live in sibling packages and be wired
// directly by those commands or services. Kind is the current example: it manages
// local cluster lifecycle, but it does not implement the CloudProvider drift
// interface and is therefore not registered in CloudProviderFactory. Baremetal and
// Talos are also outside the drift-provider registry today.
//
// # Architecture
//
// The package is organized around three main concepts:
//
//  1. CloudProvider interface - defines operations for drift detection
//  2. CloudProviderFactory - creates provider instances based on configuration
//  3. Infrastructure state types - represent cloud resources
//
// # CloudProvider Interface
//
// The CloudProvider interface defines three core operations:
//
//   - GetCurrentState: Retrieves actual infrastructure state from the cloud provider
//   - DetectDrift: Compares desired vs actual state and reports differences
//   - ReconcileDrift: Applies changes to fix detected drift
//
// # Usage Example
//
//	// Create factory and register providers
//	factory := cloud.NewCloudProviderFactory()
//	factory.RegisterProvider("openstack", openstack.NewProvider(authOpts, region))
//	factory.RegisterProvider("vmware", vmware.NewProvider())
//
//	// Get provider for cluster
//	provider, err := factory.GetProvider(cfg.Infrastructure.Provider)
//	if err != nil {
//	    return err
//	}
//
//	// Get current state from cloud
//	currentState, err := provider.GetCurrentState(ctx, cfg)
//	if err != nil {
//	    return err
//	}
//
//	// Build desired state from configuration
//	desiredState := buildDesiredState(cfg)
//
//	// Detect drift
//	report, err := provider.DetectDrift(ctx, desiredState, currentState)
//	if err != nil {
//	    return err
//	}
//
//	// Reconcile if needed
//	if report.Reconcilable && len(report.Drifts) > 0 {
//	    err = provider.ReconcileDrift(ctx, report)
//	}
//
// # Drift Detection
//
// Drift detection compares the desired infrastructure state (from configuration)
// with the actual state (from the cloud provider) and identifies differences.
//
// Each drift item includes:
//   - Resource type and identifier
//   - Field that has drifted
//   - Expected vs actual values
//   - Severity level (info, warning, critical)
//   - Whether it can be automatically reconciled
//
// # Drift Severity Levels
//
//   - SeverityInfo: Informational drift (metadata, timestamps)
//   - SeverityWarning: Warning-level drift (worker nodes, tags)
//   - SeverityCritical: Critical drift (control plane, network config)
//
// # Reconciliation
//
// Drift reconciliation applies changes to bring infrastructure back in line
// with the desired configuration. Only reconcilable drift items are processed.
//
// Non-reconcilable drift (e.g., deleted resources, manual changes) requires
// manual intervention.
//
// # Provider Implementations
//
// Provider implementations are in subpackages:
//   - openstack: OpenStack cloud provider
//   - vmware: VMware vSphere cloud provider
//
// Each provider implements the CloudProvider interface and handles
// provider-specific API calls and resource types.
//
// # Thread Safety
//
// CloudProviderFactory is safe for concurrent use. Provider implementations
// should be safe for concurrent read operations but may require synchronization
// for write operations (reconciliation).
package cloud
