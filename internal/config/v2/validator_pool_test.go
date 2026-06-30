package v2

import (
	"strings"
	"testing"
)

func TestValidator_PoolNameUniqueness_DuplicateLinux(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorker: []WorkerPoolConfig{
						{Name: "gpu-pool", Count: 2, Flavor: "gpu.0.4.16"},
						{Name: "gpu-pool", Count: 1, Flavor: "m1.large"},
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate pool name, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate pool name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidator_PoolNameUniqueness_DuplicateAcrossOSTypes(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorker: []WorkerPoolConfig{
						{Name: "shared-name", Count: 1, Flavor: "m1.large"},
					},
					AdditionalServerPoolsWorkerWindows: []WindowsWorkerPoolConfig{
						{Name: "shared-name", Count: 1, Flavor: "gp.0.4.16"},
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err == nil {
		t.Fatal("expected error for duplicate pool name across OS types, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate pool name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidator_PoolNameUniqueness_UniqueNames(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorker: []WorkerPoolConfig{
						{Name: "linux-pool", Count: 1, Flavor: "m1.large"},
					},
					AdditionalServerPoolsWorkerWindows: []WindowsWorkerPoolConfig{
						{Name: "win-pool", Count: 1, Flavor: "gp.0.4.16"},
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidator_WindowsPoolImage_RequiredWhenNoPerPoolImage(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorkerWindows: []WindowsWorkerPoolConfig{
						{Name: "win-pool", Count: 2, Flavor: "gp.0.4.16", Image: ""},
					},
				},
				Cloud: CloudConfig{
					OpenStack: &OpenStackCloudConfig{
						ImageIDWindows: "", // missing
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err == nil {
		t.Fatal("expected error for missing Windows image, got nil")
	}
	if !strings.Contains(err.Error(), "image_id_windows") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidator_WindowsPoolImage_AcceptsPerPoolImage(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorkerWindows: []WindowsWorkerPoolConfig{
						{Name: "win-pool", Count: 2, Flavor: "gp.0.4.16", Image: "win-2022-custom"},
					},
				},
				Cloud: CloudConfig{
					OpenStack: &OpenStackCloudConfig{
						ImageIDWindows: "", // empty but per-pool image set
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidator_WindowsPoolImage_SkipsValidationWhenCountZero(t *testing.T) {
	validator := NewValidator()

	cfg := &Config{
		SchemaVersion: "2.0",
		OpenCenter: OpenCenterConfig{
			Infrastructure: InfrastructureConfig{
				Compute: ComputeConfig{
					AdditionalServerPoolsWorkerWindows: []WindowsWorkerPoolConfig{
						{Name: "win-pool", Count: 0, Flavor: "gp.0.4.16", Image: ""},
					},
				},
				Cloud: CloudConfig{
					OpenStack: &OpenStackCloudConfig{
						ImageIDWindows: "",
					},
				},
			},
		},
		OpenTofu: OpenTofuConfig{Backend: BackendConfig{Type: "local", Local: &LocalBackendConfig{Path: "/tmp/tf"}}},
	}

	err := validator.ValidateBusinessRules(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
