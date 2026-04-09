package gitea

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"

	kindprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/kind"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev"
	"github.com/opencenter-cloud/opencenter-cli/internal/localdev/gitea/api"
)

const (
	defaultContainerName = "gitea"
	defaultImage         = "docker.gitea.com/gitea:1.24.5"
	defaultAdminUser     = "admin"
	defaultAdminPass     = "gitea"
	defaultAdminEmail    = "admin@example.com"
	defaultRepoUser      = "newuser"
	defaultRepoPassword  = "newuserpassword"
	defaultRepoEmail     = "newuser@example.com"
	defaultRepoName      = "test-repo"
)

// Settings configures the local Gitea container.
type Settings struct {
	Runtime       string
	Image         string
	ContainerName string
	HTTPPort      int
	HTTPSPort     int
	SSHPort       int
	AdminUser     string
	AdminPassword string
	AdminEmail    string
	RepoOwner     string
	RepoPassword  string
	RepoEmail     string
	RepoName      string
}

// Metadata persists the active local Gitea configuration.
type Metadata struct {
	Runtime       string `json:"runtime"`
	Image         string `json:"image"`
	ContainerName string `json:"container_name"`
	HTTPPort      int    `json:"http_port"`
	HTTPSPort     int    `json:"https_port"`
	SSHPort       int    `json:"ssh_port"`
	AdminUser     string `json:"admin_user"`
	RepoOwner     string `json:"repo_owner"`
	RepoName      string `json:"repo_name"`
}

// Status reports the current local Gitea state.
type Status struct {
	Metadata         Metadata
	BaseURL          string
	LocalRepoURL     string
	HostRepoURL      string
	AttachedNetworks []string
	KindAttached     bool
	KindIP           string
	HostIP           string
	Running          bool
	AdminTokenPath   string
	UserTokenPath    string
	AdminTokenExists bool
	UserTokenExists  bool
	CAPath           string
}

// AttachResult reports the Gitea Kind-network state after attach-kind.
type AttachResult struct {
	Status
	InClusterRepoURL string
}

// Service manages the disposable local Gitea instance.
type Service struct {
	executor localdev.Executor
	layout   localdev.Layout
	settings Settings
}

// DefaultSettings returns the repo-local Gitea defaults.
func DefaultSettings(runtime string) Settings {
	return Settings{
		Runtime:       kindprovider.ResolveRuntime(runtime),
		Image:         defaultImage,
		ContainerName: defaultContainerName,
		HTTPPort:      3000,
		HTTPSPort:     3001,
		SSHPort:       2222,
		AdminUser:     defaultAdminUser,
		AdminPassword: defaultAdminPass,
		AdminEmail:    defaultAdminEmail,
		RepoOwner:     defaultRepoUser,
		RepoPassword:  defaultRepoPassword,
		RepoEmail:     defaultRepoEmail,
		RepoName:      defaultRepoName,
	}
}

// NewService returns a Gitea service for the given state root.
func NewService(executor localdev.Executor, stateDir string, settings Settings) (*Service, error) {
	if executor == nil {
		executor = localdev.NewExecutor()
	}
	layout, err := localdev.ResolveLayout(stateDir)
	if err != nil {
		return nil, err
	}
	if settings.Runtime == "" {
		settings.Runtime = kindprovider.ResolveRuntime("")
	}
	if settings.Image == "" {
		settings.Image = defaultImage
	}
	if settings.ContainerName == "" {
		settings.ContainerName = defaultContainerName
	}

	return &Service{
		executor: executor,
		layout:   layout,
		settings: settings,
	}, nil
}

// Layout returns the resolved state layout.
func (s *Service) Layout() localdev.Layout {
	return s.layout
}

