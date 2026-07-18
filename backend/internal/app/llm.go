package app

import (
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	openaiProvider "github.com/ArionMiles/expensor/backend/internal/llm/openai"
	"github.com/ArionMiles/expensor/backend/internal/observability"
	"github.com/ArionMiles/expensor/backend/internal/store/instrumented"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

type llmRuntime struct {
	registry   *llm.Registry
	router     *llm.Router
	ruleDrafts assistant.RuleDrafter
	scope      *observability.Scope
}

func newLLMRuntime(content catalog.Content, st *instrumented.Store, logger *slog.Logger) (llmRuntime, error) {
	registry := llm.NewRegistry()
	if err := registry.RegisterProvider(openaiProvider.Provider(content.OpenAIModelOptions)); err != nil {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "registering OpenAI provider", err)
	}
	llmLogger := logger.With("component", "llm")
	llmScope := observability.NewScope(llmLogger, "github.com/ArionMiles/expensor/backend/internal/llm")
	router := llm.NewRouter(llm.RouterConfig{
		Registry: registry, Runtime: st, Prompts: content.PromptCatalog, Scope: llmScope, Logger: llmLogger,
	})
	assistantLogger := logger.With("component", "assistant")
	assistantScope := observability.NewScope(assistantLogger, "github.com/ArionMiles/expensor/backend/internal/assistant")
	ruleDrafts := assistant.NewInstrumentedRuleDrafter(assistant.NewRuleDraftService(router), assistantScope, assistantLogger)
	return llmRuntime{registry: registry, router: router, ruleDrafts: ruleDrafts, scope: llmScope}, nil
}
