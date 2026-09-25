// Package rustfs manages the disposable RustFS instance used by local Kind
// development.
package rustfs

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
)

const (
	// DefaultImage is an immutable official RustFS image reference. Do not
	// replace it with a floating tag: local development must be reproducible.
	DefaultImage         = "docker.io/rustfs/rustfs:1.0.0@sha256:8cc9801755448b71a786705ce76692c77e14936cccd87cf2fc31842e58f4d1ff"
	defaultContainerName = "rustfs"
	defaultAPIPort       = 9000
	defaultConsolePort   = 9001
	defaultKindNetwork   = "kind"
	readinessTimeout     = 90 * time.Second
	// Keep the disposable probe image immutable; never use a floating tag here.
	kindProbeImage      = "public.ecr.aws/aws-cli/aws-cli:2.36.22@sha256:5a4cc81c75d7b08ba6daa9439599812d00ed3b497a82a9b4bdc771a77c74268c"
	managedByLabel      = "com.opencenter.local.managed-by"
	serviceLabel        = "com.opencenter.local.service"
	specHashLabel       = "com.opencenter.local.spec-sha256"
	credentialHashLabel = "com.opencenter.local.credentials-sha256"
	managedByValue      = "opencenter-local"
	serviceValue        = "rustfs"
)

// Settings configures the local RustFS container.
type Settings struct {
	Runtime        string
	Image          string
	ContainerName  string
	APIPort        int
	ConsolePort    int
	Credentials    *Credentials
	KindNetwork    string
	AutoAttachKind bool
	Explicit       ExplicitSettings
}

// ExplicitSettings identifies command-line overrides. All other settings are
// replaced by persisted metadata when it exists.
type ExplicitSettings struct {
	Runtime        bool
	Image          bool
	APIPort        bool
	ConsolePort    bool
	KindNetwork    bool
	AutoAttachKind bool
}

// Credentials are accepted programmatically or from ParseCredentials. They
// are never included in Status or Metadata.
type Credentials struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// Metadata persists non-secret local RustFS configuration.
type Metadata struct {
	Runtime        string `json:"runtime"`
	Image          string `json:"image"`
	ContainerName  string `json:"container_name"`
	APIPort        int    `json:"api_port"`
	ConsolePort    int    `json:"console_port"`
	KindNetwork    string `json:"kind_network"`
	AutoAttachKind bool   `json:"auto_attach_kind"`
	CredentialHash string `json:"credential_hash,omitempty"`
	SpecHash       string `json:"spec_hash"`
}

// Status reports local RustFS state without exposing credentials.
type Status struct {
	Metadata                Metadata
	APIURL                  string
	ConsoleURL              string
	HealthURL               string
	CredentialsPath         string
	CredentialsExists       bool
	AttachedNetworks        []string
	KindAttached            bool
	KindIP                  string
	KindReachable           bool
	KindReachabilityChecked bool
	Running                 bool
	APIReady                bool
	HealthReady             bool
	Ready                   bool
}

// AttachResult reports the state after connecting RustFS to Kind.
type AttachResult struct {
	Status
}

// Layout is the RustFS-specific portion of local-dev state.
type Layout struct {
	Root            string
	DataDir         string
	MetadataPath    string
	CredentialsPath string
	EnvPath         string
}

// DefaultSettings returns reproducible local RustFS defaults.
func DefaultSettings(runtime string) Settings {
	return Settings{
		Runtime:        kindprovider.ResolveRuntime(runtime),
		Image:          DefaultImage,
		ContainerName:  defaultContainerName,
		APIPort:        defaultAPIPort,
		ConsolePort:    defaultConsolePort,
		KindNetwork:    defaultKindNetwork,
		AutoAttachKind: false,
	}
}

