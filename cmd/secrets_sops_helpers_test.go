package cmd

import "testing"

func TestIsSOPSYAMLFile(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// Standard YAML extensions
		{"secrets/secret.yaml", true},
		{"secrets/secret.yml", true},
		{"path/to/config.YAML", true},
		{"path/to/config.YML", true},

		// Encrypted YAML extensions (.yaml.enc / .yml.enc)
		{"infrastructure/clusters/k8s-dev/kubeconfig.yaml.enc", true},
		{"infrastructure/clusters/k8s-prod/kubeconfig.yaml.enc", true},
		{"some/path/secret.yml.enc", true},
		{"UPPER.YAML.ENC", true},

		// Non-YAML files
		{"README.md", false},
		{"script.sh", false},
		{"data.json", false},
		{"binary.enc", false},
		{"notes.txt", false},
		{"Makefile", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := isSOPSYAMLFile(tt.path); got != tt.want {
				t.Errorf("isSOPSYAMLFile(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
