package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	openstackcloud "github.com/opencenter-cloud/opencenter-cli/internal/cloud/openstack"
	"github.com/opencenter-cloud/opencenter-cli/internal/cluster/orchestration"
	"github.com/opencenter-cloud/opencenter-cli/internal/config"
	configdefaults "github.com/opencenter-cloud/opencenter-cli/internal/config/defaults"
	configservices "github.com/opencenter-cloud/opencenter-cli/internal/config/services"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/paths"
	"github.com/opencenter-cloud/opencenter-cli/internal/core/validation"
	"github.com/opencenter-cloud/opencenter-cli/internal/util"
	utilerrors "github.com/opencenter-cloud/opencenter-cli/internal/util/errors"
	"github.com/opencenter-cloud/opencenter-cli/internal/util/fs"
	"gopkg.in/yaml.v3"
)

type ConfigureOptions struct {
	Identifier   string
	Organization string
	Provider     string
}

type ConfigureResult struct {
	Created      bool
	ConfigPath   string
	ClusterPaths *paths.ClusterPaths
	Config       *v2.Config
}

type ConfigureService struct {
	pathResolver     *paths.PathResolver
	validationEngine *validation.ValidationEngine
	configManager    *config.ConfigManager
	configurationMgr *config.ConfigurationManager
	fileSystem       fs.FileSystem

	initService  *InitService
	providers    *orchestration.ProviderRegistry
	capabilities *orchestration.CapabilityRegistry
}

func NewConfigureService(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
) *ConfigureService {
	return NewConfigureServiceWithDeps(pathResolver, validationEngine, configManager, nil, nil, nil, nil)
}

func NewConfigureServiceWithDeps(
	pathResolver *paths.PathResolver,
	validationEngine *validation.ValidationEngine,
	configManager *config.ConfigManager,
	configurationMgr *config.ConfigurationManager,
	fileSystem fs.FileSystem,
	providers *orchestration.ProviderRegistry,
	capabilities *orchestration.CapabilityRegistry,
) *ConfigureService {
	if fileSystem == nil {
		errorHandler := utilerrors.NewDefaultErrorHandlerWithoutMasking()
		fileSystem = fs.NewDefaultFileSystem(errorHandler)
	}
	if configurationMgr == nil {
		configurationMgr = config.NewConfigurationManagerWithDeps(
			config.NewConfigIOHandler(fileSystem),
			validationEngine,
			config.NewConfigCache(),
			pathResolver,
			fileSystem,
		)
	}

	initService := NewInitServiceWithConfigMgr(pathResolver, validationEngine, configManager, configurationMgr, fileSystem)
	if providers == nil {
		providers = orchestration.NewProviderRegistry(
			newOpenStackConfigureOrchestrator(openstackcloud.NewDiscoveryClient()),
		)
	}
	if capabilities == nil {
		providerRegistry := configservices.GetProviderRegistry()
		capabilities = orchestration.NewCapabilityRegistry(
			newGitAuthCapabilityHandler(),
			newDNSCapabilityHandler(providerRegistry),
			newObjectStorageCapabilityHandler(providerRegistry),
		)
	}

	return &ConfigureService{
		pathResolver:     pathResolver,
		validationEngine: validationEngine,
		configManager:    configManager,
		configurationMgr: configurationMgr,
		fileSystem:       fileSystem,
		initService:      initService,
		providers:        providers,
		capabilities:     capabilities,
	}
}

