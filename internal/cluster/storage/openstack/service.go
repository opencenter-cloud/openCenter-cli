package openstack

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	cloudopenstack "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

const ResultSchemaVersion = 1

const (
	StatusPlanned  = "planned"
	StatusNoOp     = "no-op"
	StatusBlocked  = "blocked"
	StatusApplied  = "applied"
	StatusDeclined = "declined"
	StatusPartial  = "partial"
)

type Options struct {
	Service           string
	Backend           string
	Cluster           string
	Container         string
	S3Endpoint        string
	RotateCredentials bool
}

type ConfigChange struct {
	Path string `json:"path" yaml:"path"`
	Old  string `json:"old" yaml:"old"`
	New  string `json:"new" yaml:"new"`
}

type RemoteAction struct {
	Order    int    `json:"order" yaml:"order"`
	Action   string `json:"action" yaml:"action"`
	Resource string `json:"resource" yaml:"resource"`
	Name     string `json:"name,omitempty" yaml:"name,omitempty"`
	ID       string `json:"id,omitempty" yaml:"id,omitempty"`
	Scope    string `json:"scope,omitempty" yaml:"scope,omitempty"`
}

type RecoveryState struct {
	Path             string   `json:"path" yaml:"path"`
	CredentialType   string   `json:"credential_type" yaml:"credential_type"`
	S3Endpoint       string   `json:"s3_endpoint,omitempty" yaml:"s3_endpoint,omitempty"`
	Containers       []string `json:"containers,omitempty" yaml:"containers,omitempty"`
	Service          string   `json:"service" yaml:"service"`
	Backend          string   `json:"backend" yaml:"backend"`
	PersistenceState string   `json:"persistence_state" yaml:"persistence_state"`
}

// recoveryJournal is private durable state. CredentialID is intentionally not
// part of RecoveryState or Result so public output cannot expose revocation data.
type recoveryJournal struct {
	RecoveryState
	CredentialID string `json:"credential_id,omitempty" yaml:"credential_id,omitempty"`
}

type Result struct {
	SchemaVersion int            `json:"schema_version" yaml:"schema_version"`
	Operation     string         `json:"operation" yaml:"operation"`
	Status        string         `json:"status" yaml:"status"`
	Service       string         `json:"service" yaml:"service"`
	Backend       string         `json:"backend" yaml:"backend"`
	Container     string         `json:"container" yaml:"container"`
	Containers    []string       `json:"containers,omitempty" yaml:"containers,omitempty"`
	S3Endpoint    string         `json:"s3_endpoint,omitempty" yaml:"s3_endpoint,omitempty"`
	Changes       []ConfigChange `json:"changes" yaml:"changes"`
	RemoteActions []RemoteAction `json:"remote_actions" yaml:"remote_actions"`
	SecretPaths   []string       `json:"secret_paths" yaml:"secret_paths"`
	Warnings      []string       `json:"warnings,omitempty" yaml:"warnings,omitempty"`
	Recovery      *RecoveryState `json:"recovery,omitempty" yaml:"recovery,omitempty"`
}

type PlanInput struct {
	Config  *v2.Config
	Options Options
	Adapter cloudopenstack.StorageAdapter
}

type PlanOutput struct {
	Result      Result
	Preflight   cloudopenstack.StoragePreflight
	prospective *v2.Config
}

func restoreCredential(original, prospective *v2.Config, opts Options) {
	oldService, _ := serviceConfig(original, opts.Service)
	newService, _ := serviceConfig(prospective, opts.Service)
	switch old := oldService.(type) {
	case *services.LokiConfig:
		newCfg := newService.(*services.LokiConfig)
		if opts.Backend == "swift" {
			newCfg.SwiftApplicationCredentialID = old.SwiftApplicationCredentialID
			prospective.Secrets.Loki.SwiftApplicationCredentialSecret = original.Secrets.Loki.SwiftApplicationCredentialSecret
		} else {
			newCfg.S3CredentialID = old.S3CredentialID
			prospective.Secrets.Loki.S3AccessKeyID = original.Secrets.Loki.S3AccessKeyID
			prospective.Secrets.Loki.S3SecretAccessKey = original.Secrets.Loki.S3SecretAccessKey
		}
	case *services.TempoConfig:
		newCfg := newService.(*services.TempoConfig)
		if opts.Backend == "swift" {
			newCfg.SwiftApplicationCredentialID = old.SwiftApplicationCredentialID
			prospective.Secrets.Tempo.SwiftApplicationCredentialSecret = original.Secrets.Tempo.SwiftApplicationCredentialSecret
		} else {
			newCfg.S3CredentialID = old.S3CredentialID
			prospective.Secrets.Tempo.AccessKey = original.Secrets.Tempo.AccessKey
			prospective.Secrets.Tempo.SecretKey = original.Secrets.Tempo.SecretKey
		}
	case *services.EtcdBackupConfig:
		newService.(*services.EtcdBackupConfig).S3CredentialID = old.S3CredentialID
		prospective.Secrets.EtcdBackup = original.Secrets.EtcdBackup
	case *services.VeleroConfig:
		newService.(*services.VeleroConfig).S3CredentialID = old.S3CredentialID
		prospective.Secrets.Velero = original.Secrets.Velero
	case *services.HarborConfig:
		prospective.Secrets.Harbor = original.Secrets.Harbor
	case *services.MimirConfig:
		newCfg := newService.(*services.MimirConfig)
		if opts.Backend == "swift" {
			newCfg.SwiftApplicationCredentialID = old.SwiftApplicationCredentialID
			prospective.Secrets.Mimir.SwiftApplicationCredentialSecret = original.Secrets.Mimir.SwiftApplicationCredentialSecret
		} else {
			newCfg.S3CredentialID = old.S3CredentialID
			prospective.Secrets.Mimir.S3AccessKeyID = original.Secrets.Mimir.S3AccessKeyID
			prospective.Secrets.Mimir.S3SecretAccessKey = original.Secrets.Mimir.S3SecretAccessKey
		}
	}
}

