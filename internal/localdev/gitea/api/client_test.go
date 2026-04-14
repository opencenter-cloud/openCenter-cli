package api

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClientVersionAndProvisioningFlow(t *testing.T) {
	type state struct {
		userExists bool
		repoExists bool
	}
	current := &state{}

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/version":
			_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.24.5"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/admin/tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{"sha1": "admin-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/user":
			if got := r.Header.Get("Authorization"); got != "token admin-token" && got != "token user-token" {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			login := "admin"
			if r.Header.Get("Authorization") == "token user-token" {
				login = "newuser"
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"login": login})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/newuser":
			if !current.userExists {
				http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "newuser", "id": 42})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/admin/users":
			current.userExists = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"login": "newuser", "id": 42})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/newuser/tokens":
			_ = json.NewEncoder(w).Encode(map[string]any{"sha1": "user-token"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/repos/newuser/test-repo":
			if !current.repoExists {
				http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "test-repo"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/user/repos":
			current.repoExists = true
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(map[string]any{"name": "test-repo"})
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
	}))
	defer server.Close()

	caPath := writeServerCA(t, server)
	client, err := NewClient(server.URL, caPath)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	version, err := client.Version(context.Background())
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}
	if version != "1.24.5" {
		t.Fatalf("version = %q, want 1.24.5", version)
	}

	adminToken, err := client.CreateTokenWithBasicAuth(context.Background(), "admin", "gitea", "admin-token")
	if err != nil {
		t.Fatalf("CreateTokenWithBasicAuth() error = %v", err)
	}
	if adminToken != "admin-token" {
		t.Fatalf("admin token = %q", adminToken)
	}
	if err := client.VerifyToken(context.Background(), adminToken, "admin"); err != nil {
		t.Fatalf("VerifyToken(admin) error = %v", err)
	}

	if err := client.EnsureUser(context.Background(), adminToken, CreateUserRequest{
		Username: "newuser",
		Email:    "newuser@example.com",
		Password: "newuserpassword",
		FullName: "New User",
	}); err != nil {
		t.Fatalf("EnsureUser() error = %v", err)
	}
	userExists, err := client.UserExists(context.Background(), adminToken, "newuser")
	if err != nil {
		t.Fatalf("UserExists() error = %v", err)
	}
	if !userExists {
		t.Fatal("expected newuser to exist")
	}

	userToken, err := client.CreateTokenWithBasicAuth(context.Background(), "newuser", "newuserpassword", "user-token")
	if err != nil {
		t.Fatalf("CreateTokenWithBasicAuth(newuser) error = %v", err)
	}
	if err := client.VerifyToken(context.Background(), userToken, "newuser"); err != nil {
		t.Fatalf("VerifyToken(newuser) error = %v", err)
	}

	if err := client.EnsureRepository(context.Background(), userToken, "newuser", CreateRepoRequest{
		Name:        "test-repo",
		Description: "local repo",
	}); err != nil {
		t.Fatalf("EnsureRepository() error = %v", err)
	}
	repoExists, err := client.RepoExists(context.Background(), userToken, "newuser", "test-repo")
	if err != nil {
		t.Fatalf("RepoExists() error = %v", err)
	}
	if !repoExists {
		t.Fatal("expected repository to exist")
	}
}

func writeServerCA(t *testing.T, server *httptest.Server) string {
	t.Helper()

	cert := server.Certificate()
	if cert == nil {
		t.Fatal("server certificate is nil")
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw})
	if _, err := x509.ParseCertificate(cert.Raw); err != nil {
		t.Fatalf("ParseCertificate() error = %v", err)
	}

	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, pemBytes, 0o644); err != nil {
		t.Fatalf("write CA: %v", err)
	}
	if data, err := os.ReadFile(path); err != nil || !strings.Contains(string(data), "BEGIN CERTIFICATE") {
		t.Fatalf("expected PEM certificate, err=%v", err)
	}
	return path
}
