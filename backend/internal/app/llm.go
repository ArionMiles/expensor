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
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("Gemini provider metadata is not configured").Build()
	}
	gemini, err := geminiProvider.Provider(geminiMetadata)
	if err != nil {
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("building Gemini provider").Err(err).Build()
	}
	if err := registry.RegisterProvider(gemini); err != nil {
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("registering Gemini provider").Err(err).Build()
	}
	openAIMetadata, ok := content.LLMProviders[openaiProvider.ProviderName]
	if !ok {
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("OpenAI provider metadata is not configured").Build()
	}
	openAI, err := openaiProvider.Provider(openAIMetadata)
	if err != nil {
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("building OpenAI provider").Err(err).Build()
	}
	if err := registry.RegisterProvider(openAI); err != nil {
		return llmRuntime{}, errors.B.Op("app.llm.new").KindInternal().Text("registering OpenAI provider").Err(err).Build()
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