// NewService returns a RustFS service rooted below the shared local-dev state
// directory. The RustFS data directory is never removed by Up.
func NewService(executor localdev.Executor, stateDir string, settings Settings) (*Service, error) {
	if executor == nil {
		executor = localdev.NewExecutor()
	}
	base, err := localdev.ResolveLayout(stateDir)
	if err != nil {
		return nil, err
	}
	if settings.Runtime == "" {
		settings.Runtime = kindprovider.ResolveRuntime("")
	}
	if settings.Image == "" {
		settings.Image = DefaultImage
	}
	if settings.ContainerName == "" {
		settings.ContainerName = defaultContainerName
	}
	if settings.APIPort == 0 {
		settings.APIPort = defaultAPIPort
	}
	if settings.ConsolePort == 0 {
		settings.ConsolePort = defaultConsolePort
	}
	if settings.KindNetwork == "" {
		settings.KindNetwork = defaultKindNetwork
	}
	root := filepath.Join(base.Root, "rustfs")
	service := &Service{
		executor: executor,
		layout: Layout{
			Root:            root,
			DataDir:         filepath.Join(root, "data"),
			MetadataPath:    filepath.Join(root, "rustfs.json"),
			CredentialsPath: filepath.Join(root, "credentials.json"),
			EnvPath:         filepath.Join(root, "credentials.env"),
		},
		settings: settings,
	}
	if err := service.layout.validateManagedState(); err != nil {
		return nil, err
	}
	metadata, metadataErr := service.loadMetadata()
	if metadataErr == nil {
		service.settings = mergePersistedSettings(service.settings, metadata)
		if !hasSpecOverride(service.settings) && metadata.SpecHash != service.specHash() {
			return nil, fmt.Errorf("RustFS metadata spec hash does not match its effective configuration")
		}
	} else if !os.IsNotExist(metadataErr) {
		return nil, fmt.Errorf("load RustFS metadata: %w", metadataErr)
	}
	if err := validateSettings(service.settings); err != nil {
		return nil, err
	}
	return service, nil
}

// Service manages the disposable local RustFS instance.
type Service struct {
	executor localdev.Executor
	layout   Layout
	settings Settings
}

// Layout returns the resolved RustFS state layout.
func (s *Service) Layout() Layout { return s.layout }

// Up creates or starts RustFS, waits for both its S3 and health endpoints, and
// opportunistically joins the Kind network when that network already exists.
func (s *Service) Up(ctx context.Context) (*Status, error) {
	if err := s.layout.ensure(); err != nil {
		return nil, err
	}
	container, err := s.inspectContainer(ctx)
	if err != nil {
		return nil, err
	}
	if container != nil {
		if err := s.validateManagedContainer(container, nil); err != nil {
			return nil, err
		}
	}
	credentials, err := s.ensureCredentials(container != nil)
	if err != nil {
		return nil, err
	}
	if err := s.ensureContainer(ctx, container, credentials); err != nil {
		return nil, err
	}
	if _, err := s.waitForReady(ctx, credentials); err != nil {
		return nil, err
	}
	// Kind attachment is intentionally explicit. It requires a kubeconfig and
	// the authenticated temporary-pod probe performed by attach-kind.
	if err := s.saveMetadata(credentials); err != nil {
		return nil, err
	}
	return s.Status(ctx)
}

// Status reports RustFS state. Credential values are intentionally not part of
// Status and therefore cannot be printed by callers using this type.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	if err := s.layout.validateManagedState(); err != nil {
		return nil, err
	}
	metadata, err := s.loadMetadata()
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("load RustFS metadata: %w", err)
		}
		metadata = s.metadata()
	}
	container, err := s.inspectContainer(ctx)
	if err != nil {
		return nil, err
	}
	running := container != nil && container.State.Running
	networks := []string(nil)
	kindIP := ""
	var credentials Credentials
	if container != nil {
		networks = container.networks()
		kindIP = container.kindIP(metadata.KindNetwork)
		if err := s.validateManagedContainer(container, nil); err != nil {
			return nil, err
		}
		credentials, err = s.readPersistedCredentials()
		if err != nil {
			return nil, fmt.Errorf("RustFS container exists but persisted credentials are unusable: %w", err)
		}
		if err := s.validateExplicitCredentials(credentials); err != nil {
			return nil, err
		}
		if err := s.validateManagedContainer(container, &credentials); err != nil {
			return nil, err
		}
	}
	apiReady, healthReady := false, false
	kindReachable := false
	if running {
		apiReady, healthReady = s.probeReady(credentials)
		if kindIP != "" {
			kindReachable = probeKindNetwork(credentials, kindIP)
		}
	}

	return &Status{
		Metadata:                metadata,
		APIURL:                  fmt.Sprintf("http://127.0.0.1:%d", metadata.APIPort),
		ConsoleURL:              fmt.Sprintf("http://127.0.0.1:%d", metadata.ConsolePort),
		HealthURL:               fmt.Sprintf("http://127.0.0.1:%d/health/ready", metadata.APIPort),
		CredentialsPath:         s.layout.CredentialsPath,
		CredentialsExists:       fileExists(s.layout.CredentialsPath),
		AttachedNetworks:        networks,
		KindAttached:            kindIP != "",
		KindIP:                  kindIP,
		KindReachable:           kindReachable,
		KindReachabilityChecked: running && kindIP != "",
		Running:                 running,
		APIReady:                apiReady,
		HealthReady:             healthReady,
		Ready:                   running && apiReady && healthReady,
	}, nil
}