// Up starts or replaces the disposable Gitea instance and provisions test data.
func (s *Service) Up(ctx context.Context) (*Status, error) {
	if err := s.layout.Ensure(); err != nil {
		return nil, err
	}
	if err := s.writeCertificates(nil); err != nil {
		return nil, err
	}
	if err := s.writeAppINI(); err != nil {
		return nil, err
	}
	if err := s.stopAndRemove(ctx); err != nil {
		return nil, err
	}
	if err := s.runContainer(ctx); err != nil {
		return nil, err
	}
	client, err := s.waitForAPI(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.ensureAdminUser(ctx); err != nil {
		return nil, err
	}

	adminToken, err := s.ensureAdminToken(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := client.EnsureUser(ctx, adminToken, api.CreateUserRequest{
		Username: s.settings.RepoOwner,
		Email:    s.settings.RepoEmail,
		Password: s.settings.RepoPassword,
		FullName: "Local Dev User",
	}); err != nil {
		return nil, fmt.Errorf("ensure repo user: %w", err)
	}

	userToken, err := s.ensureUserToken(ctx, client)
	if err != nil {
		return nil, err
	}
	if err := client.EnsureRepository(ctx, userToken, s.settings.RepoOwner, api.CreateRepoRequest{
		Name:        s.settings.RepoName,
		Description: "Local GitOps repository for Kind development",
		Private:     false,
	}); err != nil {
		return nil, fmt.Errorf("ensure repo %s: %w", s.settings.RepoName, err)
	}

	// Opportunistically attach to the Kind network if it already exists.
	// This makes Gitea reachable from pods immediately when a Kind cluster
	// is running. If the network doesn't exist yet the step is silently
	// skipped — AttachKind can still be called later during bootstrap.
	if err := s.tryAttachKind(ctx); err != nil {
		return nil, err
	}

	if err := s.saveMetadata(); err != nil {
		return nil, err
	}
	return s.Status(ctx)
}

// Status reports the current state of the local Gitea instance.
func (s *Service) Status(ctx context.Context) (*Status, error) {
	metadata, err := s.loadMetadata()
	if err != nil {
		metadata = Metadata{
			Runtime:       s.settings.Runtime,
			Image:         s.settings.Image,
			ContainerName: s.settings.ContainerName,
			HTTPPort:      s.settings.HTTPPort,
			HTTPSPort:     s.settings.HTTPSPort,
			SSHPort:       s.settings.SSHPort,
			AdminUser:     s.settings.AdminUser,
			RepoOwner:     s.settings.RepoOwner,
			RepoName:      s.settings.RepoName,
		}
	}

	running, _ := s.containerRunning(ctx)
	networks, _ := s.containerNetworks(ctx)
	kindIP, _ := s.kindIP(ctx)

	adminTokenExists := fileExists(s.layout.AdminTokenPath)
	userTokenExists := fileExists(s.layout.UserTokenPath)

	hostIP := ""
	hostRepoURL := ""
	if kindIP != "" {
		hostIP, _ = hostRoutableIP()
		if hostIP != "" {
			hostRepoURL = fmt.Sprintf("https://%s:%d/%s/%s.git", hostIP, metadata.HTTPSPort, metadata.RepoOwner, metadata.RepoName)
		}
	}

	return &Status{
		Metadata:         metadata,
		BaseURL:          fmt.Sprintf("https://localhost:%d", metadata.HTTPSPort),
		LocalRepoURL:     fmt.Sprintf("https://localhost:%d/%s/%s.git", metadata.HTTPSPort, metadata.RepoOwner, metadata.RepoName),
		HostRepoURL:      hostRepoURL,
		AttachedNetworks: networks,
		KindAttached:     kindIP != "",
		KindIP:           kindIP,
		HostIP:           hostIP,
		Running:          running,
		AdminTokenPath:   s.layout.AdminTokenPath,
		UserTokenPath:    s.layout.UserTokenPath,
		AdminTokenExists: adminTokenExists,
		UserTokenExists:  userTokenExists,
		CAPath:           s.layout.CACertPath,
	}, nil
}

// Destroy stops the container and removes the plugin state directory.
func (s *Service) Destroy(ctx context.Context) error {
	if err := s.stopAndRemove(ctx); err != nil {
		return err
	}
	if err := os.RemoveAll(s.layout.Root); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove %s: %w", s.layout.Root, err)
	}
	return nil
}

// AttachKind connects Gitea to the default Kind network and reissues the TLS cert.
//
// The regenerated certificate includes the Kind network IP and the host's
// routable IP as SANs. This allows a single URL (using the host IP) to work
// from both the macOS host and from inside the Kind cluster, because Podman
// binds the Gitea port on 0.0.0.0.
func (s *Service) AttachKind(ctx context.Context) (*AttachResult, error) {
	if err := s.connectKindNetwork(ctx); err != nil {
		return nil, err
	}

	initialIP, err := s.kindIP(ctx)
	if err != nil {
		return nil, err
	}
	if initialIP == "" {
		return nil, fmt.Errorf("gitea container is not attached to the kind network")
	}

	certIPs := []string{initialIP}
	if hostIP, err := hostRoutableIP(); err == nil && hostIP != "" {
		certIPs = append(certIPs, hostIP)
	}

	if err := s.writeCertificates(certIPs); err != nil {
		return nil, err
	}
	if err := s.restart(ctx); err != nil {
		return nil, err
	}
	if _, err := s.waitForAPI(ctx); err != nil {
		return nil, err
	}

	finalIP, err := s.kindIP(ctx)
	if err != nil {
		return nil, err
	}
	if finalIP != "" && finalIP != initialIP {
		certIPs[0] = finalIP
		if err := s.writeCertificates(certIPs); err != nil {
			return nil, err
		}
		if err := s.restart(ctx); err != nil {
			return nil, err
		}
		if _, err := s.waitForAPI(ctx); err != nil {
			return nil, err
		}
	}

	status, err := s.Status(ctx)
	if err != nil {
		return nil, err
	}
	kindIP := status.KindIP
	if kindIP == "" {
		return nil, fmt.Errorf("gitea kind network IP is empty after attach")
	}

	hostRepoURL := status.HostRepoURL
	if hostRepoURL == "" {
		// Fallback: if no routable host IP was found, use the Kind IP
		// (reachable from inside the cluster but not from the host).
		hostRepoURL = fmt.Sprintf("https://%s:%d/%s/%s.git", kindIP, status.Metadata.HTTPSPort, status.Metadata.RepoOwner, status.Metadata.RepoName)
	}

	return &AttachResult{
		Status:           *status,
		InClusterRepoURL: hostRepoURL,
	}, nil
}

// tryAttachKind connects the Gitea container to the Kind Docker network
// and regenerates TLS certificates with the Kind IP as a SAN. If the Kind
// network does not exist (no cluster created yet), the step is silently
// skipped so that Up can be called before or after Kind cluster creation.
func (s *Service) tryAttachKind(ctx context.Context) error {
	exists, err := s.kindNetworkExists(ctx)
	if err != nil || !exists {
		return nil
	}
	if _, err := s.AttachKind(ctx); err != nil {
		return fmt.Errorf("auto-attach to kind network: %w", err)
	}
	return nil
}

// kindNetworkExists returns true when the Docker/Podman "kind" network is
// present. It returns (false, nil) when the network simply doesn't exist.
func (s *Service) kindNetworkExists(ctx context.Context) (bool, error) {
	_, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"network", "inspect", "kind"},
	})
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "no such") || strings.Contains(lower, "not found") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *Service) saveMetadata() error {
	data, err := json.MarshalIndent(Metadata{
		Runtime:       s.settings.Runtime,
		Image:         s.settings.Image,
		ContainerName: s.settings.ContainerName,
		HTTPPort:      s.settings.HTTPPort,
		HTTPSPort:     s.settings.HTTPSPort,
		SSHPort:       s.settings.SSHPort,
		AdminUser:     s.settings.AdminUser,
		RepoOwner:     s.settings.RepoOwner,
		RepoName:      s.settings.RepoName,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal gitea metadata: %w", err)
	}
	if err := os.WriteFile(s.layout.MetadataPath, data, 0o644); err != nil {
		return fmt.Errorf("write gitea metadata: %w", err)
	}
	return nil
}

func (s *Service) loadMetadata() (Metadata, error) {
	data, err := os.ReadFile(s.layout.MetadataPath)
	if err != nil {
		return Metadata{}, err
	}
	var metadata Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return Metadata{}, fmt.Errorf("parse %s: %w", s.layout.MetadataPath, err)
	}
	return metadata, nil
}

