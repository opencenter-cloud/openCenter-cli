package cluster

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/security"
)

type lifecycleBootstrapProvider interface {
	BuildSteps(cfg *v2.Config, clusterPaths *paths.ClusterPaths, opts *BootstrapOptions) ([]bootstrapStep, error)
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

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	if writer := bootstrapLogWriter(ctx); writer != nil {
		cmd.Stdout = io.MultiWriter(&stdout, writer)
		cmd.Stderr = io.MultiWriter(&stderr, writer)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	err = cmd.Run()
	output := append(stdout.Bytes(), stderr.Bytes()...)
	if err != nil {
		return output, fmt.Errorf("command failed: %s %s: %w\nOutput: %s", name, strings.Join(args, " "), err, string(output))
	}

	return output, nil
}
