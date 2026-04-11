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

// Factory creates a provider.Provider from a provider name, model name, and its config.
type Factory func(ctx context.Context, name string, model string, pc config.ProviderConfig) (provider.Provider, error)

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

// Get returns a provider for the given model reference.
// The reference can be a "provider/model" string or a short alias defined in
// the config's Models map. Providers are created lazily on first use and
// cached per resolved provider/model pair.
func (c *Container) Get(ctx context.Context, model string) (provider.Provider, error) {
	resolved := model
	if !strings.Contains(model, "/") {
		alias, ok := c.cfg.Models[model]
		if !ok {
			return nil, fmt.Errorf("container: unknown model alias %q", model)
		}
		resolved = alias
	}

	parts := strings.SplitN(resolved, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return nil, fmt.Errorf("container: model must be in provider/model format, got %q", resolved)
	}

	providerName := parts[0]

	c.mu.Lock()
	defer c.mu.Unlock()

	if p, ok := c.cache[resolved]; ok {
		return p, nil
	}

	pc, ok := c.cfg.Providers[providerName]
	if !ok {
		return nil, fmt.Errorf("container: unknown provider %q", providerName)
	}

	modelName := parts[1]
	p, err := c.factory(ctx, providerName, modelName, pc)
	if err != nil {
		return nil, fmt.Errorf("container: create provider %q: %w", providerName, err)
	}

	c.cache[resolved] = p
	return p, nil
}
