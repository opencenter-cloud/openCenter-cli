package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestShellIntegrationBashEvaluatesUseOutputInCurrentShell(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}

	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	writeFakeExecutable(t, filepath.Join(binDir, "opencenter"), `#!/bin/sh
if [ "$1" = "completion" ]; then
  exit 1
fi
if [ "$1" = "cluster" ] && [ "$2" = "use" ]; then
  if [ "${3:-}" = "--clear" ]; then
    echo "unset OPENCENTER_CLUSTER"
    echo "Active cluster cleared"
  else
    echo "export OPENCENTER_CLUSTER=$3"
    echo "Active cluster set to $3"
  fi
  exit 0
fi
exit 1
`)

	integrationPath := filepath.Join("shell-integration", "integration.bash")
	script := `set -e
source "` + integrationPath + `"
opencenter cluster use demo >/dev/null
printf 'SET:%s\n' "${OPENCENTER_CLUSTER:-}"
opencenter cluster use --clear >/dev/null
printf 'CLEAR:%s\n' "${OPENCENTER_CLUSTER-unset}"
`
	// Use a non-login shell (-c, not -lc): a login shell sources profile files
	// that can re-prepend a real opencenter binary to PATH ahead of the fake
	// one this test installs, which makes the test fail on machines/CI runners
	// that have opencenter installed. -c keeps the test hermetic to the PATH
	// set below.
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(),
		"HOME="+dir,
		"PATH="+binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"OPENCENTER_SESSION_FILE="+filepath.Join(dir, "session"),
		"OPENCENTER_CLUSTER=",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bash shell integration smoke failed: %v\n%s", err, output)
	}
	got := string(output)
	for _, want := range []string{"SET:demo\n", "CLEAR:unset\n"} {
		if !strings.Contains(got, want) {
			t.Fatalf("expected output to contain %q, got:\n%s", want, got)
		}
	}
}
