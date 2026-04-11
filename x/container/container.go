// Package container provides lazy, cached instantiation of LLM providers.
package container

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"wildgecu/pkg/provider"
	"wildgecu/x/config"
)

// Factory creates a provider.Provider from a provider name and its config.
type Factory func(ctx context.Context, name string, pc config.ProviderConfig) (provider.Provider, error)

// Container lazily creates and caches providers on demand.
type Container struct {
	cfg     *config.Config
	factory Factory

	mu    sync.Mutex
	cache map[string]provider.Provider // keyed by "provider/model"
}

// New creates a Container from the given config and factory function.
func New(cfg *config.Config, factory Factory) *Container {
	return &Container{
		cfg:     cfg,
		factory: factory,
		cache:   make(map[string]provider.Provider),
	}
}

// Get returns a provider for the given "provider/model" string.
// Providers are created lazily on first use and cached per provider/model pair.
func (c *Container) Get(ctx context.Context, model string) (provider.Provider, error) {
	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("container: model must be in provider/model format, got %q", model)
	}

	providerName := parts[0]

	c.mu.Lock()
	defer c.mu.Unlock()

	if p, ok := c.cache[model]; ok {
		return p, nil
	}

	pc, ok := c.cfg.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("container: unknown provider %q", providerName)
	}

	p, err := c.factory(ctx, providerName, pc)
	if err != nil {
		return nil, fmt.Errorf("container: create provider %q: %w", providerName, err)
	}

	c.cache[model] = p
	return p, nil
}
