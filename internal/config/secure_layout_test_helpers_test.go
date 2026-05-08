package config

import (
	"os"
	"path/filepath"
	"testing"
)

func createSecureConfigTestCluster(t testing.TB, baseDir, organization, clusterName string) string {
	t.Helper()
	stateDir := filepath.Join(baseDir, "state", organization, clusterName)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("create state dir: %v", err)
	}
	return filepath.Join(stateDir, clusterName+"-config.yaml")
}