func ValidateOptions(opts Options) error {
	opts.Service = strings.TrimSpace(opts.Service)
	opts.Backend = strings.ToLower(strings.TrimSpace(opts.Backend))
	if opts.Service == "" {
		return fmt.Errorf("exactly one service is required")
	}
	if opts.Backend != "swift" && opts.Backend != "s3" && opts.Backend != "none" && opts.Backend != "filesystem" {
		return fmt.Errorf("backend must be swift, s3, none, or filesystem")
	}
	// Tempo has no Swift storage backend upstream (rejects "unknown backend swift"
	// at startup), so tempo is S3-only here — consistent with the tempo plugin
	// validator. Loki supports both remote S3 and its explicit no-object-storage
	// mode; Harbor also supports its local filesystem mode, while
	// Velero and etcd-backup support an explicit no-object-storage mode.
	allowed := map[string]map[string]bool{
		"loki": {"swift": true, "s3": true, "none": true}, "tempo": {"s3": true}, "mimir": {"swift": true, "s3": true},
		"harbor": {"s3": true, "filesystem": true}, "etcd-backup": {"s3": true, "none": true}, "velero": {"s3": true, "none": true},
	}
	if !allowed[opts.Service][opts.Backend] {
		return fmt.Errorf("unsupported storage mapping %s=%s", opts.Service, opts.Backend)
	}
	if strings.TrimSpace(opts.Cluster) == "" {
		return fmt.Errorf("cluster is required")
	}
	if strings.TrimSpace(opts.Container) != "" && !isNonRemoteBackend(opts.Service, opts.Backend) {
		if err := validateContainerForBackend(strings.TrimSpace(opts.Container), opts.Backend); err != nil {
			return err
		}
	}
	if strings.TrimSpace(opts.S3Endpoint) != "" && !isNonRemoteBackend(opts.Service, opts.Backend) {
		if err := validateS3EndpointForService(opts.Service, opts.Backend, opts.S3Endpoint); err != nil {
			return err
		}
	}
	return nil
}

func Plan(ctx context.Context, input PlanInput) (PlanOutput, error) {
	input.Options.Service = strings.TrimSpace(input.Options.Service)
	input.Options.Backend = strings.ToLower(strings.TrimSpace(input.Options.Backend))
	if input.Config == nil {
		return PlanOutput{}, fmt.Errorf("configuration is required")
	}
	if err := ValidateOptions(input.Options); err != nil {
		return PlanOutput{}, err
	}
	serviceCfg, err := serviceConfig(input.Config, input.Options.Service)
	if err != nil {
		return PlanOutput{}, err
	}
	if input.Options.Service == "mimir" && input.Options.Backend == "s3" {
		mimir, ok := serviceCfg.(*services.MimirConfig)
		if !ok {
			return PlanOutput{}, fmt.Errorf("service %q has no canonical Mimir configuration", input.Options.Service)
		}
		endpoint := strings.TrimSpace(input.Options.S3Endpoint)
		if endpoint == "" {
			endpoint = strings.TrimSpace(mimir.S3Endpoint)
		}
		if endpoint != "" {
			if err := v2.ValidateMimirS3Endpoint(endpoint); err != nil {
				return PlanOutput{}, fmt.Errorf("invalid Mimir S3 endpoint: %w", err)
			}
		}
	}
	if isNonRemoteBackend(input.Options.Service, input.Options.Backend) {
		if input.Options.Backend == "filesystem" && v2.EffectiveStorageProfile(input.Config).Lifecycle == v2.StorageLifecycleProduction {
			return PlanOutput{}, fmt.Errorf("filesystem storage for %s is supported only in non-production environments", input.Options.Service)
		}
		prospective, err := cloneConfig(input.Config)
		if err != nil {
			return PlanOutput{}, fmt.Errorf("clone configuration for plan: %w", err)
		}
		prospectiveService, err := serviceConfig(prospective, input.Options.Service)
		if err != nil {
			return PlanOutput{}, err
		}
		changes := patchNonRemoteStorage(prospectiveService, &prospective.Secrets, input.Options)
		if harbor, ok := prospectiveService.(*services.HarborConfig); ok {
			if err := v2.ValidateHarborConfig(harbor); err != nil {
				return PlanOutput{}, fmt.Errorf("validate prospective Harbor configuration: %w", err)
			}
		}
		status := StatusPlanned
		if len(changes) == 0 {
			status = StatusNoOp
		}
		return PlanOutput{Result: Result{SchemaVersion: ResultSchemaVersion, Operation: "cluster.service.storage.plan", Status: status, Service: input.Options.Service, Backend: input.Options.Backend, Changes: changes}, prospective: prospective}, nil
	}
	if strings.ToLower(strings.TrimSpace(input.Config.OpenCenter.Infrastructure.Provider)) != "openstack" {
		return PlanOutput{}, fmt.Errorf("cluster provider is %q; OpenStack storage requires provider openstack", input.Config.OpenCenter.Infrastructure.Provider)
	}
	if input.Adapter == nil {
		return PlanOutput{}, fmt.Errorf("storage adapter is required")
	}
	container := strings.TrimSpace(input.Options.Container)
	if container == "" && input.Options.Service == "mimir" {
		container = strings.TrimSpace(serviceCfg.(*services.MimirConfig).BucketName)
	}
	if container == "" {
		container = defaultContainer(input.Config, input.Options.Service)
	}
	if err := validateContainerForBackend(container, input.Options.Backend); err != nil {
		return PlanOutput{}, err
	}
	if input.Options.Service == "mimir" && input.Options.Backend == "s3" {
		mimir, ok := serviceCfg.(*services.MimirConfig)
		if !ok {
			return PlanOutput{}, fmt.Errorf("service %q has no canonical Mimir configuration", input.Options.Service)
		}
		if err := validateMimirEffectiveBuckets(mimir, container); err != nil {
			return PlanOutput{}, err
		}
	}
	containers := effectiveContainers(input.Options.Service, input.Options.Backend, serviceCfg, container)
	for _, name := range containers {
		if err := validateContainerForBackend(name, input.Options.Backend); err != nil {
			return PlanOutput{}, fmt.Errorf("invalid effective storage container %q: %w", name, err)
		}
	}
	complete, partial := credentialState(input.Options.Service, input.Options.Backend, serviceCfg, input.Config.Secrets)
	resolveOwner := !complete || input.Options.RotateCredentials
	if partial && !input.Options.RotateCredentials {
		resolveOwner = false
	}
	preflight, err := input.Adapter.Preflight(ctx, input.Options.Backend, input.Options.S3Endpoint, resolveOwner)
	if err != nil {
		return PlanOutput{}, fmt.Errorf("OpenStack storage preflight failed: %w", redactError(err, input.Config))
	}
	storageEndpoint := strings.TrimSpace(preflight.Endpoint)
	if input.Options.Backend == "s3" {
		storageEndpoint = strings.TrimSpace(preflight.S3Endpoint)
		if err := validateS3EndpointForService(input.Options.Service, input.Options.Backend, storageEndpoint); err != nil {
			return PlanOutput{}, fmt.Errorf("OpenStack storage preflight returned invalid S3 endpoint: %w", err)
		}
	}
	if storageEndpoint == "" {
		return PlanOutput{}, fmt.Errorf("OpenStack storage preflight returned no usable storage endpoint")
	}
	preflight.Endpoint = storageEndpoint
	prospective, err := cloneConfig(input.Config)
	if err != nil {
		return PlanOutput{}, fmt.Errorf("clone configuration for plan: %w", err)
	}
	prospectiveService, err := serviceConfig(prospective, input.Options.Service)
	if err != nil {
		return PlanOutput{}, err
	}
	changes, secretPaths, _, _ := patchService(prospectiveService, &prospective.Secrets, input.Options, container, preflight, complete && !input.Options.RotateCredentials)
	if complete && !input.Options.RotateCredentials {
		restoreCredential(input.Config, prospective, input.Options)
	}
	if harbor, ok := prospectiveService.(*services.HarborConfig); ok {
		if err := v2.ValidateHarborConfig(harbor); err != nil {
			return PlanOutput{}, fmt.Errorf("validate prospective Harbor configuration: %w", err)
		}
	}
	oldCred := existingCredentialIDForConfig(input.Config, input.Options)
	result := Result{SchemaVersion: ResultSchemaVersion, Operation: "cluster.service.storage.plan", Status: StatusPlanned, Service: input.Options.Service, Backend: input.Options.Backend, Container: container, Containers: containers, S3Endpoint: storageEndpoint, Changes: changes, SecretPaths: secretPaths}
	for order, name := range containers {
		result.RemoteActions = append(result.RemoteActions, RemoteAction{Order: order + 1, Action: "ensure", Resource: "object-store-container", Name: name, Scope: "project"})
	}
	nextOrder := len(result.RemoteActions) + 1
	if partial && !input.Options.RotateCredentials {
		result.Status = StatusBlocked
		result.Warnings = append(result.Warnings, "credential pair is partial; use --rotate-credentials to replace it")
		return PlanOutput{Result: result, prospective: prospective, Preflight: preflight}, nil
	}
	if complete && !input.Options.RotateCredentials {
		result.RemoteActions = append(result.RemoteActions, RemoteAction{Order: nextOrder, Action: "reuse", Resource: credentialResource(input.Options.Backend), ID: publicCredentialID(input.Options.Service, oldCred), Scope: credentialScope(input.Options.Backend)})
	} else {
		result.RemoteActions = append(result.RemoteActions, RemoteAction{Order: nextOrder, Action: "create", Resource: credentialResource(input.Options.Backend), Scope: credentialScope(input.Options.Backend)})
		if oldCred != "" {
			result.RemoteActions = append(result.RemoteActions, RemoteAction{Order: nextOrder + 2, Action: "revoke", Resource: credentialResource(input.Options.Backend), ID: publicCredentialID(input.Options.Service, oldCred), Scope: credentialScope(input.Options.Backend)})
		}
	}
	result.RemoteActions = append(result.RemoteActions, RemoteAction{Order: nextOrder + 1, Action: "persist", Resource: "typed-configuration", Scope: "local"})
	if len(changes) == 0 && complete && !input.Options.RotateCredentials {
		result.Status = StatusNoOp
	}
	return PlanOutput{Result: result, prospective: prospective, Preflight: preflight}, nil
}

