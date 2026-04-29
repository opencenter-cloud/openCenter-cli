package cache

import "sync"

type Stats struct {
	DefaultConfigCount  int
	CompleteConfigCount int
}

// DefaultsCache memoizes generated default configs.
type DefaultsCache[T any] struct {
	mu              sync.RWMutex
	defaultConfigs  map[string]T
	completeConfigs map[string]T
	buildDefault    func(string) T
}

func NewDefaultsCache[T any](buildDefault func(string) T) *DefaultsCache[T] {
	return &DefaultsCache[T]{
		defaultConfigs:  make(map[string]T),
		completeConfigs: make(map[string]T),
		buildDefault:    buildDefault,
	}
}

func (c *DefaultsCache[T]) GetDefaultConfig(clusterName string) T {
	c.mu.RLock()
	cached, ok := c.defaultConfigs[clusterName]
	c.mu.RUnlock()
	if ok {
		return cached
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.defaultConfigs[clusterName]; ok {
		return cached
	}

	generated := c.buildDefault(clusterName)
	c.defaultConfigs[clusterName] = generated
	return generated
}

func (c *DefaultsCache[T]) Invalidate(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.defaultConfigs, clusterName)
	delete(c.completeConfigs, clusterName)
}

func (c *DefaultsCache[T]) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultConfigs = make(map[string]T)
	c.completeConfigs = make(map[string]T)
}

func (c *DefaultsCache[T]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{
		DefaultConfigCount:  len(c.defaultConfigs),
		CompleteConfigCount: len(c.completeConfigs),
	}
}
