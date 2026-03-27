package sops

import "testing"

func TestNeedsExplicitYAMLType(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		// SOPS-native extensions — no hint needed
		{"secret.yaml", false},
		{"secret.yml", false},
		{"config.json", false},
		{"vars.env", false},
		{"settings.ini", false},

		// Non-native extensions — hint required
		{"kubeconfig.yaml.enc", true},
		{"kubeconfig.yml.enc", true},
		{"secret.bak", true},
		{"data.txt", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			if got := needsExplicitYAMLType(tt.path); got != tt.want {
				t.Errorf("needsExplicitYAMLType(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}