// Destroy removes the container and all disposable RustFS state, including
// the persisted data directory.
func (s *Service) Destroy(ctx context.Context) error {
	if err := s.layout.validateManagedState(); err != nil {
		return err
	}
	container, err := s.inspectContainer(ctx)
	if err != nil {
		return err
	}
	if container != nil {
		if err := s.validateManagedContainer(container, nil); err != nil {
			return err
		}
		credentials, credentialsErr := s.readPersistedCredentials()
		if credentialsErr != nil {
			return fmt.Errorf("RustFS container exists but persisted credentials are unusable: %w", credentialsErr)
		}
		if err := s.validateExplicitCredentials(credentials); err != nil {
			return err
		}
		if err := s.validateManagedContainer(container, &credentials); err != nil {
			return err
		}
	} else if _, statErr := os.Lstat(s.layout.CredentialsPath); statErr == nil {
		credentials, credentialsErr := s.readPersistedCredentials()
		if credentialsErr != nil {
			return credentialsErr
		}
		if err := s.validateExplicitCredentials(credentials); err != nil {
			return err
		}
	}
	if err := s.stopAndRemove(ctx, container != nil); err != nil {
		return err
	}
	if err := os.RemoveAll(s.layout.Root); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", s.layout.Root, err)
	}
	return nil
}

// AttachKind requires a kubeconfig so attachment cannot claim workload
// reachability without probing an actual cluster. Use AttachKindWithKubeconfig
// from the CLI path.
func (s *Service) AttachKind(ctx context.Context) (*AttachResult, error) {
	return nil, fmt.Errorf("RustFS Kind attachment requires a kubeconfig for the authenticated workload probe")
}

// AttachKindWithKubeconfig connects RustFS to Kind and runs an authenticated
// S3 probe from a disposable temporary pod. The pod and its temporary Secret
// are deleted before returning.
func (s *Service) AttachKindWithKubeconfig(ctx context.Context, kubeconfigPath string) (*AttachResult, error) {
	if strings.TrimSpace(kubeconfigPath) == "" {
		return nil, fmt.Errorf("RustFS Kind attachment requires a kubeconfig")
	}
	container, err := s.inspectContainer(ctx)
	if err != nil {
		return nil, err
	}
	if container == nil {
		return nil, fmt.Errorf("rustfs container %q does not exist; run rustfs up first", s.settings.ContainerName)
	}
	credentials, err := s.readPersistedCredentials()
	if err != nil {
		return nil, err
	}
	if err := s.validateExplicitCredentials(credentials); err != nil {
		return nil, err
	}
	if err := s.validateManagedContainer(container, &credentials); err != nil {
		return nil, err
	}
	if err := s.connectKindNetwork(ctx); err != nil {
		return nil, err
	}
	status, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	if !status.KindAttached {
		return nil, fmt.Errorf("rustfs container is not attached to the %s network", s.settings.KindNetwork)
	}
	if err := s.probeKindWorkload(ctx, kubeconfigPath, status.KindIP, credentials); err != nil {
		return nil, err
	}
	return &AttachResult{Status: *status}, nil
}

