package magnum

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/clusters"
	"gopkg.in/yaml.v3"
)

type fakeService struct {
	createOpts clusters.CreateOpts
	createID   string
	createErr  error
	getItems   []clusters.Cluster
	getErrors  []error
	getErr     error
	listItems  []clusters.Cluster
	listErr    error
	listCalls  int
	clientCert string
	caCert     string
	certErr    error
	certID     string
	csr        string
	deleted    string
	deleteErr  error
}

func (f *fakeService) Create(opts clusters.CreateOpts) (string, error) {
	f.createOpts = opts
	return f.createID, f.createErr
}

func (f *fakeService) Get(_ string) (clusters.Cluster, error) {
	if len(f.getErrors) > 0 {
		err := f.getErrors[0]
		f.getErrors = f.getErrors[1:]
		return clusters.Cluster{}, err
	}
	if len(f.getItems) > 0 {
		item := f.getItems[0]
		f.getItems = f.getItems[1:]
		return item, nil
	}
	if f.getErr != nil {
		return clusters.Cluster{}, f.getErr
	}
	return clusters.Cluster{}, errors.New("cluster not found")
}

func (f *fakeService) List() ([]clusters.Cluster, error) {
	f.listCalls++
	return f.listItems, f.listErr
}

func (f *fakeService) CreateCertificate(id, csr string) (string, error) {
	f.certID = id
	f.csr = csr
	return f.clientCert, f.certErr
}

func (f *fakeService) GetCertificate(id string) (string, error) {
	f.certID = id
	return f.caCert, f.certErr
}

func (f *fakeService) Delete(id string) error {
	f.deleted = id
	return f.deleteErr
}

func validConfig() Config {
	return Config{
		IdentityEndpoint:            "https://identity.example.test/v3",
		Region:                      "region-one",
		TenantID:                    "project-id",
		ApplicationCredentialID:     "app-id",
		ApplicationCredentialSecret: "app-secret",
		ClusterTemplate:             "template-id",
		Labels:                      map[string]string{"purpose": "test"},
		Keypair:                     "cluster-key",
		MasterFlavorID:              "master-flavor",
		NodeFlavorID:                "node-flavor",
		CreateTimeout:               120,
	}
}

func newFakeProvider(t *testing.T, service Service) *Provider {
	t.Helper()
	provider, err := NewProviderWithService(validConfig(), service)
	if err != nil {
		t.Fatal(err)
	}
	return provider
}

func TestNewProviderValidatesCredentialsAndTemplate(t *testing.T) {
	tests := []struct {
		name   string
		change func(*Config)
		want   string
	}{
		{"missing endpoint", func(c *Config) { c.IdentityEndpoint = "" }, "identity endpoint"},
		{"missing credential", func(c *Config) { c.ApplicationCredentialID = "" }, "supplied together"},
		{"unpaired credential", func(c *Config) { c.ApplicationCredentialSecret = "" }, "supplied together"},
		{"missing template", func(c *Config) { c.ClusterTemplate = "" }, "cluster template"},
		{"negative timeout", func(c *Config) { c.CreateTimeout = -1 }, "negative"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := validConfig()
			test.change(&config)
			_, err := NewProvider(config)
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), test.want) {
				t.Fatalf("NewProvider error = %v, want text %q", err, test.want)
			}
		})
	}
}

func TestAuthOptionsDoNotCarryProjectScope(t *testing.T) {
	opts := authOptions(validConfig())
	if opts.TenantID != "" {
		t.Fatalf("application credential auth TenantID = %q, want empty", opts.TenantID)
	}
	if opts.ApplicationCredentialID != "app-id" || opts.ApplicationCredentialSecret != "app-secret" {
		t.Fatal("application credential values were not preserved")
	}
}

func TestAuthenticatedProjectValidation(t *testing.T) {
	if err := validateProjectID("project-id", "project-id"); err != nil {
		t.Fatalf("matching project rejected: %v", err)
	}
	err := validateProjectID("project-id", "other-project")
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("mismatched project error = %v", err)
	}
}