func (s *Service) writeAppINI() error {
	appINI := fmt.Sprintf(`APP_NAME = Gitea: Git with a cup of tea
RUN_MODE = prod
RUN_USER = git
WORK_PATH = /data/gitea

[repository]
ROOT = /data/gitea/git

[repository.local]
LOCAL_COPY_PATH = /data/gitea/tmp/local-repo

[repository.upload]
TEMP_PATH = /data/gitea/uploads

[server]
APP_DATA_PATH = /data/gitea
DOMAIN = localhost
SSH_DOMAIN = localhost
PROTOCOL = https
HTTP_PORT = %d
ROOT_URL = https://localhost:%d/
CERT_FILE = /data/gitea/certs/cert.pem
KEY_FILE = /data/gitea/certs/key.pem
DISABLE_SSH = false
SSH_PORT = 22
SSH_LISTEN_PORT = 22
OFFLINE_MODE = true

[database]
PATH = /data/gitea/gitea.db
DB_TYPE = sqlite3

[log]
MODE = console
LEVEL = info
ROOT_PATH = /data/gitea/log

[security]
INSTALL_LOCK = true
PASSWORD_HASH_ALGO = pbkdf2

[service]
DISABLE_REGISTRATION = false
REGISTER_EMAIL_CONFIRM = false
ENABLE_NOTIFY_MAIL = false
ALLOW_ONLY_EXTERNAL_REGISTRATION = false
ENABLE_CAPTCHA = false
`, s.settings.HTTPSPort, s.settings.HTTPSPort)

	if err := os.WriteFile(s.layout.AppIniPath, []byte(appINI), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", s.layout.AppIniPath, err)
	}
	return nil
}

