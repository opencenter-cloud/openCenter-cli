/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package template

import (
	"bytes"
	"fmt"
	"sync"
	"text/template"
	"time"
)

// TemplateCache provides caching for compiled templates and rendered results
type TemplateCache struct {
	// Compiled template cache
	templates   map[string]*template.Template
	templatesMu sync.RWMutex

	// Rendered result cache
	results   map[string]*CachedResult
	resultsMu sync.RWMutex

	// Configuration
	maxCacheSize int
	defaultTTL   time.Duration
	enableCache  bool
}

// CachedResult represents a cached template rendering result
type CachedResult struct {
	Content   string
	Timestamp time.Time
	TTL       time.Duration
	DataHash  string // Hash of the data used for rendering
}

// IsExpired checks if the cached result has expired
func (cr *CachedResult) IsExpired() bool {
	if cr.TTL <= 0 {
		return false // No expiration
	}
	return time.Since(cr.Timestamp) > cr.TTL
}

// NewTemplateCache creates a new template cache
func NewTemplateCache(maxCacheSize int, defaultTTL time.Duration) *TemplateCache {
	if maxCacheSize <= 0 {
		maxCacheSize = 100
	}
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}

	return &TemplateCache{
		templates:    make(map[string]*template.Template),
		results:      make(map[string]*CachedResult),
		maxCacheSize: maxCacheSize,
		defaultTTL:   defaultTTL,
		enableCache:  true,
	}
}

// GetTemplate retrieves a compiled template from cache
func (tc *TemplateCache) GetTemplate(name string) (*template.Template, bool) {
	if !tc.enableCache {
		return nil, false
	}

	tc.templatesMu.RLock()
	defer tc.templatesMu.RUnlock()

	tmpl, exists := tc.templates[name]
	return tmpl, exists
}

// SetTemplate stores a compiled template in cache
func (tc *TemplateCache) SetTemplate(name string, tmpl *template.Template) {
	if !tc.enableCache || tmpl == nil {
		return
	}

	tc.templatesMu.Lock()
	defer tc.templatesMu.Unlock()

	// Check cache size and evict if necessary
	if len(tc.templates) >= tc.maxCacheSize {
		tc.evictOldestTemplate()
	}

	tc.templates[name] = tmpl
}

// GetResult retrieves a cached rendering result
func (tc *TemplateCache) GetResult(key string) (string, bool) {
	if !tc.enableCache {
		return "", false
	}

	tc.resultsMu.RLock()
	defer tc.resultsMu.RUnlock()

	result, exists := tc.results[key]
	if !exists {
		return "", false
	}

	// Check if result has expired
	if result.IsExpired() {
		go tc.removeExpiredResult(key)
		return "", false
	}

	return result.Content, true
}

// SetResult stores a rendering result in cache
func (tc *TemplateCache) SetResult(key string, content string, dataHash string) {
	if !tc.enableCache {
		return
	}

	tc.resultsMu.Lock()
	defer tc.resultsMu.Unlock()

	// Check cache size and evict if necessary
	if len(tc.results) >= tc.maxCacheSize {
		tc.evictOldestResult()
	}

	tc.results[key] = &CachedResult{
		Content:   content,
		Timestamp: time.Now(),
		TTL:       tc.defaultTTL,
		DataHash:  dataHash,
	}
}

// InvalidateTemplate removes a template from cache
func (tc *TemplateCache) InvalidateTemplate(name string) {
	tc.templatesMu.Lock()
	defer tc.templatesMu.Unlock()

	delete(tc.templates, name)
}

// InvalidateResult removes a result from cache
func (tc *TemplateCache) InvalidateResult(key string) {
	tc.resultsMu.Lock()
	defer tc.resultsMu.Unlock()

	delete(tc.results, key)
}

// InvalidateAll clears all caches
func (tc *TemplateCache) InvalidateAll() {
	tc.templatesMu.Lock()
	tc.resultsMu.Lock()
	defer tc.templatesMu.Unlock()
	defer tc.resultsMu.Unlock()

	tc.templates = make(map[string]*template.Template)
	tc.results = make(map[string]*CachedResult)
}

// CleanupExpired removes all expired results from cache
func (tc *TemplateCache) CleanupExpired() {
	tc.resultsMu.Lock()
	defer tc.resultsMu.Unlock()

	keysToDelete := make([]string, 0)
	for key, result := range tc.results {
		if result.IsExpired() {
			keysToDelete = append(keysToDelete, key)
		}
	}

	for _, key := range keysToDelete {
		delete(tc.results, key)
	}
}

// StartCleanupRoutine starts a background routine to clean up expired results
func (tc *TemplateCache) StartCleanupRoutine(interval time.Duration) chan struct{} {
	if interval <= 0 {
		interval = time.Minute
	}

	stopChan := make(chan struct{})
	ticker := time.NewTicker(interval)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-stopChan:
				return
			case <-ticker.C:
				tc.CleanupExpired()
			}
		}
	}()

	return stopChan
}

// GetStats returns cache statistics
func (tc *TemplateCache) GetStats() map[string]interface{} {
	tc.templatesMu.RLock()
	tc.resultsMu.RLock()
	defer tc.templatesMu.RUnlock()
	defer tc.resultsMu.RUnlock()

	expired := 0
	for _, result := range tc.results {
		if result.IsExpired() {
			expired++
		}
	}

	return map[string]interface{}{
		"template_count":  len(tc.templates),
		"result_count":    len(tc.results),
		"expired_results": expired,
		"max_cache_size":  tc.maxCacheSize,
		"default_ttl":     tc.defaultTTL.String(),
		"cache_enabled":   tc.enableCache,
	}
}

