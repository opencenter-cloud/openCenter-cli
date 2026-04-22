package importer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"

	configregistry "github.com/opencenter-cloud/opencenter-cli/internal/config/registry"
	v2 "github.com/opencenter-cloud/opencenter-cli/internal/config/v2"
	"gopkg.in/yaml.v3"
)

type Scanner struct {
	namespaces *NamespaceRegistry
	detectors  *DetectorRegistry
}

func NewScanner() *Scanner {
	return &Scanner{
		namespaces: NewNamespaceRegistry(),
		detectors:  NewDetectorRegistry(),
	}
}

func (s *Scanner) ApplyNamespaceOverrides(overrides []string) error {
	return s.namespaces.ApplyOverrides(overrides)
}

func (s *Scanner) ScanRepo(ctx context.Context, repoPath string) (*ImportScanResult, error) {
	repoPath, err := filepath.Abs(repoPath)
	if err != nil {
		return nil, fmt.Errorf("resolve repo path: %w", err)
	}

	readmePath := filepath.Join(repoPath, "README.md")
	readmeOrg, readmeClusters := parseReadme(readmePath)
	legacyConfigs := discoverLegacyConfigs(repoPath)
	clusterNames, err := discoverClusters(repoPath, readmeClusters, legacyConfigs)
	if err != nil {
		return nil, err
	}

	result := &ImportScanResult{
		RepoPath: repoPath,
		Clusters: make([]ClusterImportResult, 0, len(clusterNames)),
	}

	for _, clusterName := range clusterNames {
		clusterResult := s.scanCluster(ctx, repoPath, clusterName, readmeOrg, readmeClusters[clusterName], legacyConfigs[clusterName])
		if len(clusterResult.Errors) > 0 {
			result.Summary.ClustersWithErrors++
		}
		result.Summary.ConflictCount += len(clusterResult.Conflicts)
		result.Summary.SkippedFieldCount += len(clusterResult.SkippedFields)
		result.Clusters = append(result.Clusters, clusterResult)
	}

	result.Summary.ClustersDiscovered = len(result.Clusters)
	return result, nil
}

