package v2

import (
	"testing"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestValidator() *validator.Validate {
	v := validator.New()
	_ = registerSchemaValidations(v)
	return v
}

func TestStorageConfig_InvalidEnums_Rejected(t *testing.T) {
	v := newTestValidator()

	tests := []struct {
		name    string
		mutate  func(*StorageConfig)
		wantTag string
	}{
		{
			name:    "invalid worker destination type",
			mutate:  func(s *StorageConfig) { s.WorkerVolumeDestinationType = "network" },
			wantTag: "oneof",
		},
		{
			name:    "invalid worker source type",
			mutate:  func(s *StorageConfig) { s.WorkerVolumeSourceType = "disk" },
			wantTag: "oneof",
		},
		{
			name:    "worker volume size zero fails required",
			mutate:  func(s *StorageConfig) { s.WorkerVolumeSize = 0 },
			wantTag: "required",
		},
		{
			name:    "master volume size negative fails min=0",
			mutate:  func(s *StorageConfig) { s.MasterVolumeSize = -1 },
			wantTag: "min",
		},
		{
			name:    "empty default storage class fails required",
			mutate:  func(s *StorageConfig) { s.DefaultStorageClass = "" },
			wantTag: "required",
		},
		{
			name:    "empty worker volume type fails required",
			mutate:  func(s *StorageConfig) { s.WorkerVolumeType = "" },
			wantTag: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStorageConfig()
			tt.mutate(&s)
			err := v.Struct(s)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantTag)
		})
	}
}

func TestStorageConfig_ValidEnums_Pass(t *testing.T) {
	v := newTestValidator()

	tests := []struct {
		name   string
		mutate func(*StorageConfig)
	}{
		{
			name:   "destination type volume",
			mutate: func(s *StorageConfig) { s.WorkerVolumeDestinationType = "volume" },
		},
		{
			name:   "destination type local",
			mutate: func(s *StorageConfig) { s.WorkerVolumeDestinationType = "local" },
		},
		{
			name:   "source type image",
			mutate: func(s *StorageConfig) { s.WorkerVolumeSourceType = "image" },
		},
		{
			name:   "source type volume",
			mutate: func(s *StorageConfig) { s.WorkerVolumeSourceType = "volume" },
		},
		{
			name:   "source type snapshot",
			mutate: func(s *StorageConfig) { s.WorkerVolumeSourceType = "snapshot" },
		},
		{
			name:   "master volume size zero is valid",
			mutate: func(s *StorageConfig) { s.MasterVolumeSize = 0 },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := validStorageConfig()
			tt.mutate(&s)
			err := v.Struct(s)
			assert.NoError(t, err)
		})
	}
}

func TestBlockDeviceConfig_Valid(t *testing.T) {
	v := newTestValidator()

	devices := []BlockDeviceConfig{
		{Name: "data", Size: 200, Type: "HA-Standard", DeleteOnTermination: true},
		{Name: "logs", Size: 50, MountPath: "/var/log"},
		{Name: "cache", Size: 25},
	}
	for _, d := range devices {
		err := v.Struct(d)
		assert.NoError(t, err, "device %q should be valid", d.Name)
	}
}

func TestBlockDeviceConfig_MissingName(t *testing.T) {
	v := newTestValidator()

	d := BlockDeviceConfig{Name: "", Size: 200}
	err := v.Struct(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Name")
}

func TestBlockDeviceConfig_SizeZero(t *testing.T) {
	v := newTestValidator()

	d := BlockDeviceConfig{Name: "data", Size: 0}
	err := v.Struct(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Size")
}

func TestBlockDeviceConfig_SizeNegative(t *testing.T) {
	v := newTestValidator()

	d := BlockDeviceConfig{Name: "data", Size: -10}
	err := v.Struct(d)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Size")
}

func TestWorkerPoolConfig_BootVolume_Validation(t *testing.T) {
	v := newTestValidator()

	tests := []struct {
		name    string
		pool    WorkerPoolConfig
		wantErr bool
	}{
		{
			name: "valid boot volume",
			pool: WorkerPoolConfig{
				Name: "gpu-pool", Count: 2, Flavor: "g1.xlarge",
				BootVolume: VolumeConfig{Size: 100, Type: "ssd"},
			},
			wantErr: false,
		},
		{
			name: "boot volume size zero is invalid",
			pool: WorkerPoolConfig{
				Name: "bad-pool", Count: 1, Flavor: "m1.large",
				BootVolume: VolumeConfig{Size: 0, Type: "ssd"},
			},
			wantErr: true,
		},
		{
			name: "invalid destination type on boot volume",
			pool: WorkerPoolConfig{
				Name: "bad-pool", Count: 1, Flavor: "m1.large",
				BootVolume: VolumeConfig{Size: 50, DestinationType: "network"},
			},
			wantErr: true,
		},
		{
			name: "invalid source type on boot volume",
			pool: WorkerPoolConfig{
				Name: "bad-pool", Count: 1, Flavor: "m1.large",
				BootVolume: VolumeConfig{Size: 50, SourceType: "disk"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Struct(tt.pool)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// validStorageConfig returns a StorageConfig that passes all struct-tag validation.
func validStorageConfig() StorageConfig {
	return StorageConfig{
		DefaultStorageClass:         "standard",
		WorkerVolumeSize:            50,
		WorkerVolumeDestinationType: "volume",
		WorkerVolumeSourceType:      "image",
		WorkerVolumeType:            "ssd",
		MasterVolumeSize:            40,
	}
}

func TestWindowsWorkerPoolConfig_Validation(t *testing.T) {
	v := newTestValidator()

	validBoot := VolumeConfig{Size: 100, Type: "ssd"}

	tests := []struct {
		name    string
		pool    WindowsWorkerPoolConfig
		wantErr bool
	}{
		{
			name:    "valid pool",
			pool:    WindowsWorkerPoolConfig{Name: "win-pool", Count: 2, Flavor: "gp.0.4.16", BootVolume: validBoot},
			wantErr: false,
		},
		{
			name:    "count zero is valid (scale-to-zero)",
			pool:    WindowsWorkerPoolConfig{Name: "win-pool", Count: 0, Flavor: "gp.0.4.16", BootVolume: validBoot},
			wantErr: false,
		},
		{
			name:    "missing name fails",
			pool:    WindowsWorkerPoolConfig{Name: "", Count: 1, Flavor: "gp.0.4.16", BootVolume: validBoot},
			wantErr: true,
		},
		{
			name:    "missing flavor fails",
			pool:    WindowsWorkerPoolConfig{Name: "win-pool", Count: 1, Flavor: "", BootVolume: validBoot},
			wantErr: true,
		},
		{
			name:    "invalid server group affinity fails",
			pool:    WindowsWorkerPoolConfig{Name: "win-pool", Count: 1, Flavor: "gp.0.4.16", BootVolume: validBoot, ServerGroupAffinity: "invalid"},
			wantErr: true,
		},
		{
			name:    "valid server group affinity",
			pool:    WindowsWorkerPoolConfig{Name: "win-pool", Count: 1, Flavor: "gp.0.4.16", BootVolume: validBoot, ServerGroupAffinity: "anti-affinity"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Struct(tt.pool)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestWorkerPoolConfig_CountZero_Valid(t *testing.T) {
	v := newTestValidator()

	pool := WorkerPoolConfig{Name: "drain-pool", Count: 0, Flavor: "m1.large", BootVolume: VolumeConfig{Size: 50}}
	err := v.Struct(pool)
	assert.NoError(t, err, "count=0 should be valid for scale-to-zero")
}
