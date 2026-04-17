package orchestration

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
)

type PromptKind string

const (
	PromptKindInput   PromptKind = "input"
	PromptKindSelect  PromptKind = "select"
	PromptKindConfirm PromptKind = "confirm"
	PromptKindSecret  PromptKind = "secret"
)

type PromptOption struct {
	Value       string
	Label       string
	Description string
}

type PromptSpec struct {
	ID          string
	Group       string
	Kind        PromptKind
	Label       string
	Description string
	Default     string
	Required    bool
	Options     []PromptOption
	Validate    func(string) error
}

type PromptAnswers map[string]string

type DiscoveryResult struct {
	Warnings []string
	Metadata map[string]any
}

type ConfigPatch struct {
	Group  string
	Path   string
	Label  string
	Value  string
	Masked bool
}

type ManagedFile struct {
	Group    string
	Path     string
	Label    string
	Contents string
	Mode     os.FileMode
	Masked   bool
}

type ChangeSet struct {
	Patches  []ConfigPatch
	Files    []ManagedFile
	Warnings []string
}

type ReviewEntry struct {
	Label  string
	Value  string
	Masked bool
}

type ReviewGroup struct {
	Name    string
	Entries []ReviewEntry
}

type ReviewSpec struct {
	Title  string
	Groups []ReviewGroup
}

type PromptRunner interface {
	Message(message string)
	Warning(message string)
	Prompt(ctx context.Context, prompts []PromptSpec) (PromptAnswers, error)
	Review(ctx context.Context, review ReviewSpec) (bool, error)
}

type ProviderContext struct {
	Provider     string
	ClusterName  string
	Organization string
	ClusterPaths *paths.ClusterPaths
	Discovery    DiscoveryResult
}

type CapabilityRequest struct {
	Name string
}

type ProviderOrchestrator interface {
	Name() string
	Supports(provider string) bool
	Discover(ctx context.Context, cfg *v2.Config) (DiscoveryResult, error)
	Prompts(cfg *v2.Config, discovery DiscoveryResult) []PromptSpec
	ApplyAnswers(cfg *v2.Config, answers PromptAnswers) (ChangeSet, error)
	CapabilityRequests(cfg *v2.Config, discovery DiscoveryResult) []CapabilityRequest
}

type CapabilityHandler interface {
	Name() string
	Applies(cfg *v2.Config, providerCtx ProviderContext) bool
	Discover(ctx context.Context, cfg *v2.Config, providerCtx ProviderContext) (DiscoveryResult, error)
	Prompts(cfg *v2.Config, providerCtx ProviderContext, discovery DiscoveryResult) []PromptSpec
	ApplyAnswers(cfg *v2.Config, answers PromptAnswers, providerCtx ProviderContext) (ChangeSet, error)
}

type ProviderRegistry struct {
	items []ProviderOrchestrator
}

func NewProviderRegistry(items ...ProviderOrchestrator) *ProviderRegistry {
	registry := &ProviderRegistry{items: append([]ProviderOrchestrator(nil), items...)}
	sort.SliceStable(registry.items, func(i, j int) bool {
		return registry.items[i].Name() < registry.items[j].Name()
	})
	return registry
}

func (r *ProviderRegistry) Resolve(provider string) (ProviderOrchestrator, error) {
	selected := strings.TrimSpace(provider)
	for _, item := range r.items {
		if item.Supports(selected) {
			return item, nil
		}
	}
	return nil, fmt.Errorf("no guided configure provider is registered for %q", provider)
}

type CapabilityRegistry struct {
	items map[string]CapabilityHandler
}

func NewCapabilityRegistry(items ...CapabilityHandler) *CapabilityRegistry {
	registry := &CapabilityRegistry{items: make(map[string]CapabilityHandler, len(items))}
	for _, item := range items {
		registry.items[item.Name()] = item
	}
	return registry
}

func (r *CapabilityRegistry) Resolve(name string) (CapabilityHandler, error) {
	item, ok := r.items[strings.TrimSpace(name)]
	if !ok {
		return nil, fmt.Errorf("guided configure capability %q is not registered", name)
	}
	return item, nil
}