func (s *Scanner) scanCluster(ctx context.Context, repoPath, clusterName, readmeOrg string, readmeInfo readmeClusterInfo, legacyPath string) ClusterImportResult {
	sources := buildClusterSources(repoPath, clusterName, legacyPath)
	legacy := loadLegacyConfig(legacyPath)

	provider, providerField, providerConflicts := chooseStringValue(
		readmeInfo.Provider,
		lookupLegacyString(legacy.root, "opencenter", "infrastructure", "provider"),
	)
	provider = canonicalProvider(provider)
	if provider == "" {
		provider = "openstack"
	}

	cfg, err := v2.NewV2Default(clusterName, provider)
	if err != nil {
		return ClusterImportResult{
			ClusterName: clusterName,
			Sources:     sources,
			Errors:      []string{fmt.Sprintf("create default config: %v", err)},
		}
	}

	disableAllServices(cfg)
	cfg.OpenCenter.GitOps.Repository.LocalDir = repoPath
	cfg.OpenCenter.GitOps.Repository.Path = filepath.ToSlash(filepath.Join("applications", "overlays", clusterName))

	organization, orgField := inferOrganization(readmeOrg, sources, legacy)
	cfg.OpenCenter.Meta.Organization = organization
	cfg.OpenCenter.Meta.Name = clusterName
	cfg.OpenCenter.Cluster.ClusterName = clusterName

	environment := inferEnvironment(clusterName)
	cfg.OpenCenter.Meta.Env = environment

	region, regionField, regionConflicts := chooseStringValue(
		readmeInfo.Region,
		lookupLegacyString(legacy.root, "opencenter", "meta", "region"),
	)
	cfg.OpenCenter.Meta.Region = strings.ToLower(strings.TrimSpace(region))

	version, versionField, versionConflicts := chooseStringValue(
		lookupLegacyString(legacy.root, "opencenter", "cluster", "kubernetes", "version"),
		readmeInfo.Version,
	)
	if version != "" {
		cfg.OpenCenter.Cluster.Kubernetes.Version = version
	}

	masterCount, masterField, masterConflicts := chooseIntValue(
		lookupLegacyInt(legacy.root, "opencenter", "cluster", "kubernetes", "master_count"),
		readmeInfo.MasterCount,
	)
	if masterCount > 0 {
		cfg.OpenCenter.Infrastructure.Compute.MasterCount = masterCount
	}

	workerCount, workerField, workerConflicts := chooseIntValue(
		lookupLegacyInt(legacy.root, "opencenter", "cluster", "kubernetes", "worker_count"),
		readmeInfo.WorkerCount,
	)
	if workerCount > 0 {
		cfg.OpenCenter.Infrastructure.Compute.WorkerCount = workerCount
	}

	if gitURL := lookupLegacyString(legacy.root, "opencenter", "gitops", "git_url"); gitURL != "" {
		cfg.OpenCenter.GitOps.Repository.URL = gitURL
	}
	if gitBranch := lookupLegacyString(legacy.root, "opencenter", "gitops", "git_branch"); gitBranch != "" {
		cfg.OpenCenter.GitOps.Repository.Branch = gitBranch
	}
	if gitDir := lookupLegacyString(legacy.root, "opencenter", "gitops", "git_dir"); gitDir != "" {
		cfg.OpenCenter.GitOps.Repository.LocalDir = gitDir
	}

	clusterResult := ClusterImportResult{
		ClusterName:    clusterName,
		Organization:   organization,
		Sources:        sources,
		ProposedConfig: cfg,
	}

	addField := func(path string, value any, confidence ConfidenceLevel, origin FieldOrigin, evidence ...EvidenceRef) {
		clusterResult.FieldResults = append(clusterResult.FieldResults, FieldInferenceResult{
			Path:       path,
			Value:      value,
			Confidence: confidence,
			Origin:     origin,
			Evidence:   evidence,
		})
	}

	addField("opencenter.meta.organization", organization, ConfidenceHigh, FieldOriginGitOps, orgField)
	addField("opencenter.meta.region", cfg.OpenCenter.Meta.Region, ConfidenceHigh, FieldOriginGitOps, regionField)
	addField("opencenter.infrastructure.provider", provider, ConfidenceHigh, FieldOriginGitOps, providerField)
	addField("opencenter.cluster.kubernetes.version", cfg.OpenCenter.Cluster.Kubernetes.Version, ConfidenceHigh, FieldOriginGitOps, versionField)
	addField("opencenter.infrastructure.compute.master_count", cfg.OpenCenter.Infrastructure.Compute.MasterCount, ConfidenceHigh, FieldOriginGitOps, masterField)
	addField("opencenter.infrastructure.compute.worker_count", cfg.OpenCenter.Infrastructure.Compute.WorkerCount, ConfidenceHigh, FieldOriginGitOps, workerField)

	clusterResult.Conflicts = append(clusterResult.Conflicts, stringConflicts("opencenter.infrastructure.provider", providerConflicts)...)
	clusterResult.Conflicts = append(clusterResult.Conflicts, stringConflicts("opencenter.meta.region", regionConflicts)...)
	clusterResult.Conflicts = append(clusterResult.Conflicts, stringConflicts("opencenter.cluster.kubernetes.version", versionConflicts)...)
	clusterResult.Conflicts = append(clusterResult.Conflicts, intConflicts("opencenter.infrastructure.compute.master_count", masterConflicts)...)
	clusterResult.Conflicts = append(clusterResult.Conflicts, intConflicts("opencenter.infrastructure.compute.worker_count", workerConflicts)...)

	detectCtx := &detectContext{
		clusterName: clusterName,
		sources:     sources,
		legacy:      legacy,
		namespaces:  s.namespaces,
		config:      cfg,
	}

	for _, serviceName := range s.detectors.Names() {
		detector := s.detectors.Detector(serviceName)
		if detector == nil {
			continue
		}
		serviceResult := detector.Detect(ctx, detectCtx)
		clusterResult.ServiceResults = append(clusterResult.ServiceResults, serviceResult)
		clusterResult.Conflicts = append(clusterResult.Conflicts, serviceResult.Conflicts...)
		clusterResult.SkippedFields = append(clusterResult.SkippedFields, serviceResult.Skipped...)
	}

	return clusterResult
}

