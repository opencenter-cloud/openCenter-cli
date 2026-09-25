package openstack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

type fakeAdapter struct {
	preflight                                                                  cloudopenstack.StoragePreflight
	preflightErr                                                               error
	preflightS3Endpoint                                                        string
	preflightCalls, containers, appCreates, appDeletes, ec2Creates, ec2Deletes int
	containerNames                                                             []string
	appUserID, ec2UserID, appDeleteUserID, ec2DeleteUserID                     string
	app                                                                        cloudopenstack.AppCredential
	ec2                                                                        cloudopenstack.EC2Credentials
	revokeErr                                                                  error
}

func (f *fakeAdapter) Preflight(_ context.Context, _ string, explicitS3Endpoint string, _ bool) (cloudopenstack.StoragePreflight, error) {
	f.preflightCalls++
	f.preflightS3Endpoint = explicitS3Endpoint
	preflight := f.preflight
	if explicitS3Endpoint != "" {
		preflight.S3Endpoint = explicitS3Endpoint
	}
	return preflight, f.preflightErr
}
func (f *fakeAdapter) EnsureContainer(_ context.Context, req cloudopenstack.ContainerRequest) error {
	f.containers++
	f.containerNames = append(f.containerNames, req.Name)
	return nil
}
func (f *fakeAdapter) CreateAppCredential(_ context.Context, req cloudopenstack.AppCredentialRequest) (cloudopenstack.AppCredential, error) {
	f.appCreates++
	f.appUserID = req.UserID
	return f.app, nil
}
func (f *fakeAdapter) DeleteAppCredential(_ context.Context, _ string, userID string) error {
	f.appDeletes++
	f.appDeleteUserID = userID
	return f.revokeErr
}
func (f *fakeAdapter) CreateEC2Credentials(_ context.Context, req cloudopenstack.EC2CredentialRequest) (cloudopenstack.EC2Credentials, error) {
	f.ec2Creates++
	f.ec2UserID = req.UserID
	return f.ec2, nil
}
func (f *fakeAdapter) DeleteEC2Credentials(_ context.Context, _ string, userID string) error {
	f.ec2Deletes++
	f.ec2DeleteUserID = userID
	return f.revokeErr
}

type fakeFS struct {
	data                              []byte
	backup, atomic, recovery, removed bool
	recoveryJournals                  []recoveryJournal
	updateRecoveryErr                 error
	atomicErr                         error
}

func (f *fakeFS) ReadFile(string) ([]byte, error) { return append([]byte(nil), f.data...), nil }
func (f *fakeFS) CheckRecoveryPath(string) error  { return nil }
func (f *fakeFS) Backup(string, []byte) error     { f.backup = true; return nil }
func (f *fakeFS) WriteAtomic(_ string, data []byte) error {
	f.atomic = true
	if f.atomicErr != nil {
		return f.atomicErr
	}
	f.data = append([]byte(nil), data...)
	return nil
}
func (f *fakeFS) WriteRecovery(_ string, state recoveryJournal) error {
	f.recovery = true
	f.recoveryJournals = append(f.recoveryJournals, state)
	if strings.Contains(state.CredentialID, "secret") {
		return errors.New("secret leaked")
	}
	return nil
}
func (f *fakeFS) UpdateRecovery(_ string, state recoveryJournal) error {
	if err := f.WriteRecovery("", state); err != nil {
		return err
	}
	return f.updateRecoveryErr
}
func (f *fakeFS) RemoveRecovery(string) error { f.removed = true; return nil }

func testConfig(t *testing.T) *v2.Config {
	t.Helper()
	cfg, err := v2.NewV2Default("prod", "openstack")
	if err != nil {
		t.Fatal(err)
	}
	cfg.OpenCenter.Infrastructure.Provider = "openstack"
	if cfg.OpenCenter.Infrastructure.Cloud.OpenStack == nil {
		t.Fatal("default OpenStack config missing")
	}
	cfg.OpenCenter.Infrastructure.Cloud.OpenStack.ProjectID = "project-1"
	cfg.OpenCenter.Services["loki"] = &services.LokiConfig{}
	cfg.OpenCenter.Services["tempo"] = &services.TempoConfig{}
	cfg.OpenCenter.Services["etcd-backup"] = &services.EtcdBackupConfig{}
	cfg.OpenCenter.Services["velero"] = &services.VeleroConfig{}
	cfg.Secrets.Loki = v2.LokiSecrets{}
	cfg.Secrets.Tempo = v2.TempoSecrets{}
	cfg.Secrets.EtcdBackup = v2.EtcdBackupSecrets{}
	cfg.Secrets.Velero = v2.VeleroSecrets{}
	return cfg
}

