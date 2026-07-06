package llm

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// PromptVariable describes a template variable accepted by a prompt.
type PromptVariable struct {
	Name        string `json:"name" yaml:"name"`
	Description string `json:"description,omitempty" yaml:"description,omitempty"`
	Required    bool   `json:"required" yaml:"required"`
}

// PromptDefinition is a centrally managed prompt file.
type PromptDefinition struct {
	ID                   string           `json:"id" yaml:"id"`
	Version              int              `json:"version" yaml:"version"`
	Workflow             string           `json:"workflow" yaml:"workflow"`
	Purpose              string           `json:"purpose" yaml:"purpose"`
	Description          string           `json:"description,omitempty" yaml:"description,omitempty"`
	RequiredCapabilities []Capability     `json:"required_capabilities,omitempty" yaml:"required_capabilities,omitempty"`
	Variables            []PromptVariable `json:"variables,omitempty" yaml:"variables,omitempty"`
	Messages             []Message        `json:"messages" yaml:"messages"`
	ResultLimits         ResultLimits     `json:"result_limits,omitempty" yaml:"result_limits,omitempty"`
}

// PromptCatalog stores prompt definitions keyed by workflow and purpose.
type PromptCatalog struct {
	prompts map[promptKey]PromptDefinition
}

type promptKey struct {
	workflow string
	purpose  string
}

// LoadPromptCatalog loads YAML prompt definitions from dir. Non-YAML files are ignored.
func LoadPromptCatalog(fsys fs.FS, dir string) (*PromptCatalog, error) {
	catalog := &PromptCatalog{prompts: make(map[promptKey]PromptDefinition)}
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return catalog, nil
		}
		return nil, fmt.Errorf("reading prompt catalog %q: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".yaml") && !strings.HasSuffix(name, ".yml") {
			continue
		}
		body, err := fs.ReadFile(fsys, path.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("reading prompt %q: %w", name, err)
		}
		var prompt PromptDefinition
		if err := yaml.Unmarshal(body, &prompt); err != nil {
			return nil, fmt.Errorf("parsing prompt %q: %w", name, err)
		}
		if err := prompt.validate(name); err != nil {
			return nil, err
		}
		key := promptKey{workflow: prompt.Workflow, purpose: prompt.Purpose}
		if existing, ok := catalog.prompts[key]; ok {
			return nil, fmt.Errorf("duplicate prompt for workflow %q purpose %q: %s and %s", key.workflow, key.purpose, existing.ID, prompt.ID)
		}
		catalog.prompts[key] = prompt
	}
	return catalog, nil
}

// Len returns the number of prompt definitions.
func (c *PromptCatalog) Len() int {
	if c == nil {
		return 0
	}
	return len(c.prompts)
}

// Get returns the prompt for a workflow and purpose.
func (c *PromptCatalog) Get(workflow, purpose string) (PromptDefinition, bool) {
	if c == nil {
		return PromptDefinition{}, false
	}
	prompt, ok := c.prompts[promptKey{workflow: workflow, purpose: purpose}]
	return prompt, ok
}

// List returns all prompts sorted by workflow and purpose.
func (c *PromptCatalog) List() []PromptDefinition {
	if c == nil {
		return nil
	}
	out := make([]PromptDefinition, 0, len(c.prompts))
	for _, prompt := range c.prompts {
		out = append(out, prompt)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Workflow == out[j].Workflow {
			return out[i].Purpose < out[j].Purpose
		}
		return out[i].Workflow < out[j].Workflow
	})
	return out
}

func (p PromptDefinition) validate(filename string) error {
	if strings.TrimSpace(p.ID) == "" {
		return fmt.Errorf("prompt %q id is required", filename)
	}
	if p.Version <= 0 {
		return fmt.Errorf("prompt %q version must be positive", filename)
	}
	if strings.TrimSpace(p.Workflow) == "" {
		return fmt.Errorf("prompt %q workflow is required", filename)
	}
	if strings.TrimSpace(p.Purpose) == "" {
		return fmt.Errorf("prompt %q purpose is required", filename)
	}
	if len(p.Messages) == 0 {
		return fmt.Errorf("prompt %q requires at least one message", filename)
	}
	return nil
}