func Apply(ctx context.Context, input ApplyInput) (Result, error) {
	if input.FileSystem == nil {
		input.FileSystem = OSFileSystem{}
	}
	current, err := input.FileSystem.ReadFile(input.ConfigPath)
	if err != nil {
		return Result{}, err
	}
	if input.OriginalBytes != nil && !bytes.Equal(current, input.OriginalBytes) {
		return Result{}, fmt.Errorf("configuration changed since planning; re-run the operation")
	}
	cfg, err := v2.DecodePublicConfig(current)
	if err != nil {
		return Result{}, fmt.Errorf("decode configuration: %w", err)
	}
	planned, err := Plan(ctx, PlanInput{Config: cfg, Options: input.Options, Adapter: input.Adapter})
	if err != nil {
		return Result{}, err
	}
	result := planned.Result
	result.Operation = "cluster.service.storage.apply"
	if result.Status == StatusBlocked {
		return result, nil
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if len(result.Containers) > 0 {
		if input.Adapter == nil {
			return Result{}, fmt.Errorf("storage adapter is required")
		}
		for _, container := range result.Containers {
			if err := input.Adapter.EnsureContainer(ctx, cloudopenstack.ContainerRequest{Name: container, Region: planned.Preflight.Region}); err != nil {
				return Result{}, fmt.Errorf("ensure storage container %q: %w", container, redactError(err, cfg))
			}
		}
	}
	if result.Status == StatusNoOp {
		return result, nil
	}
	if input.Validate != nil {
		if err := input.Validate(planned.prospective); err != nil {
			return Result{}, fmt.Errorf("prospective configuration validation failed: %w", redactError(err, cfg))
		}
	}
	complete, rotate := hasCompleteCredential(cfg, input.Options)
	if len(result.RemoteActions) == 0 {
		complete, rotate = true, false
	}
	oldID := existingCredentialIDForConfig(cfg, input.Options)
	var createdType, createdID string
	var createdSecret, createdAccess string
	var journal recoveryJournal
	needsCredential := !complete || rotate
	if needsCredential {
		journal = recoveryJournal{RecoveryState: RecoveryState{Path: input.RecoveryPath, CredentialType: credentialType(input.Options.Backend), S3Endpoint: result.S3Endpoint, Containers: append([]string(nil), result.Containers...), Service: result.Service, Backend: result.Backend, PersistenceState: "creation-pending"}}
		if err := input.FileSystem.WriteRecovery(input.RecoveryPath, journal); err != nil {
			return Result{}, fmt.Errorf("reserve recovery record: %w", redactError(err, cfg))
		}
		result.Recovery = &journal.RecoveryState
		var createErr error
		if input.Options.Backend == "swift" {
			var created cloudopenstack.AppCredential
			created, createErr = input.Adapter.CreateAppCredential(ctx, cloudopenstack.AppCredentialRequest{UserID: planned.Preflight.CredentialOwnerID, Name: result.Service + "-" + result.Backend, Description: "OpenCenter storage credential for " + result.Service, AccessRules: scopedRules(planned.Result.Container, planned.Preflight.ProjectID)})
			createdType, createdID, createdSecret = "application", created.ID, created.Secret
		} else {
			var created cloudopenstack.EC2Credentials
			created, createErr = input.Adapter.CreateEC2Credentials(ctx, cloudopenstack.EC2CredentialRequest{UserID: planned.Preflight.CredentialOwnerID, ProjectID: planned.Preflight.ProjectID, ProjectName: cfg.OpenCenter.Meta.Name, UserName: result.Service + "-s3-user"})
			createdType, createdID, createdAccess, createdSecret = "ec2", created.AccessKeyID, created.AccessKeyID, created.Secret
		}
		if createErr != nil {
			if removeErr := input.FileSystem.RemoveRecovery(input.RecoveryPath); removeErr != nil {
				return partialResult(result, fmt.Errorf("create storage credential: %w; remove recovery record: %v", redactError(createErr, cfg), redactError(removeErr, cfg)))
			}
			result.Recovery = nil
			return Result{}, fmt.Errorf("create storage credential: %w", redactError(createErr, cfg))
		}
		journal.CredentialType = createdType
		journal.CredentialID = createdID
		result.Recovery.CredentialType = createdType
		result.Recovery.PersistenceState = "created-not-persisted"
		if err := input.FileSystem.UpdateRecovery(input.RecoveryPath, journal); err != nil {
			return partialResult(result, fmt.Errorf("update recovery record after credential creation: %w", redactError(err, cfg, createdAccess, createdSecret)))
		}
		applyCredential(planned.prospective, input.Options, createdID, createdAccess, createdSecret)
		if input.Validate != nil {
			if err := input.Validate(planned.prospective); err != nil {
				return partialResult(result, fmt.Errorf("validate persisted credential configuration: %w", redactError(err, cfg, createdAccess, createdSecret)))
			}
		}
	}
	finalBytes, err := v2.MarshalPublicConfig(planned.prospective)
	if err != nil {
		return partialResult(result, fmt.Errorf("marshal updated configuration: %w", redactError(err, cfg, createdAccess, createdSecret)))
	}
	latest, err := input.FileSystem.ReadFile(input.ConfigPath)
	if err != nil {
		return partialResult(result, fmt.Errorf("re-read configuration before persistence: %w", redactError(err, cfg, createdAccess, createdSecret)))
	}
	if !bytes.Equal(latest, current) {
		return partialResult(result, redactError(fmt.Errorf("configuration changed before persistence; created credential requires recovery"), cfg, createdAccess, createdSecret))
	}
	if err := input.FileSystem.Backup(input.ConfigPath+".backup", latest); err != nil {
		return partialResult(result, fmt.Errorf("backup configuration before persistence: %w", redactError(err, cfg, createdAccess, createdSecret)))
	}
	if err := input.FileSystem.WriteAtomic(input.ConfigPath, finalBytes); err != nil {
		return partialResult(result, fmt.Errorf("persist updated configuration: %w", redactError(err, cfg, createdAccess, createdSecret)))
	}
	result.Status = StatusApplied
	if result.Recovery != nil {
		result.Recovery.PersistenceState = "persisted-revoke-pending"
		if err := input.FileSystem.UpdateRecovery(input.RecoveryPath, journal); err != nil {
			return partialResult(result, fmt.Errorf("update recovery record after persistence: %w", redactError(err, cfg, createdAccess, createdSecret)))
		}
	}
	if rotate && oldID != "" {
		var revokeErr error
		if input.Options.Backend == "swift" {
			revokeErr = input.Adapter.DeleteAppCredential(ctx, oldID, planned.Preflight.CredentialOwnerID)
		} else {
			revokeErr = input.Adapter.DeleteEC2Credentials(ctx, oldID, planned.Preflight.CredentialOwnerID)
		}
		if revokeErr != nil {
			result.Status = StatusPartial
			if result.Recovery != nil {
				result.Recovery.PersistenceState = "persisted-revoke-failed"
				if recoveryErr := input.FileSystem.UpdateRecovery(input.RecoveryPath, journal); recoveryErr != nil {
					result.Warnings = append(result.Warnings, redactError(fmt.Errorf("retain recovery record after revoke failure: %w", recoveryErr), cfg, createdAccess, createdSecret).Error())
				}
			}
			result.Warnings = append(result.Warnings, redactError(fmt.Errorf("old credential revocation failed: %w", revokeErr), cfg, createdAccess, createdSecret).Error())
			return result, redactError(revokeErr, cfg, createdAccess, createdSecret)
		}
	}
	if result.Recovery != nil {
		result.Recovery.PersistenceState = "revoked"
		if err := input.FileSystem.UpdateRecovery(input.RecoveryPath, journal); err != nil {
			return partialResult(result, fmt.Errorf("update recovery record after revoke: %w", redactError(err, cfg, createdAccess, createdSecret)))
		}
		if err := input.FileSystem.RemoveRecovery(input.RecoveryPath); err != nil {
			return partialResult(result, fmt.Errorf("remove recovery record: %w", redactError(err, cfg, createdAccess, createdSecret)))
		}
		result.Recovery = nil
	}
	return result, nil
}

type ApplyInput struct {
	ConfigPath    string
	RecoveryPath  string
	OriginalBytes []byte
	Options       Options
	Adapter       cloudopenstack.StorageAdapter
	FileSystem    FileSystem
	Validate      func(*v2.Config) error
}

type FileSystem interface {
	ReadFile(string) ([]byte, error)
	CheckRecoveryPath(string) error
	Backup(string, []byte) error
	WriteAtomic(string, []byte) error
	WriteRecovery(string, recoveryJournal) error
	UpdateRecovery(string, recoveryJournal) error
	RemoveRecovery(string) error
}

type OSFileSystem struct{}

func (OSFileSystem) ReadFile(path string) ([]byte, error) { return os.ReadFile(path) }
func (OSFileSystem) WriteAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencenter-storage-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(info.Mode().Perm()); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func (OSFileSystem) CheckRecoveryPath(path string) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("recovery record already exists at %s", path)
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(filepath.Dir(path), ".opencenter-recovery-check-"+filepath.Base(path)), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	name := f.Name()
	if syncErr := f.Sync(); syncErr != nil {
		err = syncErr
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if removeErr := os.Remove(name); err == nil {
		err = removeErr
	}
	if err == nil {
		err = syncDirectory(filepath.Dir(path))
	}
	return err
}
func (OSFileSystem) Backup(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err = f.Write(data); err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err == nil {
		err = syncDirectory(filepath.Dir(path))
	}
	return err
}
func (OSFileSystem) WriteRecovery(path string, state recoveryJournal) error {
	return writeRecoveryExclusive(path, state)
}
func (OSFileSystem) UpdateRecovery(path string, state recoveryJournal) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".opencenter-recovery-update-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err = tmp.Chmod(0o600); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(tmpName, path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func (OSFileSystem) RemoveRecovery(path string) error {
	if err := os.Remove(path); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func writeRecoveryExclusive(path string, state recoveryJournal) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if err = f.Chmod(0o600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(path))
}
func syncDirectory(path string) error {
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	err = dir.Sync()
	if closeErr := dir.Close(); err == nil {
		err = closeErr
	}
	return err
}

