package magnum

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/certificates"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/clusters"
	tokens3 "github.com/gophercloud/gophercloud/openstack/identity/v3/tokens"
	"gopkg.in/yaml.v3"
)

const (
	defaultPollInterval   = time.Second
	httpClientTimeout     = 30 * time.Second
	createVisibilityGrace = 10 * time.Second
)

// Config contains the credentials and Magnum defaults used by Provider.
//
// AuthURL and ProjectID are accepted as aliases for IdentityEndpoint and
// TenantID respectively. They are useful when translating an existing
// OpenStack configuration, while the canonical fields keep this package
// independent of the application's configuration types.
type Config struct {
	IdentityEndpoint string
	AuthURL          string
	KeystoneEndpoint string
	// CAPath is an optional PEM bundle used to verify Keystone and Magnum.
	// CA is retained as an alias for configurations that already use that name.
	CAPath                      string
	CA                          string
	Region                      string
	TenantID                    string
	ProjectID                   string
	ApplicationCredentialID     string
	ApplicationCredentialSecret string
	Insecure                    bool
	ClusterTemplate             string
	Labels                      map[string]string
	Keypair                     string
	MasterFlavorID              string
	NodeFlavorID                string
	CreateTimeout               int
	MasterLBEnabled             *bool
}

// Request describes a new Magnum cluster.
type Request struct {
	Name        string
	ClusterName string
	MasterCount int
	NodeCount   int
	WorkerCount int
}

// Cluster is the stable, package-level representation of a Magnum cluster.
type Cluster struct {
	ID              string
	Name            string
	Status          string
	StatusReason    string
	APIAddress      string
	MasterAddresses []string
	NodeAddresses   []string
}

// Service is the small portion of Magnum used by Provider. It is exported so
// callers can inject a fake service in tests without making network calls.
type Service interface {
	Create(clusters.CreateOpts) (string, error)
	Get(string) (clusters.Cluster, error)
	List() ([]clusters.Cluster, error)
	CreateCertificate(string, string) (string, error)
	GetCertificate(string) (string, error)
	Delete(string) error
}

// contextService is implemented by the real Gophercloud adapter. Keeping the
// original Service methods context-free preserves a simple injectable fake,
// while real requests use the context attached before that operation's client
// was authenticated.
type contextService interface {
	CreateContext(context.Context, clusters.CreateOpts) (string, error)
	GetContext(context.Context, string) (clusters.Cluster, error)
	ListContext(context.Context) ([]clusters.Cluster, error)
	CreateCertificateContext(context.Context, string, string) (string, error)
	GetCertificateContext(context.Context, string) (string, error)
	DeleteContext(context.Context, string) error
}

// Provider manages Magnum clusters.
type Provider struct {
	config Config

	mu                   sync.Mutex
	service              Service
	authProjectValidator func(*gophercloud.ProviderClient, string) error
	createdAt            map[string]time.Time
	// serviceFactory is intentionally a hook rather than an eager client. This
	// keeps construction testable and avoids authentication until an operation
	// actually needs the service.
	serviceFactory func(context.Context) (Service, error)
}

// NewProvider validates config and creates a lazy Magnum provider. No network
// request is made by this function.
func NewProvider(config Config) (*Provider, error) {
	config = normalizeConfig(config)
	if err := validateConfig(config); err != nil {
		return nil, err
	}

	p := &Provider{config: config, createdAt: make(map[string]time.Time)}
	p.authProjectValidator = validateAuthenticatedProject
	p.serviceFactory = func(ctx context.Context) (Service, error) {
		return newGophercloudServiceWithValidator(config, ctx, p.authProjectValidator)
	}
	return p, nil
}

// NewProviderWithService is a test and integration hook for supplying an
// already-created Magnum service implementation.
func NewProviderWithService(config Config, service Service) (*Provider, error) {
	p, err := NewProvider(config)
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, errors.New("magnum service is nil")
	}
	p.service = service
	return p, nil
}

