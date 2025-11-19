// Copyright 2025 Victor Palma <victor.palma@rackspace.com>
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package barbican

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rackerlabs/openCenter-cli/internal/config"
)

var server *httptest.Server

func TestMain(m *testing.M) {
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Subject-Token", "a-fake-token")
		switch r.URL.Path {
		case "/v3/auth/tokens":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprintln(w, `{"token": {"catalog": [{"type": "key-manager", "endpoints": [{"region": "DFW", "url": "`+server.URL+`", "interface": "public"}]}]}}`)
		case "/v1/secrets":
			fmt.Fprintln(w, `{"secrets": [{"name": "secret1", "secret_ref": "ref1"}, {"name": "secret2", "secret_ref": "ref2"}]}`)
		case "/v1/secrets/ref1/payload":
			fmt.Fprint(w, `"c2VjcmV0MQ=="`)
		case "/v1/secrets/ref1":
			if r.Method == http.MethodDelete {
				w.WriteHeader(http.StatusNoContent)
			} else {
				fmt.Fprintln(w, `{"name": "secret1", "secret_ref": "ref1"}`)
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	m.Run()
}

func TestSecrets(t *testing.T) {
	os.Setenv("OS_USERNAME", "testuser")
	os.Setenv("OS_PASSWORD", "testpass")
	cfg := &config.BarbicanConfig{
		AuthURL: server.URL + "/v3",
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("ListSecrets", func(t *testing.T) {
		secrets, err := client.ListSecrets(context.Background(), nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(secrets) != 2 {
			t.Errorf("expected 2 secrets, got %d", len(secrets))
		}
	})

	t.Run("GetSecret", func(t *testing.T) {
		payload, err := client.GetSecret(context.Background(), "secret1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if string(payload) != "c2VjcmV0MQ==" {
			t.Errorf("expected 'c2VjcmV0MQ==', got '%s'", string(payload))
		}
	})

	t.Run("DescribeSecret", func(t *testing.T) {
		secret, err := client.DescribeSecret(context.Background(), "secret1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if secret.Name != "secret1" {
			t.Errorf("expected 'secret1', got '%s'", secret.Name)
		}
	})

	t.Run("DeleteSecret", func(t *testing.T) {
		err := client.DeleteSecret(context.Background(), "secret1")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}
