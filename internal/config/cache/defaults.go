package cache

import "sync"

type Stats struct {
	DefaultConfigCount  int
	SchemaDefaultsCount int
	CompleteConfigCount int
}

// DefaultsCache memoizes generated default configs and schema defaults.
type DefaultsCache[T any] struct {
	mu              sync.RWMutex
	defaultConfigs  map[string]T
	schemaDefaults  map[string][]byte
	completeConfigs map[string]T
	buildDefault    func(string) T
	buildSchema     func(string) ([]byte, error)
}

func NewDefaultsCache[T any](buildDefault func(string) T, buildSchema func(string) ([]byte, error)) *DefaultsCache[T] {
	return &DefaultsCache[T]{
		defaultConfigs:  make(map[string]T),
		schemaDefaults:  make(map[string][]byte),
		completeConfigs: make(map[string]T),
		buildDefault:    buildDefault,
		buildSchema:     buildSchema,
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

func (c *DefaultsCache[T]) GetSchemaDefaults(clusterName string) ([]byte, error) {
	c.mu.RLock()
	cached, ok := c.schemaDefaults[clusterName]
	c.mu.RUnlock()
	if ok {
		out := make([]byte, len(cached))
		copy(out, cached)
		return out, nil
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if cached, ok := c.schemaDefaults[clusterName]; ok {
		out := make([]byte, len(cached))
		copy(out, cached)
		return out, nil
	}

	generated, err := c.buildSchema(clusterName)
	if err != nil {
		return nil, err
	}
	c.schemaDefaults[clusterName] = generated

	out := make([]byte, len(generated))
	copy(out, generated)
	return out, nil
}

func (c *DefaultsCache[T]) Invalidate(clusterName string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.defaultConfigs, clusterName)
	delete(c.schemaDefaults, clusterName)
	delete(c.completeConfigs, clusterName)
}

func (c *DefaultsCache[T]) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.defaultConfigs = make(map[string]T)
	c.schemaDefaults = make(map[string][]byte)
	c.completeConfigs = make(map[string]T)
}

func (c *DefaultsCache[T]) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return Stats{
		DefaultConfigCount:  len(c.defaultConfigs),
		SchemaDefaultsCount: len(c.schemaDefaults),
		CompleteConfigCount: len(c.completeConfigs),
	}
}