func TestCAHandlingAndFiniteHTTPTimeout(t *testing.T) {
	client, err := configuredHTTPClient(validConfig())
	if err != nil {
		t.Fatal(err)
	}
	if client.Transport != nil {
		t.Fatal("secure no-CA client has a typed-nil/custom transport; want net/http default")
	}
	if client.Timeout != httpClientTimeout {
		t.Fatalf("HTTP timeout = %s, want %s", client.Timeout, httpClientTimeout)
	}

	t.Run("unreadable", func(t *testing.T) {
		config := validConfig()
		config.CAPath = filepath.Join(t.TempDir(), "missing.pem")
		_, err := NewProvider(config)
		if err == nil || !strings.Contains(err.Error(), "read CA certificate") {
			t.Fatalf("NewProvider error = %v", err)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "invalid.pem")
		if err := os.WriteFile(path, []byte("not PEM"), 0600); err != nil {
			t.Fatal(err)
		}
		config := validConfig()
		config.CAPath = path
		_, err := NewProvider(config)
		if err == nil || !strings.Contains(err.Error(), "no valid PEM") {
			t.Fatalf("NewProvider error = %v", err)
		}
	})

	certPEM := testCertificatePEM(t)
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, []byte(certPEM), 0600); err != nil {
		t.Fatal(err)
	}
	config := validConfig()
	config.CAPath = path
	transport, err := configuredTransport(config)
	if err != nil {
		t.Fatal(err)
	}
	if transport.TLSClientConfig == nil || transport.TLSClientConfig.RootCAs == nil || transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("CA transport did not preserve certificate verification")
	}
	if httpClientTimeout <= 0 {
		t.Fatal("HTTP client timeout is not finite")
	}
}

func testCertificatePEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"}, IsCA: true, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
}

func TestCreateClusterBuildsGophercloudOptions(t *testing.T) {
	fake := &fakeService{createID: "cluster-id"}
	provider := newFakeProvider(t, fake)
	got, err := provider.CreateCluster(context.Background(), Request{Name: "demo", MasterCount: 3, NodeCount: 7})
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "cluster-id" || got.Name != "demo" {
		t.Fatalf("created cluster = %+v", got)
	}
	opts := fake.createOpts
	if opts.ClusterTemplateID != "template-id" || opts.Name != "demo" || opts.FlavorID != "node-flavor" || opts.MasterFlavorID != "master-flavor" || opts.Keypair != "cluster-key" {
		t.Fatalf("unexpected create options: %+v", opts)
	}
	if opts.MasterCount == nil || *opts.MasterCount != 3 || opts.NodeCount == nil || *opts.NodeCount != 7 || opts.CreateTimeout == nil || *opts.CreateTimeout != 120 {
		t.Fatalf("counts/timeout not converted to pointers: %+v", opts)
	}
	if !reflect.DeepEqual(opts.Labels, validConfig().Labels) {
		t.Fatalf("labels = %#v", opts.Labels)
	}
	if opts.MasterLBEnabled != nil {
		t.Fatalf("unset optional load balancer flag = %v", *opts.MasterLBEnabled)
	}
}

func TestWaitReadyTransitionsAndReportsFailure(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		fake := &fakeService{getItems: []clusters.Cluster{
			{UUID: "cluster-id", Name: "demo", Status: "CREATE_IN_PROGRESS"},
			{UUID: "cluster-id", Name: "demo", Status: "CREATE_COMPLETE", APIAddress: "https://api"},
		}}
		got, err := newFakeProvider(t, fake).WaitReady(context.Background(), "cluster-id", time.Millisecond)
		if err != nil || got.Status != "CREATE_COMPLETE" || got.APIAddress != "https://api" {
			t.Fatalf("WaitReady = %+v, %v", got, err)
		}
	})

	t.Run("failed", func(t *testing.T) {
		fake := &fakeService{getItems: []clusters.Cluster{{UUID: "cluster-id", Name: "demo", Status: "CREATE_FAILED", StatusReason: "quota exceeded"}}}
		got, err := newFakeProvider(t, fake).WaitReady(context.Background(), "cluster-id", time.Millisecond)
		if err == nil || !strings.Contains(err.Error(), "quota exceeded") || got.Status != "CREATE_FAILED" {
			t.Fatalf("WaitReady = %+v, %v", got, err)
		}
	})
}

