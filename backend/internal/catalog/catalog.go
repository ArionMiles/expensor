// Package catalog loads the application content embedded in the server binary.
package catalog

import (
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/url"
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
	openAIProviderPath   = "content/llm/providers/openai.json"
	geminiProviderPath   = "content/llm/providers/gemini.json"
)

//go:embed content
var contentFS embed.FS

// Content is the validated, typed application content loaded at startup.
type Content struct {
	Seed          store.SeedContent
	SystemRules   []api.Rule
	BanksJSON     []byte
	ReaderGuides  map[string][]byte
	PromptCatalog *llm.PromptCatalog
	LLMProviders  map[string]llm.ProviderMetadata
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
	openAIProvider, err := loadLLMProvider(fsys, openAIProviderPath, "openai", true)
	if err != nil {
		return Content{}, err
	}
	geminiProvider, err := loadLLMProvider(fsys, geminiProviderPath, "gemini", false)
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
		PromptCatalog: prompts,
		LLMProviders: map[string]llm.ProviderMetadata{
			openAIProvider.Name: openAIProvider,
			geminiProvider.Name: geminiProvider,
		},
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

type llmConfigSchema struct {
	Type string `json:"type"`
}

type llmProviderCatalog struct {
	Name           string            `json:"name"`
	DisplayName    string            `json:"display_name"`
	APIKeyURL      string            `json:"api_key_url"`
	APIKeyLinkText string            `json:"api_key_link_text"`
	DataUse        llm.DataUseSpec   `json:"data_use"`
	Auth           llm.AuthSpec      `json:"auth"`
	ConfigSchema   json.RawMessage   `json:"config_schema"`
	ModelOptions   []llm.ModelOption `json:"model_options"`
}

func (entry llmProviderCatalog) metadata() llm.ProviderMetadata {
	return llm.ProviderMetadata{
		Name:           entry.Name,
		DisplayName:    entry.DisplayName,
		APIKeyURL:      entry.APIKeyURL,
		APIKeyLinkText: entry.APIKeyLinkText,
		DataUse:        entry.DataUse,
		Auth:           entry.Auth,
		ConfigSchema:   entry.ConfigSchema,
		ModelOptions:   entry.ModelOptions,
	}
}

func loadLLMProvider(fsys fs.FS, path, expectedName string, requireBaseURL bool) (llm.ProviderMetadata, error) {
	entry, err := decodeStrict[llmProviderCatalog](fsys, path)
	if err != nil {
		return llm.ProviderMetadata{}, err
	}
	provider := entry.metadata()
	if err := validateLLMProvider(expectedName, provider, requireBaseURL); err != nil {
		return llm.ProviderMetadata{}, err
	}
	return provider, nil
}

func validateLLMProvider(expectedName string, provider llm.ProviderMetadata, requireBaseURL bool) error {
	prefix := expectedName + " provider catalog"
	if provider.Name != expectedName {
		return invalid(prefix + " has an unexpected provider name")
	}
	if blank(provider.DisplayName) || blank(provider.APIKeyLinkText) {
		return invalid(prefix + " contains incomplete display metadata")
	}
	if !validHTTPSURL(provider.APIKeyURL) || !validHTTPSURL(provider.DataUse.PolicyURL) {
		return invalid(prefix + " contains an invalid external URL")
	}
	switch provider.DataUse.Mode {
	case llm.DataUseNoTrainingByDefault, llm.DataUseFreeTierImprovement:
	default:
		return invalid(prefix + " contains an unsupported data-use mode")
	}
	if provider.Auth.Type != llm.AuthTypeAPIKey || !provider.Auth.Required {
		return invalid(prefix + " must require API-key authentication")
	}
	var schema llmConfigSchema
	if err := json.Unmarshal(provider.ConfigSchema, &schema); err != nil {
		return invalid(prefix + " contains an invalid configuration schema")
	}
	if schema.Type != "object" {
		return invalid(prefix + " configuration schema must describe an object")
	}
	modelDefault, ok := llm.ConfigStringDefault(provider.ConfigSchema, "model")
	if !ok {
		return invalid(prefix + " configuration schema must declare a model default")
	}
	if requireBaseURL {
		baseURL, ok := llm.ConfigStringDefault(provider.ConfigSchema, "base_url")
		if !ok || !validHTTPSURL(baseURL) {
			return invalid(prefix + " configuration schema must declare a valid base URL default")
		}
	}
	return validateModels(provider.DisplayName, provider.ModelOptions, modelDefault)
}

func validateModels(provider string, models []llm.ModelOption, defaultModel string) error {
	if len(models) == 0 {
		return invalid(provider + " model catalog is empty")
	}
	recommended := 0
	for _, model := range models {
		if blank(model.ID) || blank(model.DisplayName) || blank(model.Quality) || blank(model.Cost) {
			return invalid(provider + " model catalog contains an incomplete entry")
		}
		if model.Recommended {
			recommended++
			if model.ID != defaultModel {
				return invalid(provider + " model catalog default and recommended model differ")
			}
		}
	}
	if recommended != 1 {
		return invalid(provider + " model catalog must contain exactly one recommended model")
	}
	return nil
}

func validHTTPSURL(value string) bool {
	parsed, err := url.ParseRequestURI(strings.TrimSpace(value))
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func decodeStrict[T any](fsys fs.FS, name string) (T, error) {
	var value T
	body, err := read(fsys, name)
	if err != nil {
		return value, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return value, errors.E("catalog.load", errors.Internal, fmt.Sprintf("parsing %s", name), err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return value, invalid(fmt.Sprintf("%s contains trailing JSON data", name))
	}
	return value, nil
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