func (s *ConfigureService) Configure(ctx context.Context, opts ConfigureOptions, runner orchestration.PromptRunner) (*ConfigureResult, error) {
	if runner == nil {
		return nil, fmt.Errorf("prompt runner is required")
	}

	cfg, clusterPaths, created, err := s.loadOrCreateConfig(ctx, opts)
	if err != nil {
		return nil, err
	}

	if err := s.initService.createDirectories(ctx, clusterPaths, cfg.OpenCenter.Meta.Organization); err != nil {
		return nil, fmt.Errorf("create cluster directories: %w", err)
	}

	provider, err := s.providers.Resolve(cfg.OpenCenter.Infrastructure.Provider)
	if err != nil {
		return nil, err
	}

	review := newChangeReview()

	providerDiscovery, err := s.runProviderFlow(ctx, cfg, runner, provider, review)
	if err != nil {
		return nil, err
	}

	providerCtx := orchestration.ProviderContext{
		Provider:     cfg.OpenCenter.Infrastructure.Provider,
		ClusterName:  cfg.ClusterName(),
		Organization: cfg.Organization(),
		ClusterPaths: clusterPaths,
		Discovery:    providerDiscovery,
	}

	for _, request := range provider.CapabilityRequests(cfg, providerDiscovery) {
		handler, err := s.capabilities.Resolve(request.Name)
		if err != nil {
			return nil, err
		}
		if !handler.Applies(cfg, providerCtx) {
			continue
		}
		if err := s.runCapabilityFlow(ctx, cfg, runner, handler, providerCtx, review); err != nil {
			return nil, err
		}
	}

	confirmed, err := runner.Review(ctx, review.Spec())
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, fmt.Errorf("guided configuration cancelled")
	}

	if _, err := s.initService.generateKeys(clusterPaths, cfg, InitOptions{}); err != nil {
		return nil, fmt.Errorf("ensure cluster keys: %w", err)
	}

	restoreFiles, err := s.writeManagedFiles(review.Files())
	if err != nil {
		return nil, err
	}
	rollbackNeeded := true
	defer func() {
		if rollbackNeeded {
			_ = restoreFiles()
		}
	}()

	if err := s.initService.validateConfig(cfg); err != nil {
		return nil, fmt.Errorf("validate configured cluster: %w", err)
	}

	if err := s.initService.saveConfig(ctx, cfg, clusterPaths.ConfigPath, false); err != nil {
		return nil, fmt.Errorf("save configured cluster: %w", err)
	}

	rollbackNeeded = false
	return &ConfigureResult{
		Created:      created,
		ConfigPath:   clusterPaths.ConfigPath,
		ClusterPaths: clusterPaths,
		Config:       cfg,
	}, nil
}

func (s *ConfigureService) runProviderFlow(
	ctx context.Context,
	cfg *v2.Config,
	runner orchestration.PromptRunner,
	provider orchestration.ProviderOrchestrator,
	review *changeReview,
) (orchestration.DiscoveryResult, error) {
	previousPromptIDs := ""
	lastDiscovery := orchestration.DiscoveryResult{}

	for attempt := 0; attempt < 3; attempt++ {
		discovery, err := provider.Discover(ctx, cfg)
		if err != nil {
			runner.Warning(fmt.Sprintf("%s discovery failed; falling back to manual entry: %v", provider.Name(), err))
		}
		lastDiscovery = discovery
		for _, warning := range discovery.Warnings {
			runner.Warning(warning)
		}

		prompts := provider.Prompts(cfg, discovery)
		promptIDs := joinedPromptIDs(prompts)
		if promptIDs == "" || promptIDs == previousPromptIDs {
			return lastDiscovery, nil
		}
		previousPromptIDs = promptIDs

		answers, err := runner.Prompt(ctx, prompts)
		if err != nil {
			return orchestration.DiscoveryResult{}, err
		}
		changeSet, err := provider.ApplyAnswers(cfg, answers)
		if err != nil {
			return orchestration.DiscoveryResult{}, err
		}
		if err := applyChangeSet(cfg, changeSet); err != nil {
			return orchestration.DiscoveryResult{}, err
		}
		review.Add(changeSet)
	}

	return lastDiscovery, nil
}

func (s *ConfigureService) runCapabilityFlow(
	ctx context.Context,
	cfg *v2.Config,
	runner orchestration.PromptRunner,
	handler orchestration.CapabilityHandler,
	providerCtx orchestration.ProviderContext,
	review *changeReview,
) error {
	previousPromptIDs := ""

	for attempt := 0; attempt < 3; attempt++ {
		discovery, err := handler.Discover(ctx, cfg, providerCtx)
		if err != nil {
			runner.Warning(fmt.Sprintf("%s discovery failed: %v", handler.Name(), err))
		}
		for _, warning := range discovery.Warnings {
			runner.Warning(warning)
		}

		prompts := handler.Prompts(cfg, providerCtx, discovery)
		promptIDs := joinedPromptIDs(prompts)
		if promptIDs == "" || promptIDs == previousPromptIDs {
			return nil
		}
		previousPromptIDs = promptIDs

		answers, err := runner.Prompt(ctx, prompts)
		if err != nil {
			return err
		}
		changeSet, err := handler.ApplyAnswers(cfg, answers, providerCtx)
		if err != nil {
			return err
		}
		if err := applyChangeSet(cfg, changeSet); err != nil {
			return err
		}
		review.Add(changeSet)
	}

	return nil
}