func testAdapter() *fakeAdapter {
	return &fakeAdapter{preflight: cloudopenstack.StoragePreflight{Endpoint: "https://swift.example/v1/AUTH_project-1", S3Endpoint: "https://s3.example", AuthURL: "https://identity.example/v3", Region: "RegionOne", ProjectID: "project-1", CredentialOwnerID: "owner-1"}, app: cloudopenstack.AppCredential{ID: "app-1", Secret: "app-secret"}, ec2: cloudopenstack.EC2Credentials{ID: "ec2-1", AccessKeyID: "access-1", Secret: "ec2-secret", Endpoint: "https://s3.example", Region: "RegionOne", ProjectID: "project-1"}}
}

func TestValidateOptionsMappings(t *testing.T) {
	for _, tc := range []struct {
		service, backend string
		wantErr          bool
	}{{"loki", "swift", false}, {"loki", "s3", false}, {"loki", "none", false}, {"tempo", "swift", true}, {"tempo", "s3", false}, {"mimir", "s3", false}, {"mimir", "swift", false}, {"harbor", "s3", false}, {"harbor", "filesystem", false}, {"harbor", "swift", true}, {"etcd-backup", "s3", false}, {"etcd-backup", "none", false}, {"velero", "s3", false}, {"velero", "none", false}, {"velero", "swift", true}, {"other", "s3", true}} {
		err := ValidateOptions(Options{Service: tc.service, Backend: tc.backend, Cluster: "prod"})
		if (err != nil) != tc.wantErr {
			t.Errorf("%s=%s error=%v, wantErr=%v", tc.service, tc.backend, err, tc.wantErr)
		}
	}
}

func mimirStorageConfig(t *testing.T) *v2.Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.OpenCenter.Services["mimir"] = &services.MimirConfig{
		BaseConfig:  services.BaseConfig{Enabled: true},
		StorageType: "s3",
	}
	cfg.Secrets.Mimir = v2.MimirSecrets{}
	return cfg
}

func TestPlanMimirS3UsesDedicatedCredentialPaths(t *testing.T) {
	cfg := mimirStorageConfig(t)
	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"},
		Adapter: testAdapter(),
	})
	require.NoError(t, err)
	require.Equal(t, StatusPlanned, planned.Result.Status)
	require.Contains(t, planned.Result.SecretPaths, "secrets.mimir.s3_access_key_id")
	require.Contains(t, planned.Result.SecretPaths, "secrets.mimir.s3_secret_access_key")
	mimir := planned.prospective.OpenCenter.Services["mimir"].(*services.MimirConfig)
	require.Equal(t, "s3", mimir.StorageType)
	require.Equal(t, "prod-mimir", mimir.BucketName)
	require.Equal(t, "prod-mimir-ruler", mimir.RulerBucketName)
	require.Equal(t, "prod-mimir-alertmanager", mimir.AlertmanagerBucketName)
	require.Equal(t, []string{"prod-mimir", "prod-mimir-ruler", "prod-mimir-alertmanager"}, planned.Result.Containers)
	require.Equal(t, []string{"prod-mimir", "prod-mimir-ruler", "prod-mimir-alertmanager"}, []string{planned.Result.RemoteActions[0].Name, planned.Result.RemoteActions[1].Name, planned.Result.RemoteActions[2].Name})
	require.Equal(t, "https://s3.example", mimir.S3Endpoint)
	require.Equal(t, "RegionOne", mimir.S3Region)
	require.True(t, mimir.S3ForcePathStyle)
	require.False(t, mimir.S3Insecure)
	require.Empty(t, planned.prospective.Secrets.Mimir.S3AccessKeyID)
	require.Empty(t, planned.prospective.Secrets.Mimir.S3SecretAccessKey)
}

func TestPlanMimirS3RejectsPathEndpointBeforePreflight(t *testing.T) {
	cfg := mimirStorageConfig(t)
	adapter := testAdapter()
	_, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod", S3Endpoint: "https://s3.example/api"},
		Adapter: adapter,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "Mimir S3 endpoint must not contain a path")
	require.Zero(t, adapter.preflightCalls)
	require.Zero(t, adapter.containers)
	require.Zero(t, adapter.appCreates)
	require.Zero(t, adapter.ec2Creates)
}