func partialResult(result Result, err error) (Result, error) {
	result.Status = StatusPartial
	return result, err
}

func serviceConfig(cfg *v2.Config, name string) (any, error) {
	value, ok := cfg.OpenCenter.Services[name]
	if !ok || value == nil {
		return nil, fmt.Errorf("service %q is not configured", name)
	}
	switch typed := value.(type) {
	case *services.LokiConfig, *services.TempoConfig, *services.MimirConfig, *services.HarborConfig, *services.EtcdBackupConfig, *services.VeleroConfig:
		return typed, nil
	default:
		return nil, fmt.Errorf("service %q has no canonical typed configuration", name)
	}
}

func isNonRemoteBackend(service, backend string) bool {
	switch service {
	case "loki", "etcd-backup", "velero":
		return backend == "none"
	case "harbor":
		return backend == "filesystem"
	default:
		return false
	}
}

// IsNonRemoteBackend reports whether the backend needs no OpenStack cloud,
// container, endpoint, or credential provisioning.
func IsNonRemoteBackend(service, backend string) bool {
	return isNonRemoteBackend(strings.ToLower(strings.TrimSpace(service)), strings.ToLower(strings.TrimSpace(backend)))
}

func patchNonRemoteStorage(service any, secrets *v2.SecretsConfig, opts Options) []ConfigChange {
	changes := make([]ConfigChange, 0)
	set := func(path, old, next string) {
		if old != next {
			changes = append(changes, ConfigChange{Path: path, Old: redacted(path, old), New: redacted(path, next)})
		}
	}
	clear := func(path string, value *string) {
		set(path, *value, "")
		*value = ""
	}
	switch typed := service.(type) {
	case *services.LokiConfig:
		set("opencenter.services.loki.storage_type", typed.StorageType, opts.Backend)
		typed.StorageType = opts.Backend
		clear("opencenter.services.loki.bucket_name", &typed.BucketName)
		clear("opencenter.services.loki.s3_endpoint", &typed.S3Endpoint)
		clear("opencenter.services.loki.s3_region", &typed.S3Region)
		clear("opencenter.services.loki.s3_credential_id", &typed.S3CredentialID)
		clear("opencenter.services.loki.swift_application_credential_id", &typed.SwiftApplicationCredentialID)
		secrets.Loki.S3AccessKeyID, secrets.Loki.S3SecretAccessKey = "", ""
		secrets.Loki.SwiftApplicationCredentialSecret = ""
	case *services.HarborConfig:
		set("opencenter.services.harbor.storage_type", typed.StorageType, opts.Backend)
		typed.StorageType = opts.Backend
		clear("opencenter.services.harbor.s3_bucket", &typed.S3Bucket)
		clear("opencenter.services.harbor.s3_region", &typed.S3Region)
		clear("opencenter.services.harbor.s3_endpoint", &typed.S3Endpoint)
		secrets.Harbor.S3AccessKeyID, secrets.Harbor.S3SecretAccessKey = "", ""
	case *services.EtcdBackupConfig:
		set("opencenter.services.etcd-backup.storage_type", typed.StorageType, opts.Backend)
		typed.StorageType = opts.Backend
		clear("opencenter.services.etcd-backup.s3_host", &typed.S3Host)
		clear("opencenter.services.etcd-backup.s3_endpoint", &typed.S3Endpoint)
		clear("opencenter.services.etcd-backup.s3_bucket_name", &typed.S3BucketName)
		clear("opencenter.services.etcd-backup.s3_region", &typed.S3Region)
		clear("opencenter.services.etcd-backup.s3_credential_id", &typed.S3CredentialID)
		secrets.EtcdBackup.AccessKeyID, secrets.EtcdBackup.SecretAccessKey = "", ""
	case *services.VeleroConfig:
		set("opencenter.services.velero.storage_type", typed.StorageType, opts.Backend)
		typed.StorageType = opts.Backend
		clear("opencenter.services.velero.backup_bucket", &typed.BackupBucket)
		clear("opencenter.services.velero.region", &typed.Region)
		clear("opencenter.services.velero.s3_endpoint", &typed.S3Endpoint)
		clear("opencenter.services.velero.s3_region", &typed.S3Region)
		clear("opencenter.services.velero.s3_credential_id", &typed.S3CredentialID)
		secrets.Velero.AccessKeyID, secrets.Velero.SecretAccessKey = "", ""
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes
}

func patchService(service any, secrets *v2.SecretsConfig, opts Options, container string, preflight cloudopenstack.StoragePreflight, reuse bool) ([]ConfigChange, []string, []string, string) {
	changes := []ConfigChange{}
	secretPaths := []string{}
	sensitive := []string{}
	oldID := ""
	set := func(path string, old, new string) {
		if old != new {
			changes = append(changes, ConfigChange{Path: path, Old: redacted(path, old), New: redacted(path, new)})
		}
	}
	setSecret := func(path string, old, new string) {
		secretPaths = append(secretPaths, path)
		sensitive = append(sensitive, new)
		set(path, old, new)
	}
	setCredential := func(path string, old, new string) {
		if !reuse {
			set(path, old, new)
		}
	}
	setCredentialSecret := func(path string, old, new string) {
		if !reuse {
			setSecret(path, old, new)
		}
	}
	backend := opts.Backend
	authURL := preflight.AuthURL
	if authURL == "" {
		authURL = preflight.Endpoint
	}
	switch typed := service.(type) {
	case *services.LokiConfig:
		oldID = typed.SwiftApplicationCredentialID
		if backend == "s3" {
			oldID = typed.S3CredentialID
		}
		set("opencenter.services.loki.storage_type", typed.StorageType, backend)
		typed.StorageType = backend
		set("opencenter.services.loki.bucket_name", typed.BucketName, container)
		typed.BucketName = container
		if backend == "swift" {
			set("opencenter.services.loki.swift_auth_url", typed.SwiftAuthURL, authURL)
			typed.SwiftAuthURL = authURL
			set("opencenter.services.loki.swift_region", typed.SwiftRegion, preflight.Region)
			typed.SwiftRegion = preflight.Region
			set("opencenter.services.loki.swift_container_name", typed.SwiftContainerName, container)
			typed.SwiftContainerName = container
			setCredential("opencenter.services.loki.swift_application_credential_id", typed.SwiftApplicationCredentialID, "[generated]")
			typed.SwiftApplicationCredentialID = "[generated]"
			setCredentialSecret("secrets.loki.swift_application_credential_secret", secrets.Loki.SwiftApplicationCredentialSecret, "[generated]")
		} else {
			set("opencenter.services.loki.s3_endpoint", typed.S3Endpoint, preflight.Endpoint)
			typed.S3Endpoint = preflight.Endpoint
			set("opencenter.services.loki.s3_region", typed.S3Region, preflight.Region)
			typed.S3Region = preflight.Region
			set("opencenter.services.loki.s3_force_path_style", fmt.Sprint(typed.S3ForcePathStyle), "true")
			typed.S3ForcePathStyle = true
			set("opencenter.services.loki.s3_insecure", fmt.Sprint(typed.S3Insecure), fmt.Sprint(strings.HasPrefix(preflight.Endpoint, "http://")))
			typed.S3Insecure = strings.HasPrefix(preflight.Endpoint, "http://")
			setCredential("opencenter.services.loki.s3_credential_id", typed.S3CredentialID, "[generated]")
			typed.S3CredentialID = "[generated]"
			setCredentialSecret("secrets.loki.s3_access_key_id", secrets.Loki.S3AccessKeyID, "[generated]")
			setCredentialSecret("secrets.loki.s3_secret_access_key", secrets.Loki.S3SecretAccessKey, "[generated]")
		}
	case *services.TempoConfig:
		oldID = typed.SwiftApplicationCredentialID
		if backend == "s3" {
			oldID = typed.S3CredentialID
		}
		set("opencenter.services.tempo.storage_type", typed.StorageType, backend)
		typed.StorageType = backend
		set("opencenter.services.tempo.bucket_name", typed.BucketName, container)
		typed.BucketName = container
		if backend == "swift" {
			set("opencenter.services.tempo.swift_auth_url", typed.SwiftAuthURL, authURL)
			typed.SwiftAuthURL = authURL
			set("opencenter.services.tempo.swift_region", typed.SwiftRegion, preflight.Region)
			typed.SwiftRegion = preflight.Region
			set("opencenter.services.tempo.swift_container_name", typed.SwiftContainerName, container)
			typed.SwiftContainerName = container
			setCredential("opencenter.services.tempo.swift_application_credential_id", typed.SwiftApplicationCredentialID, "[generated]")
			typed.SwiftApplicationCredentialID = "[generated]"
			setCredentialSecret("secrets.tempo.swift_application_credential_secret", secrets.Tempo.SwiftApplicationCredentialSecret, "[generated]")
		} else {
			set("opencenter.services.tempo.s3_endpoint", typed.S3Endpoint, preflight.Endpoint)
			typed.S3Endpoint = preflight.Endpoint
			set("opencenter.services.tempo.s3_region", typed.S3Region, preflight.Region)
			typed.S3Region = preflight.Region
			set("opencenter.services.tempo.s3_force_path_style", fmt.Sprint(typed.S3ForcePathStyle), "true")
			typed.S3ForcePathStyle = true
			set("opencenter.services.tempo.s3_insecure", fmt.Sprint(typed.S3Insecure), fmt.Sprint(strings.HasPrefix(preflight.Endpoint, "http://")))
			typed.S3Insecure = strings.HasPrefix(preflight.Endpoint, "http://")
			setCredential("opencenter.services.tempo.s3_credential_id", typed.S3CredentialID, "[generated]")
			typed.S3CredentialID = "[generated]"
			setCredentialSecret("secrets.tempo.access_key", secrets.Tempo.AccessKey, "[generated]")
			setCredentialSecret("secrets.tempo.secret_key", secrets.Tempo.SecretKey, "[generated]")
		}
	case *services.MimirConfig:
		if backend == "swift" {
			oldID = typed.SwiftApplicationCredentialID
		} else {
			oldID = typed.S3CredentialID
		}
		set("opencenter.services.mimir.storage_type", typed.StorageType, backend)
		typed.StorageType = backend
		set("opencenter.services.mimir.bucket_name", typed.BucketName, container)
		typed.BucketName = container
		if backend == "swift" {
			authURL := preflight.AuthURL
			if authURL == "" {
				authURL = preflight.Endpoint
			}
			set("opencenter.services.mimir.swift_auth_url", typed.SwiftAuthURL, authURL)
			typed.SwiftAuthURL = authURL
			set("opencenter.services.mimir.swift_region", typed.SwiftRegion, preflight.Region)
			typed.SwiftRegion = preflight.Region
			set("opencenter.services.mimir.swift_container_name", typed.SwiftContainerName, container)
			typed.SwiftContainerName = container
			setCredential("opencenter.services.mimir.swift_application_credential_id", typed.SwiftApplicationCredentialID, "[generated]")
			typed.SwiftApplicationCredentialID = "[generated]"
			setCredentialSecret("secrets.mimir.swift_application_credential_secret", secrets.Mimir.SwiftApplicationCredentialSecret, "[generated]")
		} else {
			set("opencenter.services.mimir.s3_endpoint", typed.S3Endpoint, preflight.Endpoint)
			typed.S3Endpoint = preflight.Endpoint
			set("opencenter.services.mimir.s3_region", typed.S3Region, preflight.Region)
			typed.S3Region = preflight.Region
			set("opencenter.services.mimir.s3_force_path_style", fmt.Sprint(typed.S3ForcePathStyle), "true")
			typed.S3ForcePathStyle = true
			set("opencenter.services.mimir.s3_insecure", fmt.Sprint(typed.S3Insecure), fmt.Sprint(strings.HasPrefix(preflight.Endpoint, "http://")))
			typed.S3Insecure = strings.HasPrefix(preflight.Endpoint, "http://")
			buckets := mimirEffectiveBuckets(typed, container)
			set("opencenter.services.mimir.ruler_bucket_name", typed.RulerBucketName, buckets[1])
			typed.RulerBucketName = buckets[1]
			set("opencenter.services.mimir.alertmanager_bucket_name", typed.AlertmanagerBucketName, buckets[2])
			typed.AlertmanagerBucketName = buckets[2]
			setCredential("opencenter.services.mimir.s3_credential_id", typed.S3CredentialID, "[generated]")
			typed.S3CredentialID = "[generated]"
			setCredentialSecret("secrets.mimir.s3_access_key_id", secrets.Mimir.S3AccessKeyID, "[generated]")
			setCredentialSecret("secrets.mimir.s3_secret_access_key", secrets.Mimir.S3SecretAccessKey, "[generated]")
		}
	case *services.EtcdBackupConfig:
		oldID = typed.S3CredentialID
		set("opencenter.services.etcd-backup.storage_type", typed.StorageType, "s3")
		typed.StorageType = "s3"
		host := endpointHost(preflight.Endpoint)
		set("opencenter.services.etcd-backup.s3_host", typed.S3Host, host)
		typed.S3Host = host
		set("opencenter.services.etcd-backup.s3_endpoint", typed.S3Endpoint, preflight.Endpoint)
		typed.S3Endpoint = preflight.Endpoint
		set("opencenter.services.etcd-backup.s3_bucket_name", typed.S3BucketName, container)
		typed.S3BucketName = container
		set("opencenter.services.etcd-backup.s3_region", typed.S3Region, preflight.Region)
		typed.S3Region = preflight.Region
		setCredential("opencenter.services.etcd-backup.s3_credential_id", typed.S3CredentialID, "[generated]")
		typed.S3CredentialID = "[generated]"
		setCredentialSecret("secrets.etcd_backup.access_key_id", secrets.EtcdBackup.AccessKeyID, "[generated]")
		setCredentialSecret("secrets.etcd_backup.secret_access_key", secrets.EtcdBackup.SecretAccessKey, "[generated]")
	case *services.VeleroConfig:
		oldID = typed.S3CredentialID
		set("opencenter.services.velero.storage_type", typed.StorageType, backend)
		typed.StorageType = backend
		set("opencenter.services.velero.backup_bucket", typed.BackupBucket, container)
		typed.BackupBucket = container
		set("opencenter.services.velero.region", typed.Region, preflight.Region)
		typed.Region = preflight.Region
		set("opencenter.services.velero.s3_endpoint", typed.S3Endpoint, preflight.Endpoint)
		typed.S3Endpoint = preflight.Endpoint
		set("opencenter.services.velero.s3_region", typed.S3Region, preflight.Region)
		typed.S3Region = preflight.Region
		set("opencenter.services.velero.s3_force_path_style", fmt.Sprint(typed.S3ForcePathStyle), "true")
		typed.S3ForcePathStyle = true
		set("opencenter.services.velero.s3_insecure", fmt.Sprint(typed.S3Insecure), fmt.Sprint(strings.HasPrefix(preflight.Endpoint, "http://")))
		typed.S3Insecure = strings.HasPrefix(preflight.Endpoint, "http://")
		setCredential("opencenter.services.velero.s3_credential_id", typed.S3CredentialID, "[generated]")
		typed.S3CredentialID = "[generated]"
		setCredentialSecret("secrets.velero.access_key_id", secrets.Velero.AccessKeyID, "[generated]")
		setCredentialSecret("secrets.velero.secret_access_key", secrets.Velero.SecretAccessKey, "[generated]")
	case *services.HarborConfig:
		oldID = secrets.Harbor.S3AccessKeyID
		set("opencenter.services.harbor.storage_type", typed.StorageType, backend)
		typed.StorageType = backend
		set("opencenter.services.harbor.s3_bucket", typed.S3Bucket, container)
		typed.S3Bucket = container
		set("opencenter.services.harbor.s3_region", typed.S3Region, preflight.Region)
		typed.S3Region = preflight.Region
		set("opencenter.services.harbor.s3_endpoint", typed.S3Endpoint, preflight.S3Endpoint)
		typed.S3Endpoint = preflight.S3Endpoint
		setCredentialSecret("secrets.harbor.s3_access_key_id", secrets.Harbor.S3AccessKeyID, "[generated]")
		setCredentialSecret("secrets.harbor.s3_secret_access_key", secrets.Harbor.S3SecretAccessKey, "[generated]")
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	sort.Strings(secretPaths)
	return changes, secretPaths, sensitive, oldID
}

func credentialState(service, backend string, cfg any, secrets v2.SecretsConfig) (bool, bool) {
	complete, partial := false, false
	id := existingCredentialIDFromConfig(cfg, backend)
	var access, secret string
	switch service {
	case "loki":
		if backend == "swift" {
			secret = secrets.Loki.SwiftApplicationCredentialSecret
		} else {
			access, secret = secrets.Loki.S3AccessKeyID, secrets.Loki.S3SecretAccessKey
		}
	case "tempo":
		if backend == "swift" {
			secret = secrets.Tempo.SwiftApplicationCredentialSecret
		} else {
			access, secret = secrets.Tempo.AccessKey, secrets.Tempo.SecretKey
		}
	case "mimir":
		if backend == "swift" {
			secret = secrets.Mimir.SwiftApplicationCredentialSecret
		} else {
			access, secret = secrets.Mimir.S3AccessKeyID, secrets.Mimir.S3SecretAccessKey
		}
	case "etcd-backup":
		access, secret = secrets.EtcdBackup.AccessKeyID, secrets.EtcdBackup.SecretAccessKey
	case "velero":
		access, secret = secrets.Velero.AccessKeyID, secrets.Velero.SecretAccessKey
	case "harbor":
		access, secret = secrets.Harbor.S3AccessKeyID, secrets.Harbor.S3SecretAccessKey
		id = access
	}
	if service == "harbor" && backend == "s3" {
		accessPresent := usableCredentialValue(access)
		secretPresent := usableCredentialValue(secret)
		return accessPresent && secretPresent, (accessPresent || secretPresent) && (!accessPresent || !secretPresent)
	}
	if backend == "swift" {
		complete = id != "" && secret != ""
		partial = (id != "" || secret != "") && !complete
	} else {
		complete = id != "" && access != "" && secret != ""
		partial = (id != "" || access != "" || secret != "") && !complete
	}
	return complete, partial
}

func hasCompleteCredential(cfg *v2.Config, opts Options) (bool, bool) {
	svc, _ := serviceConfig(cfg, opts.Service)
	complete, _ := credentialState(opts.Service, opts.Backend, svc, cfg.Secrets)
	return complete, opts.RotateCredentials
}
func existingCredentialIDFromConfig(cfg any, backend string) string {
	switch s := cfg.(type) {
	case *services.LokiConfig:
		if backend == "swift" {
			return s.SwiftApplicationCredentialID
		}
		return s.S3CredentialID
	case *services.TempoConfig:
		if backend == "swift" {
			return s.SwiftApplicationCredentialID
		}
		return s.S3CredentialID
	case *services.MimirConfig:
		if backend == "swift" {
			return s.SwiftApplicationCredentialID
		}
		return s.S3CredentialID
	case *services.EtcdBackupConfig:
		return s.S3CredentialID
	case *services.VeleroConfig:
		return s.S3CredentialID
	}
	return ""
}

func existingCredentialIDForConfig(cfg *v2.Config, opts Options) string {
	if opts.Service == "harbor" && opts.Backend == "s3" {
		return cfg.Secrets.Harbor.S3AccessKeyID
	}
	svc, _ := serviceConfig(cfg, opts.Service)
	return existingCredentialIDFromConfig(svc, opts.Backend)
}

func applyCredential(cfg *v2.Config, opts Options, id, access, secret string) {
	svc, _ := serviceConfig(cfg, opts.Service)
	switch s := svc.(type) {
	case *services.LokiConfig:
		if opts.Backend == "swift" {
			s.SwiftApplicationCredentialID = id
			cfg.Secrets.Loki.SwiftApplicationCredentialSecret = secret
		} else {
			s.S3CredentialID = id
			cfg.Secrets.Loki.S3AccessKeyID = access
			cfg.Secrets.Loki.S3SecretAccessKey = secret
		}
	case *services.TempoConfig:
		if opts.Backend == "swift" {
			s.SwiftApplicationCredentialID = id
			cfg.Secrets.Tempo.SwiftApplicationCredentialSecret = secret
		} else {
			s.S3CredentialID = id
			cfg.Secrets.Tempo.AccessKey = access
			cfg.Secrets.Tempo.SecretKey = secret
		}
	case *services.MimirConfig:
		if opts.Backend == "swift" {
			s.SwiftApplicationCredentialID = id
			cfg.Secrets.Mimir.SwiftApplicationCredentialSecret = secret
		} else {
			s.S3CredentialID = id
			cfg.Secrets.Mimir.S3AccessKeyID = access
			cfg.Secrets.Mimir.S3SecretAccessKey = secret
		}
	case *services.EtcdBackupConfig:
		s.S3CredentialID = id
		cfg.Secrets.EtcdBackup.AccessKeyID = access
		cfg.Secrets.EtcdBackup.SecretAccessKey = secret
	case *services.VeleroConfig:
		s.S3CredentialID = id
		cfg.Secrets.Velero.AccessKeyID = access
		cfg.Secrets.Velero.SecretAccessKey = secret
	case *services.HarborConfig:
		cfg.Secrets.Harbor.S3AccessKeyID = access
		cfg.Secrets.Harbor.S3SecretAccessKey = secret
	}
}

func cloneConfig(cfg *v2.Config) (copyCfg *v2.Config, err error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			copyCfg = nil
			err = fmt.Errorf("marshal configuration: %v", recovered)
		}
	}()
	data, err := v2.MarshalPublicConfig(cfg)
	if err != nil {
		return nil, err
	}
	copyCfg, err = v2.DecodePublicConfig(data)
	if err != nil {
		return nil, err
	}
	return copyCfg, nil
}
func defaultContainer(cfg *v2.Config, service string) string {
	cluster := cfg.ClusterName()
	if cluster == "" {
		cluster = strings.TrimSpace(cfg.OpenCenter.Meta.Name)
	}
	return strings.Trim(cluster, "-") + "-" + service
}

