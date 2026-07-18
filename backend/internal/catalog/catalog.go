// Package catalog loads the application content embedded in the server binary.
package catalog

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"strings"

	"github.com/ArionMiles/expensor/backend/internal/llm"
	"github.com/ArionMiles/expensor/backend/internal/plugins"
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
	return loadFromFS(contentFS)
}

func loadFromFS(fsys fs.FS) (Content, error) {
	rulesBody, err := read(fsys, "content/rules.json")
	if err != nil {
		return Content{}, err
	}
	doc, err := rules.ParseDocument(rulesBody)
	if err != nil {
		return Content{}, errors.E("catalog.load", errors.Internal, "parsing bundled rules", err)
	}
	if len(doc.Rules) == 0 {
		return Content{}, invalid("bundled rules are empty")
	}

	mccEntries, err := decode[[]store.MCCEntry](fsys, "content/mcc.json")
	if err != nil {
		return Content{}, err
	}
	if err := validateMCCEntries(mccEntries); err != nil {
		return Content{}, err
	}
	categoryEntries, err := decode[[]store.MerchantCategoryEntry](fsys, "content/categories.json")
	if err != nil {
		return Content{}, err
	}
	if err := validateCategoryEntries(categoryEntries); err != nil {
		return Content{}, err
	}
	banks, err := read(fsys, "content/banks.json")
	if err != nil {
		return Content{}, err
	}
	var bankEntries []bankEntry
	if err := json.Unmarshal(banks, &bankEntries); err != nil {
		return Content{}, errors.E("catalog.load", errors.Internal, "parsing bundled banks", err)
	}
	if err := validateBanks(bankEntries); err != nil {
		return Content{}, err
	}

	gmailGuide, err := loadGuide(fsys, gmailGuidePath)
	if err != nil {
		return Content{}, err
	}
	thunderbirdGuide, err := loadGuide(fsys, thunderbirdGuidePath)
	if err != nil {
		return Content{}, err
	}
	prompts, err := llm.LoadPromptCatalog(fsys, promptPath)
	if err != nil {
		return Content{}, errors.E("catalog.load", errors.Internal, "loading llm prompts", err)
	}
	if prompts.Len() == 0 {
		return Content{}, invalid("llm prompt catalog is empty")
	}
	models, err := decode[[]llm.ModelOption](fsys, openAIModelsPath)
	if err != nil {
		return Content{}, err
	}
	if err := validateModels(models); err != nil {
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

type bankEntry struct {
	Fragment string `json:"fragment"`
	Color    string `json:"color"`
	Name     string `json:"name"`
}

func validateMCCEntries(entries []store.MCCEntry) error {
	if len(entries) == 0 {
		return invalid("mcc catalog is empty")
	}
	for _, entry := range entries {
		if blank(entry.Code) || blank(entry.Description) || blank(entry.Category) || blank(entry.Bucket) {
			return invalid("mcc catalog contains an incomplete entry")
		}
	}
	return nil
}

func validateCategoryEntries(entries []store.MerchantCategoryEntry) error {
	if len(entries) == 0 {
		return invalid("merchant category catalog is empty")
	}
	for _, entry := range entries {
		if blank(entry.Fragment) || (entry.MCC == nil && entry.Category == nil && entry.Bucket == nil) {
			return invalid("merchant category catalog contains an incomplete entry")
		}
	}
	return nil
}

func validateBanks(entries []bankEntry) error {
	if len(entries) == 0 {
		return invalid("bank catalog is empty")
	}
	for _, entry := range entries {
		if blank(entry.Fragment) || blank(entry.Color) || blank(entry.Name) {
			return invalid("bank catalog contains an incomplete entry")
		}
	}
	return nil
}

func loadGuide(fsys fs.FS, name string) ([]byte, error) {
	body, err := read(fsys, name)
	if err != nil {
		return nil, err
	}
	var guide plugins.ProviderGuide
	if err := json.Unmarshal(body, &guide); err != nil {
		return nil, errors.E("catalog.load", errors.Internal, fmt.Sprintf("parsing %s", name), err)
	}
	if len(guide.Sections) == 0 {
		return nil, invalid(fmt.Sprintf("%s has no sections", name))
	}
	for _, section := range guide.Sections {
		if blank(section.Title) || len(section.Steps) == 0 {
			return nil, invalid(fmt.Sprintf("%s contains an incomplete section", name))
		}
		for _, step := range section.Steps {
			if blank(step.Text) {
				return nil, invalid(fmt.Sprintf("%s contains an empty step", name))
			}
		}
	}
	return body, nil
}

func validateModels(models []llm.ModelOption) error {
	if len(models) == 0 {
		return invalid("OpenAI model catalog is empty")
	}
	for _, model := range models {
		if blank(model.ID) || blank(model.DisplayName) || blank(model.Quality) || blank(model.Cost) {
			return invalid("OpenAI model catalog contains an incomplete entry")
		}
	}
	return nil
}

func decode[T any](fsys fs.FS, name string) (T, error) {
	var value T
	body, err := read(fsys, name)
	if err != nil {
		return value, err
	}
	if err := json.Unmarshal(body, &value); err != nil {
		return value, errors.E("catalog.load", errors.Internal, fmt.Sprintf("parsing %s", name), err)
	}
	return value, nil
}

func read(fsys fs.FS, name string) ([]byte, error) {
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, errors.E("catalog.load", errors.Internal, fmt.Sprintf("reading %s", name), err)
	}
	return body, nil
}

func invalid(message string) error {
	return errors.E("catalog.load", errors.Internal, message)
}

func blank(value string) bool {
	return strings.TrimSpace(value) == ""
}