func discoverClusters(repoPath string, readmeClusters map[string]readmeClusterInfo, legacyConfigs map[string]string) ([]string, error) {
	names := make(map[string]struct{})

	addDirs := func(root string) error {
		entries, err := os.ReadDir(root)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names[entry.Name()] = struct{}{}
			}
		}
		return nil
	}

	if err := addDirs(filepath.Join(repoPath, "infrastructure", "clusters")); err != nil {
		return nil, fmt.Errorf("read infrastructure clusters: %w", err)
	}
	if err := addDirs(filepath.Join(repoPath, "applications", "overlays")); err != nil {
		return nil, fmt.Errorf("read application overlays: %w", err)
	}

	for name := range legacyConfigs {
		names[name] = struct{}{}
	}
	for name := range readmeClusters {
		names[name] = struct{}{}
	}

	discovered := make([]string, 0, len(names))
	for name := range names {
		discovered = append(discovered, name)
	}
	sort.Strings(discovered)
	return discovered, nil
}

func buildClusterSources(repoPath, clusterName, legacyPath string) ClusterSources {
	clusterDir := filepath.Join(repoPath, "infrastructure", "clusters", clusterName)
	overlayDir := filepath.Join(repoPath, "applications", "overlays", clusterName)
	kubeconfigs, _ := filepath.Glob(filepath.Join(clusterDir, "kubeconfig*.yaml*"))
	sort.Strings(kubeconfigs)

	return ClusterSources{
		RepoPath:         repoPath,
		ClusterName:      clusterName,
		ClusterDir:       clusterDir,
		OverlayDir:       overlayDir,
		LegacyConfigPath: legacyPath,
		ReadmePath:       filepath.Join(repoPath, "README.md"),
		KubeconfigPaths:  kubeconfigs,
	}
}

type legacyConfigData struct {
	root map[string]any
}

func discoverLegacyConfigs(repoPath string) map[string]string {
	matches, _ := filepath.Glob(filepath.Join(repoPath, ".k8s-*-config.yaml"))
	results := make(map[string]string, len(matches))
	for _, match := range matches {
		base := strings.TrimPrefix(filepath.Base(match), ".")
		base = strings.TrimSuffix(base, "-config.yaml")
		results[base] = match
	}
	return results
}

func loadLegacyConfig(path string) legacyConfigData {
	if path == "" {
		return legacyConfigData{}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return legacyConfigData{}
	}

	root := make(map[string]any)
	if err := yaml.Unmarshal(data, &root); err != nil {
		return legacyConfigData{}
	}

	return legacyConfigData{root: root}
}

func (l legacyConfigData) serviceConfigSection(serviceName string) (any, bool) {
	if l.root == nil {
		return nil, false
	}

	if serviceName == "alert-proxy" {
		if value, ok := lookupLegacyValue(l.root, "opencenter", "managed-service", serviceName); ok {
			return value, true
		}
		if value, ok := lookupLegacyValue(l.root, "opencenter", "managed_services", serviceName); ok {
			return value, true
		}
	}

	return lookupLegacyValue(l.root, "opencenter", "services", serviceName)
}

func decodeLegacyServiceConfig(raw any, target any) error {
	if raw == nil || target == nil {
		return fmt.Errorf("raw service config cannot be nil")
	}

	data, err := yaml.Marshal(raw)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, target)
}