func TestPlanMimirS3AcceptsHostRootEndpoint(t *testing.T) {
	cfg := mimirStorageConfig(t)
	adapter := testAdapter()
	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod", S3Endpoint: "https://s3.example"},
		Adapter: adapter,
	})
	require.NoError(t, err)
	require.Equal(t, "https://s3.example", adapter.preflightS3Endpoint)
	require.Equal(t, "https://s3.example", planned.Result.S3Endpoint)
}

func TestPlanMimirS3UsesConfiguredBuckets(t *testing.T) {
	cfg := mimirStorageConfig(t)
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.BucketName = "blocks-overridden"
	mimir.RulerBucketName = "rules-overridden"
	mimir.AlertmanagerBucketName = "alerts-overridden"

	planned, err := Plan(context.Background(), PlanInput{
		Config: cfg, Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(),
	})
	require.NoError(t, err)
	require.Equal(t, []string{"blocks-overridden", "rules-overridden", "alerts-overridden"}, planned.Result.Containers)
	require.Equal(t, []string{"blocks-overridden", "rules-overridden", "alerts-overridden"}, []string{planned.Result.RemoteActions[0].Name, planned.Result.RemoteActions[1].Name, planned.Result.RemoteActions[2].Name})
}

func TestPlanMimirS3RejectsCoincidentBuckets(t *testing.T) {
	cfg := mimirStorageConfig(t)
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.BucketName = "shared-mimir"
	mimir.RulerBucketName = "shared-mimir"
	mimir.AlertmanagerBucketName = "shared-mimir"

	adapter := testAdapter()
	planned, err := Plan(context.Background(), PlanInput{
		Config: cfg, Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"}, Adapter: adapter,
	})
	require.Error(t, err)
	require.ErrorContains(t, err, "Mimir S3 bucket names must be distinct")
	require.Empty(t, planned.Result.RemoteActions)
	require.Zero(t, adapter.preflightCalls)
}

func TestApplyMimirS3RejectsCoincidentBucketsBeforeRemoteActions(t *testing.T) {
	cfg := mimirStorageConfig(t)
	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.BucketName = "shared-mimir"
	mimir.RulerBucketName = "shared-mimir"
	mimir.AlertmanagerBucketName = "shared-mimir"
	raw, err := v2.MarshalPublicConfig(cfg)
	require.NoError(t, err)

	adapter := testAdapter()
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"}, Adapter: adapter, FileSystem: fs,
	})
	require.Error(t, err)
	require.Empty(t, result.RemoteActions)
	require.Zero(t, adapter.preflightCalls)
	require.Zero(t, adapter.containers)
	require.Zero(t, adapter.appCreates)
	require.Zero(t, adapter.ec2Creates)
	require.Zero(t, adapter.appDeletes)
	require.Zero(t, adapter.ec2Deletes)
	require.False(t, fs.backup)
	require.False(t, fs.atomic)
	require.False(t, fs.recovery)
}

func TestApplyMimirS3EnsuresAllBucketsAndRecordsThemForRecovery(t *testing.T) {
	cfg := mimirStorageConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	require.NoError(t, err)
	fs := &fakeFS{data: raw}
	adapter := testAdapter()
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"}, Adapter: adapter, FileSystem: fs,
	})
	require.NoError(t, err)
	require.Equal(t, StatusApplied, result.Status)
	require.Equal(t, []string{"prod-mimir", "prod-mimir-ruler", "prod-mimir-alertmanager"}, adapter.containerNames)
	require.NotEmpty(t, fs.recoveryJournals)
	require.Equal(t, []string{"prod-mimir", "prod-mimir-ruler", "prod-mimir-alertmanager"}, fs.recoveryJournals[0].Containers)
}

func TestApplyMimirS3PersistsAndRotatesCredentials(t *testing.T) {
	cfg := mimirStorageConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	require.NoError(t, err)
	fs := &fakeFS{data: raw}
	adapter := testAdapter()
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod"}, Adapter: adapter, FileSystem: fs,
	})
	require.NoError(t, err)
	require.Equal(t, StatusApplied, result.Status)
	require.Contains(t, string(fs.data), "access-1")
	require.Contains(t, string(fs.data), "ec2-secret")

	mimir := cfg.OpenCenter.Services["mimir"].(*services.MimirConfig)
	mimir.S3CredentialID = "old-ec2"
	cfg.Secrets.Mimir.S3AccessKeyID, cfg.Secrets.Mimir.S3SecretAccessKey = "old-access", "old-secret"
	raw, err = v2.MarshalPublicConfig(cfg)
	require.NoError(t, err)
	fs = &fakeFS{data: raw}
	adapter = testAdapter()
	result, err = Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "mimir", Backend: "s3", Cluster: "prod", RotateCredentials: true}, Adapter: adapter, FileSystem: fs,
	})
	require.NoError(t, err)
	require.Equal(t, StatusApplied, result.Status)
	require.Equal(t, 1, adapter.ec2Creates)
	require.Equal(t, 1, adapter.ec2Deletes)
}

