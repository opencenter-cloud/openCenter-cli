package security

import (
	"context"
	"os/exec"
)

// CommandRunner centralizes sanitized external command construction.
type CommandRunner interface {
	PrepareCommand(name string, args ...string) (*exec.Cmd, error)
	PrepareCommandContext(ctx context.Context, name string, args ...string) (*exec.Cmd, error)
}

// DefaultCommandRunner prepares sanitized commands using a CommandSanitizer.
type DefaultCommandRunner struct {
	sanitizer CommandSanitizer
}

var defaultCommandRunner = NewCommandRunner(nil)

// NewCommandRunner creates a command runner backed by the provided sanitizer.
func NewCommandRunner(sanitizer CommandSanitizer) *DefaultCommandRunner {
	if sanitizer == nil {
		sanitizer = NewDefaultCommandSanitizer()
	}

	return &DefaultCommandRunner{sanitizer: sanitizer}
}

// GetDefaultCommandRunner returns the process-wide default command runner.
func GetDefaultCommandRunner() CommandRunner {
	return defaultCommandRunner
}

// PrepareCommand returns a sanitized exec.Cmd without a bound context.
func (r *DefaultCommandRunner) PrepareCommand(name string, args ...string) (*exec.Cmd, error) {
	commandPath, commandArgs, err := r.sanitize(name, args)
	if err != nil {
		return nil, err
	}

	return exec.Command(commandPath, commandArgs...), nil
}

// PrepareCommandContext returns a sanitized exec.Cmd bound to ctx.
func (r *DefaultCommandRunner) PrepareCommandContext(ctx context.Context, name string, args ...string) (*exec.Cmd, error) {
	commandPath, commandArgs, err := r.sanitize(name, args)
	if err != nil {
		return nil, err
	}

	return exec.CommandContext(ctx, commandPath, commandArgs...), nil
}

func (r *DefaultCommandRunner) sanitize(name string, args []string) (string, []string, error) {
	cmd, err := r.sanitizer.SanitizeCommand(name, append([]string(nil), args...))
	if err != nil {
		return "", nil, err
	}

	return cmd.Path, cmd.Args[1:], nil
}