func (s *Service) writeCertificates(extraIPs []string) error {
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	for _, rawIP := range extraIPs {
		if ip := net.ParseIP(strings.TrimSpace(rawIP)); ip != nil {
			ips = append(ips, ip)
		}
	}

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate gitea CA key: %w", err)
	}
	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("generate gitea server key: %w", err)
	}

	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate CA serial: %w", err)
	}
	now := time.Now().UTC()

	caTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   "opencenter-local-gitea-ca",
			Organization: []string{"openCenter"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create CA certificate: %w", err)
	}

	serverSerial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return fmt.Errorf("generate server serial: %w", err)
	}
	serverTemplate := &x509.Certificate{
		SerialNumber: serverSerial,
		Subject: pkix.Name{
			CommonName:   "localhost",
			Organization: []string{"openCenter"},
		},
		NotBefore:             now.Add(-5 * time.Minute),
		NotAfter:              now.Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		DNSNames:              []string{"localhost", "gitea"},
		IPAddresses:           ips,
	}
	serverDER, err := x509.CreateCertificate(rand.Reader, serverTemplate, caTemplate, &serverKey.PublicKey, caKey)
	if err != nil {
		return fmt.Errorf("create server certificate: %w", err)
	}

	if err := writePEMFile(s.layout.CACertPath, "CERTIFICATE", caDER, 0o644); err != nil {
		return err
	}
	if err := writePEMFile(s.layout.ServerCertPath, "CERTIFICATE", serverDER, 0o644); err != nil {
		return err
	}
	serverKeyBytes := x509.MarshalPKCS1PrivateKey(serverKey)
	if err := writePEMFile(s.layout.ServerKeyPath, "RSA PRIVATE KEY", serverKeyBytes, 0o600); err != nil {
		return err
	}
	return nil
}

func (s *Service) runContainer(ctx context.Context) error {
	runtime := s.commandRuntime()
	args := []string{
		"run", "-d",
		"--name", s.settings.ContainerName,
		"-v", fmt.Sprintf("%s:/data", s.layout.GiteaDataDir),
		"-p", fmt.Sprintf("%d:3000", s.settings.HTTPPort),
		"-p", fmt.Sprintf("%d:3001", s.settings.HTTPSPort),
		"-p", fmt.Sprintf("%d:22", s.settings.SSHPort),
		s.settings.Image,
	}
	if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: runtime, Args: args}); err != nil {
		return fmt.Errorf("start gitea container: %w", err)
	}
	return nil
}

func (s *Service) ensureAdminUser(ctx context.Context) error {
	runtime := s.commandRuntime()
	args := []string{
		"exec", "--user", "1000:1000",
		s.settings.ContainerName,
		"gitea", "admin", "user", "create",
		"--config", "/data/gitea/conf/app.ini",
		"--username", s.settings.AdminUser,
		"--password", s.settings.AdminPassword,
		"--email", s.settings.AdminEmail,
		"--admin",
		"--must-change-password=false",
	}
	if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: runtime, Args: args}); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "already exists") {
			return nil
		}
		return fmt.Errorf("create admin user: %w", err)
	}
	return nil
}

func (s *Service) ensureAdminToken(ctx context.Context, client *api.Client) (string, error) {
	if token, err := os.ReadFile(s.layout.AdminTokenPath); err == nil {
		current := strings.TrimSpace(string(token))
		if current != "" && client.VerifyToken(ctx, current, s.settings.AdminUser) == nil {
			return current, nil
		}
	}

	token, err := client.CreateTokenWithBasicAuth(ctx, s.settings.AdminUser, s.settings.AdminPassword, "opencenter-admin-"+fmt.Sprint(time.Now().Unix()))
	if err != nil {
		return "", fmt.Errorf("create admin token: %w", err)
	}
	if err := os.WriteFile(s.layout.AdminTokenPath, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("write admin token: %w", err)
	}
	return token, nil
}