// mimirEffectiveBuckets mirrors the values consumed by the Mimir S3
// renderer. The blocks bucket is supplied by the storage operation, while the
// two auxiliary stores retain configured overrides and otherwise derive from
// that blocks bucket.
func mimirEffectiveBuckets(cfg *services.MimirConfig, blocks string) [3]string {
	blocks = strings.TrimSpace(blocks)
	ruler := strings.TrimSpace(cfg.RulerBucketName)
	if ruler == "" {
		ruler = blocks + "-ruler"
	}
	alertmanager := strings.TrimSpace(cfg.AlertmanagerBucketName)
	if alertmanager == "" {
		alertmanager = blocks + "-alertmanager"
	}
	return [3]string{blocks, ruler, alertmanager}
}

func validateMimirEffectiveBuckets(cfg *services.MimirConfig, blocks string) error {
	buckets := mimirEffectiveBuckets(cfg, blocks)
	labels := [...]string{"blocks", "ruler", "alertmanager"}
	for i := 0; i < len(buckets); i++ {
		for j := i + 1; j < len(buckets); j++ {
			if buckets[i] == buckets[j] {
				return fmt.Errorf("Mimir S3 bucket names must be distinct: %s and %s both resolve to %q; set distinct bucket names", labels[i], labels[j], buckets[i])
			}
		}
	}
	return nil
}