// CreateCluster creates a cluster and returns its initial normalized identity.
// Magnum's create endpoint returns the UUID rather than the complete resource,
// so status and addresses are populated by a later GetCluster or WaitReady.
func (p *Provider) CreateCluster(ctx context.Context, request Request) (Cluster, error) {
	ctx = nonNilContext(ctx)
	if err := validateConfig(p.config); err != nil {
		return Cluster{}, err
	}
	request = normalizeRequest(request)
	if err := validateRequest(request); err != nil {
		return Cluster{}, err
	}
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}

	masterCount := request.MasterCount
	nodeCount := request.NodeCount
	createTimeout := p.config.CreateTimeout
	opts := clusters.CreateOpts{
		ClusterTemplateID: p.config.ClusterTemplate,
		CreateTimeout:     &createTimeout,
		FlavorID:          p.config.NodeFlavorID,
		Keypair:           p.config.Keypair,
		Labels:            copyLabels(p.config.Labels),
		MasterCount:       &masterCount,
		MasterFlavorID:    p.config.MasterFlavorID,
		Name:              request.Name,
		NodeCount:         &nodeCount,
		MasterLBEnabled:   copyBool(p.config.MasterLBEnabled),
	}

	service, err := p.getService(ctx)
	if err != nil {
		return Cluster{}, err
	}
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	id, callErr := serviceCreate(ctx, service, opts)
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	if callErr != nil {
		return Cluster{}, fmt.Errorf("create Magnum cluster: %w", callErr)
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return Cluster{}, errors.New("create Magnum cluster returned an empty ID")
	}
	p.rememberCreated(id)
	return Cluster{ID: id, Name: request.Name}, nil
}

// GetCluster gets a cluster by UUID or name.
func (p *Provider) GetCluster(ctx context.Context, idOrName string) (Cluster, error) {
	ctx = nonNilContext(ctx)
	if err := validateConfig(p.config); err != nil {
		return Cluster{}, err
	}
	identifier := strings.TrimSpace(idOrName)
	if identifier == "" {
		return Cluster{}, errors.New("cluster ID or name is required")
	}
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}

	service, err := p.getService(ctx)
	if err != nil {
		return Cluster{}, err
	}
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	return p.getClusterWithService(ctx, service, identifier)
}

// WaitVisible waits until a cluster can be read by UUID or by a unique name.
// A not-found response is treated as eventual consistency only for this
// bounded reconciliation operation; authorization and server errors are
// returned immediately.
func (p *Provider) WaitVisible(ctx context.Context, idOrName string, interval time.Duration) (Cluster, error) {
	ctx = nonNilContext(ctx)
	if interval <= 0 {
		interval = defaultPollInterval
	}
	identifier := strings.TrimSpace(idOrName)
	if identifier == "" {
		return Cluster{}, errors.New("cluster ID or name is required")
	}
	if err := validateConfig(p.config); err != nil {
		return Cluster{}, err
	}
	service, err := p.getService(ctx)
	if err != nil {
		return Cluster{}, err
	}
	var current Cluster
	for {
		if err := ctx.Err(); err != nil {
			return current, err
		}
		cluster, getErr := p.getClusterWithService(ctx, service, identifier)
		if getErr == nil {
			return cluster, nil
		}
		if !isNotFound(getErr) {
			return current, getErr
		}
		if err := waitInterval(ctx, interval); err != nil {
			return current, err
		}
	}
}