func (s *ConfigureService) loadOrCreateConfig(ctx context.Context, opts ConfigureOptions) (*v2.Config, *paths.ClusterPaths, bool, error) {
	organization, clusterName, err := config.ParseClusterIdentifier(opts.Identifier)
	if err != nil {
		return nil, nil, false, fmt.Errorf("parse cluster identifier: %w", err)
	}
	if strings.TrimSpace(opts.Organization) != "" {
		organization = strings.TrimSpace(opts.Organization)
	}

	var clusterPaths *paths.ClusterPaths
	if organization != "" {
		clusterPaths, err = s.pathResolver.Resolve(ctx, clusterName, organization)
	} else {
		clusterPaths, err = s.pathResolver.ResolveWithFallback(ctx, clusterName)
	}
	if err == nil {
		cfg, loadErr := s.loadV2Config(clusterPaths.ConfigPath)
		if loadErr != nil {
			return nil, nil, false, loadErr
		}
		return cfg, clusterPaths, false, nil
	}

	if organization == "" {
		organization = "opencenter"
	}
	strategy := s.pathResolver.GetStrategies()[0]
	clusterPaths, err = strategy.Resolve(ctx, clusterName, organization)
	if err != nil {
		return nil, nil, false, fmt.Errorf("resolve new cluster paths: %w", err)
	}

	provider := strings.TrimSpace(opts.Provider)
	if provider == "" {
		provider = "openstack"
	}

	initOpts := InitOptions{
		ClusterName:  clusterName,
		Organization: organization,
		Provider:     provider,
		NoGitInit:    true,
	}
	cfg, configMap, err := s.initService.createDefaultConfig(initOpts)
	if err != nil {
		return nil, nil, false, err
	}
	if err := s.initService.applyOverrides(cfg, configMap, initOpts); err != nil {
		return nil, nil, false, err
	}
	s.initService.updateConfigPaths(cfg, configMap, clusterPaths, initOpts)

	return cfg, clusterPaths, true, nil
}

func (s *ConfigureService) loadV2Config(path string) (*v2.Config, error) {
	loader := v2.NewConfigLoader(configdefaults.NewRegistry())
	cfg, err := loader.LoadFromFile(path)
	if err != nil {
		return nil, fmt.Errorf("load native v2 config: %w", err)
	}
	return cfg, nil
}

func applyChangeSet(cfg *v2.Config, changeSet orchestration.ChangeSet) error {
	for _, patch := range changeSet.Patches {
		if err := setConfigField(cfg, patch.Path, patch.Value); err != nil {
			return fmt.Errorf("apply config patch %s: %w", patch.Path, err)
		}
	}
	return nil
}

func setConfigField(obj any, path string, value string) error {
	v := reflect.ValueOf(obj)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("expected pointer to struct, got %T", obj)
	}
	current := v.Elem()
	parts := strings.Split(path, ".")

	for index, part := range parts {
		field := util.FindField(current, part)
		if !field.IsValid() {
			if current.Kind() == reflect.Map {
				if current.IsNil() {
					current.Set(reflect.MakeMap(current.Type()))
				}
				if current.Type().Key().Kind() != reflect.String {
					return fmt.Errorf("map key type must be string")
				}

				if index == len(parts)-1 {
					elemType := current.Type().Elem()
					newValue := reflect.New(elemType).Elem()
					if err := setReflectValue(newValue, value); err != nil {
						return err
					}
					current.SetMapIndex(reflect.ValueOf(part), newValue)
					return nil
				}

				existingValue := current.MapIndex(reflect.ValueOf(part))
				if existingValue.IsValid() {
					resolvedValue := existingValue
					if resolvedValue.Kind() == reflect.Interface && !resolvedValue.IsNil() {
						resolvedValue = resolvedValue.Elem()
					}
					switch {
					case resolvedValue.Kind() == reflect.Ptr && resolvedValue.Type().Elem().Kind() == reflect.Struct:
						current = resolvedValue.Elem()
						continue
					case resolvedValue.Kind() == reflect.Struct:
						current = resolvedValue
						continue
					case resolvedValue.Kind() == reflect.Map:
						current = resolvedValue
						continue
					}
					switch existingValue.Kind() {
					case reflect.Map:
						current = existingValue
						continue
					}
				}

				nestedMap := map[string]any{}
				current.SetMapIndex(reflect.ValueOf(part), reflect.ValueOf(nestedMap))
				current = reflect.ValueOf(nestedMap)
				continue
			}
			return fmt.Errorf("field %q not found in %s", part, current.Type().Name())
		}

		if index == len(parts)-1 {
			return setReflectValue(field, value)
		}

		switch {
		case field.Kind() == reflect.Struct:
			current = field
		case field.Kind() == reflect.Ptr && field.Type().Elem().Kind() == reflect.Struct:
			if field.IsNil() {
				field.Set(reflect.New(field.Type().Elem()))
			}
			current = field.Elem()
		case field.Kind() == reflect.Map:
			if field.IsNil() {
				field.Set(reflect.MakeMap(field.Type()))
			}
			current = field
		default:
			return fmt.Errorf("field %q is not traversable", part)
		}
	}

	return nil
}