func TestPlanNonRemoteStorageSkipsOpenStackProvisioning(t *testing.T) {
	for _, tc := range []struct {
		service, backend string
	}{
		{"harbor", "filesystem"}, {"velero", "none"}, {"etcd-backup", "none"},
	} {
		t.Run(tc.service+"-"+tc.backend, func(t *testing.T) {
			cfg := testConfig(t)
			if tc.service == "harbor" {
				cfg.OpenCenter.Services["harbor"] = &services.HarborConfig{BaseConfig: services.BaseConfig{Enabled: true}, RegistryVolumeSize: 10, JobserviceVolumeSize: 10, DatabaseVolumeSize: 10, RedisVolumeSize: 10, TrivyVolumeSize: 10}
			}
			planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: tc.service, Backend: tc.backend, Cluster: "prod"}})
			require.NoError(t, err)
			require.Empty(t, planned.Result.RemoteActions)
			require.Empty(t, planned.Result.S3Endpoint)
			service, err := serviceConfig(planned.prospective, tc.service)
			require.NoError(t, err)
			switch typed := service.(type) {
			case *services.HarborConfig:
				require.Equal(t, tc.backend, typed.StorageType)
			case *services.VeleroConfig:
				require.Equal(t, tc.backend, typed.StorageType)
			case *services.EtcdBackupConfig:
				require.Equal(t, tc.backend, typed.StorageType)
			}
		})
	}
}

func harborStorageConfig(t *testing.T) *v2.Config {
	t.Helper()
	cfg := testConfig(t)
	cfg.OpenCenter.Services["harbor"] = &services.HarborConfig{StorageType: "s3", RegistryVolumeSize: 100, JobserviceVolumeSize: 5, DatabaseVolumeSize: 10, RedisVolumeSize: 5, TrivyVolumeSize: 5}
	cfg.Secrets.Harbor = v2.HarborSecrets{}
	return cfg
}

func TestPlanHarborS3UsesDedicatedCredentialPathsWithoutMutation(t *testing.T) {
	cfg := harborStorageConfig(t)
	before, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"},
		Adapter: testAdapter(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusPlanned {
		t.Fatalf("status=%s, want planned", planned.Result.Status)
	}
	if !containsString(planned.Result.SecretPaths, "secrets.harbor.s3_access_key_id") || !containsString(planned.Result.SecretPaths, "secrets.harbor.s3_secret_access_key") {
		t.Fatalf("secret paths=%v, want dedicated Harbor S3 paths", planned.Result.SecretPaths)
	}
	if planned.prospective.Secrets.Harbor.S3AccessKeyID != "" || planned.prospective.Secrets.Harbor.S3SecretAccessKey != "" {
		t.Fatalf("read-only plan populated Harbor credentials=%#v", planned.prospective.Secrets.Harbor)
	}
	if got := planned.prospective.OpenCenter.Services["harbor"].(*services.HarborConfig); got.StorageType != "s3" || got.S3Bucket != "prod-harbor" || got.S3Region != "RegionOne" {
		t.Fatalf("planned Harbor storage=%+v", got)
	}
	after, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("Harbor plan mutated source config")
	}
	encoded, _ := json.Marshal(planned.Result)
	if strings.Contains(string(encoded), "ec2-secret") || strings.Contains(string(encoded), "access-1") {
		t.Fatalf("Harbor plan exposed adapter credentials: %s", encoded)
	}
}

