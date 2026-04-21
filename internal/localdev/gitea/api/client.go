package api

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

// Client is a minimal Gitea API client for the local dev workflow.
type Client struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// Auth represents a supported API authentication mode.
type Auth struct {
	Token    string
	Username string
	Password string
}

// CreateUserRequest describes a user to create.
type CreateUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	FullName string `json:"full_name"`
}

// CreateRepoRequest describes a repository to create.
type CreateRepoRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Private     bool   `json:"private"`
}

type tokenResponse struct {
	SHA1 string `json:"sha1"`
}

type userResponse struct {
	Login string `json:"login"`
	ID    int64  `json:"id"`
}

type versionResponse struct {
	Version string `json:"version"`
}

type repositoryResponse struct {
	Name string `json:"name"`
}

func newClientWithHTTPClient(baseURL string, httpClient *http.Client) (*Client, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, fmt.Errorf("parse gitea URL %q: %w", baseURL, err)
	}
	if httpClient == nil {
		return nil, fmt.Errorf("http client is required")
	}

	return &Client{
		baseURL:    parsed,
		httpClient: httpClient,
	}, nil
}

// NewClient returns a Gitea API client rooted at baseURL and trusted by caPath.
func NewClient(baseURL, caPath string) (*Client, error) {
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, fmt.Errorf("read CA cert %s: %w", caPath, err)
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(caPEM); !ok {
		return nil, fmt.Errorf("parse CA cert %s", caPath)
	}

	return newClientWithHTTPClient(baseURL, &http.Client{
		Timeout: 15 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12},
		},
	})
}

func (c *Client) request(ctx context.Context, method, requestPath string, auth Auth, payload any, out any, expectedStatus ...int) error {
	endpoint := *c.baseURL
	endpoint.Path = path.Join(strings.TrimSuffix(c.baseURL.Path, "/"), requestPath)

	var bodyReader *strings.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal request for %s %s: %w", method, requestPath, err)
		}
		bodyReader = strings.NewReader(string(data))
	} else {
		bodyReader = strings.NewReader("")
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), bodyReader)
	if err != nil {
		return fmt.Errorf("build request %s %s: %w", method, requestPath, err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth.Token != "" {
		req.Header.Set("Authorization", "token "+auth.Token)
	}
	if auth.Username != "" || auth.Password != "" {
		req.SetBasicAuth(auth.Username, auth.Password)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, requestPath, err)
	}
	defer resp.Body.Close()

	allowed := map[int]bool{}
	for _, status := range expectedStatus {
		allowed[status] = true
	}
	if len(allowed) == 0 {
		allowed[http.StatusOK] = true
	}

	if !allowed[resp.StatusCode] {
		var responseBody map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&responseBody)
		return fmt.Errorf("%s %s returned %d: %v", method, requestPath, resp.StatusCode, responseBody)
	}

	if out == nil {
		return nil
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode response for %s %s: %w", method, requestPath, err)
	}

	return nil
}

// Version checks that the API is responding.
func (c *Client) Version(ctx context.Context) (string, error) {
	var response versionResponse
	if err := c.request(ctx, http.MethodGet, "/api/v1/version", Auth{}, nil, &response, http.StatusOK); err != nil {
		return "", err
	}
	return response.Version, nil
}

// VerifyToken ensures the token resolves to the expected user login.
func (c *Client) VerifyToken(ctx context.Context, token, expectedLogin string) error {
	var response userResponse
	if err := c.request(ctx, http.MethodGet, "/api/v1/user", Auth{Token: token}, nil, &response, http.StatusOK); err != nil {
		return err
	}
	if expectedLogin != "" && response.Login != expectedLogin {
		return fmt.Errorf("token login mismatch: got %q want %q", response.Login, expectedLogin)
	}
	return nil
}

// CreateTokenWithBasicAuth creates a personal access token using basic auth.
func (c *Client) CreateTokenWithBasicAuth(ctx context.Context, username, password, tokenName string) (string, error) {
	payload := map[string]any{
		"name":   tokenName,
		"scopes": []string{"all"},
	}

	var response tokenResponse
	requestPath := fmt.Sprintf("/api/v1/users/%s/tokens", username)
	if err := c.request(ctx, http.MethodPost, requestPath, Auth{Username: username, Password: password}, payload, &response, http.StatusCreated, http.StatusOK); err != nil {
		return "", err
	}
	if strings.TrimSpace(response.SHA1) == "" {
		return "", fmt.Errorf("gitea returned an empty token for %s", username)
	}
	return response.SHA1, nil
}

// UserExists reports whether username already exists.
func (c *Client) UserExists(ctx context.Context, token, username string) (bool, error) {
	var response userResponse
	requestPath := fmt.Sprintf("/api/v1/users/%s", username)
	err := c.request(ctx, http.MethodGet, requestPath, Auth{Token: token}, nil, &response, http.StatusOK, http.StatusNotFound)
	if err != nil {
		if strings.Contains(err.Error(), "returned 404") {
			return false, nil
		}
		return false, err
	}
	return response.Login == username, nil
}

// EnsureUser creates the requested user when it does not already exist.
func (c *Client) EnsureUser(ctx context.Context, adminToken string, request CreateUserRequest) error {
	exists, err := c.UserExists(ctx, adminToken, request.Username)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	payload := map[string]any{
		"username":             request.Username,
		"email":                request.Email,
		"password":             request.Password,
		"full_name":            request.FullName,
		"must_change_password": false,
		"send_notify":          false,
	}

	var response userResponse
	if err := c.request(ctx, http.MethodPost, "/api/v1/admin/users", Auth{Token: adminToken}, payload, &response, http.StatusCreated, http.StatusOK); err != nil {
		return err
	}
	return nil
}

// RepoExists reports whether the repository already exists for the owner.
func (c *Client) RepoExists(ctx context.Context, token, owner, repo string) (bool, error) {
	var response repositoryResponse
	requestPath := fmt.Sprintf("/api/v1/repos/%s/%s", owner, repo)
	err := c.request(ctx, http.MethodGet, requestPath, Auth{Token: token}, nil, &response, http.StatusOK, http.StatusNotFound)
	if err != nil {
		if strings.Contains(err.Error(), "returned 404") {
			return false, nil
		}
		return false, err
	}
	return response.Name == repo, nil
}

// EnsureRepository creates the repository when it does not already exist.
func (c *Client) EnsureRepository(ctx context.Context, userToken, owner string, request CreateRepoRequest) error {
	exists, err := c.RepoExists(ctx, userToken, owner, request.Name)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	payload := map[string]any{
		"name":        request.Name,
		"description": request.Description,
		"private":     request.Private,
		"auto_init":   false,
	}

	var response repositoryResponse
	if err := c.request(ctx, http.MethodPost, "/api/v1/user/repos", Auth{Token: userToken}, payload, &response, http.StatusCreated, http.StatusOK); err != nil {
		return err
	}
	return nil
}
