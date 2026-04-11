package container

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"

	"wildgecu/pkg/provider"
	"wildgecu/x/config"
)

// fakeProvider is a minimal provider for testing.
type fakeProvider struct{ id string }

func (f *fakeProvider) Generate(_ context.Context, _ *provider.GenerateParams) (*provider.Response, error) {
	return &provider.Response{}, nil
}

func TestContainer(t *testing.T) {
	t.Run("GetReturnsProviderForValidModel", func(t *testing.T) {
		var called int
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			called++
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"mygemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "mygemini/gemini-flash",
		}

		c := New(cfg, factory)

		p, err := c.Get(context.Background(), "mygemini/gemini-flash")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}
		if p == nil {
			t.Fatal("Get() returned nil provider")
		}
		fp := p.(*fakeProvider)
		if fp.id != "mygemini" {
			t.Errorf("provider id = %q, want %q", fp.id, "mygemini")
		}
		if called != 1 {
			t.Errorf("factory called %d times, want 1", called)
		}
	})

	t.Run("GetCachesPerProviderModelPair", func(t *testing.T) {
		var calls atomic.Int32
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			calls.Add(1)
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"mygemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "mygemini/gemini-flash",
		}

		c := New(cfg, factory)
		ctx := context.Background()

		p1, _ := c.Get(ctx, "mygemini/gemini-flash")
		p2, _ := c.Get(ctx, "mygemini/gemini-flash")

		if p1 != p2 {
			t.Error("Get() returned different providers for same model; expected cached instance")
		}
		if calls.Load() != 1 {
			t.Errorf("factory called %d times, want 1 (should be cached)", calls.Load())
		}
	})

	t.Run("GetErrorsOnUnknownProvider", func(t *testing.T) {
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "gemini/flash",
		}

		c := New(cfg, factory)
		_, err := c.Get(context.Background(), "unknown/model")
		if err == nil {
			t.Error("Get() expected error for unknown provider, got nil")
		}
	})

	t.Run("GetErrorsOnInvalidFormat", func(t *testing.T) {
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "gemini/flash",
		}

		c := New(cfg, factory)
		_, err := c.Get(context.Background(), "no-slash")
		if err == nil {
			t.Error("Get() expected error for invalid model format, got nil")
		}
	})

	t.Run("DifferentModelsFromSameProviderCreateSeparateInstances", func(t *testing.T) {
		var calls atomic.Int32
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			calls.Add(1)
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "gemini/flash",
		}

		c := New(cfg, factory)
		ctx := context.Background()

		p1, _ := c.Get(ctx, "gemini/flash")
		p2, _ := c.Get(ctx, "gemini/pro")

		if p1 == p2 {
			t.Error("Get() returned same provider for different models; expected separate instances")
		}
		if calls.Load() != 2 {
			t.Errorf("factory called %d times, want 2", calls.Load())
		}
	})

	t.Run("ConcurrentGetIsThreadSafe", func(t *testing.T) {
		var calls atomic.Int32
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			calls.Add(1)
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"gemini": {Type: "gemini", APIKey: "key"},
			},
			DefaultModel: "gemini/flash",
		}

		c := New(cfg, factory)
		ctx := context.Background()

		var wg sync.WaitGroup
		for range 50 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				p, err := c.Get(ctx, "gemini/flash")
				if err != nil {
					t.Errorf("Get() error = %v", err)
				}
				if p == nil {
					t.Error("Get() returned nil")
				}
			}()
		}
		wg.Wait()

		if calls.Load() != 1 {
			t.Errorf("factory called %d times under concurrency, want 1", calls.Load())
		}
	})

	t.Run("UnusedProvidersNotInstantiated", func(t *testing.T) {
		var calls atomic.Int32
		factory := func(_ context.Context, name string, pc config.ProviderConfig) (provider.Provider, error) {
			calls.Add(1)
			return &fakeProvider{id: name}, nil
		}

		cfg := &config.Config{
			Providers: map[string]config.ProviderConfig{
				"gemini":  {Type: "gemini", APIKey: "key"},
				"invalid": {Type: "openai", APIKey: "bad-key"},
			},
			DefaultModel: "gemini/flash",
		}

		c := New(cfg, factory)

		// Only use gemini, never touch invalid.
		_, err := c.Get(context.Background(), "gemini/flash")
		if err != nil {
			t.Fatalf("Get() error = %v", err)
		}

		if calls.Load() != 1 {
			t.Errorf("factory called %d times, want 1 (unused provider should not be created)", calls.Load())
		}
	})
}