func TestPlanHarborS3BlocksPartialCredentialPair(t *testing.T) {
	cfg := harborStorageConfig(t)
	cfg.Secrets.Harbor.S3AccessKeyID = "only-access"
	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"},
		Adapter: testAdapter(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusBlocked || !strings.Contains(strings.Join(planned.Result.Warnings, "\\n"), "partial") {
		t.Fatalf("status=%s warnings=%v", planned.Result.Status, planned.Result.Warnings)
	}
}

func TestPlanHarborS3ReusesCompleteExternalCredentials(t *testing.T) {
	cfg := harborStorageConfig(t)
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.StorageType, harbor.S3Bucket, harbor.S3Region, harbor.S3Endpoint = "s3", "prod-harbor", "RegionOne", "https://s3.example"
	cfg.Secrets.Harbor.S3AccessKeyID, cfg.Secrets.Harbor.S3SecretAccessKey = "access-1", "secret-1"

	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"},
		Adapter: testAdapter(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusNoOp || planned.Result.RemoteActions[1].Action != "reuse" || planned.Result.RemoteActions[1].ID != "" {
		t.Fatalf("result=%+v, want no-op reuse without public access-key ID", planned.Result)
	}
	encoded, err := json.Marshal(planned.Result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "access-1") {
		t.Fatalf("Harbor plan exposed access key: %s", encoded)
	}
}

func TestPlanHarborS3PropagatesCatalogAndExplicitEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name     string
		explicit string
		catalog  string
		want     string
	}{
		{name: "catalog", catalog: "https://catalog-s3.example", want: "https://catalog-s3.example"},
		{name: "explicit", explicit: "https://explicit-s3.example", catalog: "https://catalog-s3.example", want: "https://explicit-s3.example"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := harborStorageConfig(t)
			adapter := testAdapter()
			adapter.preflight.S3Endpoint = tc.catalog
			planned, err := Plan(context.Background(), PlanInput{
				Config:  cfg,
				Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod", S3Endpoint: tc.explicit},
				Adapter: adapter,
			})
			if err != nil {
				t.Fatal(err)
			}
			harbor := planned.prospective.OpenCenter.Services["harbor"].(*services.HarborConfig)
			if harbor.S3Endpoint != tc.want || planned.Result.S3Endpoint != tc.want {
				t.Fatalf("Harbor endpoint=%q result endpoint=%q, want %q", harbor.S3Endpoint, planned.Result.S3Endpoint, tc.want)
			}
			if adapter.preflightS3Endpoint != tc.explicit {
				t.Fatalf("adapter explicit endpoint=%q, want %q", adapter.preflightS3Endpoint, tc.explicit)
			}
		})
	}
}

func TestApplyHarborS3KeepsAccessKeyPrivateToJournal(t *testing.T) {
	cfg := harborStorageConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(), FileSystem: fs,
	})
	if err != nil || result.Status != StatusApplied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if len(fs.recoveryJournals) == 0 || fs.recoveryJournals[1].CredentialID != "access-1" {
		t.Fatalf("private recovery journal=%+v, want access key", fs.recoveryJournals)
	}
	publicJSON, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(publicJSON), "access-1") {
		t.Fatalf("public Harbor result exposed access key: %s", publicJSON)
	}
	if fs.recoveryJournals[1].S3Endpoint != "https://s3.example" {
		t.Fatalf("recovery endpoint=%q, want https://s3.example", fs.recoveryJournals[1].S3Endpoint)
	}
}

func TestApplyHarborS3PersistsExternallyIssuedCredentials(t *testing.T) {
	cfg := harborStorageConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(), FileSystem: fs,
	})
	if err != nil || result.Status != StatusApplied {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if !strings.Contains(string(fs.data), "access-1") || !strings.Contains(string(fs.data), "ec2-secret") {
		t.Fatalf("persisted Harbor S3 credentials missing: %s", fs.data)
	}
	if !fs.recovery || !fs.removed {
		t.Fatalf("Harbor credential recovery lifecycle incomplete: %+v", fs)
	}
}

func TestApplyHarborS3RotationRevokesPreviousAccessKey(t *testing.T) {
	cfg := harborStorageConfig(t)
	harbor := cfg.OpenCenter.Services["harbor"].(*services.HarborConfig)
	harbor.StorageType, harbor.S3Bucket, harbor.S3Region = "s3", "prod-harbor", "RegionOne"
	cfg.Secrets.Harbor.S3AccessKeyID, cfg.Secrets.Harbor.S3SecretAccessKey = "old-access", "old-secret"
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	adapter := testAdapter()
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod", RotateCredentials: true}, Adapter: adapter, FileSystem: fs,
	})
	if err != nil || result.Status != StatusApplied || adapter.ec2Creates != 1 || adapter.ec2Deletes != 1 {
		t.Fatalf("result=%+v err=%v adapter=%+v", result, err, adapter)
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func TestPlanDoesNotMutateAndReusesCompleteCredentials(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.S3Endpoint, loki.S3Region, loki.S3CredentialID = "s3", "prod-loki", "https://s3.example", "RegionOne", "ec2-old"
	loki.S3ForcePathStyle = true
	cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey = "access-secret", "secret-value"
	before, _ := v2.MarshalPublicConfig(cfg)
	adapter := testAdapter()
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: adapter})
	if err != nil {
		t.Fatal(err)
	}
	if adapter.preflightCalls != 1 || planned.Result.Status != StatusNoOp {
		t.Fatalf("calls=%d status=%s", adapter.preflightCalls, planned.Result.Status)
	}
	after, _ := v2.MarshalPublicConfig(cfg)
	if string(before) != string(after) {
		t.Fatal("plan mutated source config")
	}
	if planned.prospective.Secrets.Loki.S3SecretAccessKey != "secret-value" {
		t.Fatal("reuse replaced existing secret")
	}
	encoded, _ := json.Marshal(planned.Result)
	if strings.Contains(string(encoded), "secret-value") || strings.Contains(string(encoded), "access-secret") {
		t.Fatal("plan result exposed a secret")
	}
}

