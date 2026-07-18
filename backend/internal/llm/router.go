package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

// RuntimeStore is the tenant-scoped LLM runtime state needed by the router.
type RuntimeStore interface {
	GetActiveLLMProviderRuntime(ctx context.Context, tenant store.Tenant) (store.LLMProviderRuntime, bool, error)
}

// RouterConfig holds router dependencies.
type RouterConfig struct {
	Registry *Registry
	Runtime  RuntimeStore
	Prompts  *PromptCatalog
	Scope    *observability.Scope
	Logger   *slog.Logger
}

// Router resolves the active tenant provider and enforces capability requirements.
type Router struct {
	registry *Registry
	runtime  RuntimeStore
	prompts  *PromptCatalog
	scope    *observability.Scope
	logger   *slog.Logger
}

// NewRouter creates an LLM router.
func NewRouter(cfg RouterConfig) *Router {
	registry := cfg.Registry
	if registry == nil {
		registry = NewRegistry()
	}
	return &Router{
		registry: registry,
		runtime:  cfg.Runtime,
		prompts:  cfg.Prompts,
		scope:    cfg.Scope,
		logger:   cfg.Logger,
	}
}

// Complete resolves the active provider and delegates the request.
func (r *Router) Complete(ctx context.Context, tenant store.Tenant, req Request) (Response, error) {
	const op = "llm.Router.Complete"

	if r.runtime == nil {
		return Response{}, errors.E(op, KindNoProviderConfigured, errors.User("No LLM provider is configured."), "no llm provider configured")
	}
	runtime, found, err := r.runtime.GetActiveLLMProviderRuntime(ctx, tenant)
	if err != nil {
		return Response{}, errors.E(op, err)
	}
	if !found {
		return Response{}, errors.E(op, KindNoProviderConfigured, errors.User("No LLM provider is configured."), "no llm provider configured")
	}
	provider, err := r.registry.GetProvider(runtime.Provider)
	if err != nil {
		return Response{}, errors.E(op, err)
	}
	if err := provider.RequireCapabilities(req.RequiredCapabilities...); err != nil {
		return Response{}, errors.E(op, err)
	}
	client, err := provider.NewClient(ClientConfig{
		Config:      cloneRawMessage(runtime.Config),
		Credentials: cloneRawMessage(runtime.Credentials),
	})
	if err != nil {
		return Response{}, errors.E(op, fmt.Sprintf("creating llm provider %q client", runtime.Provider), err)
	}
	client = NewInstrumentedClient(client, runtime.Provider, r.scope, r.logger)
	return client.Complete(ctx, req)
}

// PromptCatalog returns the router prompt catalog.
func (r *Router) PromptCatalog() *PromptCatalog {
	return r.prompts
}

func cloneRawMessage(in []byte) json.RawMessage {
	if len(in) == 0 {
		return nil
	}
	out := make([]byte, len(in))
	copy(out, in)
	return json.RawMessage(out)
}
