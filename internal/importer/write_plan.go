package importer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"gopkg.in/yaml.v3"
)

type ClusterWritePlan struct {
	ClusterName     string
	Organization    string
	ConfigPath      string
	Create          bool
	ApprovedFields  []FieldInferenceResult
	SkippedFields   []SkippedField
	RenderedContent []byte
	Diff            string
}

func PrepareClusterWritePlan(ctx context.Context, cluster ClusterImportResult) (*ClusterWritePlan, error) {
	if cluster.ProposedConfig == nil {
		return nil, fmt.Errorf("cluster %q has no proposed config", cluster.ClusterName)
	}

	organization := strings.TrimSpace(cluster.Organization)
	if organization == "" {
		organization = strings.TrimSpace(cluster.ProposedConfig.OpenCenter.Meta.Organization)
	}
	if organization == "" {
		return nil, fmt.Errorf("cluster %q is missing organization metadata", cluster.ClusterName)
	}

	clusterName := strings.TrimSpace(cluster.ClusterName)
	if clusterName == "" {
		clusterName = strings.TrimSpace(cluster.ProposedConfig.OpenCenter.Meta.Name)
	}
	if clusterName == "" {
		clusterName = strings.TrimSpace(cluster.ProposedConfig.OpenCenter.Cluster.ClusterName)
	}
	if clusterName == "" {
		return nil, fmt.Errorf("cluster is missing a name")
	}

	resolver := paths.NewPathResolver(config.ResolveClustersDir())
	if err := resolver.CreateClusterDirectories(ctx, clusterName, organization); err != nil {
		return nil, fmt.Errorf("create cluster directories: %w", err)
	}

	clusterPaths, err := resolver.Resolve(ctx, clusterName, organization)
	if err != nil {
		return nil, fmt.Errorf("resolve cluster paths: %w", err)
	}

	approved, skipped := SelectApprovedFields(cluster)
	skipped = append(skipped, cluster.SkippedFields...)

	plan := &ClusterWritePlan{
		ClusterName:    clusterName,
		Organization:   organization,
		ConfigPath:     clusterPaths.ConfigPath,
		ApprovedFields: approved,
		SkippedFields:  skipped,
	}

	if _, err := os.Stat(clusterPaths.ConfigPath); err == nil {
		plan.Create = false
		rendered, diff, err := patchYAMLBytesFromFile(clusterPaths.ConfigPath, approved)
		if err != nil {
			return nil, err
		}
		plan.RenderedContent = rendered
		plan.Diff = diff
		return plan, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("stat config path: %w", err)
	}

	plan.Create = true
	cfgCopy, err := cloneConfig(cluster.ProposedConfig)
	if err != nil {
		return nil, err
	}
	pruneDisabledServices(cfgCopy)
	content, err := marshalConfig(cfgCopy)
	if err != nil {
		return nil, err
	}
	diff, err := buildUnifiedDiff(clusterPaths.ConfigPath, "", string(content))
	if err != nil {
		return nil, err
	}
	plan.RenderedContent = content
	plan.Diff = diff
	return plan, nil
}

func ApplyClusterWritePlan(plan *ClusterWritePlan) error {
	if plan == nil {
		return fmt.Errorf("write plan cannot be nil")
	}
	if plan.ConfigPath == "" {
		return fmt.Errorf("write plan is missing config path")
	}

	if err := os.MkdirAll(filepath.Dir(plan.ConfigPath), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	if !plan.Create {
		if current, err := os.ReadFile(plan.ConfigPath); err == nil {
			if err := os.WriteFile(plan.ConfigPath+".backup", current, 0o600); err != nil {
				return fmt.Errorf("write config backup: %w", err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read existing config for backup: %w", err)
		}
	}

	if err := os.WriteFile(plan.ConfigPath, plan.RenderedContent, 0o600); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func patchYAMLBytesFromFile(path string, updates []FieldInferenceResult) ([]byte, string, error) {
	original, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("read yaml file: %w", err)
	}
	return patchYAMLBytes(path, original, updates)
}

func patchYAMLBytes(path string, original []byte, updates []FieldInferenceResult) ([]byte, string, error) {
	var document yaml.Node
	if err := yaml.Unmarshal(original, &document); err != nil {
		return nil, "", fmt.Errorf("decode yaml document: %w", err)
	}

	root := ensureDocumentRoot(&document)
	for _, update := range updates {
		if err := setYAMLPath(root, update.Path, update.Value); err != nil {
			return nil, "", fmt.Errorf("set yaml path %q: %w", update.Path, err)
		}
	}

	var rendered bytes.Buffer
	encoder := yaml.NewEncoder(&rendered)
	encoder.SetIndent(2)
	if err := encoder.Encode(&document); err != nil {
		return nil, "", fmt.Errorf("encode patched yaml: %w", err)
	}
	if err := encoder.Close(); err != nil {
		return nil, "", fmt.Errorf("close yaml encoder: %w", err)
	}

	patched := rendered.Bytes()
	diff, err := buildUnifiedDiff(path, string(original), string(patched))
	if err != nil {
		return nil, "", fmt.Errorf("build yaml diff: %w", err)
	}
	return patched, diff, nil
}

func cloneConfig(cfg *v2.Config) (*v2.Config, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config clone: %w", err)
	}
	var cloned v2.Config
	if err := yaml.Unmarshal(data, &cloned); err != nil {
		return nil, fmt.Errorf("unmarshal config clone: %w", err)
	}
	return &cloned, nil
}

func pruneDisabledServices(cfg *v2.Config) {
	if cfg == nil {
		return
	}
	for name, service := range cfg.OpenCenter.Services {
		if base := baseConfigPointer(service); base != nil && !base.Enabled {
			delete(cfg.OpenCenter.Services, name)
		}
	}
	for name, service := range cfg.OpenCenter.ManagedServices {
		if base := baseConfigPointer(service); base != nil && !base.Enabled {
			delete(cfg.OpenCenter.ManagedServices, name)
		}
	}
}

func marshalConfig(cfg *v2.Config) ([]byte, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("marshal config: %w", err)
	}
	return data, nil
}