func TestPlanBlocksPartialPairUnlessRotation(t *testing.T) {
	cfg := testConfig(t)
	cfg.Secrets.Loki.S3AccessKeyID = "only-access"
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusBlocked {
		t.Fatalf("status=%s", planned.Result.Status)
	}
	if !strings.Contains(planned.Result.Warnings[0], "partial") {
		t.Fatal(planned.Result.Warnings)
	}
}

func TestScopedRulesIncludeContainerAndSegmentsOnly(t *testing.T) {
	rules := scopedRules("logs", "project-1")
	if len(rules) == 0 {
		t.Fatal("no rules")
	}
	for _, rule := range rules {
		if !strings.HasPrefix(rule.Path, "/v1/AUTH_project-1/logs") {
			t.Fatalf("unscoped rule: %+v", rule)
		}
		if strings.Contains(rule.Path, "/v1/AUTH_project-1/logs/../") {
			t.Fatalf("traversal rule: %+v", rule)
		}
	}
}

func TestApplyCreatesAndPersistsRecoverySafely(t *testing.T) {
	cfg := testConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	adapter := testAdapter()
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "tempo", Backend: "s3", Cluster: "prod"}, Adapter: adapter, FileSystem: fs})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != StatusApplied || adapter.ec2Creates != 1 || adapter.ec2UserID != "owner-1" || !fs.backup || !fs.recovery || !fs.removed {
		t.Fatalf("result=%+v adapter=%+v fs=%+v", result, adapter, fs)
	}
	if strings.Contains(string(fs.data), "ec2-secret") == false {
		t.Fatal("persisted typed secret missing")
	}
}

func TestApplyReturnsPartialWhenLocalPersistenceFails(t *testing.T) {
	cfg := testConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw, atomicErr: errors.New("write denied")}
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "velero", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(), FileSystem: fs})
	if err == nil || result.Status != StatusPartial || !fs.recovery {
		t.Fatalf("result=%+v err=%v recovery=%v", result, err, fs.recovery)
	}
}

func TestApplyRotationKeepsReplacementWhenRevokeFails(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.SwiftAuthURL, loki.SwiftRegion, loki.SwiftContainerName, loki.SwiftApplicationCredentialID = "swift", "prod-loki", "https://identity.example/v3", "RegionOne", "prod-loki", "old-app"
	cfg.Secrets.Loki.SwiftApplicationCredentialSecret = "old-secret"
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	adapter := testAdapter()
	adapter.revokeErr = errors.New("revoke unavailable")
	fs := &fakeFS{data: raw}
	result, err := Apply(context.Background(), ApplyInput{ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw, Options: Options{Service: "loki", Backend: "swift", Cluster: "prod", RotateCredentials: true}, Adapter: adapter, FileSystem: fs})
	if err == nil || result.Status != StatusPartial {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if adapter.appCreates != 1 || adapter.appDeletes != 1 || adapter.appUserID != "owner-1" || adapter.appDeleteUserID != "owner-1" || result.Recovery == nil || result.Recovery.PersistenceState != "persisted-revoke-failed" || len(result.Warnings) == 0 {
		t.Fatalf("rotation lifecycle incomplete: result=%+v adapter=%+v", result, adapter)
	}
	if !strings.Contains(string(fs.data), "app-secret") {
		t.Fatal("replacement secret was not persisted")
	}
}

func TestRecoveryJournalKeepsPrivateIDButPublicDTODoesNot(t *testing.T) {
	journal := recoveryJournal{RecoveryState: RecoveryState{CredentialType: "ec2", PersistenceState: "created-not-persisted"}, CredentialID: "access-key-1"}
	privateData, err := json.Marshal(journal)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(privateData), "access-key-1") {
		t.Fatalf("private recovery journal lost credential ID: %s", privateData)
	}

	public := Result{Recovery: &journal.RecoveryState, RemoteActions: []RemoteAction{{ID: ""}}}
	jsonData, err := json.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.Marshal(public)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range []string{string(jsonData), string(yamlData)} {
		if strings.Contains(output, "access-key-1") || strings.Contains(output, "credential_id") {
			t.Fatalf("public storage DTO leaked private recovery identity: %s", output)
		}
	}
}

