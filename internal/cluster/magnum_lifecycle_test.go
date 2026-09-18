package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack/containerinfra/v1/clusters"
	magnumprovider "github.com/opencenter-cloud/opencenter-cli/internal/cloud/magnum"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

func validMagnumLifecycleConfig(name string) *v2.Config {
	cfg := mustNewClusterTestConfig(name, "magnum")
	cfg.OpenCenter.Infrastructure.Cloud.Magnum = &v2.MagnumCloudConfig{
		AuthURL:                     "https://keystone.example.test/v3/",
		Region:                      "RegionOne",
		ProjectID:                   "project-id",
		ApplicationCredentialID:     "application-id",
		ApplicationCredentialSecret: "application-secret",
		ClusterTemplate:             "kubernetes-template",
		MasterFlavorID:              "master-flavor",
		NodeFlavorID:                "worker-flavor",
	}
	cfg.OpenCenter.GitOps.Auth.Token = nil
	cfg.OpenTofu.Enabled = false
	return &cfg
}

func TestMagnumBootstrapProviderBuildSteps(t *testing.T) {
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	cfg.OpenCenter.Infrastructure.Compute.MasterCount = 3
	cfg.OpenCenter.Infrastructure.Compute.WorkerCount = 5

	service := NewBootstrapService(paths.NewPathResolver(t.TempDir()), nil)
	steps, err := service.buildBootstrapSteps(cfg, &paths.ClusterPaths{}, &BootstrapOptions{KubeconfigPath: "/tmp/magnum-kubeconfig"})
	if err != nil {
		t.Fatalf("buildBootstrapSteps() error = %v", err)
	}
	if got := bootstrapStepIDsForTest(steps); strings.Join(got, ",") != "magnum-create,magnum-wait-ready,magnum-export-kubeconfig" {
		t.Fatalf("step IDs = %v", got)
	}
	for _, step := range steps {
		if strings.Contains(step.Plan.WorkingDir, "infrastructure") {
			t.Fatalf("Magnum step unexpectedly requires infrastructure directory: %#v", step.Plan)
		}
		for _, env := range step.Plan.Environment {
			if env.Redacted || strings.Contains(env.Name, "SECRET") || strings.Contains(env.Value, "application-secret") {
				t.Fatalf("Magnum plan must not emit credentials: %#v", step.Plan.Environment)
			}
		}
	}
	if writes := steps[2].Plan.Writes; len(writes) == 0 || !containsString(writes, "/tmp/magnum-kubeconfig") {
		t.Fatalf("export step writes = %v, want kubeconfig write", writes)
	}
}

func TestMagnumBootstrapProviderNilOptionsReturnsClearError(t *testing.T) {
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	provider := newMagnumBootstrapProvider(nil)
	_, err := provider.BuildSteps(cfg, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "kubeconfig path must be set") {
		t.Fatalf("BuildSteps(nil opts) error = %v, want clear kubeconfig error", err)
	}
}

func TestMagnumBootstrapDryRunDoesNotRunLifecycleSteps(t *testing.T) {
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	runner := &recordingLifecycleRunner{}
	service := &BootstrapService{runner: runner}
	opts := &BootstrapOptions{KubeconfigPath: "/tmp/magnum-kubeconfig", DryRun: true}
	steps, err := service.buildBootstrapSteps(cfg, &paths.ClusterPaths{}, opts)
	if err != nil {
		t.Fatalf("buildBootstrapSteps() error = %v", err)
	}
	selected, _, err := service.filterSteps(steps, opts)
	if err != nil {
		t.Fatalf("filterSteps() error = %v", err)
	}
	plan := service.buildDryRunPlan(cfg, &paths.ClusterPaths{}, nil, opts, selected, "")
	if plan == nil || len(plan.Steps) != 3 {
		t.Fatalf("dry-run plan = %#v, want three Magnum steps", plan)
	}
	if len(runner.calls) != 0 {
		t.Fatalf("dry-run called lifecycle runner: %v", runner.calls)
	}
}