func (p *Provider) getClusterWithService(ctx context.Context, service Service, identifier string) (Cluster, error) {
	cluster, getErr := serviceGet(ctx, service, identifier)
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	if getErr == nil {
		return normalizeCluster(cluster), nil
	}
	if looksLikeUUID(identifier) {
		return Cluster{}, fmt.Errorf("get Magnum cluster %q: %w", identifier, getErr)
	}
	if !isNotFound(getErr) {
		return Cluster{}, fmt.Errorf("get Magnum cluster %q: %w", identifier, getErr)
	}

	// Magnum's GET endpoint is ID-only. Falling back to a list makes names
	// useful, but only a documented 404 permits this fallback.
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	items, listErr := serviceList(ctx, service)
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	if listErr != nil {
		return Cluster{}, fmt.Errorf("list Magnum clusters while resolving %q: %w", identifier, listErr)
	}
	matches := make([]clusters.Cluster, 0, 1)
	for _, item := range items {
		if item.UUID == identifier || item.Name == identifier {
			matches = append(matches, item)
		}
	}
	if len(matches) > 1 {
		return Cluster{}, fmt.Errorf("cluster name %q is ambiguous: found %d matches", identifier, len(matches))
	}
	if len(matches) == 1 {
		return normalizeCluster(matches[0]), nil
	}
	return Cluster{}, fmt.Errorf("get Magnum cluster %q: %w", identifier, getErr)
}

// WaitReady waits until a cluster reaches a successful creation/update state.
func (p *Provider) WaitReady(ctx context.Context, idOrName string, interval time.Duration) (Cluster, error) {
	ctx = nonNilContext(ctx)
	if interval <= 0 {
		interval = defaultPollInterval
	}
	identifier := strings.TrimSpace(idOrName)
	if identifier == "" {
		return Cluster{}, errors.New("cluster ID or name is required")
	}
	if err := validateConfig(p.config); err != nil {
		return Cluster{}, err
	}
	if err := ctx.Err(); err != nil {
		return Cluster{}, err
	}
	service, err := p.getService(ctx)
	if err != nil {
		return Cluster{}, err
	}
	var current Cluster
	for {
		if err := ctx.Err(); err != nil {
			return current, err
		}
		cluster, err := p.getClusterWithService(ctx, service, identifier)
		if err != nil {
			if isNotFound(err) {
				if remaining, ok := p.visibilityRemaining(identifier); ok {
					if remaining < interval {
						interval = remaining
					}
					if err := waitInterval(ctx, interval); err != nil {
						return current, err
					}
					continue
				}
			}
			return cluster, err
		}
		p.forgetCreated(identifier)
		current = cluster
		switch strings.ToUpper(strings.TrimSpace(cluster.Status)) {
		case "CREATE_COMPLETE", "UPDATE_COMPLETE":
			return cluster, nil
		case "CREATE_FAILED", "UPDATE_FAILED", "DELETE_FAILED":
			p.forgetCreated(identifier)
			return cluster, fmt.Errorf("Magnum cluster %q failed with status %s: %s", cluster.Name, cluster.Status, cluster.StatusReason)
		}

		if err := waitInterval(ctx, interval); err != nil {
			return current, err
		}
	}
}