func (s *Service) probeKindWorkload(ctx context.Context, kubeconfigPath, kindIP string, credentials Credentials) (err error) {
	name := fmt.Sprintf("opencenter-rustfs-probe-%d", time.Now().UnixNano())
	secretName := name + "-auth"
	manifest := fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
type: Opaque
data:
  AWS_ACCESS_KEY_ID: %s
  AWS_SECRET_ACCESS_KEY: %s
---
apiVersion: v1
kind: Pod
metadata:
  name: %s
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext:
    runAsNonRoot: true
    runAsUser: 65532
    runAsGroup: 65532
    seccompProfile:
      type: RuntimeDefault
  containers:
  - name: s3-probe
    image: %s
    securityContext:
      allowPrivilegeEscalation: false
      readOnlyRootFilesystem: true
      capabilities:
        drop:
        - ALL
    env:
    - name: AWS_ACCESS_KEY_ID
      valueFrom:
        secretKeyRef:
          name: %s
          key: AWS_ACCESS_KEY_ID
    - name: AWS_SECRET_ACCESS_KEY
      valueFrom:
        secretKeyRef:
          name: %s
          key: AWS_SECRET_ACCESS_KEY
    - name: HOME
      value: /dev
    - name: AWS_CONFIG_FILE
      value: /dev/null
    - name: AWS_SHARED_CREDENTIALS_FILE
      value: /dev/null
    - name: AWS_CLI_HISTORY_FILE
      value: /dev/null
    - name: AWS_PAGER
      value: ""
    - name: AWS_EC2_METADATA_DISABLED
      value: "true"
    command: ["aws", "s3api", "list-buckets", "--endpoint-url", "http://%s:9000", "--region", "us-east-1"]
`, secretName, base64.StdEncoding.EncodeToString([]byte(credentials.AccessKey)), base64.StdEncoding.EncodeToString([]byte(credentials.SecretKey)), name, kindProbeImage, secretName, secretName, kindIP)

	kubectl := func(args ...string) ([]byte, error) {
		return s.executor.Run(ctx, localdev.RunOptions{Name: "kubectl", Args: append([]string{"--kubeconfig", kubeconfigPath}, args...)})
	}
	defer func() {
		if _, cleanupErr := kubectl("delete", "pod", name, "--ignore-not-found", "--wait=false"); cleanupErr != nil && err == nil {
			err = fmt.Errorf("delete RustFS Kind S3 probe: %w", cleanupErr)
		}
		if _, cleanupErr := kubectl("delete", "secret", secretName, "--ignore-not-found"); cleanupErr != nil && err == nil {
			err = fmt.Errorf("delete RustFS Kind probe credentials: %w", cleanupErr)
		}
	}()
	if _, err := s.executor.Run(ctx, localdev.RunOptions{
		Name:  "kubectl",
		Args:  []string{"--kubeconfig", kubeconfigPath, "apply", "-f", "-"},
		Stdin: strings.NewReader(manifest),
	}); err != nil {
		return fmt.Errorf("create RustFS Kind S3 probe: %w", err)
	}
	if _, err := kubectl("wait", "--for=jsonpath={.status.phase}=Succeeded", "--timeout=90s", "pod/"+name); err != nil {
		return fmt.Errorf("authenticated RustFS S3 probe from Kind failed: %w", err)
	}
	if _, err := kubectl("logs", "pod/"+name); err != nil {
		return fmt.Errorf("read authenticated RustFS S3 probe result: %w", err)
	}
	return nil
}

func (l Layout) ensure() error {
	if err := l.validateManagedState(); err != nil {
		return err
	}
	if err := os.MkdirAll(l.DataDir, 0o777); err != nil {
		return fmt.Errorf("create RustFS data directory: %w", err)
	}
	// The official image runs as UID 10001. A rootless runtime cannot chown a
	// host bind mount to that UID, so keep the disposable object-data mount
	// writable while protecting credentials and metadata separately.
	if err := os.Chmod(l.DataDir, 0o777); err != nil {
		return fmt.Errorf("make RustFS data directory writable: %w", err)
	}
	if err := os.Chmod(l.Root, 0o700); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("protect RustFS state directory: %w", err)
	}
	return nil
}

func (l Layout) validateManagedState() error {
	for _, path := range []string{l.Root, l.DataDir, l.MetadataPath, l.CredentialsPath, l.EnvPath} {
		info, err := os.Lstat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("inspect RustFS state %s: %w", path, err)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing symlinked RustFS state path %s", path)
		}
	}
	return nil
}

// ParseCredentials parses the JSON credentials file accepted by the CLI.
func ParseCredentials(r io.Reader) (*Credentials, error) {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	var current Credentials
	if err := decoder.Decode(&current); err != nil {
		return nil, fmt.Errorf("parse RustFS credentials: %w", err)
	}
	var extra struct{}
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return nil, fmt.Errorf("parse RustFS credentials: multiple JSON values")
		}
		return nil, fmt.Errorf("parse RustFS credentials: %w", err)
	}
	if err := validateCredentials(current); err != nil {
		return nil, err
	}
	return &current, nil
}

func (s *Service) ensureCredentials(existingContainer bool) (Credentials, error) {
	current, err := s.readPersistedCredentials()
	if err == nil {
		if err := s.validateExplicitCredentials(current); err != nil {
			return Credentials{}, err
		}
		if metadata, metadataErr := s.loadMetadata(); metadataErr == nil && metadata.CredentialHash != credentialHash(current) {
			return Credentials{}, fmt.Errorf("persisted RustFS credential hash does not match metadata; rotate by destroying and recreating RustFS")
		} else if metadataErr != nil && !os.IsNotExist(metadataErr) {
			return Credentials{}, fmt.Errorf("load RustFS metadata: %w", metadataErr)
		}
		if err := s.writeCredentialsEnv(current); err != nil {
			return Credentials{}, err
		}
		return current, nil
	}
	if existingContainer {
		if os.IsNotExist(err) {
			return Credentials{}, fmt.Errorf("RustFS container exists but persisted credentials are missing")
		}
		return Credentials{}, fmt.Errorf("RustFS container exists but persisted credentials are unusable: %w", err)
	}
	if !os.IsNotExist(err) {
		return Credentials{}, err
	}

	current = Credentials{}
	if s.settings.Credentials != nil {
		current = *s.settings.Credentials
	}
	if current.AccessKey == "" {
		current.AccessKey, err = randomSecret(16)
		if err != nil {
			return Credentials{}, fmt.Errorf("generate RustFS access key: %w", err)
		}
	}
	if current.SecretKey == "" {
		current.SecretKey, err = randomSecret(32)
		if err != nil {
			return Credentials{}, fmt.Errorf("generate RustFS secret key: %w", err)
		}
	}
	if err := validateCredentials(current); err != nil {
		return Credentials{}, err
	}

	data, err := json.MarshalIndent(current, "", "  ")
	if err != nil {
		return Credentials{}, fmt.Errorf("marshal RustFS credentials: %w", err)
	}
	if err := writePrivateFile(s.layout.CredentialsPath, append(data, '\n')); err != nil {
		return Credentials{}, fmt.Errorf("write RustFS credentials: %w", err)
	}
	if err := s.writeCredentialsEnv(current); err != nil {
		return Credentials{}, err
	}
	return current, nil
}

func (s *Service) validateExplicitCredentials(persisted Credentials) error {
	if s.settings.Credentials != nil && credentialHash(*s.settings.Credentials) != credentialHash(persisted) {
		return fmt.Errorf("supplied RustFS credentials do not match persisted credentials; rotate by destroying and recreating RustFS")
	}
	return nil
}

func (s *Service) readPersistedCredentials() (Credentials, error) {
	info, err := os.Lstat(s.layout.CredentialsPath)
	if err != nil {
		return Credentials{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Credentials{}, fmt.Errorf("refusing symlinked RustFS credentials %s", s.layout.CredentialsPath)
	}
	data, err := os.Open(s.layout.CredentialsPath)
	if err != nil {
		return Credentials{}, fmt.Errorf("open RustFS credentials: %w", err)
	}
	defer data.Close()
	parsed, err := ParseCredentials(data)
	if err != nil {
		return Credentials{}, err
	}
	if err := os.Chmod(s.layout.CredentialsPath, 0o600); err != nil {
		return Credentials{}, fmt.Errorf("protect RustFS credentials: %w", err)
	}
	return *parsed, nil
}

func (s *Service) writeCredentialsEnv(current Credentials) error {
	env := fmt.Sprintf("RUSTFS_ACCESS_KEY=%s\nRUSTFS_SECRET_KEY=%s\n", current.AccessKey, current.SecretKey)
	if err := writePrivateFile(s.layout.EnvPath, []byte(env)); err != nil {
		return fmt.Errorf("write RustFS environment: %w", err)
	}
	return nil
}

func (s *Service) ensureContainer(ctx context.Context, container *containerInspect, credentials Credentials) error {
	if container == nil {
		return s.runContainer(ctx, credentials)
	}
	if err := s.validateManagedContainer(container, &credentials); err != nil {
		return err
	}
	if !container.State.Running {
		if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: s.commandRuntime(), Args: []string{"start", s.settings.ContainerName}}); err != nil {
			return fmt.Errorf("start RustFS container: %w", err)
		}
	}
	return nil
}

func (s *Service) runContainer(ctx context.Context, credentials Credentials) error {
	labels := s.containerLabels(credentials)
	args := []string{
		"run", "-d",
		"--name", s.settings.ContainerName,
		"--user", "10001:10001",
		"--env-file", s.layout.EnvPath,
		"-v", s.dataMount(),
		"-p", fmt.Sprintf("127.0.0.1:%d:9000", s.settings.APIPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:9001", s.settings.ConsolePort),
	}
	for key, value := range labels {
		args = append(args, "--label", key+"="+value)
	}
	args = append(args, s.settings.Image)
	args = append(args, expectedCommand()...)
	if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: s.commandRuntime(), Args: args}); err != nil {
		return fmt.Errorf("start RustFS container: %w", err)
	}
	return nil
}

func (s *Service) waitForReady(ctx context.Context, credentials Credentials) (*readiness, error) {
	deadline := time.Now().Add(readinessTimeout)
	for {
		apiReady, healthReady := s.probeReady(credentials)
		if apiReady && healthReady {
			return &readiness{APIReady: apiReady, HealthReady: healthReady}, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for RustFS S3 API and health readiness at http://localhost:%d", s.settings.APIPort)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

type readiness struct {
	APIReady    bool
	HealthReady bool
}

func (s *Service) probeReady(credentials Credentials) (bool, bool) {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	apiReady := probeS3API(client, fmt.Sprintf("http://127.0.0.1:%d/", s.settings.APIPort), credentials)
	healthReady := probeURL(client, fmt.Sprintf("http://127.0.0.1:%d/health/ready", s.settings.APIPort), false)
	if !healthReady {
		// Older RustFS images expose /health while retaining the same readiness
		// semantics. Keep the primary endpoint explicit and support that image too.
		healthReady = probeURL(client, fmt.Sprintf("http://127.0.0.1:%d/health", s.settings.APIPort), false)
	}
	return apiReady, healthReady
}

// probeKindNetwork validates the container endpoint from the host through its
// Kind-network IP. This is deliberately not described as a Kubernetes pod
// probe: that requires a live kubeconfig and belongs to integration tests.
func probeKindNetwork(credentials Credentials, kindIP string) bool {
	client := &http.Client{Timeout: 750 * time.Millisecond}
	health := probeURL(client, "http://"+kindIP+":9000/health/ready", false)
	s3 := probeS3API(client, "http://"+kindIP+":9000/", credentials)
	return health && s3
}

func probeURL(client *http.Client, rawURL string, s3API bool) bool {
	resp, err := client.Get(rawURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if s3API {
		return resp.StatusCode >= 200 && resp.StatusCode < 300
	}
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// probeS3API signs the request with the persisted credentials. A successful
// authenticated response, rather than an arbitrary response from port 9000,
// proves both S3 readiness and credential validity.
func probeS3API(client *http.Client, rawURL string, credentials Credentials) bool {
	req, err := signedS3Request(rawURL, credentials, time.Now().UTC())
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

func signedS3Request(rawURL string, credentials Credentials, now time.Time) (*http.Request, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	date := now.Format("20060102")
	amzDate := now.Format("20060102T150405Z")
	payloadHash := sha256.Sum256(nil)
	payloadHex := hex.EncodeToString(payloadHash[:])
	host := parsed.Host
	canonicalHeaders := "host:" + host + "\n" + "x-amz-content-sha256:" + payloadHex + "\n" + "x-amz-date:" + amzDate + "\n"
	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalRequest := "GET\n/\n\n" + canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHex
	canonicalHash := sha256.Sum256([]byte(canonicalRequest))
	scope := date + "/us-east-1/s3/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + scope + "\n" + hex.EncodeToString(canonicalHash[:])
	signingKey := hmacSHA256(hmacSHA256(hmacSHA256(hmacSHA256([]byte("AWS4"+credentials.SecretKey), []byte(date)), []byte("us-east-1")), []byte("s3")), []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Host = host
	req.Header.Set("Host", host)
	req.Header.Set("x-amz-content-sha256", payloadHex)
	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+credentials.AccessKey+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
	return req, nil
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write(data)
	return h.Sum(nil)
}

func validateCredentials(current Credentials) error {
	if !validCredential(current.AccessKey, false) || !validCredential(current.SecretKey, true) {
		return fmt.Errorf("RustFS credentials must be non-empty and cannot contain whitespace, newline, or '='")
	}
	return nil
}

func isNotFoundError(err error) bool {
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "no such") || strings.Contains(lower, "not found") || strings.Contains(lower, "does not exist")
}

type containerSpec struct {
	Image       string `json:"image"`
	DataDir     string `json:"data_dir"`
	APIPort     int    `json:"api_port"`
	ConsolePort int    `json:"console_port"`
	User        string `json:"user"`
	KindNetwork string `json:"kind_network"`
	Command     string `json:"command"`
}

func (s *Service) specHash() string {
	spec, _ := json.Marshal(containerSpec{
		Image:       s.settings.Image,
		DataDir:     s.layout.DataDir,
		APIPort:     s.settings.APIPort,
		ConsolePort: s.settings.ConsolePort,
		User:        "10001:10001",
		KindNetwork: s.settings.KindNetwork,
		Command:     "/data --address :9000 --console-address :9001",
	})
	hash := sha256.Sum256(spec)
	return hex.EncodeToString(hash[:])
}

func credentialHash(credentials Credentials) string {
	hash := sha256.Sum256([]byte(credentials.AccessKey + "\x00" + credentials.SecretKey))
	return hex.EncodeToString(hash[:])
}

func (s *Service) containerLabels(credentials Credentials) map[string]string {
	return map[string]string{
		managedByLabel:      managedByValue,
		serviceLabel:        serviceValue,
		specHashLabel:       s.specHash(),
		credentialHashLabel: credentialHash(credentials),
	}
}

func (s *Service) dataMount() string {
	mount := s.layout.DataDir + ":/data"
	if strings.EqualFold(s.settings.Runtime, "podman") {
		return mount + ":Z"
	}
	return mount
}

type containerInspect struct {
	Config struct {
		Image  string            `json:"Image"`
		User   string            `json:"User"`
		Cmd    []string          `json:"Cmd"`
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
	State struct {
		Running bool `json:"Running"`
	} `json:"State"`
	Mounts []struct {
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
	HostConfig struct {
		PortBindings map[string][]struct {
			HostIP   string `json:"HostIp"`
			HostPort string `json:"HostPort"`
		} `json:"PortBindings"`
	} `json:"HostConfig"`
	NetworkSettings struct {
		Networks map[string]struct {
			IPAddress string `json:"IPAddress"`
		} `json:"Networks"`
	} `json:"NetworkSettings"`
}

func (c *containerInspect) networks() []string {
	networks := make([]string, 0, len(c.NetworkSettings.Networks))
	for network := range c.NetworkSettings.Networks {
		networks = append(networks, network)
	}
	sort.Strings(networks)
	return networks
}

func (c *containerInspect) kindIP(network string) string {
	if current, ok := c.NetworkSettings.Networks[network]; ok {
		return current.IPAddress
	}
	return ""
}

func (s *Service) inspectContainer(ctx context.Context) (*containerInspect, error) {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"inspect", "--format", "{{json .}}", s.settings.ContainerName},
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("inspect RustFS container: %w", err)
	}
	var container containerInspect
	if err := json.Unmarshal(output, &container); err != nil {
		return nil, fmt.Errorf("parse RustFS container inspection: %w", err)
	}
	return &container, nil
}

func (s *Service) validateManagedContainer(container *containerInspect, credentials *Credentials) error {
	labels := container.Config.Labels
	if labels[managedByLabel] != managedByValue || labels[serviceLabel] != serviceValue {
		return fmt.Errorf("refusing RustFS container %q: ownership labels do not identify openCenter RustFS", s.settings.ContainerName)
	}
	if labels[specHashLabel] != s.specHash() {
		return fmt.Errorf("RustFS container %q has configuration drift; destroy it explicitly before recreating", s.settings.ContainerName)
	}
	if container.Config.Image != s.settings.Image {
		return fmt.Errorf("RustFS container %q image drifted from %q to %q", s.settings.ContainerName, s.settings.Image, container.Config.Image)
	}
	if container.Config.User != "10001:10001" {
		return fmt.Errorf("RustFS container %q user configuration drifted to %q", s.settings.ContainerName, container.Config.User)
	}
	if !reflect.DeepEqual(container.Config.Cmd, expectedCommand()) {
		return fmt.Errorf("RustFS container %q command configuration drifted", s.settings.ContainerName)
	}
	if len(container.Mounts) != 1 || container.Mounts[0].Destination != "/data" || filepath.Clean(container.Mounts[0].Source) != filepath.Clean(s.layout.DataDir) {
		return fmt.Errorf("RustFS container %q data mount is absent or drifted", s.settings.ContainerName)
	}
	if len(container.HostConfig.PortBindings) != 2 || !hasPublishedPort(container, "9000/tcp", s.settings.APIPort) || !hasPublishedPort(container, "9001/tcp", s.settings.ConsolePort) {
		return fmt.Errorf("RustFS container %q published loopback ports are absent or drifted", s.settings.ContainerName)
	}
	for _, port := range []string{"9000/tcp", "9001/tcp"} {
		for _, binding := range container.HostConfig.PortBindings[port] {
			if binding.HostIP != "127.0.0.1" {
				return fmt.Errorf("RustFS container %q published %s outside loopback", s.settings.ContainerName, port)
			}
		}
	}
	if credentials != nil && labels[credentialHashLabel] != credentialHash(*credentials) {
		return fmt.Errorf("RustFS container %q credential state does not match persisted credentials", s.settings.ContainerName)
	}
	return nil
}

func hasPublishedPort(container *containerInspect, containerPort string, hostPort int) bool {
	if len(container.HostConfig.PortBindings[containerPort]) != 1 {
		return false
	}
	for _, binding := range container.HostConfig.PortBindings[containerPort] {
		if binding.HostPort == fmt.Sprint(hostPort) && binding.HostIP == "127.0.0.1" {
			return true
		}
	}
	return false
}

func expectedCommand() []string {
	return []string{"/data", "--address", ":9000", "--console-address", ":9001"}
}

func (s *Service) connectKindNetwork(ctx context.Context) error {
	_, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"network", "connect", s.settings.KindNetwork, s.settings.ContainerName},
	})
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "already exists") || strings.Contains(lower, "already connected") {
			return nil
		}
		return fmt.Errorf("connect RustFS to %s network: %w", s.settings.KindNetwork, err)
	}
	return nil
}

func (s *Service) stopAndRemove(ctx context.Context, exists bool) error {
	if !exists {
		return nil
	}
	for _, args := range [][]string{{"stop", s.settings.ContainerName}, {"rm", s.settings.ContainerName}} {
		if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: s.commandRuntime(), Args: args}); err != nil {
			if isNotFoundError(err) {
				continue
			}
			return fmt.Errorf("remove RustFS container: %w", err)
		}
	}
	return nil
}

func (s *Service) metadata() Metadata {
	return Metadata{
		Runtime:        s.settings.Runtime,
		Image:          s.settings.Image,
		ContainerName:  s.settings.ContainerName,
		APIPort:        s.settings.APIPort,
		ConsolePort:    s.settings.ConsolePort,
		KindNetwork:    s.settings.KindNetwork,
		AutoAttachKind: s.settings.AutoAttachKind,
	}
}

func mergePersistedSettings(settings Settings, metadata Metadata) Settings {
	if !settings.Explicit.Runtime {
		settings.Runtime = metadata.Runtime
	}
	if !settings.Explicit.Image {
		settings.Image = metadata.Image
	}
	settings.ContainerName = metadata.ContainerName
	if !settings.Explicit.APIPort {
		settings.APIPort = metadata.APIPort
	}
	if !settings.Explicit.ConsolePort {
		settings.ConsolePort = metadata.ConsolePort
	}
	if !settings.Explicit.KindNetwork {
		settings.KindNetwork = metadata.KindNetwork
	}
	if !settings.Explicit.AutoAttachKind {
		settings.AutoAttachKind = metadata.AutoAttachKind
	}
	return settings
}

func (s *Service) saveMetadata(creds Credentials) error {
	metadata := s.metadata()
	metadata.CredentialHash = credentialHash(creds)
	metadata.SpecHash = s.specHash()
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal RustFS metadata: %w", err)
	}
	if err := writePrivateFile(s.layout.MetadataPath, append(data, '\n')); err != nil {
		return fmt.Errorf("write RustFS metadata: %w", err)
	}
	return nil
}

func (s *Service) loadMetadata() (Metadata, error) {
	info, err := os.Lstat(s.layout.MetadataPath)
	if err != nil {
		return Metadata{}, err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return Metadata{}, fmt.Errorf("refusing symlinked RustFS metadata %s", s.layout.MetadataPath)
	}
	data, err := os.ReadFile(s.layout.MetadataPath)
	if err != nil {
		return Metadata{}, err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("parse %s: %w", s.layout.MetadataPath, err)
	}
	if metadata.Runtime == "" || metadata.Image == "" || metadata.ContainerName == "" || metadata.APIPort == 0 || metadata.ConsolePort == 0 || metadata.KindNetwork == "" || metadata.CredentialHash == "" || metadata.SpecHash == "" {
		return Metadata{}, fmt.Errorf("invalid RustFS metadata")
	}
	if !pinnedImage(metadata.Image) {
		return Metadata{}, fmt.Errorf("invalid RustFS metadata image %q", metadata.Image)
	}
	return metadata, nil
}

func (s *Service) commandRuntime() string {
	return s.settings.Runtime
}

func validateSettings(settings Settings) error {
	if !pinnedImage(settings.Image) {
		return fmt.Errorf("RustFS image %q must be an immutable @sha256:<64 hex> reference", settings.Image)
	}
	if settings.APIPort < 1 || settings.APIPort > 65535 || settings.ConsolePort < 1 || settings.ConsolePort > 65535 {
		return fmt.Errorf("RustFS ports must be between 1 and 65535")
	}
	if strings.TrimSpace(settings.KindNetwork) == "" {
		return fmt.Errorf("RustFS Kind network must not be empty")
	}
	return nil
}

func hasSpecOverride(settings Settings) bool {
	return settings.Explicit.Image || settings.Explicit.APIPort || settings.Explicit.ConsolePort || settings.Explicit.KindNetwork
}

func pinnedImage(image string) bool {
	image = strings.TrimSpace(image)
	if image == "" || strings.ContainsAny(image, " \t\r\n") {
		return false
	}
	if at := strings.LastIndex(image, "@"); at >= 0 {
		digest := image[at+1:]
		if !strings.HasPrefix(digest, "sha256:") || len(digest) != len("sha256:")+64 {
			return false
		}
		for _, char := range digest[len("sha256:"):] {
			if !strings.ContainsRune("0123456789abcdef", char) {
				return false
			}
		}
		return at > 0 && !strings.HasSuffix(image[:at], ":")
	}
	return false
}

func validCredential(value string, secret bool) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.ContainsAny(value, "\r\n=") {
		return false
	}
	if !secret && strings.ContainsAny(value, " \t") {
		return false
	}
	return true
}

func randomSecret(size int) (string, error) {
	data := make([]byte, size)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func writePrivateFile(path string, data []byte) error {
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing symlinked private file %s", path)
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".rustfs-private-")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