func TestMagnumBootstrapReusesDurableUUID(t *testing.T) {
	stateRoot := t.TempDir()
	t.Setenv("OPENCENTER_STATE_DIR", stateRoot)
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	fake := &lifecycleMagnumService{
		getByID:                clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_COMPLETE"},
		getNameErr:             gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound},
		createID:               testMagnumUUID,
		nameVisibleAfterCreate: true,
	}
	client := newLifecycleMagnumProvider(t, fake)
	providerFactory := func(magnumprovider.Config) (*magnumprovider.Provider, error) { return client, nil }
	provider := &magnumBootstrapProvider{providerFactory: providerFactory, pollInterval: time.Millisecond}
	opts := &BootstrapOptions{KubeconfigPath: filepath.Join(t.TempDir(), "kubeconfig")}
	paths := &paths.ClusterPaths{}
	steps, err := provider.BuildSteps(cfg, paths, opts)
	if err != nil {
		t.Fatalf("BuildSteps() error = %v", err)
	}
	if err := steps[0].Run(context.Background()); err != nil {
		t.Fatalf("first create step error = %v", err)
	}
	if fake.createCalls != 1 {
		t.Fatalf("create calls after first run = %d, want 1", fake.createCalls)
	}
	if _, err := os.Stat(filepath.Join(stateRoot, "magnum", "opencenter", cfg.ClusterName(), "identity.json")); err != nil {
		t.Fatalf("durable identity state missing: %v", err)
	}
	statePath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadMagnumIdentityState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.ClusterID != testMagnumUUID || state.Observed {
		t.Fatalf("accepted create state = %#v, want unobserved UUID", state)
	}
	steps, err = provider.BuildSteps(cfg, paths, opts)
	if err != nil {
		t.Fatalf("second BuildSteps() error = %v", err)
	}
	if err := steps[0].Run(context.Background()); err != nil {
		t.Fatalf("second create step error = %v", err)
	}
	if fake.createCalls != 1 {
		t.Fatalf("create calls after reuse = %d, want 1", fake.createCalls)
	}
}

func TestMagnumBootstrapLostCreateResponseStaysInFlight(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	fake := &lifecycleMagnumService{
		getByID:                clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_IN_PROGRESS"},
		getNameErr:             gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound},
		createID:               testMagnumUUID,
		createErr:              errors.New("response lost"),
		nameVisibleAfterCreate: true,
	}
	client := newLifecycleMagnumProvider(t, fake)
	provider := &magnumBootstrapProvider{
		providerFactory: func(magnumprovider.Config) (*magnumprovider.Provider, error) { return client, nil },
		pollInterval:    time.Millisecond,
	}
	opts := &BootstrapOptions{KubeconfigPath: "/tmp/kubeconfig", Timeout: 25 * time.Millisecond}
	steps, err := provider.BuildSteps(cfg, &paths.ClusterPaths{}, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := steps[0].Run(context.Background()); err == nil {
		t.Fatal("first create should report the lost response")
	}
	if fake.createCalls != 1 {
		t.Fatalf("create calls after lost response = %d, want 1", fake.createCalls)
	}
	if err := steps[0].Run(context.Background()); err != nil {
		t.Fatalf("recovery create step error = %v", err)
	}
	if fake.createCalls != 1 {
		t.Fatalf("recovery issued duplicate create: %d calls", fake.createCalls)
	}
	statePath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	state, err := loadMagnumIdentityState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.ClusterID != testMagnumUUID || !state.Observed {
		t.Fatalf("recovered state = %#v, want observed UUID", state)
	}
}

