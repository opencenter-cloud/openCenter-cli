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

package cluster

import (
	"context"
	"fmt"
	"os"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
)

// kindCalicoEnabled reports whether Calico must be installed imperatively for a
// kind cluster. Calico applies only in managed-CNI mode: kind's default CNI
// (kindnet) is disabled and Calico is the enabled network plugin with the
// "helm" install method. In the default kind mode (kindnet on) this returns
// false and no imperative install runs.
func kindCalicoEnabled(cfg *v2.Config) bool {
	if cfg == nil || !cfg.IsKind() {
		return false
	}
	kind := cfg.OpenCenter.Infrastructure.Kind
	if kind == nil || !kind.DisableDefaultCNI {
		return false
	}
	calico := cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico
	if calico == nil || !calico.Enabled {
		return false
	}
	method := strings.TrimSpace(calico.InstallMethod)
	if method == "" {
		method = "helm"
	}
	return method == "helm"
}

// installKindCalico installs Calico into a kind cluster via the official Helm
// chart, mirroring the OpenStack imperative install. It runs after the
// kubeconfig is exported and before flux-bootstrap so that a CNI exists before
// Flux schedules its controllers (which cannot start on NotReady, CNI-less
// nodes). Without this, kind's managed-CNI mode deadlocks: Flux is what would
// install Calico, but Flux cannot run until a CNI is present.
func (p *kindBootstrapProvider) installKindCalico(ctx context.Context, cfg *v2.Config, kubeconfigPath string) error {
	if strings.TrimSpace(kubeconfigPath) == "" {
		return fmt.Errorf("kubeconfig path must be set before installing Calico")
	}

	selection, err := openStackCalicoSelection(cfg.OpenCenter.Cluster.Kubernetes.NetworkPlugin.Calico)
	if err != nil {
		return err
	}

	env, err := buildProviderBootstrapEnvironment(cfg, kubeconfigPath)
	if err != nil {
		return err
	}

	tmpDir, err := os.MkdirTemp("", "opencenter-kind-cni-*")
	if err != nil {
		return fmt.Errorf("create temporary CNI install directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()

	env, err = withBootstrapHelmDirectories(env, tmpDir)
	if err != nil {
		return err
	}

	// Idempotency guard: if Calico is already Available, skip the helm upgrade to
	// avoid fighting the tigera-operator for field ownership on re-deploy.
	if out, err := p.runner.Run(ctx, tmpDir, env, "kubectl",
		kubectlArgs(kubeconfigPath, "get", "tigerastatus/calico",
			"-o", "jsonpath={.status.conditions[?(@.type=='Available')].status}")...); err == nil {
		if strings.TrimSpace(string(out)) == "True" {
			logging.Debugf("bootstrap: calico already Available; skipping helm install")
			return nil
		}
	}

	// Apply the operator.tigera.io/v1 CRDs before helm. The tigera-operator
	// chart's own crds/ auto-install is unreliable, so apply them server-side
	// first (matches the OpenStack path).
	crdsURL := fmt.Sprintf(calicoOperatorCRDsURLFormat, selection.Version)
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "apply", "--server-side", "-f", crdsURL)...); err != nil {
		return fmt.Errorf("apply Calico operator CRDs from %s: %w", crdsURL, err)
	}

	// Wait for the required operator CRDs to become Established before running
	// helm. helm's RESTMapper queries kube-apiserver discovery, which registers
	// new CRDs asynchronously after apply returns; without this wait, helm can
	// race and fail with "resource mapping not found ... ensure CRDs are installed first".
	waitArgs := append([]string{"wait", "--for=condition=established"}, calicoRequiredCRDs...)
	waitArgs = append(waitArgs, "--timeout=60s")
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, waitArgs...)...); err != nil {
		return fmt.Errorf("wait for Calico operator CRDs to become established: %w", err)
	}

	if _, err := p.runner.Run(ctx, tmpDir, env, "helm", "repo", "add", calicoHelmRepoName, calicoHelmRepo); err != nil {
		return fmt.Errorf("add Calico Helm repo: %w", err)
	}
	if _, err := p.runner.Run(ctx, tmpDir, env, "helm", "repo", "update", calicoHelmRepoName); err != nil {
		return fmt.Errorf("update Calico Helm repo: %w", err)
	}

	valuesPath, err := resolveCalicoValuesPath(cfg)
	if err != nil {
		return err
	}
	if _, err := os.Stat(valuesPath); err != nil {
		return fmt.Errorf("calico helm values file not found at %s: %w", valuesPath, err)
	}

	if _, err := p.runner.Run(ctx, tmpDir, env, "helm",
		"upgrade", "--install",
		selection.ReleaseName,
		calicoHelmChart,
		"--version", selection.Version,
		"--namespace", selection.Namespace,
		"--create-namespace",
		// The CRDs are applied server-side above. Helm 4 would otherwise apply the
		// chart's crds/ under its own field manager and conflict with "kubectl".
		"--skip-crds",
		"--force-conflicts",
		"--kubeconfig", kubeconfigPath,
		"-f", valuesPath,
	); err != nil {
		return fmt.Errorf("helm install Calico %s: %w", selection.Version, err)
	}

	// Wait for Calico to become Available so nodes go Ready before flux-bootstrap.
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "-n", selection.Namespace, "rollout", "status", "deployment/tigera-operator", "--timeout=5m")...); err != nil {
		return err
	}
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=create", "tigerastatus/calico", "--timeout=5m")...); err != nil {
		return err
	}
	if _, err := p.runner.Run(ctx, tmpDir, env, "kubectl", kubectlArgs(kubeconfigPath, "wait", "--for=condition=Available", "tigerastatus/calico", "--timeout=10m")...); err != nil {
		return err
	}
	return nil
}