func TestWaitReadyHonorsCancellation(t *testing.T) {
	fake := &fakeService{getItems: []clusters.Cluster{{UUID: "cluster-id", Name: "demo", Status: "CREATE_IN_PROGRESS"}}}
	ctx, cancel := context.WithCancel(context.Background())
	provider := newFakeProvider(t, fake)
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()
	_, err := provider.WaitReady(ctx, "cluster-id", time.Hour)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("WaitReady error = %v, want context cancellation", err)
	}
}

func TestExportKubeconfigUsesSecurePathAndPermissions(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "nested", "kubeconfig")
	clusterID := "11111111-1111-1111-1111-111111111111"
	fake := &fakeService{
		getItems:   []clusters.Cluster{{UUID: clusterID, Name: "demo", APIAddress: "https://api.example.test"}},
		clientCert: "-----BEGIN CERTIFICATE-----\nclient\n-----END CERTIFICATE-----\n",
		caCert:     "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----\n",
	}
	if err := newFakeProvider(t, fake).ExportKubeconfig(context.Background(), clusterID, path); err != nil {
		t.Fatal(err)
	}
	fileInfo, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0600 {
		t.Fatalf("kubeconfig mode = %o, want 0600", mode)
	}
	parentInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if mode := parentInfo.Mode().Perm(); mode != 0700 {
		t.Fatalf("parent mode = %o, want 0700", mode)
	}

	if err := os.Chmod(path, 0644); err != nil {
		t.Fatal(err)
	}
	fake.getItems = []clusters.Cluster{{UUID: clusterID, Name: "demo", APIAddress: "https://api.example.test"}}
	if err := newFakeProvider(t, fake).ExportKubeconfig(context.Background(), clusterID, path); err != nil {
		t.Fatal(err)
	}
	fileInfo, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := fileInfo.Mode().Perm(); mode != 0600 {
		t.Fatalf("rewritten kubeconfig mode = %o, want 0600", mode)
	}
}

func TestExportKubeconfigUsesCertificateFlowAndEmbedsCredentials(t *testing.T) {
	clusterID := "11111111-1111-1111-1111-111111111111"
	fake := &fakeService{
		getItems:   []clusters.Cluster{{UUID: clusterID, Name: "demo", APIAddress: "https://api.example.test"}},
		clientCert: "client certificate",
		caCert:     "ca certificate",
	}
	path := filepath.Join(t.TempDir(), "kubeconfig")
	if err := newFakeProvider(t, fake).ExportKubeconfig(context.Background(), clusterID, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var got kubeconfig
	if err := yaml.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.Kind != "Config" || len(got.Clusters) != 1 || got.Clusters[0].Cluster.Server != "https://api.example.test" {
		t.Fatalf("unexpected kubeconfig cluster: %+v", got)
	}
	ca, err := base64.StdEncoding.DecodeString(got.Clusters[0].Cluster.CertificateAuthorityData)
	if err != nil || string(ca) != fake.caCert {
		t.Fatalf("CA data = %q, %v", ca, err)
	}
	if len(got.Users) != 1 {
		t.Fatalf("users = %+v", got.Users)
	}
	clientCert, err := base64.StdEncoding.DecodeString(got.Users[0].User.ClientCertificateData)
	if err != nil || string(clientCert) != fake.clientCert {
		t.Fatalf("client certificate data = %q, %v", clientCert, err)
	}
	privateKey, err := base64.StdEncoding.DecodeString(got.Users[0].User.ClientKeyData)
	if err != nil {
		t.Fatal(err)
	}
	block, _ := pem.Decode(privateKey)
	if block == nil || block.Type != "RSA PRIVATE KEY" {
		t.Fatalf("private key PEM = %q", privateKey)
	}
	if _, err := x509.ParsePKCS1PrivateKey(block.Bytes); err != nil {
		t.Fatalf("private key is not RSA: %v", err)
	}
	csrBlock, _ := pem.Decode([]byte(fake.csr))
	if csrBlock == nil || csrBlock.Type != "CERTIFICATE REQUEST" {
		t.Fatalf("CSR = %q", fake.csr)
	}
	request, err := x509.ParseCertificateRequest(csrBlock.Bytes)
	if err != nil {
		t.Fatalf("CSR is invalid: %v", err)
	}
	if request.Subject.CommonName != "admin" || !reflect.DeepEqual(request.Subject.Organization, []string{"system:masters"}) {
		t.Fatalf("CSR subject = %+v, want CN=admin O=system:masters", request.Subject)
	}
	if fake.certID != clusterID {
		t.Fatalf("certificate request cluster ID = %q", fake.certID)
	}
}

func TestExportKubeconfigRejectsEmptyCertificate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kubeconfig")
	clusterID := "11111111-1111-1111-1111-111111111111"
	fake := &fakeService{
		getItems:   []clusters.Cluster{{UUID: clusterID, Name: "demo", APIAddress: "https://api.example.test"}},
		clientCert: "client certificate",
	}
	err := newFakeProvider(t, fake).ExportKubeconfig(context.Background(), clusterID, path)
	if err == nil || !strings.Contains(err.Error(), "empty") {
		t.Fatalf("ExportKubeconfig error = %v", err)
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Fatalf("empty response created file: %v", statErr)
	}
}