func TestApplyHarborFailureAfterCredentialCreationRetainsPrivateJournalID(t *testing.T) {
	cfg := harborStorageConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	validationCalls := 0
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter(), FileSystem: fs,
		Validate: func(*v2.Config) error {
			validationCalls++
			if validationCalls == 2 {
				return errors.New("post-create validation failed")
			}
			return nil
		},
	})
	if err == nil || result.Status != StatusPartial {
		t.Fatalf("result=%+v err=%v, want partial failure", result, err)
	}
	if len(fs.recoveryJournals) < 2 || fs.recoveryJournals[1].CredentialID != "access-1" {
		t.Fatalf("private recovery journal=%+v, want created access key", fs.recoveryJournals)
	}
	publicJSON, marshalErr := json.Marshal(result)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	if strings.Contains(string(publicJSON), "access-1") || (result.Recovery != nil && strings.Contains(result.Recovery.PersistenceState, "access-1")) {
		t.Fatalf("public Harbor result leaked access key: %s", publicJSON)
	}
}

func TestStorageResultFormatsMaskCredentialValues(t *testing.T) {
	cfg := testConfig(t)
	cfg.Secrets.Loki.S3SecretAccessKey = "profile-secret"
	result := Result{Warnings: []string{redactError(fmt.Errorf("adapter failed with %s and %s", "profile-secret", "generated-secret"), cfg, "generated-secret").Error()}}
	jsonData, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	yamlData, err := yaml.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.Join(result.Warnings, "\n")
	for _, output := range []string{text, string(jsonData), string(yamlData)} {
		if strings.Contains(output, "profile-secret") || strings.Contains(output, "generated-secret") {
			t.Fatalf("credential leaked in output: %s", output)
		}
	}
}

func TestPlanIsolatesAllValidMappings(t *testing.T) {
	cases := []struct{ service, backend string }{
		{"loki", "swift"}, {"loki", "s3"},
		{"tempo", "s3"}, {"etcd-backup", "s3"}, {"velero", "s3"},
	}
	for _, tc := range cases {
		t.Run(tc.service+"-"+tc.backend, func(t *testing.T) {
			cfg := testConfig(t)
			raw, err := v2.MarshalPublicConfig(cfg)
			require.NoError(t, err)
			_, err = Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: tc.service, Backend: tc.backend, Cluster: "prod"}, Adapter: testAdapter()})
			require.NoError(t, err)
			after, err := v2.MarshalPublicConfig(cfg)
			require.NoError(t, err)
			require.Equal(t, raw, after)
		})
	}
}

func TestCloneConfigFailureDoesNotReturnOriginal(t *testing.T) {
	original := &v2.Config{Secrets: v2.SecretsConfig{ServiceSecrets: map[string]any{"invalid": func() {}}}}
	cloned, err := cloneConfig(original)
	if err == nil {
		t.Fatal("expected clone failure")
	}
	if cloned != nil || cloned == original {
		t.Fatalf("clone failure returned a configuration: %#v", cloned)
	}
}

func TestStorageValidationRejectsSwiftEndpointAndS3Names(t *testing.T) {
	if err := ValidateOptions(Options{Service: "loki", Backend: "s3", Cluster: "prod", S3Endpoint: "https://swift.example/v1/AUTH_project"}); err == nil {
		t.Fatal("expected Swift endpoint rejection")
	}
	if err := ValidateOptions(Options{Service: "loki", Backend: "s3", Cluster: "prod", Container: "Bad_Bucket"}); err == nil {
		t.Fatal("expected S3 bucket validation failure")
	}
	if err := ValidateOptions(Options{Service: "loki", Backend: "swift", Cluster: "prod", Container: "tenant/container"}); err == nil {
		t.Fatal("expected Swift container validation failure")
	}
}