func (s *Service) ensureUserToken(ctx context.Context, client *api.Client) (string, error) {
	if token, err := os.ReadFile(s.layout.UserTokenPath); err == nil {
		current := strings.TrimSpace(string(token))
		if current != "" && client.VerifyToken(ctx, current, s.settings.RepoOwner) == nil {
			return current, nil
		}
	}

	token, err := client.CreateTokenWithBasicAuth(ctx, s.settings.RepoOwner, s.settings.RepoPassword, "opencenter-user-"+fmt.Sprint(time.Now().Unix()))
	if err != nil {
		return "", fmt.Errorf("create user token: %w", err)
	}
	if err := os.WriteFile(s.layout.UserTokenPath, []byte(token), 0o600); err != nil {
		return "", fmt.Errorf("write user token: %w", err)
	}
	return token, nil
}

func (s *Service) waitForAPI(ctx context.Context) (*api.Client, error) {
	client, err := api.NewClient(fmt.Sprintf("https://localhost:%d", s.settings.HTTPSPort), s.layout.CACertPath)
	if err != nil {
		return nil, err
	}

	deadline := time.Now().Add(90 * time.Second)
	for {
		if _, err := client.Version(ctx); err == nil {
			return client, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("timed out waiting for gitea at https://localhost:%d", s.settings.HTTPSPort)
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func (s *Service) connectKindNetwork(ctx context.Context) error {
	runtime := s.commandRuntime()
	_, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: runtime,
		Args: []string{"network", "connect", "kind", s.settings.ContainerName},
	})
	if err != nil {
		lower := strings.ToLower(err.Error())
		if strings.Contains(lower, "already exists") || strings.Contains(lower, "already connected") {
			return nil
		}
		return fmt.Errorf("connect gitea to kind network: %w", err)
	}
	return nil
}

func (s *Service) restart(ctx context.Context) error {
	if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: s.commandRuntime(), Args: []string{"restart", s.settings.ContainerName}}); err != nil {
		return fmt.Errorf("restart gitea: %w", err)
	}
	return nil
}

func (s *Service) stopAndRemove(ctx context.Context) error {
	runtime := s.commandRuntime()
	for _, args := range [][]string{
		{"stop", s.settings.ContainerName},
		{"rm", s.settings.ContainerName},
	} {
		if _, err := s.executor.Run(ctx, localdev.RunOptions{Name: runtime, Args: args}); err != nil {
			lower := strings.ToLower(err.Error())
			if strings.Contains(lower, "no such") || strings.Contains(lower, "not found") {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *Service) containerRunning(ctx context.Context) (bool, error) {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"inspect", "--format", "{{.State.Running}}", s.settings.ContainerName},
	})
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(string(output)) == "true", nil
}

func (s *Service) containerNetworks(ctx context.Context) ([]string, error) {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"inspect", "--format", "{{range $name, $_ := .NetworkSettings.Networks}}{{$name}}{{\"\\n\"}}{{end}}", s.settings.ContainerName},
	})
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	networks := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			networks = append(networks, line)
		}
	}
	return networks, nil
}

func (s *Service) kindIP(ctx context.Context) (string, error) {
	output, err := s.executor.Run(ctx, localdev.RunOptions{
		Name: s.commandRuntime(),
		Args: []string{"inspect", "--format", "{{with index .NetworkSettings.Networks \"kind\"}}{{.IPAddress}}{{end}}", s.settings.ContainerName},
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func writePEMFile(pathname, pemType string, der []byte, mode os.FileMode) error {
	block := &pem.Block{Type: pemType, Bytes: der}
	if err := os.WriteFile(pathname, pem.EncodeToMemory(block), mode); err != nil {
		return fmt.Errorf("write %s: %w", pathname, err)
	}
	return nil
}

func fileExists(pathname string) bool {
	_, err := os.Stat(pathname)
	return err == nil
}

// hostRoutableIP returns the first non-loopback IPv4 address of the host.
// Podman binds container ports on 0.0.0.0, so this IP is reachable from
// both the host and from inside the Kind cluster (via the Podman VM bridge).
func hostRoutableIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("list network interfaces: %w", err)
	}
	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.To4() == nil {
			continue
		}
		return ip.String(), nil
	}
	return "", fmt.Errorf("no routable IPv4 address found")
}

func (s *Service) commandRuntime() string {
	if metadata, err := s.loadMetadata(); err == nil && strings.TrimSpace(metadata.Runtime) != "" {
		return metadata.Runtime
	}
	return s.settings.Runtime
}