func lookupLegacyValue(root map[string]any, path ...string) (any, bool) {
	var current any = root
	for _, part := range path {
		nextMap, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := nextMap[part]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func lookupLegacyString(root map[string]any, path ...string) string {
	value, ok := lookupLegacyValue(root, path...)
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	}
	return ""
}

func lookupLegacyInt(root map[string]any, path ...string) int {
	value, ok := lookupLegacyValue(root, path...)
	if !ok {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case string:
		i, _ := strconv.Atoi(strings.TrimSpace(typed))
		return i
	}
	return 0
}

func inferOrganization(readmeOrg string, sources ClusterSources, legacy legacyConfigData) (string, EvidenceRef) {
	if org := lookupLegacyString(legacy.root, "opencenter", "meta", "organization"); org != "" {
		return org, EvidenceRef{Source: "legacy-config", Path: sources.LegacyConfigPath}
	}
	if gitDir := lookupLegacyString(legacy.root, "opencenter", "gitops", "git_dir"); gitDir != "" {
		return filepath.Base(gitDir), EvidenceRef{Source: "legacy-config", Path: sources.LegacyConfigPath, Detail: "git_dir"}
	}
	if gitURL := lookupLegacyString(legacy.root, "opencenter", "gitops", "git_url"); gitURL != "" {
		base := strings.TrimSuffix(filepath.Base(gitURL), ".git")
		return base, EvidenceRef{Source: "legacy-config", Path: sources.LegacyConfigPath, Detail: "git_url"}
	}
	if readmeOrg != "" {
		return readmeOrg, EvidenceRef{Source: "readme", Path: sources.ReadmePath}
	}
	return filepath.Base(sources.RepoPath), EvidenceRef{Source: "repo", Path: sources.RepoPath}
}

type readmeClusterInfo struct {
	Region      string
	Provider    string
	Version     string
	MasterCount int
	WorkerCount int
}

var readmeClusterHeader = regexp.MustCompile(`^### .*?\(([^)]+)\)`)
var readmeRegionLine = regexp.MustCompile(`^\- \*\*Region:\*\* ([A-Za-z0-9-]+)`)
var readmeVersionLine = regexp.MustCompile(`v\d+\.\d+\.\d+`)

func parseReadme(path string) (string, map[string]readmeClusterInfo) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", map[string]readmeClusterInfo{}
	}

	lines := strings.Split(string(data), "\n")
	info := make(map[string]readmeClusterInfo)
	organization := ""
	currentCluster := ""

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "# ") && organization == "" {
			organization = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}

		if matches := readmeClusterHeader.FindStringSubmatch(line); len(matches) == 2 {
			currentCluster = strings.TrimSpace(matches[1])
			info[currentCluster] = info[currentCluster]
			continue
		}

		if currentCluster == "" {
			continue
		}

		clusterInfo := info[currentCluster]

		if matches := readmeRegionLine.FindStringSubmatch(line); len(matches) == 2 {
			clusterInfo.Region = strings.ToLower(strings.TrimSpace(matches[1]))
		}

		if strings.Contains(strings.ToLower(line), "vmware vsphere") {
			clusterInfo.Provider = "vmware"
		}
		if strings.Contains(strings.ToLower(line), "openstack") {
			clusterInfo.Provider = "openstack"
		}

		if strings.Contains(line, currentCluster+"-") && strings.Contains(line, "Ready") {
			if strings.Contains(line, "control-plane") {
				clusterInfo.MasterCount++
			} else if strings.Contains(line, "worker") || strings.Contains(line, "<none>") {
				clusterInfo.WorkerCount++
			}
			if clusterInfo.Version == "" {
				clusterInfo.Version = readmeVersionLine.FindString(line)
				if clusterInfo.Version != "" {
					clusterInfo.Version = strings.TrimPrefix(clusterInfo.Version, "v")
				}
			}
		}

		info[currentCluster] = clusterInfo
	}

	return organization, info
}

