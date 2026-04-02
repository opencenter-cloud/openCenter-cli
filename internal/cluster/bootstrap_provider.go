package cluster

import (
	"context"
	"fmt"
	"os"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

type lifecycleBootstrapProvider interface {
	BuildSteps(cfg *config.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error)
}

type lifecycleCommandRunner interface {
	Run(ctx context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error)
}

type execLifecycleCommandRunner struct {
	commandRunner security.CommandRunner
}

func newExecLifecycleCommandRunner() execLifecycleCommandRunner {
	return execLifecycleCommandRunner{commandRunner: security.GetDefaultCommandRunner()}
}

func (r execLifecycleCommandRunner) Run(ctx context.Context, dir string, env map[string]string, name string, args ...string) ([]byte, error) {
	cmd, err := r.commandRunner.PrepareCommandContext(ctx, name, args...)
	if err != nil {
		return nil, fmt.Errorf("preparing command %s: %w", name, err)
	}
	cmd.Dir = dir

	envList := os.Environ()
	for key, value := range env {
		envList = append(envList, fmt.Sprintf("%s=%s", key, value))
	}
	cmd.Env = envList

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %s %v: %w\nOutput: %s", name, args, err, string(output))
	}

	return output, nil
}
