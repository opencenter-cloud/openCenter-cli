package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

func jsonResponse(t *testing.T, req *http.Request, status int, body any) *http.Response {
	t.Helper()

	data, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}

	return &http.Response{
		StatusCode: status,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(bytes.NewReader(data)),
		Request: req,
	}
}

func rawJSONResponse(req *http.Request, status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header: http.Header{
			"Content-Type": []string{"application/json"},
		},
		Body:    io.NopCloser(strings.NewReader(body)),
		Request: req,
	}
}

func TestClientVersionAndProvisioningFlow(t *testing.T) {
	type state struct {
		userExists bool
		repoExists bool
	}
	current := &state{}

	httpClient := &http.Client{
		Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
			switch {
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/version":
				return jsonResponse(t, req, http.StatusOK, map[string]any{"version": "1.24.5"}), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/users/admin/tokens":
				username, password, ok := req.BasicAuth()
				if !ok || username != "admin" || password != "gitea" {
					t.Fatalf("unexpected admin basic auth: ok=%v user=%q", ok, username)
				}
				return jsonResponse(t, req, http.StatusCreated, map[string]any{"sha1": "admin-token"}), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/user":
				if got := req.Header.Get("Authorization"); got != "token admin-token" && got != "token user-token" {
					return rawJSONResponse(req, http.StatusUnauthorized, `{"message":"unauthorized"}`), nil
				}
				login := "admin"
				if req.Header.Get("Authorization") == "token user-token" {
					login = "newuser"
				}
				return jsonResponse(t, req, http.StatusOK, map[string]any{"login": login}), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/users/newuser":
				if !current.userExists {
					return rawJSONResponse(req, http.StatusNotFound, `{"message":"not found"}`), nil
				}
				return jsonResponse(t, req, http.StatusOK, map[string]any{"login": "newuser", "id": 42}), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/admin/users":
				current.userExists = true
				return jsonResponse(t, req, http.StatusCreated, map[string]any{"login": "newuser", "id": 42}), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/users/newuser/tokens":
				username, password, ok := req.BasicAuth()
				if !ok || username != "newuser" || password != "newuserpassword" {
					t.Fatalf("unexpected user basic auth: ok=%v user=%q", ok, username)
				}
				return jsonResponse(t, req, http.StatusCreated, map[string]any{"sha1": "user-token"}), nil
			case req.Method == http.MethodGet && req.URL.Path == "/api/v1/repos/newuser/test-repo":
				if !current.repoExists {
					return rawJSONResponse(req, http.StatusNotFound, `{"message":"not found"}`), nil
				}
				return jsonResponse(t, req, http.StatusOK, map[string]any{"name": "test-repo"}), nil
			case req.Method == http.MethodPost && req.URL.Path == "/api/v1/user/repos":
				current.repoExists = true
				return jsonResponse(t, req, http.StatusCreated, map[string]any{"name": "test-repo"}), nil
			default:
				t.Fatalf("unexpected request: %s %s", req.Method, req.URL.Path)
				return nil, nil
			}
		}),
	}

	client, err := newClientWithHTTPClient("https://gitea.local", httpClient)
	if err != nil {
		t.Fatalf("newClientWithHTTPClient() error = %v", err)
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