func setReflectValue(field reflect.Value, value string) error {
	if !field.CanSet() {
		return fmt.Errorf("field cannot be set")
	}

	switch field.Kind() {
	case reflect.String:
		field.SetString(value)
	case reflect.Bool:
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid boolean %q", value)
		}
		field.SetBool(parsed)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
		if err != nil {
			return fmt.Errorf("invalid integer %q", value)
		}
		field.SetInt(parsed)
	case reflect.Interface:
		field.Set(reflect.ValueOf(value))
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}

type managedFileBackup struct {
	path    string
	existed bool
	data    []byte
	mode    os.FileMode
}

func (s *ConfigureService) writeManagedFiles(files []orchestration.ManagedFile) (func() error, error) {
	backups := make([]managedFileBackup, 0, len(files))

	for _, file := range files {
		backup := managedFileBackup{path: file.Path}
		if info, err := os.Stat(file.Path); err == nil {
			backup.existed = true
			backup.mode = info.Mode()
			data, readErr := os.ReadFile(file.Path)
			if readErr != nil {
				return nil, fmt.Errorf("backup managed file %s: %w", file.Path, readErr)
			}
			backup.data = data
		}
		backups = append(backups, backup)

		if err := os.MkdirAll(filepath.Dir(file.Path), 0o755); err != nil {
			return nil, fmt.Errorf("create managed file directory for %s: %w", file.Path, err)
		}

		mode := file.Mode
		if mode == 0 {
			mode = 0o600
		}
		if err := s.fileSystem.WriteFileAtomic(file.Path, []byte(file.Contents), mode); err != nil {
			return nil, fmt.Errorf("write managed file %s: %w", file.Path, err)
		}
	}

	return func() error {
		for index := len(backups) - 1; index >= 0; index-- {
			backup := backups[index]
			if backup.existed {
				if err := s.fileSystem.WriteFileAtomic(backup.path, backup.data, backup.mode); err != nil {
					return err
				}
				continue
			}
			if err := os.Remove(backup.path); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		return nil
	}, nil
}

type changeReview struct {
	patches map[string]orchestration.ConfigPatch
	files   map[string]orchestration.ManagedFile
}

func newChangeReview() *changeReview {
	return &changeReview{
		patches: make(map[string]orchestration.ConfigPatch),
		files:   make(map[string]orchestration.ManagedFile),
	}
}

func (r *changeReview) Add(changeSet orchestration.ChangeSet) {
	for _, patch := range changeSet.Patches {
		r.patches[patch.Path] = patch
	}
	for _, file := range changeSet.Files {
		r.files[file.Path] = file
	}
}

func (r *changeReview) Files() []orchestration.ManagedFile {
	files := make([]orchestration.ManagedFile, 0, len(r.files))
	for _, file := range r.files {
		files = append(files, file)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files
}

func (r *changeReview) Spec() orchestration.ReviewSpec {
	grouped := make(map[string][]orchestration.ReviewEntry)
	for _, patch := range r.patches {
		grouped[patch.Group] = append(grouped[patch.Group], orchestration.ReviewEntry{
			Label:  patch.Label,
			Value:  patch.Value,
			Masked: patch.Masked,
		})
	}
	for _, file := range r.files {
		grouped[file.Group] = append(grouped[file.Group], orchestration.ReviewEntry{
			Label:  file.Label,
			Value:  file.Path,
			Masked: false,
		})
	}

	groupNames := make([]string, 0, len(grouped))
	for groupName := range grouped {
		groupNames = append(groupNames, groupName)
	}
	sort.Strings(groupNames)

	groups := make([]orchestration.ReviewGroup, 0, len(groupNames))
	for _, groupName := range groupNames {
		entries := grouped[groupName]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Label < entries[j].Label })
		groups = append(groups, orchestration.ReviewGroup{Name: groupName, Entries: entries})
	}

	return orchestration.ReviewSpec{
		Title:  "Guided cluster configuration review",
		Groups: groups,
	}
}

func joinedPromptIDs(prompts []orchestration.PromptSpec) string {
	if len(prompts) == 0 {
		return ""
	}
	ids := make([]string, 0, len(prompts))
	for _, prompt := range prompts {
		ids = append(ids, prompt.ID)
	}
	return strings.Join(ids, ",")
}

func cloneConfigMap(cfg *v2.Config) (map[string]any, error) {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	var configMap map[string]any
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return nil, err
	}
	return configMap, nil
}
