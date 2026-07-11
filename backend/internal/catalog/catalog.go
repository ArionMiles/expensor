// Package catalog loads the application content embedded in the server binary.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/rules"
	"github.com/ArionMiles/expensor/backend/internal/store"
	"github.com/ArionMiles/expensor/backend/pkg/api"
	"github.com/ArionMiles/expensor/backend/pkg/errors"
)

const (
	gmailGuidePath       = "content/readers/gmail/guide.json"
	thunderbirdGuidePath = "content/readers/thunderbird/guide.json"
	promptPath           = "content/llm/prompts"
	openAIModelsPath     = "content/llm/providers/openai_models.json"
)

//go:embed content
var contentFS embed.FS

// Content is the validated, typed application content loaded at startup.
type Content struct {
	Seed               store.SeedContent
	SystemRules        []api.Rule
	BanksJSON          []byte
	ReaderGuides       map[string][]byte
	PromptCatalog      *llm.PromptCatalog
	OpenAIModelOptions []llm.ModelOption
}

// Load parses and validates all bundled application content.
func Load() (Content, error) {
	rulesBody, err := read("content/rules.json")
	if err != nil {
		return Content{}, err
	}
	doc, err := rules.ParseDocument(rulesBody)
	if err != nil {
		return Content{}, errors.E("catalog.load", errors.Internal, "parsing bundled rules", err)
	}

	mccEntries, err := decode[[]store.MCCEntry]("content/mcc.json")
	if err != nil {
		return Content{}, err
	}
	categoryEntries, err := decode[[]store.MerchantCategoryEntry]("content/categories.json")
	if err != nil {
		return Content{}, err
	}
	banks, err := read("content/banks.json")
	if err != nil {
		return Content{}, err
	}
	if !json.Valid(banks) {
		return Content{}, errors.E("catalog.load", errors.Internal, "parsing bundled banks")
	}

	gmailGuide, err := validatedJSON(gmailGuidePath)
	if err != nil {
		return Content{}, err
	}
	thunderbirdGuide, err := validatedJSON(thunderbirdGuidePath)
	if err != nil {
		return Content{}, err
	}
	prompts, err := llm.LoadPromptCatalog(contentFS, promptPath)
	if err != nil {
		return Content{}, errors.E("catalog.load", errors.Internal, "loading llm prompts", err)
	}
	models, err := decode[[]llm.ModelOption](openAIModelsPath)
	if err != nil {
		return Content{}, err
	}

	return Content{
		Seed: store.SeedContent{
			Rules:              doc.Rules,
			MCCEntries:         mccEntries,
			MerchantCategories: categoryEntries,
		},
		SystemRules: doc.Rules,
		BanksJSON:   banks,
		ReaderGuides: map[string][]byte{
			"gmail":       gmailGuide,
			"thunderbird": thunderbirdGuide,
		},
		PromptCatalog:      prompts,
		OpenAIModelOptions: models,
	}, nil
}

func validatedJSON(name string) ([]byte, error) {
	body, err := read(name)
	if err != nil {
		return nil, err
	}
	if !json.Valid(body) {
		return nil, errors.E("catalog.load", errors.Internal, fmt.Sprintf("parsing %s", name))
	}
	return body, nil
}

func decode[T any](name string) (T, error) {
	var value T
	body, err := read(name)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return value, errors.E("catalog.load", errors.Internal, fmt.Sprintf("parsing %s", name), err)
	}
	return value, nil
}

func read(name string) ([]byte, error) {
	body, err := contentFS.ReadFile(name)
	if err != nil {
		return nil, errors.E("catalog.load", errors.Internal, fmt.Sprintf("reading %s", name), err)
	}
	return body, nil
}