// SetEnabled enables or disables caching
func (tc *TemplateCache) SetEnabled(enabled bool) {
	tc.enableCache = enabled
}

// IsEnabled returns whether caching is enabled
func (tc *TemplateCache) IsEnabled() bool {
	return tc.enableCache
}

// SetMaxCacheSize sets the maximum cache size
func (tc *TemplateCache) SetMaxCacheSize(size int) {
	if size <= 0 {
		return
	}

	tc.maxCacheSize = size

	// Evict entries if current size exceeds new max size
	tc.templatesMu.Lock()
	for len(tc.templates) > tc.maxCacheSize {
		tc.evictOldestTemplate()
	}
	tc.templatesMu.Unlock()

	tc.resultsMu.Lock()
	for len(tc.results) > tc.maxCacheSize {
		tc.evictOldestResult()
	}
	tc.resultsMu.Unlock()
}

// SetDefaultTTL sets the default TTL for cached results
func (tc *TemplateCache) SetDefaultTTL(ttl time.Duration) {
	if ttl > 0 {
		tc.defaultTTL = ttl
	}
}

// removeExpiredResult removes an expired result (called asynchronously)
func (tc *TemplateCache) removeExpiredResult(key string) {
	tc.resultsMu.Lock()
	defer tc.resultsMu.Unlock()

	result, exists := tc.results[key]
	if exists && result.IsExpired() {
		delete(tc.results, key)
	}
}

// evictOldestTemplate removes the oldest template from cache
func (tc *TemplateCache) evictOldestTemplate() {
	if len(tc.templates) == 0 {
		return
	}

	// Simple eviction: remove first entry
	// In a production system, you might want LRU or LFU eviction
	for key := range tc.templates {
		delete(tc.templates, key)
		break
	}
}

// evictOldestResult removes the oldest result from cache
func (tc *TemplateCache) evictOldestResult() {
	if len(tc.results) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time
	first := true

	for key, result := range tc.results {
		if first || result.Timestamp.Before(oldestTime) {
			oldestKey = key
			oldestTime = result.Timestamp
			first = false
		}
	}

	if oldestKey != "" {
		delete(tc.results, oldestKey)
	}
}

// GenerateCacheKey generates a cache key from template name and data hash
func GenerateCacheKey(templateName string, dataHash string) string {
	return fmt.Sprintf("%s:%s", templateName, dataHash)
}

// CachedTemplateRenderer wraps a TemplateRenderer with caching capabilities
type CachedTemplateRenderer struct {
	renderer TemplateRenderer
	cache    *TemplateCache
}

// NewCachedTemplateRenderer creates a new cached template renderer
func NewCachedTemplateRenderer(renderer TemplateRenderer, cache *TemplateCache) *CachedTemplateRenderer {
	if cache == nil {
		cache = NewTemplateCache(100, 5*time.Minute)
	}

	return &CachedTemplateRenderer{
		renderer: renderer,
		cache:    cache,
	}
}

// RenderTemplate renders a template with caching
func (ctr *CachedTemplateRenderer) RenderTemplate(templateName string, data interface{}) (string, error) {
	// Generate data hash for cache key
	dataHash := fmt.Sprintf("%v", data) // Simple hash - in production use proper hashing
	cacheKey := GenerateCacheKey(templateName, dataHash)

	// Check cache first
	if content, found := ctr.cache.GetResult(cacheKey); found {
		return content, nil
	}

	// Render template
	content, err := ctr.renderer.RenderTemplate(templateName, data)
	if err != nil {
		return "", err
	}

	// Cache the result
	ctr.cache.SetResult(cacheKey, content, dataHash)

	return content, nil
}

// RenderTemplateToWriter renders a template to a writer (no caching for streaming)
func (ctr *CachedTemplateRenderer) RenderTemplateToWriter(templateName string, data interface{}, writer *bytes.Buffer) error {
	return ctr.renderer.RenderTemplateToWriter(templateName, data, writer)
}

// GetTemplate retrieves a template with caching
func (ctr *CachedTemplateRenderer) GetTemplate(templateName string) (*template.Template, error) {
	// Check cache first
	if tmpl, found := ctr.cache.GetTemplate(templateName); found {
		return tmpl, nil
	}

	// Get template from renderer
	tmpl, err := ctr.renderer.GetTemplate(templateName)
	if err != nil {
		return nil, err
	}

	// Cache the template
	ctr.cache.SetTemplate(templateName, tmpl)

	return tmpl, nil
}

// ListTemplates lists available templates
func (ctr *CachedTemplateRenderer) ListTemplates() []string {
	return ctr.renderer.ListTemplates()
}

// AddFunctions adds custom functions to the renderer
func (ctr *CachedTemplateRenderer) AddFunctions(funcMap template.FuncMap) error {
	// Invalidate cache when functions change
	ctr.cache.InvalidateAll()
	return ctr.renderer.AddFunctions(funcMap)
}

// GetCache returns the underlying cache for statistics and management
func (ctr *CachedTemplateRenderer) GetCache() *TemplateCache {
	return ctr.cache
}