func inferEnvironment(clusterName string) string {
	name := strings.ToLower(clusterName)
	switch {
	case strings.Contains(name, "prod"):
		return "production"
	case strings.Contains(name, "dev"):
		return "dev"
	default:
		return "staging"
	}
}

func chooseStringValue(values ...string) (string, EvidenceRef, []FieldConflict) {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return "", EvidenceRef{}, nil
	}

	primary := normalized[0]
	conflicts := make([]FieldConflict, 0)
	for _, other := range normalized[1:] {
		if !strings.EqualFold(primary, other) {
			conflicts = append(conflicts, FieldConflict{
				GitOpsValue: primary,
				LiveValue:   other,
				Recommended: "review source disagreement before applying",
			})
		}
	}

	return primary, EvidenceRef{Source: "gitops", Detail: primary}, conflicts
}

func chooseIntValue(values ...int) (int, EvidenceRef, []FieldConflict) {
	normalized := make([]int, 0, len(values))
	for _, value := range values {
		if value > 0 {
			normalized = append(normalized, value)
		}
	}
	if len(normalized) == 0 {
		return 0, EvidenceRef{}, nil
	}

	primary := normalized[0]
	conflicts := make([]FieldConflict, 0)
	for _, other := range normalized[1:] {
		if primary != other {
			conflicts = append(conflicts, FieldConflict{
				GitOpsValue: primary,
				LiveValue:   other,
				Recommended: "review source disagreement before applying",
			})
		}
	}

	return primary, EvidenceRef{Source: "gitops", Detail: strconv.Itoa(primary)}, conflicts
}

func stringConflicts(path string, conflicts []FieldConflict) []FieldConflict {
	for i := range conflicts {
		conflicts[i].Path = path
	}
	return conflicts
}

func intConflicts(path string, conflicts []FieldConflict) []FieldConflict {
	for i := range conflicts {
		conflicts[i].Path = path
	}
	return conflicts
}

func canonicalProvider(provider string) string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "vmware", "vsphere":
		return "vmware"
	case "openstack":
		return "openstack"
	case "kind":
		return "kind"
	case "baremetal":
		return "baremetal"
	default:
		return strings.ToLower(strings.TrimSpace(provider))
	}
}

func overlayServiceEnabled(sources ClusterSources, serviceName string) (bool, EvidenceRef) {
	servicePath := filepath.Join(sources.OverlayDir, "services", serviceName)
	if stat, err := os.Stat(servicePath); err == nil && stat.IsDir() {
		return true, EvidenceRef{Source: "overlay", Path: servicePath}
	}

	managedPath := filepath.Join(sources.OverlayDir, "managed-services", serviceName)
	if stat, err := os.Stat(managedPath); err == nil && stat.IsDir() {
		return true, EvidenceRef{Source: "overlay", Path: managedPath}
	}

	return false, EvidenceRef{}
}

func disableAllServices(cfg *v2.Config) {
	for name, service := range cfg.OpenCenter.Services {
		if base := baseConfigPointer(service); base != nil {
			base.Enabled = false
			base.Namespace = ""
		}
		cfg.OpenCenter.Services[name] = service
	}
	for name, service := range cfg.OpenCenter.ManagedServices {
		if base := baseConfigPointer(service); base != nil {
			base.Enabled = false
			base.Namespace = ""
		}
		cfg.OpenCenter.ManagedServices[name] = service
	}

	// Ensure every registered service has a typed slot available even if NewV2Default omitted it.
	for _, serviceName := range configregistry.GetRegisteredServices() {
		if _, _, ok := serviceConfigTarget(cfg, serviceName); ok {
			continue
		}
		typ := configregistry.GetServiceConfigType(serviceName)
		if typ == nil {
			continue
		}
		instance := reflect.New(typ).Interface()
		enableService(instance)
		if base := baseConfigPointer(instance); base != nil {
			base.Enabled = false
			base.Namespace = ""
		}
		cfg.OpenCenter.Services[serviceName] = instance
	}
}