func TestGetClusterNameLookupOnlyFallsBackOnNotFound(t *testing.T) {
	t.Run("server error is preserved", func(t *testing.T) {
		serverErr := errors.New("authorization failed")
		fake := &fakeService{getErr: serverErr, listItems: []clusters.Cluster{{UUID: "id", Name: "demo"}}}
		_, err := newFakeProvider(t, fake).GetCluster(context.Background(), "demo")
		if !errors.Is(err, serverErr) || fake.listCalls != 0 {
			t.Fatalf("GetCluster error = %v, list calls = %d", err, fake.listCalls)
		}
	})

	notFound := gophercloud.ErrUnexpectedResponseCode{Actual: 404}
	t.Run("ambiguous name is rejected", func(t *testing.T) {
		fake := &fakeService{getErr: notFound, listItems: []clusters.Cluster{{UUID: "one", Name: "demo"}, {UUID: "two", Name: "demo"}}}
		_, err := newFakeProvider(t, fake).GetCluster(context.Background(), "demo")
		if err == nil || !strings.Contains(err.Error(), "ambiguous") {
			t.Fatalf("GetCluster error = %v", err)
		}
	})
	t.Run("zero matches remains not found", func(t *testing.T) {
		fake := &fakeService{getErr: notFound}
		_, err := newFakeProvider(t, fake).GetCluster(context.Background(), "demo")
		if !isNotFound(err) {
			t.Fatalf("GetCluster error = %v, want not found", err)
		}
	})
}

func TestWaitDeletedPollsAndHandlesDeleteFailure(t *testing.T) {
	t.Run("confirmed deleted", func(t *testing.T) {
		notFound := gophercloud.ErrUnexpectedResponseCode{Actual: 404}
		fake := &fakeService{
			getItems: []clusters.Cluster{{UUID: "cluster-id", Name: "demo", Status: "DELETE_IN_PROGRESS"}},
			getErr:   notFound,
		}
		if err := newFakeProvider(t, fake).WaitDeleted(context.Background(), "cluster-id"); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("delete failed", func(t *testing.T) {
		fake := &fakeService{getItems: []clusters.Cluster{{UUID: "cluster-id", Name: "demo", Status: "DELETE_FAILED", StatusReason: "stack error"}}}
		err := newFakeProvider(t, fake).WaitDeleted(context.Background(), "cluster-id")
		if err == nil || !strings.Contains(err.Error(), "stack error") {
			t.Fatalf("WaitDeleted error = %v", err)
		}
	})
}

func TestInsecureTransportIsOptIn(t *testing.T) {
	if insecureTransport().TLSClientConfig == nil || !insecureTransport().TLSClientConfig.InsecureSkipVerify {
		t.Fatal("insecure transport does not disable verification")
	}
	config := validConfig()
	config.Insecure = false
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatal(err)
	}
	if provider.config.Insecure {
		t.Fatal("secure config unexpectedly enabled insecure TLS")
	}
}

func TestGophercloudServiceUsesPreAuthenticatedOperationContext(t *testing.T) {
	baseProvider := &gophercloud.ProviderClient{}
	baseClient := &gophercloud.ServiceClient{ProviderClient: baseProvider, Endpoint: "https://magnum.example.test/"}
	service := &gophercloudService{client: baseClient}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	baseProvider.Context = ctx
	if service.client.Context != ctx {
		t.Fatal("operation context was not attached before service use")
	}
	if service.client.ProviderClient != baseProvider {
		t.Fatal("service unexpectedly replaced its authenticated provider client")
	}
}

func TestGophercloudReauthenticationRefreshesExactProviderToken(t *testing.T) {
	var requests int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch requests {
		case 1:
			if got := r.Header.Get("X-Auth-Token"); got != "old-token" {
				t.Errorf("initial token = %q", got)
			}
			w.WriteHeader(http.StatusUnauthorized)
		case 2:
			if got := r.Header.Get("X-Auth-Token"); got != "new-token" {
				t.Errorf("retry token = %q, want refreshed token", got)
			}
			_, _ = w.Write([]byte(`{"uuid":"cluster-id","name":"demo","status":"CREATE_COMPLETE"}`))
		}
	}))
	defer server.Close()

	provider := &gophercloud.ProviderClient{HTTPClient: http.Client{Timeout: time.Second}, Context: context.Background()}
	provider.UseTokenLock()
	provider.SetToken("old-token")
	provider.ReauthFunc = func() error {
		provider.SetToken("new-token")
		return nil
	}
	serviceClient := &gophercloud.ServiceClient{ProviderClient: provider, ResourceBase: server.URL + "/v1/"}
	service := &gophercloudService{client: serviceClient}
	if _, err := service.GetContext(context.Background(), "cluster-id"); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("request count = %d, want initial request plus retry", requests)
	}
}

func TestAuthenticationHonorsInitialContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("canceled authentication reached the identity server")
	}))
	defer server.Close()
	config := validConfig()
	config.IdentityEndpoint = server.URL + "/v3"
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newGophercloudServiceWithValidator(config, ctx, func(*gophercloud.ProviderClient, string) error {
		t.Fatal("project validation ran after canceled authentication")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("authentication error = %v, want context cancellation", err)
	}
}

func TestProviderHonorsCancellationDuringReauthentication(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	provider := &gophercloud.ProviderClient{HTTPClient: http.Client{Timeout: time.Second}, Context: ctx}
	provider.UseTokenLock()
	provider.SetToken("expired-token")
	provider.ReauthFunc = func() error {
		<-ctx.Done()
		return ctx.Err()
	}
	service := &gophercloudService{client: &gophercloud.ServiceClient{ProviderClient: provider, ResourceBase: server.URL + "/v1/"}}
	magnum := newFakeProvider(t, service)
	result := make(chan error, 1)
	go func() {
		_, err := magnum.GetCluster(ctx, "11111111-1111-1111-1111-111111111111")
		result <- err
	}()
	time.Sleep(10 * time.Millisecond)
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("GetCluster error = %v, want context cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("GetCluster did not stop during reauthentication")
	}
}

func TestWaitReadyAllowsOnlyRecentCreateVisibility404(t *testing.T) {
	notFound := gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}
	t.Run("within grace", func(t *testing.T) {
		fake := &fakeService{
			createID:  "cluster-id",
			getErrors: []error{notFound},
			getItems:  []clusters.Cluster{{UUID: "cluster-id", Name: "demo", Status: "CREATE_COMPLETE"}},
		}
		provider := newFakeProvider(t, fake)
		if _, err := provider.CreateCluster(context.Background(), Request{Name: "demo", MasterCount: 1, NodeCount: 1}); err != nil {
			t.Fatal(err)
		}
		got, err := provider.WaitReady(context.Background(), "cluster-id", time.Millisecond)
		if err != nil || got.Status != "CREATE_COMPLETE" {
			t.Fatalf("WaitReady = %+v, %v", got, err)
		}
	})

	t.Run("expired", func(t *testing.T) {
		fake := &fakeService{getErr: notFound}
		provider := newFakeProvider(t, fake)
		provider.createdAt["cluster-id"] = time.Now().Add(-createVisibilityGrace - time.Second)
		_, err := provider.WaitReady(context.Background(), "cluster-id", time.Millisecond)
		if !isNotFound(err) {
			t.Fatalf("WaitReady error = %v, want 404 after grace expiry", err)
		}
	})
}