func effectiveContainers(service, backend string, cfg any, container string) []string {
	if service != "mimir" || backend != "s3" {
		return []string{strings.TrimSpace(container)}
	}
	mimir, ok := cfg.(*services.MimirConfig)
	if !ok {
		return []string{strings.TrimSpace(container)}
	}
	buckets := mimirEffectiveBuckets(mimir, container)
	result := make([]string, 0, len(buckets))
	for _, bucket := range buckets {
		bucket = strings.TrimSpace(bucket)
		if bucket == "" {
			continue
		}
		result = append(result, bucket)
	}
	return result
}

func validateContainerForBackend(name, backend string) error {
	if backend == "s3" {
		return validateS3Bucket(name)
	}
	return validateContainer(name)
}
func validateContainer(name string) error {
	if len(name) == 0 || len(name) > 255 || strings.ContainsAny(name, "/\\") {
		return fmt.Errorf("invalid Swift container name")
	}
	return nil
}
func validateS3Bucket(name string) error {
	if len(name) < 3 || len(name) > 63 || strings.ToLower(name) != name || strings.HasPrefix(name, ".") || strings.HasSuffix(name, ".") || strings.HasPrefix(name, "-") || strings.HasSuffix(name, "-") || strings.Contains(name, "..") {
		return fmt.Errorf("invalid S3 bucket name")
	}
	for _, part := range strings.Split(name, ".") {
		if part == "" || strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
			return fmt.Errorf("invalid S3 bucket name")
		}
		for _, r := range part {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return fmt.Errorf("invalid S3 bucket name")
			}
		}
	}
	return nil
}
func validateS3Endpoint(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" || u.User != nil {
		return fmt.Errorf("S3 endpoint must be an absolute HTTP(S) URL")
	}
	if strings.Contains(strings.ToUpper(u.Path), "/V1/AUTH_") {
		return fmt.Errorf("S3 endpoint must not be a Swift /v1/AUTH_* endpoint")
	}
	return nil
}

