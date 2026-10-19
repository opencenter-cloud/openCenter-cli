package cmd

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	provideropenstack "github.com/opencenter-cloud/opencenter-cli/internal/cluster/provider/openstack"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"

	"github.com/spf13/cobra"
)

func TestClusterProviderOpenStackCommandHierarchy(t *testing.T) {
	cluster := NewClusterCmd()
	provider, _, err := cluster.Find([]string{"provider"})
	if err != nil || provider == nil {
		t.Fatalf("cluster provider command not registered: command=%v err=%v", provider, err)
	}
	openstack, _, err := cluster.Find([]string{"provider", "openstack"})
	if err != nil || openstack == nil {
		t.Fatalf("cluster provider openstack command not registered: command=%v err=%v", openstack, err)
	}
	for _, operation := range []string{"plan", "apply"} {
		operationCmd, _, err := cluster.Find([]string{"provider", "openstack", operation})
		if err != nil || operationCmd == nil {
			t.Fatalf("cluster provider openstack %s command not registered: command=%v err=%v", operation, operationCmd, err)
		}
		for _, flag := range []string{"os-cloud", "clouds-yaml", "image-name", "windows-image-name", "network-id", "external-network-id", "subnet-id", "availability-zone", "replace", "import-auth", "import-tls"} {
			if operationCmd.Flags().Lookup(flag) == nil {
				t.Fatalf("%s missing --%s", operation, flag)
			}
		}
	}
}

func TestClusterProviderOpenStackGuardRunsBeforeProfileAccess(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	_, clusterPaths := saveKindConfigForCommandTest(t, dir, "prod", "acme")

	root := NewBuiltinRootCmd()
	root.SetArgs([]string{"cluster", "provider", "openstack", "plan", "acme/prod", "--os-cloud", "missing", "--clouds-yaml", clusterPaths.ConfigPath + ".missing"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "requires provider openstack") {
		t.Fatalf("error = %v, want provider guard before profile access", err)
	}
}

func TestConfirmProviderApplyUsesStderrAndYesBypassesPrompt(t *testing.T) {
	cmd := &cobra.Command{}
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetIn(strings.NewReader("yes\n"))
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Output: OutputText}))
	confirmed, err := confirmProviderApply(cmd)
	if err != nil {
		t.Fatal(err)
	}
	if !confirmed || !strings.Contains(stderr.String(), "Apply the provider-only OpenStack patch?") {
		t.Fatalf("confirmation = %v, stderr = %q", confirmed, stderr.String())
	}

	yesCmd := &cobra.Command{}
	yesCmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Yes: true, Output: OutputText}))
	confirmed, err = confirmProviderApply(yesCmd)
	if err != nil || !confirmed {
		t.Fatalf("--yes confirmation = %v, err = %v", confirmed, err)
	}
}

func TestProviderApplyReviewIsStderrOnlyAndRedacted(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	renderProviderApplyReview(cmd, provideropenstack.Result{
		Changes:  []provideropenstack.Change{{Path: "secret", Old: "<redacted>", New: "<redacted>"}},
		Warnings: []string{"profile secret was redacted"},
	})
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "Provider-only OpenStack apply review") || strings.Contains(stderr.String(), "super-secret") {
		t.Fatalf("review stream or redaction incorrect: stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestProviderExitClassification(t *testing.T) {
	if got := ExitCode(NewExitError(2, "blocked", nil)); got != 2 {
		t.Fatalf("blocked exit code = %d, want 2", got)
	}
	missing := v2.NewConfigNotFoundError("prod", errors.New("not found"))
	if got := ExitCode(missing); got != 3 {
		t.Fatalf("missing config exit code = %d, want 3", got)
	}
	if got := ExitCode(errors.New("operational failure")); got != 1 {
		t.Fatalf("operational exit code = %d, want 1", got)
	}
}

func TestProviderMissingClusterIsConfigNotFound(t *testing.T) {
	dir := t.TempDir()
	prepareCommandTestEnv(t, dir)
	root := NewBuiltinRootCmd()
	root.SetArgs([]string{"cluster", "provider", "openstack", "plan", "missing", "--os-cloud", "prod", "--clouds-yaml", filepath.Join(dir, "clouds.yaml")})
	err := root.Execute()
	var missing *v2.ConfigNotFoundError
	if !errors.As(err, &missing) || ExitCode(err) != 3 {
		t.Fatalf("missing cluster error = %v, type=%T, exit=%d", err, err, ExitCode(err))
	}
}

func TestStructuredProviderApplyRequiresYes(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.SetContext(context.WithValue(context.Background(), globalOptionsContextKey{}, GlobalOptions{Output: OutputJSON}))
	if err := validateProviderApplyInteraction(cmd); err == nil || ExitCode(err) != 2 || !strings.Contains(err.Error(), "requires --yes") {
		t.Fatalf("structured apply guard = %v, exit=%d", err, ExitCode(err))
	}
}

func TestClusterProviderOpenStackCreateInternalNetworkFlagIsShared(t *testing.T) {
	cluster := NewClusterCmd()
	for _, operation := range []string{"plan", "apply"} {
		operationCmd, _, err := cluster.Find([]string{"provider", "openstack", operation})
		if err != nil || operationCmd == nil {
			t.Fatalf("find %s: command=%v err=%v", operation, operationCmd, err)
		}
		flag := operationCmd.Flags().Lookup("create-internal-network")
		if flag == nil || flag.DefValue != "false" {
			t.Fatalf("%s missing shared --create-internal-network flag: %#v", operation, flag)
		}
	}
}

func TestWriteProviderOpenStackOutputIncludesInternalNetworkMode(t *testing.T) {
	cmd := &cobra.Command{}
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	if err := writeProviderOpenStackOutput(cmd, OutputText, provideropenstack.Result{
		Operation:           "cluster.provider.openstack.plan",
		Cluster:             "acme/prod",
		Status:              provideropenstack.StatusNoOp,
		InternalNetworkMode: "tofu-managed",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stdout.String(), "Internal network mode: tofu-managed") {
		t.Fatalf("text output = %q", stdout.String())
	}
}