func TestMagnumIdentityStateRejectsInvalidStateWithoutNameRecovery(t *testing.T) {
	tests := []struct {
		name string
		data string
	}{
		{name: "corrupt", data: "not-json"},
		{name: "unsupported version", data: `{"version":99,"cluster_id":"` + testMagnumUUID + `"}`},
		{name: "invalid UUID", data: `{"version":1,"cluster_id":"not-a-uuid"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
			cfg := validMagnumLifecycleConfig("magnum-cluster")
			identityPath, err := magnumIdentityStatePath(cfg)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(identityPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(identityPath, []byte(tt.data), 0o600); err != nil {
				t.Fatal(err)
			}
			fake := &lifecycleMagnumService{getNameErr: gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}, createID: testMagnumUUID}
			client := newLifecycleMagnumProvider(t, fake)
			provider := &magnumBootstrapProvider{
				providerFactory: func(magnumprovider.Config) (*magnumprovider.Provider, error) { return client, nil },
			}
			steps, err := provider.BuildSteps(cfg, &paths.ClusterPaths{}, &BootstrapOptions{KubeconfigPath: "/tmp/kubeconfig"})
			if err != nil {
				t.Fatal(err)
			}
			err = steps[0].Run(context.Background())
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), "identity state") {
				t.Fatalf("create error = %v, want explicit identity-state error", err)
			}
			if len(fake.getIDs) != 0 || fake.createCalls != 0 {
				t.Fatalf("invalid state triggered remote recovery: gets=%v creates=%d", fake.getIDs, fake.createCalls)
			}
		})
	}
}

func TestMagnumBootstrapReadinessTimeoutUsesUUID(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistObservedMagnumIdentity(identityPath, cfg.ClusterName(), testMagnumUUID); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleMagnumService{getByID: clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_IN_PROGRESS"}}
	client := newLifecycleMagnumProvider(t, fake)
	provider := &magnumBootstrapProvider{
		providerFactory: func(magnumprovider.Config) (*magnumprovider.Provider, error) { return client, nil },
		pollInterval:    time.Millisecond,
	}
	steps, err := provider.BuildSteps(cfg, &paths.ClusterPaths{}, &BootstrapOptions{KubeconfigPath: "/tmp/kubeconfig", Timeout: 5 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	err = steps[1].Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "within 5ms") || !strings.Contains(err.Error(), "deadline") {
		t.Fatalf("readiness error = %v, want bounded timeout", err)
	}
	if len(fake.getIDs) == 0 || fake.getIDs[0] != testMagnumUUID {
		t.Fatalf("readiness identifiers = %v, want UUID", fake.getIDs)
	}
}

func TestMagnumDestroyWaitFailurePreservesIdentity(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistObservedMagnumIdentity(identityPath, cfg.ClusterName(), testMagnumUUID); err != nil {
		t.Fatal(err)
	}
	runtimePaths, err := resolveBootstrapRuntimePaths(cfg, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestBootstrapState(runtimePaths.StatePath); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleMagnumService{
		getByID:         clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_COMPLETE"},
		keepAfterDelete: true,
	}
	client := newLifecycleMagnumProvider(t, fake)
	provider := &magnumDestroyProvider{provider: client, identityPath: identityPath, timeout: 100 * time.Millisecond}
	steps, err := provider.BuildSteps(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	err = steps[0].Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "wait for Magnum cluster deletion") {
		t.Fatalf("destroy error = %v, want wait failure", err)
	}
	if _, statErr := os.Stat(identityPath); statErr != nil {
		t.Fatalf("identity state should be preserved after failed wait: %v", statErr)
	}
	if _, statErr := os.Stat(runtimePaths.StatePath); statErr != nil {
		t.Fatalf("bootstrap state should be preserved after failed wait: %v", statErr)
	}
	if fake.deleteIDs == nil || fake.deleteIDs[0] != testMagnumUUID {
		t.Fatalf("delete identifiers = %v, want UUID", fake.deleteIDs)
	}
}

func TestMagnumDestroyAcceptedInvisibleClusterRetainsState(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	state := newMagnumIdentityState(cfg.ClusterName(), magnumCreateAccepted)
	state.ClusterID = testMagnumUUID
	if err := persistMagnumIdentityState(identityPath, state); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleMagnumService{
		getByID:    clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_IN_PROGRESS"},
		idNotFound: true,
	}
	client := newLifecycleMagnumProvider(t, fake)
	provider := &magnumDestroyProvider{provider: client, identityPath: identityPath, timeout: 8 * time.Millisecond}
	steps, err := provider.BuildSteps(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := steps[0].Run(context.Background()); err == nil {
		t.Fatal("invisible accepted cluster should require recovery")
	}
	if len(fake.deleteIDs) != 0 {
		t.Fatalf("invisible cluster was deleted: %v", fake.deleteIDs)
	}
	if _, err := os.Stat(identityPath); err != nil {
		t.Fatalf("identity state should remain after visibility failure: %v", err)
	}
}

func TestMagnumDestroyClearsIdentityAndBootstrapStateAfterDeletion(t *testing.T) {
	t.Setenv("OPENCENTER_STATE_DIR", t.TempDir())
	cfg := validMagnumLifecycleConfig("magnum-cluster")
	identityPath, err := magnumIdentityStatePath(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := persistObservedMagnumIdentity(identityPath, cfg.ClusterName(), testMagnumUUID); err != nil {
		t.Fatal(err)
	}
	runtimePaths, err := resolveBootstrapRuntimePaths(cfg, "", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestBootstrapState(runtimePaths.StatePath); err != nil {
		t.Fatal(err)
	}
	fake := &lifecycleMagnumService{
		getByID: clusters.Cluster{UUID: testMagnumUUID, Name: cfg.ClusterName(), Status: "CREATE_COMPLETE"},
	}
	client := newLifecycleMagnumProvider(t, fake)
	provider := &magnumDestroyProvider{provider: client, identityPath: identityPath, timeout: time.Second}
	steps, err := provider.BuildSteps(cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := steps[0].Run(context.Background()); err != nil {
		t.Fatalf("destroy error = %v", err)
	}
	if _, err := os.Stat(identityPath); !os.IsNotExist(err) {
		t.Fatalf("identity state still exists: %v", err)
	}
	if _, err := os.Stat(runtimePaths.StatePath); !os.IsNotExist(err) {
		t.Fatalf("bootstrap state still exists: %v", err)
	}
	if len(fake.deleteIDs) != 1 || fake.deleteIDs[0] != testMagnumUUID {
		t.Fatalf("delete identifiers = %v, want UUID", fake.deleteIDs)
	}
}

const testMagnumUUID = "11111111-1111-4111-8111-111111111111"

func newLifecycleMagnumProvider(t *testing.T, service *lifecycleMagnumService) *magnumprovider.Provider {
	t.Helper()
	provider, err := magnumprovider.NewProviderWithService(magnumConfigFromV2(validMagnumLifecycleConfig("magnum-cluster").OpenCenter.Infrastructure.Cloud.Magnum), service)
	if err != nil {
		t.Fatalf("NewProviderWithService() error = %v", err)
	}
	return provider
}

type lifecycleMagnumService struct {
	getByID                clusters.Cluster
	getNameErr             error
	createID               string
	createErr              error
	createCalls            int
	getIDs                 []string
	deleteIDs              []string
	keepAfterDelete        bool
	nameVisibleAfterCreate bool
	idNotFound             bool
	deleted                bool
}

func (f *lifecycleMagnumService) Create(_ clusters.CreateOpts) (string, error) {
	f.createCalls++
	return f.createID, f.createErr
}

func (f *lifecycleMagnumService) Get(id string) (clusters.Cluster, error) {
	f.getIDs = append(f.getIDs, id)
	if id == f.getByID.UUID {
		if f.idNotFound {
			return clusters.Cluster{}, gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}
		}
		if f.deleted && !f.keepAfterDelete {
			return clusters.Cluster{}, gophercloud.ErrUnexpectedResponseCode{Actual: http.StatusNotFound}
		}
		return f.getByID, nil
	}
	if f.nameVisibleAfterCreate && f.createCalls > 0 {
		return f.getByID, nil
	}
	if f.getNameErr != nil {
		return clusters.Cluster{}, f.getNameErr
	}
	return f.getByID, nil
}

func (f *lifecycleMagnumService) List() ([]clusters.Cluster, error) {
	if f.getNameErr != nil {
		return nil, nil
	}
	return []clusters.Cluster{f.getByID}, nil
}

func (f *lifecycleMagnumService) CreateCertificate(string, string) (string, error) {
	return "", errors.New("not used")
}

func (f *lifecycleMagnumService) GetCertificate(string) (string, error) {
	return "", errors.New("not used")
}

func (f *lifecycleMagnumService) Delete(id string) error {
	f.deleteIDs = append(f.deleteIDs, id)
	f.deleted = true
	return nil
}

func writeTestBootstrapState(path string) error {
	data, err := json.Marshal(&bootstrapState{Version: bootstrapStateVersion, Steps: map[string]bootstrapStepState{
		"magnum-create":            {Status: bootstrapStatusSuccess},
		"magnum-wait-ready":        {Status: bootstrapStatusSuccess},
		"magnum-export-kubeconfig": {Status: bootstrapStatusSuccess},
		"openstack-flux-bootstrap": {Status: bootstrapStatusSuccess},
	}})
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func persistObservedMagnumIdentity(path, clusterName, clusterID string) error {
	state := newMagnumIdentityState(clusterName, magnumCreateAccepted)
	state.ClusterID = clusterID
	state.Observed = true
	return persistMagnumIdentityState(path, state)
}

func bootstrapStepIDsForTest(steps []bootstrapStep) []string {
	ids := make([]string, 0, len(steps))
	for _, step := range steps {
		ids = append(ids, step.ID)
	}
	return ids
}

type recordingLifecycleRunner struct{ calls []string }

func (r *recordingLifecycleRunner) Run(_ context.Context, _ string, _ map[string]string, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, name+" "+strings.Join(args, " "))
	return nil, nil
}