func validateS3EndpointForService(service, backend, raw string) error {
	if service == "mimir" && backend == "s3" {
		return v2.ValidateMimirS3Endpoint(raw)
	}
	return validateS3Endpoint(raw)
}

func endpointHost(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}
	return u.Host
}
func credentialType(backend string) string {
	if backend == "swift" {
		return "application"
	}
	return "ec2"
}

func credentialResource(backend string) string {
	if backend == "swift" {
		return "keystone-application-credential"
	}
	return "keystone-ec2-credential"
}
func credentialScope(backend string) string {
	if backend == "swift" {
		return "object-store container and segments"
	}
	return "project"
}
func usableCredentialValue(value string) bool {
	trimmed := strings.TrimSpace(value)
	return trimmed != "" && !strings.EqualFold(trimmed, v2.PlaceholderSecret)
}

func redacted(path, value string) string {
	if strings.Contains(path, "secret") || strings.Contains(path, "access_key") || strings.Contains(path, "password") || value == "[generated]" {
		if value == "" {
			return ""
		}
		return "[redacted]"
	}
	return value
}
func scopedRules(container, project string) []cloudopenstack.AccessRule {
	base := "/v1/AUTH_" + project + "/" + container
	segments := base + "_segments"
	rules := []cloudopenstack.AccessRule{}
	for _, path := range []string{base, segments} {
		rules = append(rules, cloudopenstack.AccessRule{Service: "object-store", Method: "GET", Path: path}, cloudopenstack.AccessRule{Service: "object-store", Method: "HEAD", Path: path})
		methods := []string{"GET", "HEAD", "PUT", "DELETE"}
		if path == base {
			methods = append(methods, "POST")
		}
		for _, method := range methods {
			rules = append(rules, cloudopenstack.AccessRule{Service: "object-store", Method: method, Path: path + "/**"})
		}
	}
	return rules
}
func (o Options) String() string { return o.Service + "=" + o.Backend }
func redactError(err error, cfg *v2.Config, extra ...string) error {
	if err == nil {
		return nil
	}
	values := append([]string{}, extra...)
	if cfg != nil {
		values = append(values,
			cfg.Secrets.Loki.SwiftApplicationCredentialSecret, cfg.Secrets.Loki.S3AccessKeyID, cfg.Secrets.Loki.S3SecretAccessKey,
			cfg.Secrets.Tempo.SwiftApplicationCredentialSecret, cfg.Secrets.Tempo.AccessKey, cfg.Secrets.Tempo.SecretKey,
			cfg.Secrets.Mimir.SwiftApplicationCredentialSecret, cfg.Secrets.Mimir.S3AccessKeyID, cfg.Secrets.Mimir.S3SecretAccessKey,
			cfg.Secrets.EtcdBackup.AccessKeyID, cfg.Secrets.EtcdBackup.SecretAccessKey,
			cfg.Secrets.Velero.AccessKeyID, cfg.Secrets.Velero.SecretAccessKey,
			cfg.Secrets.Harbor.S3AccessKeyID, cfg.Secrets.Harbor.S3SecretAccessKey,
			cfg.Secrets.Global.OpenStackPassword,
		)
	}
	message := security.MaskSecrets(err.Error(), values...)
	return fmt.Errorf("%s", message)
}

func publicCredentialID(service, id string) string {
	if service == "harbor" {
		return ""
	}
	return id
}
