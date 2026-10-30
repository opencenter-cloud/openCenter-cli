package cluster

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	corePaths "github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/logging"
)

type bootstrapRuntimePaths struct {
	StatePath       string
	LegacyStatePath string
	LogPath         string

	// GenerationDir is <state>/bootstrap/<org>/<cluster>/<generation-id>. Every
	// resume artifact of one generation lives under it, so a destroy/recreate
	// cycle - which mints a new generation identifier at init - cannot read the
	// dead generation's state: the old file simply is not at the path the new
	// generation looks at. Name-only state could not tell two generations of the
	// same cluster apart, and resumed a wiped cluster into "already completed"
	// steps whose artifacts were gone.
	GenerationDir string

	// UnscopedStatePath is the pre-generation location
	// <state>/bootstrap/<org>/<cluster>/state.json. It is never resumed from: a
	// file there cannot be attributed to any generation. Deploy removes it once
	// the cluster has a generation identifier, so the ambiguous file an older CLI
	// left behind cannot outlive the first run under the new layout.
	UnscopedStatePath string
}

type bootstrapLogWriterKey struct{}

func resolveBootstrapRuntimePaths(cfg *v2.Config, logPath string, startedAt time.Time) (*bootstrapRuntimePaths, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration is nil")
	}

	clusterName := sanitizeRuntimeSegment(cfg.ClusterName())
	if clusterName == "" {
		return nil, fmt.Errorf("cluster name must be set")
	}
	organization := sanitizeRuntimeSegment(cfg.Organization())
	if organization == "" {
		organization = "opencenter"
	}

	stateDir := config.GetStateDir()
	clusterStateDir := filepath.Join(stateDir, "bootstrap", organization, clusterName)
	// The generation identifier scopes the resume state. Configurations written
	// before the field existed have none and keep using the cluster directory
	// directly, so their in-flight resume keeps working; the identifier is
	// stamped on the next load/save, and the generation after that is scoped.
	generationDir := clusterStateDir
	if generation := strings.TrimSpace(cfg.GenerationID()); v2.IsGenerationID(generation) {
		generationDir = filepath.Join(clusterStateDir, generation)
	}
	statePath, err := resolveBootstrapPath(filepath.Join(generationDir, "state.json"))
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap state path: %w", err)
	}

	legacyStatePath := ""
	if legacyClusterDir, err := infrastructureClusterDir(cfg); err == nil {
		legacyStatePath = filepath.Join(legacyClusterDir, "logs", "bootstrap-state.json")
	}

	resolvedLogPath := strings.TrimSpace(logPath)
	if resolvedLogPath == "" {
		resolvedLogPath = filepath.Join(stateDir, "logs", "bootstrap", organization, clusterName, bootstrapLogFileName(startedAt))
	}
	resolvedLogPath, err = resolveBootstrapPath(resolvedLogPath)
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap log path: %w", err)
	}

	resolvedGenerationDir, err := resolveBootstrapPath(generationDir)
	if err != nil {
		return nil, fmt.Errorf("resolve bootstrap generation directory: %w", err)
	}
	// The pre-generation location, resolved only so deploy can retire an
	// ambiguous state.json an older CLI left behind. It is never a resume source.
	unscopedStatePath, err := resolveBootstrapPath(filepath.Join(clusterStateDir, "state.json"))
	if err != nil {
		return nil, fmt.Errorf("resolve pre-generation bootstrap state path: %w", err)
	}

	return &bootstrapRuntimePaths{
		StatePath:         statePath,
		LegacyStatePath:   legacyStatePath,
		LogPath:           resolvedLogPath,
		GenerationDir:     resolvedGenerationDir,
		UnscopedStatePath: unscopedStatePath,
	}, nil
}

func bootstrapLogFileName(startedAt time.Time) string {
	return "bootstrap-" + startedAt.UTC().Format("20060102T150405Z") + ".log"
}

// ResolveDestroyLogPath resolves the log path for cluster destroy operations.
func ResolveDestroyLogPath(cfg *v2.Config, logPath string, startedAt time.Time) (string, error) {
	if cfg == nil {
		return "", fmt.Errorf("configuration is nil")
	}

	clusterName := sanitizeRuntimeSegment(cfg.ClusterName())
	if clusterName == "" {
		return "", fmt.Errorf("cluster name must be set")
	}
	organization := sanitizeRuntimeSegment(cfg.Organization())
	if organization == "" {
		organization = "opencenter"
	}

	stateDir := config.GetStateDir()
	resolvedLogPath := strings.TrimSpace(logPath)
	if resolvedLogPath == "" {
		resolvedLogPath = filepath.Join(stateDir, "logs", "destroy", organization, clusterName, destroyLogFileName(startedAt))
	}
	return resolveBootstrapPath(resolvedLogPath)
}

func destroyLogFileName(startedAt time.Time) string {
	return "destroy-" + startedAt.UTC().Format("20060102T150405Z") + ".log"
}

// BootstrapStateClusterDir returns the runtime state directory holding a
// cluster's deploy resume state: <state-dir>/bootstrap/<org>/<cluster>. It
// contains one subdirectory per cluster generation (plus the unscoped
// state.json of configurations that predate generation identifiers), so
// removing it retires every resume artifact of every past attempt.
//
// Destroy uses it for local-file cleanup; deploy's --restart clears only the
// current generation's file. Returns "" when the cluster name is unusable.
func BootstrapStateClusterDir(clusterName, organization string) string {
	cluster := sanitizeRuntimeSegment(clusterName)
	if cluster == "" {
		return ""
	}
	org := sanitizeRuntimeSegment(organization)
	if org == "" {
		org = "opencenter"
	}
	resolved, err := resolveBootstrapPath(filepath.Join(config.GetStateDir(), "bootstrap", org, cluster))
	if err != nil {
		return ""
	}
	return resolved
}

func resolveBootstrapPath(path string) (string, error) {
	expanded := corePaths.ExpandPath(strings.TrimSpace(path))
	if expanded == "" {
		return "", nil
	}
	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	absolute, err := filepath.Abs(expanded)
	if err != nil {
		return "", err
	}
	return filepath.Clean(absolute), nil
}

func sanitizeRuntimeSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, string(os.PathSeparator), "-")
	return value
}

// OpenActionLogFile creates the directory if needed and opens a log file for writing.
func OpenActionLogFile(path string) (*os.File, error) {
	if strings.TrimSpace(path) == "" {
		return nil, nil
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create log directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}

	return file, nil
}

func openBootstrapLogFile(path string) (*os.File, error) {
	return OpenActionLogFile(path)
}

func withBootstrapLogWriter(ctx context.Context, writer io.Writer) context.Context {
	if writer == nil {
		return ctx
	}
	ctx = logging.WithActionLogWriter(ctx, writer)
	return context.WithValue(ctx, bootstrapLogWriterKey{}, writer)
}

func bootstrapLogWriter(ctx context.Context) io.Writer {
	if ctx == nil {
		return nil
	}
	if writer, ok := ctx.Value(bootstrapLogWriterKey{}).(io.Writer); ok && writer != nil {
		return writer
	}
	return logging.ActionLogWriter(ctx)
}

func logBootstrapMessage(ctx context.Context, format string, args ...interface{}) {
	logging.LogActionMessage(ctx, format, args...)
}
