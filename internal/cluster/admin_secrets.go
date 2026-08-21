package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
)

const grafanaAdminSecretStepID = "reconcile-grafana-admin-secret"

// newGrafanaAdminSecretStep applies the grafana-admin-password Secret referenced
// by kube-prometheus-stack's Helm values into the observability namespace.
// No-op when secrets.grafana.admin_password is unset.
func newGrafanaAdminSecretStep(cfg *v2.Config, kubeconfigPath string, runner lifecycleCommandRunner) bootstrapStep {
	password := strings.TrimSpace(cfg.Secrets.Grafana.AdminPassword)

	return bootstrapStep{
		ID:          grafanaAdminSecretStepID,
		Description: "Reconcile grafana-admin-password Secret",
		Plan: BootstrapPlanStep{
			ID:     grafanaAdminSecretStepID,
			Action: "Create Secret/grafana-admin-password in observability namespace",
			Reads:  []string{kubeconfigPath},
			Writes: []string{"Secret/grafana-admin-password in namespace observability"},
			Notes:  []string{"Applied imperatively; not tracked in the GitOps repository."},
		},
		Run: func(ctx context.Context) error {
			if password == "" {
				return nil
			}

			manifest := fmt.Sprintf(`---
apiVersion: v1
kind: Namespace
metadata:
  name: observability
---
apiVersion: v1
kind: Secret
metadata:
  name: grafana-admin-password
  namespace: observability
type: Opaque
stringData:
  admin-user: admin
  admin-password: %q
`, password)

			tempFile, err := os.CreateTemp("", "opencenter-grafana-admin-*.yaml")
			if err != nil {
				return fmt.Errorf("create temporary manifest: %w", err)
			}
			defer os.Remove(tempFile.Name())
			if _, err := tempFile.WriteString(manifest); err != nil {
				tempFile.Close()
				return fmt.Errorf("write temporary manifest: %w", err)
			}
			tempFile.Close()
			if err := os.Chmod(tempFile.Name(), 0o600); err != nil {
				return fmt.Errorf("secure temporary manifest: %w", err)
			}
			if _, err := runner.Run(ctx, "", nil, "kubectl", "--kubeconfig", kubeconfigPath, "apply", "-f", tempFile.Name()); err != nil {
				return fmt.Errorf("apply grafana-admin-password Secret: %w", err)
			}
			return nil
		},
	}
}
