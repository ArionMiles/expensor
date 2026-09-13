package app

import (
	"log/slog"

	"github.com/ArionMiles/expensor/backend/internal/assistant"
	"github.com/ArionMiles/expensor/backend/internal/catalog"
	"github.com/ArionMiles/expensor/backend/internal/llm"
	geminiProvider "github.com/ArionMiles/expensor/backend/internal/llm/gemini"
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
	geminiMetadata, ok := content.LLMProviders[geminiProvider.ProviderName]
	if !ok {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "Gemini provider metadata is not configured")
	}
	gemini, err := geminiProvider.Provider(geminiMetadata)
	if err != nil {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "building Gemini provider", err)
	}
	if err := registry.RegisterProvider(gemini); err != nil {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "registering Gemini provider", err)
	}
	openAIMetadata, ok := content.LLMProviders[openaiProvider.ProviderName]
	if !ok {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "OpenAI provider metadata is not configured")
	}
	openAI, err := openaiProvider.Provider(openAIMetadata)
	if err != nil {
		return llmRuntime{}, errors.E("app.llm.new", errors.Internal, "building OpenAI provider", err)
	}
	if err := registry.RegisterProvider(openAI); err != nil {
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