func TestPlanRequiresCanonicalS3CredentialIDForReuse(t *testing.T) {
	cfg := testConfig(t)
	loki := cfg.OpenCenter.Services["loki"].(*services.LokiConfig)
	loki.StorageType, loki.BucketName, loki.S3Endpoint, loki.S3Region = "s3", "prod-loki", "https://s3.example", "RegionOne"
	cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey = "access-1", "secret-1"

	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "loki", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err != nil {
		t.Fatal(err)
	}
	if planned.Result.Status != StatusBlocked || !strings.Contains(strings.Join(planned.Result.Warnings, "\n"), "partial") {
		t.Fatalf("status=%s warnings=%v", planned.Result.Status, planned.Result.Warnings)
	}
}

func TestPlanValidatesClusterDerivedContainer(t *testing.T) {
	cfg := testConfig(t)
	cfg.OpenCenter.Cluster.ClusterName = "Bad_Cluster"
	_, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "velero", Backend: "s3", Cluster: "prod"}, Adapter: testAdapter()})
	if err == nil || !strings.Contains(err.Error(), "invalid S3 bucket") {
		t.Fatalf("invalid cluster-derived bucket accepted: %v", err)
	}
}

func TestPlanUsesCanonicalS3EndpointAndEtcdHost(t *testing.T) {
	cfg := testConfig(t)
	planned, err := Plan(context.Background(), PlanInput{Config: cfg, Options: Options{Service: "etcd-backup", Backend: "s3", Cluster: "prod"}, Adapter: &fakeAdapter{preflight: cloudopenstack.StoragePreflight{Endpoint: "https://swift.example/v1/AUTH_project-1", S3Endpoint: "https://s3.example:8443/api", AuthURL: "https://identity.example/v3", Region: "RegionOne", ProjectID: "project-1"}}})
	if err != nil {
		t.Fatal(err)
	}
	service := planned.prospective.OpenCenter.Services["etcd-backup"].(*services.EtcdBackupConfig)
	if service.S3Endpoint != "https://s3.example:8443/api" || service.S3Host != "s3.example:8443" {
		t.Fatalf("canonical S3 endpoint was not consumed: %+v", service)
	}
}

func TestRecoveryReservationIsExclusive(t *testing.T) {
	path := t.TempDir() + "/recovery.json"
	fs := OSFileSystem{}
	state := recoveryJournal{RecoveryState: RecoveryState{Path: path, Service: "loki", Backend: "s3", PersistenceState: "creation-pending"}}
	if err := fs.WriteRecovery(path, state); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteRecovery(path, state); err == nil {
		t.Fatal("second recovery reservation succeeded")
	}
}

func TestApplyOwnerPreflightFailureHasNoMutations(t *testing.T) {
	cfg := testConfig(t)
	raw, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	fs := &fakeFS{data: raw}
	adapter := testAdapter()
	adapter.preflightErr = errors.New("resolve storage credential owner: Keystone v3 token user ID is blank")
	result, err := Apply(context.Background(), ApplyInput{
		ConfigPath: "/tmp/prod.yaml", RecoveryPath: "/tmp/prod.recovery", OriginalBytes: raw,
		Options: Options{Service: "loki", Backend: "swift", Cluster: "prod"}, Adapter: adapter, FileSystem: fs,
	})
	if err == nil || result.Status == StatusPartial {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if adapter.containers != 0 || adapter.appCreates != 0 || adapter.ec2Creates != 0 || adapter.appDeletes != 0 || adapter.ec2Deletes != 0 {
		t.Fatalf("remote mutations occurred: adapter=%+v", adapter)
	}
	if fs.recovery || fs.backup || fs.atomic || fs.removed {
		t.Fatalf("local mutations occurred: fs=%+v", fs)
	}
}

func TestPlanHarborS3TreatsPlaceholdersAsMissing(t *testing.T) {
	cfg := harborStorageConfig(t)
	cfg.Secrets.Harbor.S3AccessKeyID = v2.PlaceholderSecret
	cfg.Secrets.Harbor.S3SecretAccessKey = v2.PlaceholderSecret
	planned, err := Plan(context.Background(), PlanInput{
		Config:  cfg,
		Options: Options{Service: "harbor", Backend: "s3", Cluster: "prod"},
		Adapter: testAdapter(),
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(planned.Result.RemoteActions) < 2 || planned.Result.RemoteActions[1].Action != "create" {
		t.Fatalf("remote actions=%v, want Harbor S3 credential creation", planned.Result.RemoteActions)
	}
}