// ExportKubeconfig creates a client certificate through Magnum's certificate
// API and writes a standard kubeconfig. The private key and CSR are generated
// locally and are never sent anywhere except the CSR signing request.
func (p *Provider) ExportKubeconfig(ctx context.Context, idOrName, path string) error {
	ctx = nonNilContext(ctx)
	if err := validateConfig(p.config); err != nil {
		return err
	}
	identifier := strings.TrimSpace(idOrName)
	if identifier == "" {
		return errors.New("cluster ID or name is required")
	}
	if strings.TrimSpace(path) == "" {
		return errors.New("kubeconfig path is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, err := p.getService(ctx)
	if err != nil {
		return err
	}
	cluster, err := p.getClusterWithService(ctx, service, identifier)
	if err != nil {
		return err
	}
	if cluster.ID == "" {
		return errors.New("Magnum cluster returned an empty ID")
	}
	if strings.TrimSpace(cluster.APIAddress) == "" {
		return errors.New("Magnum cluster has no API address")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	privateKey, csr, err := generateCSR()
	if err != nil {
		return fmt.Errorf("generate client certificate request: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	clientCertificate, callErr := serviceCreateCertificate(ctx, service, cluster.ID, csr)
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return fmt.Errorf("create Magnum client certificate: %w", callErr)
	}
	if strings.TrimSpace(clientCertificate) == "" {
		return errors.New("Magnum returned an empty client certificate")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	caCertificate, callErr := serviceGetCertificate(ctx, service, cluster.ID)
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return fmt.Errorf("get Magnum CA certificate: %w", callErr)
	}
	if strings.TrimSpace(caCertificate) == "" {
		return errors.New("Magnum returned an empty CA certificate")
	}
	configText, err := marshalKubeconfig(cluster, caCertificate, clientCertificate, privateKey)
	if err != nil {
		return fmt.Errorf("build kubeconfig: %w", err)
	}

	return writeSecureFile(path, configText)
}

// DeleteCluster deletes a cluster by UUID or name.
func (p *Provider) DeleteCluster(ctx context.Context, idOrName string) error {
	ctx = nonNilContext(ctx)
	if err := validateConfig(p.config); err != nil {
		return err
	}
	identifier := strings.TrimSpace(idOrName)
	if identifier == "" {
		return errors.New("cluster ID or name is required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, err := p.getService(ctx)
	if err != nil {
		return err
	}
	identifier, err = p.resolveIdentifier(ctx, service, identifier)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	callErr := serviceDelete(ctx, service, identifier)
	if err := ctx.Err(); err != nil {
		return err
	}
	if callErr != nil {
		return fmt.Errorf("delete Magnum cluster %q: %w", identifier, callErr)
	}
	return nil
}

// WaitDeleted waits until Magnum confirms that a cluster no longer exists.
// A successful return means a GET reached a documented not-found response;
// an empty list alone is not treated as proof of deletion.
func (p *Provider) WaitDeleted(ctx context.Context, id string) error {
	ctx = nonNilContext(ctx)
	identifier := strings.TrimSpace(id)
	if identifier == "" {
		return errors.New("cluster ID is required")
	}
	if err := validateConfig(p.config); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	service, err := p.getService(ctx)
	if err != nil {
		return err
	}
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		rawCluster, callErr := serviceGet(ctx, service, identifier)
		if err := ctx.Err(); err != nil {
			return err
		}
		var cluster Cluster
		if callErr == nil {
			cluster = normalizeCluster(rawCluster)
		} else {
			err = callErr
		}
		if err != nil {
			if isNotFound(err) {
				return nil
			}
			return err
		}
		if strings.EqualFold(strings.TrimSpace(cluster.Status), "DELETE_FAILED") {
			return fmt.Errorf("Magnum cluster %q deletion failed: %s", cluster.Name, cluster.StatusReason)
		}

		timer := time.NewTimer(defaultPollInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (p *Provider) getService(ctx context.Context) (Service, error) {
	p.mu.Lock()
	if p.service != nil {
		service := p.service
		p.mu.Unlock()
		return service, nil
	}
	factory := p.serviceFactory
	p.mu.Unlock()
	if factory == nil {
		return nil, errors.New("Magnum service is not configured")
	}
	service, err := factory(nonNilContext(ctx))
	if err != nil {
		return nil, err
	}
	if service == nil {
		return nil, errors.New("Magnum service factory returned nil")
	}
	return service, nil
}

func (p *Provider) rememberCreated(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.createdAt == nil {
		p.createdAt = make(map[string]time.Time)
	}
	p.createdAt[id] = time.Now()
}

func (p *Provider) visibilityRemaining(id string) (time.Duration, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	created, ok := p.createdAt[id]
	if !ok {
		return 0, false
	}
	remaining := createVisibilityGrace - time.Since(created)
	if remaining <= 0 {
		delete(p.createdAt, id)
		return 0, false
	}
	return remaining, true
}

func (p *Provider) forgetCreated(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.createdAt, id)
}

func waitInterval(ctx context.Context, interval time.Duration) error {
	if interval <= 0 {
		interval = time.Nanosecond
	}
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func serviceCreate(ctx context.Context, service Service, opts clusters.CreateOpts) (string, error) {
	if operationService, ok := service.(contextService); ok {
		return operationService.CreateContext(ctx, opts)
	}
	return service.Create(opts)
}

func serviceGet(ctx context.Context, service Service, identifier string) (clusters.Cluster, error) {
	if operationService, ok := service.(contextService); ok {
		return operationService.GetContext(ctx, identifier)
	}
	return service.Get(identifier)
}

func serviceList(ctx context.Context, service Service) ([]clusters.Cluster, error) {
	if operationService, ok := service.(contextService); ok {
		return operationService.ListContext(ctx)
	}
	return service.List()
}

func serviceCreateCertificate(ctx context.Context, service Service, clusterID, csr string) (string, error) {
	if operationService, ok := service.(contextService); ok {
		return operationService.CreateCertificateContext(ctx, clusterID, csr)
	}
	return service.CreateCertificate(clusterID, csr)
}

func serviceGetCertificate(ctx context.Context, service Service, clusterID string) (string, error) {
	if operationService, ok := service.(contextService); ok {
		return operationService.GetCertificateContext(ctx, clusterID)
	}
	return service.GetCertificate(clusterID)
}

func serviceDelete(ctx context.Context, service Service, identifier string) error {
	if operationService, ok := service.(contextService); ok {
		return operationService.DeleteContext(ctx, identifier)
	}
	return service.Delete(identifier)
}

func (p *Provider) resolveIdentifier(ctx context.Context, service Service, identifier string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if looksLikeUUID(identifier) {
		return identifier, nil
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	cluster, err := p.getClusterWithService(ctx, service, identifier)
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if cluster.ID != "" {
		return cluster.ID, nil
	}
	return identifier, nil
}

func validateConfig(config Config) error {
	if strings.TrimSpace(config.IdentityEndpoint) == "" {
		return errors.New("Keystone identity endpoint is required")
	}
	endpoint, err := url.Parse(config.IdentityEndpoint)
	if err != nil || endpoint.Scheme == "" || endpoint.Host == "" {
		return errors.New("Keystone identity endpoint must be a valid URL")
	}
	if strings.TrimSpace(config.Region) == "" {
		return errors.New("Magnum region is required")
	}
	if strings.TrimSpace(config.TenantID) == "" {
		return errors.New("project ID (TenantID) is required")
	}
	if (strings.TrimSpace(config.ApplicationCredentialID) == "") != (strings.TrimSpace(config.ApplicationCredentialSecret) == "") {
		return errors.New("application credential ID and secret must be supplied together")
	}
	if strings.TrimSpace(config.ApplicationCredentialID) == "" {
		return errors.New("application credential ID and secret are required")
	}
	if strings.TrimSpace(config.ClusterTemplate) == "" {
		return errors.New("cluster template is required")
	}
	if config.CreateTimeout < 0 {
		return errors.New("create timeout must not be negative")
	}
	if config.CAPath != "" {
		if _, err := loadCAPool(config.CAPath); err != nil {
			return err
		}
	}
	return nil
}

func validateRequest(request Request) error {
	if strings.TrimSpace(request.Name) == "" {
		return errors.New("cluster name is required")
	}
	if request.MasterCount < 0 {
		return errors.New("master count must not be negative")
	}
	if request.NodeCount < 0 {
		return errors.New("node count must not be negative")
	}
	return nil
}

func normalizeConfig(config Config) Config {
	if strings.TrimSpace(config.IdentityEndpoint) == "" {
		config.IdentityEndpoint = config.AuthURL
	}
	if strings.TrimSpace(config.IdentityEndpoint) == "" {
		config.IdentityEndpoint = config.KeystoneEndpoint
	}
	if strings.TrimSpace(config.CAPath) == "" {
		config.CAPath = config.CA
	}
	if strings.TrimSpace(config.TenantID) == "" {
		config.TenantID = config.ProjectID
	}
	config.IdentityEndpoint = strings.TrimSpace(config.IdentityEndpoint)
	config.CAPath = strings.TrimSpace(config.CAPath)
	config.Region = strings.TrimSpace(config.Region)
	config.TenantID = strings.TrimSpace(config.TenantID)
	config.ApplicationCredentialID = strings.TrimSpace(config.ApplicationCredentialID)
	config.ApplicationCredentialSecret = strings.TrimSpace(config.ApplicationCredentialSecret)
	config.ClusterTemplate = strings.TrimSpace(config.ClusterTemplate)
	config.Keypair = strings.TrimSpace(config.Keypair)
	config.MasterFlavorID = strings.TrimSpace(config.MasterFlavorID)
	config.NodeFlavorID = strings.TrimSpace(config.NodeFlavorID)
	return config
}

func normalizeRequest(request Request) Request {
	if strings.TrimSpace(request.Name) == "" {
		request.Name = request.ClusterName
	}
	if request.NodeCount == 0 && request.WorkerCount != 0 {
		request.NodeCount = request.WorkerCount
	}
	return request
}

func normalizeCluster(cluster clusters.Cluster) Cluster {
	return Cluster{
		ID:              cluster.UUID,
		Name:            cluster.Name,
		Status:          cluster.Status,
		StatusReason:    cluster.StatusReason,
		APIAddress:      cluster.APIAddress,
		MasterAddresses: append([]string(nil), cluster.MasterAddresses...),
		NodeAddresses:   append([]string(nil), cluster.NodeAddresses...),
	}
}

func copyLabels(labels map[string]string) map[string]string {
	if labels == nil {
		return nil
	}
	copy := make(map[string]string, len(labels))
	for key, value := range labels {
		copy[key] = value
	}
	return copy
}

func copyBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func looksLikeUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

func newGophercloudServiceWithValidator(config Config, ctx context.Context, projectValidator func(*gophercloud.ProviderClient, string) error) (Service, error) {
	provider, err := openstack.NewClient(config.IdentityEndpoint)
	if err != nil {
		return nil, fmt.Errorf("create Keystone client: %w", err)
	}
	provider.Context = nonNilContext(ctx)
	httpClient, err := configuredHTTPClient(config)
	if err != nil {
		return nil, err
	}
	provider.HTTPClient = httpClient
	authOpts := authOptions(config)
	if err := openstack.AuthenticateV3(provider, &authOpts, gophercloud.EndpointOpts{}); err != nil {
		return nil, fmt.Errorf("authenticate with Keystone: %w", err)
	}
	if err := provider.Context.Err(); err != nil {
		return nil, err
	}
	if projectValidator == nil {
		projectValidator = validateAuthenticatedProject
	}
	if err := projectValidator(provider, config.TenantID); err != nil {
		return nil, err
	}
	client, err := openstack.NewContainerInfraV1(provider, gophercloud.EndpointOpts{Region: config.Region})
	if err != nil {
		return nil, fmt.Errorf("create Magnum client: %w", err)
	}
	return &gophercloudService{client: client}, nil
}

func insecureTransport() *http.Transport {
	transport := cloneTransport()
	transport.TLSClientConfig.InsecureSkipVerify = true
	return transport
}

func cloneTransport() *http.Transport {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		return &http.Transport{TLSClientConfig: &tls.Config{}}
	}
	transport := base.Clone()
	if transport.TLSClientConfig == nil {
		transport.TLSClientConfig = &tls.Config{}
	} else {
		transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	}
	return transport
}

func authOptions(config Config) gophercloud.AuthOptions {
	// Application credentials are project-scoped by definition. Supplying a
	// TenantID here changes the Keystone request into a project-scoped auth
	// request and is rejected by several Keystone deployments.
	return gophercloud.AuthOptions{
		IdentityEndpoint:            config.IdentityEndpoint,
		ApplicationCredentialID:     config.ApplicationCredentialID,
		ApplicationCredentialSecret: config.ApplicationCredentialSecret,
		AllowReauth:                 true,
	}
}

func configuredTransport(config Config) (*http.Transport, error) {
	if !config.Insecure && config.CAPath == "" {
		// Returning nil preserves net/http's normal, certificate-validating
		// DefaultTransport while the caller still applies a finite timeout.
		return nil, nil
	}
	transport := cloneTransport()
	if config.CAPath != "" {
		pool, err := loadCAPool(config.CAPath)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig.RootCAs = pool
	}
	if config.Insecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	return transport, nil
}

func configuredHTTPClient(config Config) (http.Client, error) {
	transport, err := configuredTransport(config)
	if err != nil {
		return http.Client{}, err
	}
	client := http.Client{Timeout: httpClientTimeout}
	// Do not assign a typed-nil *http.Transport to the interface field. A
	// nil interface tells net/http to use its secure DefaultTransport.
	if transport != nil {
		client.Transport = transport
	}
	return client, nil
}

func loadCAPool(path string) (*x509.CertPool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate %q: %w", path, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return nil, fmt.Errorf("CA certificate %q contains no valid PEM certificates", path)
	}
	return pool, nil
}

func validateAuthenticatedProject(provider *gophercloud.ProviderClient, expected string) error {
	result := provider.GetAuthResult()
	if result == nil {
		return errors.New("Keystone authentication returned no token result")
	}
	actual, err := authenticatedProjectID(result)
	if err != nil {
		return fmt.Errorf("inspect authenticated Keystone project: %w", err)
	}
	return validateProjectID(expected, actual)
}

func validateProjectID(expected, actual string) error {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	if expected == "" {
		return errors.New("project ID is required for authenticated project validation")
	}
	if actual != expected {
		return fmt.Errorf("authenticated Keystone project %q does not match configured project %q", actual, expected)
	}
	return nil
}

func authenticatedProjectID(result gophercloud.AuthResult) (string, error) {
	// AuthenticateV3 stores this concrete result value in ProviderClient. Keep
	// the pointer case for callers that install an equivalent result in tests.
	switch typed := result.(type) {
	case tokens3.CreateResult:
		project, err := typed.ExtractProject()
		if err != nil {
			return "", err
		}
		if project == nil {
			return "", errors.New("token contains no project")
		}
		return project.ID, nil
	case *tokens3.CreateResult:
		if typed == nil {
			return "", errors.New("token result is nil")
		}
		project, err := typed.ExtractProject()
		if err != nil {
			return "", err
		}
		if project == nil {
			return "", errors.New("token contains no project")
		}
		return project.ID, nil
	default:
		return "", fmt.Errorf("unsupported Keystone auth result type %T", result)
	}
}

func generateCSR() (privateKeyPEM, csrPEM string, err error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return "", "", err
	}
	requestDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "admin", Organization: []string{"system:masters"}},
	}, privateKey)
	if err != nil {
		return "", "", err
	}
	privateKeyPEM = string(pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}))
	csrPEM = string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE REQUEST", Bytes: requestDER}))
	return privateKeyPEM, csrPEM, nil
}

type kubeconfig struct {
	APIVersion     string              `yaml:"apiVersion"`
	Kind           string              `yaml:"kind"`
	Clusters       []kubeconfigCluster `yaml:"clusters"`
	Contexts       []kubeconfigContext `yaml:"contexts"`
	CurrentContext string              `yaml:"current-context"`
	Users          []kubeconfigUser    `yaml:"users"`
}

type kubeconfigCluster struct {
	Name    string                `yaml:"name"`
	Cluster kubeconfigClusterData `yaml:"cluster"`
}

type kubeconfigClusterData struct {
	Server                   string `yaml:"server"`
	CertificateAuthorityData string `yaml:"certificate-authority-data"`
}

type kubeconfigContext struct {
	Name    string                `yaml:"name"`
	Context kubeconfigContextData `yaml:"context"`
}

type kubeconfigContextData struct {
	Cluster string `yaml:"cluster"`
	User    string `yaml:"user"`
}

type kubeconfigUser struct {
	Name string             `yaml:"name"`
	User kubeconfigUserData `yaml:"user"`
}

type kubeconfigUserData struct {
	ClientCertificateData string `yaml:"client-certificate-data"`
	ClientKeyData         string `yaml:"client-key-data"`
}

func marshalKubeconfig(cluster Cluster, caPEM, clientCertificatePEM, privateKeyPEM string) (string, error) {
	name := strings.TrimSpace(cluster.Name)
	if name == "" {
		name = cluster.ID
	}
	value := kubeconfig{
		APIVersion: "v1",
		Kind:       "Config",
		Clusters: []kubeconfigCluster{{Name: name, Cluster: kubeconfigClusterData{
			Server:                   strings.TrimSpace(cluster.APIAddress),
			CertificateAuthorityData: base64.StdEncoding.EncodeToString([]byte(caPEM)),
		}}},
		Contexts:       []kubeconfigContext{{Name: name, Context: kubeconfigContextData{Cluster: name, User: name}}},
		CurrentContext: name,
		Users: []kubeconfigUser{{Name: name, User: kubeconfigUserData{
			ClientCertificateData: base64.StdEncoding.EncodeToString([]byte(clientCertificatePEM)),
			ClientKeyData:         base64.StdEncoding.EncodeToString([]byte(privateKeyPEM)),
		}}},
	}
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func writeSecureFile(path, content string) error {
	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(cleanPath), 0700); err != nil {
		return fmt.Errorf("create kubeconfig directory: %w", err)
	}
	file, err := os.OpenFile(cleanPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("open kubeconfig: %w", err)
	}
	if err := file.Chmod(0600); err != nil {
		_ = file.Close()
		return fmt.Errorf("secure kubeconfig: %w", err)
	}
	if _, err := file.WriteString(content); err != nil {
		_ = file.Close()
		return fmt.Errorf("write kubeconfig: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close kubeconfig: %w", err)
	}
	return nil
}

func isNotFound(err error) bool {
	var statusError gophercloud.StatusCodeError
	return errors.As(err, &statusError) && statusError.GetStatusCode() == http.StatusNotFound
}

type gophercloudService struct {
	client *gophercloud.ServiceClient
}

func (s *gophercloudService) Create(opts clusters.CreateOpts) (string, error) {
	return s.CreateContext(context.Background(), opts)
}

func (s *gophercloudService) Get(identifier string) (clusters.Cluster, error) {
	return s.GetContext(context.Background(), identifier)
}

func (s *gophercloudService) List() ([]clusters.Cluster, error) {
	return s.ListContext(context.Background())
}

func (s *gophercloudService) CreateCertificate(clusterID, csr string) (string, error) {
	return s.CreateCertificateContext(context.Background(), clusterID, csr)
}

func (s *gophercloudService) GetCertificate(clusterID string) (string, error) {
	return s.GetCertificateContext(context.Background(), clusterID)
}

func (s *gophercloudService) Delete(identifier string) error {
	return s.DeleteContext(context.Background(), identifier)
}

func (s *gophercloudService) CreateContext(_ context.Context, opts clusters.CreateOpts) (string, error) {
	return clusters.Create(s.client, opts).Extract()
}

func (s *gophercloudService) GetContext(_ context.Context, identifier string) (clusters.Cluster, error) {
	cluster, err := clusters.Get(s.client, url.PathEscape(identifier)).Extract()
	if err != nil {
		return clusters.Cluster{}, err
	}
	return *cluster, nil
}

func (s *gophercloudService) ListContext(_ context.Context) ([]clusters.Cluster, error) {
	pages, err := clusters.ListDetail(s.client, clusters.ListOpts{}).AllPages()
	if err != nil {
		return nil, err
	}
	return clusters.ExtractClusters(pages)
}

func (s *gophercloudService) CreateCertificateContext(_ context.Context, clusterID, csr string) (string, error) {
	certificate, err := certificates.Create(s.client, certificates.CreateOpts{
		ClusterUUID: clusterID,
		CSR:         csr,
	}).Extract()
	if err != nil {
		return "", err
	}
	return certificate.PEM, nil
}

func (s *gophercloudService) GetCertificateContext(_ context.Context, clusterID string) (string, error) {
	certificate, err := certificates.Get(s.client, url.PathEscape(clusterID)).Extract()
	if err != nil {
		return "", err
	}
	return certificate.PEM, nil
}

func (s *gophercloudService) DeleteContext(_ context.Context, identifier string) error {
	return clusters.Delete(s.client, url.PathEscape(identifier)).ExtractErr()
}
